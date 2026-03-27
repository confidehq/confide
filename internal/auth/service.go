package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phantompunk/confide/internal/db/queries"
)

// Sentinel errors returned from service methods.
var (
	ErrNotFound         = errors.New("not found")
	ErrDuplicateAccount = errors.New("credential already registered")
	ErrInvalidCode      = errors.New("invalid or expired recovery code")
)

// DB is the subset of queries.Queries used by the service.
// Extracted as an interface to allow mock injection in unit tests.
type DB interface {
	CreateAccount(ctx context.Context, arg queries.CreateAccountParams) (queries.Account, error)
	GetAccountByCredentialID(ctx context.Context, credentialID []byte) (queries.Account, error)
	GetAccountByID(ctx context.Context, id string) (queries.Account, error)
	UpdateAccountCredential(ctx context.Context, arg queries.UpdateAccountCredentialParams) error
	CreateSession(ctx context.Context, arg queries.CreateSessionParams) (queries.Session, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (queries.GetSessionByTokenHashRow, error)
	TouchSession(ctx context.Context, id string) error
	DeleteSession(ctx context.Context, arg queries.DeleteSessionParams) error
	DeleteStaleSessions(ctx context.Context) error
	ListSessionsByAccount(ctx context.Context, accountID string) ([]queries.ListSessionsByAccountRow, error)
	CreateRecoveryCodes(ctx context.Context, arg []queries.CreateRecoveryCodesParams) (int64, error)
	GetUnusedRecoveryCode(ctx context.Context, arg queries.GetUnusedRecoveryCodeParams) (queries.RecoveryCode, error)
	BurnRecoveryCode(ctx context.Context, id string) error
	CountUnusedRecoveryCodes(ctx context.Context, accountID string) (int64, error)
	DeleteRecoveryCodesByAccount(ctx context.Context, accountID string) error
	UpdateAccountBackupEligible(ctx context.Context, arg queries.UpdateAccountBackupEligibleParams) error
	ListCredentials(ctx context.Context) ([]queries.ListCredentialsRow, error)
}

// challengeEntry holds WebAuthn session data with an expiry.
type challengeEntry struct {
	data    *webauthn.SessionData
	expires time.Time
}

// challengeStore is an in-memory TTL store for WebAuthn challenge session data.
type challengeStore struct {
	mu    sync.Mutex
	items map[string]*challengeEntry
}

func newChallengeStore() *challengeStore {
	cs := &challengeStore{items: make(map[string]*challengeEntry)}
	go cs.gcLoop()
	return cs
}

func (cs *challengeStore) set(key string, sd *webauthn.SessionData) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.items[key] = &challengeEntry{data: sd, expires: time.Now().Add(5 * time.Minute)}
}

func (cs *challengeStore) take(key string) (*webauthn.SessionData, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	e, ok := cs.items[key]
	if !ok || time.Now().After(e.expires) {
		delete(cs.items, key)
		return nil, false
	}
	delete(cs.items, key)
	return e.data, true
}

func (cs *challengeStore) gcLoop() {
	t := time.NewTicker(2 * time.Minute)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		cs.mu.Lock()
		for k, e := range cs.items {
			if now.After(e.expires) {
				delete(cs.items, k)
			}
		}
		cs.mu.Unlock()
	}
}

// startSessionCleanup runs a daily background job that deletes stale sessions.
// A session is stale if it has been idle for more than 14 days or is older than 30 days.
func (s *Service) startSessionCleanup() {
	t := time.NewTicker(24 * time.Hour)
	go func() {
		defer t.Stop()
		for range t.C {
			if err := s.db.DeleteStaleSessions(context.Background()); err != nil {
				slog.Error("session cleanup failed", "err", err)
			} else {
				slog.Info("session cleanup complete")
			}
		}
	}()
}

// Service handles auth business logic.
type Service struct {
	db         DB
	pool       *pgxpool.Pool
	wa         *webauthn.WebAuthn
	challenges *challengeStore
	rekeys     *rekeyTokenStore
}

func NewService(pool *pgxpool.Pool, wa *webauthn.WebAuthn) *Service {
	svc := &Service{
		db:         queries.New(pool),
		pool:       pool,
		wa:         wa,
		challenges: newChallengeStore(),
		rekeys:     newRekeyTokenStore(),
	}
	svc.startSessionCleanup()
	return svc
}

// newServiceWithDB is used by unit tests to inject a mock DB.
func newServiceWithDB(db DB, wa *webauthn.WebAuthn) *Service {
	return &Service{
		db:         db,
		wa:         wa,
		challenges: newChallengeStore(),
		rekeys:     newRekeyTokenStore(),
	}
}

// ─── Registration ────────────────────────────────────────────────────────────

type RegisterBeginResult struct {
	AccountID string
	PRFSalt   []byte
	Creation  *protocol.CredentialCreation
}

func (s *Service) RegisterBegin(ctx context.Context) (*RegisterBeginResult, error) {
	accountID, err := randomBase64URL(16) // 22-char base64url from 16 bytes
	if err != nil {
		return nil, err
	}
	prfSalt, err := randomBytes(32)
	if err != nil {
		return nil, err
	}

	user := &waUser{id: []byte(accountID), name: accountID, displayName: accountID}

	creation, sd, err := s.wa.BeginRegistration(user,
		webauthn.WithExtensions(protocol.AuthenticationExtensions{
			"prf": map[string]any{
				"eval": map[string]any{
					// URLEncodedBase64 marshals as base64url (no padding), which is
					// what @simplewebauthn/browser expects for extension byte values.
					"first": protocol.URLEncodedBase64(prfSalt),
				},
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("BeginRegistration: %w", err)
	}

	s.challenges.set(accountID, sd)
	return &RegisterBeginResult{AccountID: accountID, PRFSalt: prfSalt, Creation: creation}, nil
}

type RegisterFinishRequest struct {
	AccountID             string   `json:"accountId"`
	WrappedMasterKey      []byte   `json:"wrappedMasterKey"`         // base64 in JSON
	RecoveryWrappedMaster []byte   `json:"recoveryWrappedMasterKey"` // base64 in JSON
	RecoveryVerifier      []byte   `json:"recoveryVerifier"`         // base64 in JSON
	RecoveryCodes         [][]byte `json:"recoveryCodes"`            // 12 × SHA-256 hashes
	PRFSalt               []byte   `json:"prfSalt"`
}

type RegisterFinishResult struct {
	AccountID string
	Token     string // raw session token (caller sets cookie)
}

func (s *Service) RegisterFinish(ctx context.Context, req *RegisterFinishRequest, userAgent string, r *http.Request) (*RegisterFinishResult, error) {
	sd, ok := s.challenges.take(req.AccountID)
	if !ok {
		return nil, fmt.Errorf("challenge not found or expired")
	}

	user := &waUser{id: []byte(req.AccountID), name: req.AccountID, displayName: req.AccountID}
	cred, err := s.wa.FinishRegistration(user, *sd, r)
	if err != nil {
		return nil, fmt.Errorf("FinishRegistration: %w", err)
	}

	if len(req.RecoveryCodes) != 12 {
		return nil, fmt.Errorf("expected 12 recovery code hashes, got %d", len(req.RecoveryCodes))
	}

	token, tokenHash, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	sessionID, err := randomBase64URL(16)
	if err != nil {
		return nil, err
	}

	// Write account, recovery codes, and initial session in a transaction.
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		q := queries.New(tx)

		_, err := q.CreateAccount(ctx, queries.CreateAccountParams{
			ID:                    req.AccountID,
			CredentialID:          cred.ID,
			PublicKey:             cred.PublicKey,
			PrfSalt:               req.PRFSalt,
			WrappedMasterKey:      req.WrappedMasterKey,
			RecoveryWrappedMaster: req.RecoveryWrappedMaster,
			RecoveryVerifier:      req.RecoveryVerifier,
			BackupEligible:        cred.Flags.BackupEligible,
		})
		if err != nil {
			if isDuplicateKey(err) {
				return ErrDuplicateAccount
			}
			return fmt.Errorf("CreateAccount: %w", err)
		}

		today := pgtype.Date{Time: time.Now().UTC(), Valid: true}
		codes := make([]queries.CreateRecoveryCodesParams, 12)
		for i, hash := range req.RecoveryCodes {
			codeID, err := randomBase64URL(16)
			if err != nil {
				return err
			}
			codes[i] = queries.CreateRecoveryCodesParams{
				ID:        codeID,
				AccountID: req.AccountID,
				CodeHash:  hash,
				Used:      false,
				CreatedAt: today,
			}
		}
		if _, err := q.CreateRecoveryCodes(ctx, codes); err != nil {
			return fmt.Errorf("CreateRecoveryCodes: %w", err)
		}

		if _, err := q.CreateSession(ctx, queries.CreateSessionParams{
			ID:           sessionID,
			AccountID:    req.AccountID,
			TokenHash:    tokenHash,
			CredentialID: cred.ID,
			UserAgent:    userAgent,
		}); err != nil {
			return fmt.Errorf("CreateSession: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return &RegisterFinishResult{AccountID: req.AccountID, Token: token}, nil
}

// ─── Login ────────────────────────────────────────────────────────────────────

type LoginBeginResult struct {
	Assertion    *protocol.CredentialAssertion
	ChallengeKey string // must be echoed back in LoginFinish
}

// LoginBegin starts a WebAuthn assertion ceremony.
//
// If credentialID is non-nil, targeted mode: embeds the account's PRF salt via
// prf.eval.first (Chrome 116+). The challenge is keyed by base64url(credentialID).
//
// If credentialID is nil, discoverable mode: queries all credentials and embeds
// their PRF salts via prf.evalByCredential (Chrome 132+, 1Password). The
// challenge is keyed by a random nonce returned as ChallengeKey.
func (s *Service) LoginBegin(ctx context.Context, credentialID []byte) (*LoginBeginResult, error) {
	if len(credentialID) > 0 {
		return s.loginBeginTargeted(ctx, credentialID)
	}
	return s.loginBeginDiscoverable(ctx)
}

// loginBeginTargeted uses prf.eval.first with the known credential's PRF salt.
// Requires Chrome 116+. Used when credentialId is stored in localStorage.
func (s *Service) loginBeginTargeted(ctx context.Context, credentialID []byte) (*LoginBeginResult, error) {
	account, err := s.db.GetAccountByCredentialID(ctx, credentialID)
	if err != nil {
		return nil, ErrNotFound
	}
	assertion, sd, err := s.wa.BeginDiscoverableLogin(
		webauthn.WithAssertionExtensions(protocol.AuthenticationExtensions{
			"prf": map[string]any{
				"eval": map[string]any{
					"first": protocol.URLEncodedBase64(account.PrfSalt),
				},
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("BeginDiscoverableLogin: %w", err)
	}
	challengeKey := base64.RawURLEncoding.EncodeToString(credentialID)
	s.challenges.set(challengeKey, sd)
	return &LoginBeginResult{Assertion: assertion, ChallengeKey: challengeKey}, nil
}

// loginBeginDiscoverable uses prf.eval.first for discoverable login (empty allowCredentials).
// evalByCredential is forbidden by the spec when allowCredentials is empty — only eval.first
// is valid in discoverable mode. Uses the first registered credential's PRF salt.
func (s *Service) loginBeginDiscoverable(ctx context.Context) (*LoginBeginResult, error) {
	creds, err := s.db.ListCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListCredentials: %w", err)
	}
	if len(creds) == 0 {
		return nil, fmt.Errorf("no registered accounts: please sign up first")
	}
	assertion, sd, err := s.wa.BeginDiscoverableLogin(
		webauthn.WithAssertionExtensions(protocol.AuthenticationExtensions{
			"prf": map[string]any{
				"eval": map[string]any{
					"first": protocol.URLEncodedBase64(creds[0].PrfSalt),
				},
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("BeginDiscoverableLogin: %w", err)
	}
	challengeKey, err := randomBase64URL(16)
	if err != nil {
		return nil, err
	}
	s.challenges.set(challengeKey, sd)
	return &LoginBeginResult{Assertion: assertion, ChallengeKey: challengeKey}, nil
}

type LoginFinishResult struct {
	AccountID        string
	WrappedMasterKey []byte
	Token            string // raw session token (caller sets cookie)
}

func (s *Service) LoginFinish(ctx context.Context, challengeKey, userAgent string, r *http.Request) (*LoginFinishResult, error) {
	sd, ok := s.challenges.take(challengeKey)
	if !ok {
		return nil, fmt.Errorf("challenge not found or expired")
	}

	// FinishDiscoverableLogin looks up the account via the userHandle (account ID
	// set during registration) which is the authoritative identifier for discoverable
	// login. Falls back to rawID lookup for backward compatibility.
	var account queries.Account
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		if len(userHandle) > 0 {
			acc, err := s.db.GetAccountByID(ctx, string(userHandle))
			if err == nil {
				account = acc
				return accountToWAUser(acc), nil
			}
		}
		acc, err := s.db.GetAccountByCredentialID(ctx, rawID)
		if err != nil {
			return nil, ErrNotFound
		}
		account = acc
		return accountToWAUser(acc), nil
	}

	cred, err := s.wa.FinishDiscoverableLogin(handler, *sd, r)
	if err != nil {
		return nil, fmt.Errorf("FinishDiscoverableLogin: %w", err)
	}

	// Sync BackupEligible if the stored value is stale (accounts created before
	// the field was tracked default to false).
	if account.BackupEligible != cred.Flags.BackupEligible {
		_ = s.db.UpdateAccountBackupEligible(ctx, queries.UpdateAccountBackupEligibleParams{
			CredentialID:   account.CredentialID,
			BackupEligible: cred.Flags.BackupEligible,
		})
	}

	token, tokenHash, err := newSessionToken()
	if err != nil {
		return nil, err
	}

	sessionID, err := randomBase64URL(16)
	if err != nil {
		return nil, err
	}

	if _, err := s.db.CreateSession(ctx, queries.CreateSessionParams{
		ID:           sessionID,
		AccountID:    account.ID,
		TokenHash:    tokenHash,
		CredentialID: cred.ID,
		UserAgent:    userAgent,
	}); err != nil {
		return nil, fmt.Errorf("CreateSession: %w", err)
	}

	return &LoginFinishResult{
		AccountID:        account.ID,
		WrappedMasterKey: account.WrappedMasterKey,
		Token:            token,
	}, nil
}

// ─── Reauth ───────────────────────────────────────────────────────────────────

// ReauthBeginResult is returned by ReauthBegin.
type ReauthBeginResult struct {
	Assertion    *protocol.CredentialAssertion
	ChallengeKey string
}

// ReauthBegin issues a WebAuthn challenge for an already-authenticated user.
// The account is identified from the session context — no credential ID needed.
func (s *Service) ReauthBegin(ctx context.Context, accountID string) (*ReauthBeginResult, error) {
	account, err := s.db.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, ErrNotFound
	}
	assertion, sd, err := s.wa.BeginDiscoverableLogin(
		webauthn.WithAssertionExtensions(protocol.AuthenticationExtensions{
			"prf": map[string]any{
				"eval": map[string]any{
					"first": protocol.URLEncodedBase64(account.PrfSalt),
				},
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("BeginDiscoverableLogin: %w", err)
	}
	challengeKey, err := randomBase64URL(16)
	if err != nil {
		return nil, err
	}
	s.challenges.set(challengeKey, sd)
	return &ReauthBeginResult{Assertion: assertion, ChallengeKey: challengeKey}, nil
}

// ReauthFinishResult is returned by ReauthFinish.
type ReauthFinishResult struct {
	AccountID        string
	WrappedMasterKey []byte
}

// ReauthFinish verifies the WebAuthn assertion and returns the wrapped master
// key for the existing session. No new session row is created.
func (s *Service) ReauthFinish(ctx context.Context, challengeKey string, accountID string, r *http.Request) (*ReauthFinishResult, error) {
	sd, ok := s.challenges.take(challengeKey)
	if !ok {
		return nil, fmt.Errorf("challenge not found or expired")
	}

	var account queries.Account
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		if len(userHandle) > 0 {
			acc, err := s.db.GetAccountByID(ctx, string(userHandle))
			if err == nil {
				account = acc
				return accountToWAUser(acc), nil
			}
		}
		acc, err := s.db.GetAccountByCredentialID(ctx, rawID)
		if err != nil {
			return nil, ErrNotFound
		}
		account = acc
		return accountToWAUser(acc), nil
	}

	cred, err := s.wa.FinishDiscoverableLogin(handler, *sd, r)
	if err != nil {
		return nil, fmt.Errorf("FinishDiscoverableLogin: %w", err)
	}

	if account.BackupEligible != cred.Flags.BackupEligible {
		_ = s.db.UpdateAccountBackupEligible(ctx, queries.UpdateAccountBackupEligibleParams{
			CredentialID:   account.CredentialID,
			BackupEligible: cred.Flags.BackupEligible,
		})
	}

	return &ReauthFinishResult{
		AccountID:        account.ID,
		WrappedMasterKey: account.WrappedMasterKey,
	}, nil
}

// ─── Recovery ─────────────────────────────────────────────────────────────────

type RecoverResult struct {
	RecoveryWrappedMaster []byte
	RekeyToken            string // short-lived proof-of-recovery token
}

func (s *Service) Recover(ctx context.Context, accountID, code string) (*RecoverResult, error) {
	normalised := strings.ToUpper(strings.ReplaceAll(code, "-", ""))
	hash := sha256Sum([]byte(normalised))

	rc, err := s.db.GetUnusedRecoveryCode(ctx, queries.GetUnusedRecoveryCodeParams{
		AccountID: accountID,
		CodeHash:  hash,
	})
	if err != nil {
		return nil, ErrInvalidCode
	}

	if err := s.db.BurnRecoveryCode(ctx, rc.ID); err != nil {
		return nil, fmt.Errorf("BurnRecoveryCode: %w", err)
	}

	account, err := s.db.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, ErrNotFound
	}

	rekeyToken, err := s.rekeys.issue(accountID)
	if err != nil {
		return nil, fmt.Errorf("issue rekey token: %w", err)
	}

	return &RecoverResult{RecoveryWrappedMaster: account.RecoveryWrappedMaster, RekeyToken: rekeyToken}, nil
}

// SyncBackupEligible updates the stored BackupEligible flag when it differs
// from the value observed in the authenticator. Called before FinishDiscoverableLogin
// so the consistency check inside go-webauthn sees a matching value.
func (s *Service) SyncBackupEligible(ctx context.Context, credentialID []byte, be bool) {
	acc, err := s.db.GetAccountByCredentialID(ctx, credentialID)
	if err != nil || acc.BackupEligible == be {
		return
	}
	_ = s.db.UpdateAccountBackupEligible(ctx, queries.UpdateAccountBackupEligibleParams{
		CredentialID:   credentialID,
		BackupEligible: be,
	})
}

// ─── Session management ───────────────────────────────────────────────────────

// ValidateSession looks up the session by cookie token, touches last_seen, and
// returns the accountID and wrappedMasterKey.
func (s *Service) ValidateSession(ctx context.Context, token string) (accountID string, wrappedMasterKey []byte, sessionID string, err error) {
	tokenBytes, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", nil, "", errors.New("invalid token")
	}
	hash := sha256Sum(tokenBytes)

	row, err := s.db.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return "", nil, "", errors.New("session not found")
	}

	_ = s.db.TouchSession(ctx, row.ID)
	return row.AccountID, row.WrappedMasterKey, row.ID, nil
}

func (s *Service) Logout(ctx context.Context, accountID, sessionID string) error {
	return s.db.DeleteSession(ctx, queries.DeleteSessionParams{ID: sessionID, AccountID: accountID})
}

func (s *Service) ListSessions(ctx context.Context, accountID string) ([]queries.ListSessionsByAccountRow, error) {
	return s.db.ListSessionsByAccount(ctx, accountID)
}

func (s *Service) DeleteSession(ctx context.Context, accountID, sessionID string) error {
	err := s.db.DeleteSession(ctx, queries.DeleteSessionParams{ID: sessionID, AccountID: accountID})
	if err != nil {
		return ErrNotFound
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (s *Service) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	if s.pool == nil {
		// Unit test mode: no pool available — fn receives nil tx, DB mock handles it.
		return fn(nil)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("rand.Read: %w", err)
	}
	return b, nil
}

func randomBase64URL(n int) (string, error) {
	b, err := randomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func newSessionToken() (raw string, hash []byte, err error) {
	b, err := randomBytes(32)
	if err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	hash = sha256Sum(b)
	return raw, hash, nil
}

func isDuplicateKey(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate key")
}

// ─── WebAuthn user adapter ────────────────────────────────────────────────────

type waUser struct {
	id          []byte
	name        string
	displayName string
	credentials []webauthn.Credential
}

func (u *waUser) WebAuthnID() []byte                         { return u.id }
func (u *waUser) WebAuthnName() string                       { return u.name }
func (u *waUser) WebAuthnDisplayName() string                { return u.displayName }
func (u *waUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

func accountToWAUser(a queries.Account) *waUser {
	return &waUser{
		id:          []byte(a.ID),
		name:        a.ID,
		displayName: a.ID,
		credentials: []webauthn.Credential{
			{
				ID:        a.CredentialID,
				PublicKey: a.PublicKey,
				Flags: webauthn.CredentialFlags{
					BackupEligible: a.BackupEligible,
				},
			},
		},
	}
}
