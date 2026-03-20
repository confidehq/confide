package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phantompunk/wisp/internal/db/queries"
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
	DeleteSession(ctx context.Context, id string) error
	ListSessionsByAccount(ctx context.Context, accountID string) ([]queries.ListSessionsByAccountRow, error)
	CreateRecoveryCodes(ctx context.Context, arg []queries.CreateRecoveryCodesParams) (int64, error)
	GetUnusedRecoveryCode(ctx context.Context, arg queries.GetUnusedRecoveryCodeParams) (queries.RecoveryCode, error)
	BurnRecoveryCode(ctx context.Context, id string) error
	CountUnusedRecoveryCodes(ctx context.Context, accountID string) (int64, error)
	DeleteRecoveryCodesByAccount(ctx context.Context, accountID string) error
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

// Service handles auth business logic.
type Service struct {
	db         DB
	pool       *pgxpool.Pool
	wa         *webauthn.WebAuthn
	challenges *challengeStore
	rekeys     *rekeyTokenStore
}

func NewService(pool *pgxpool.Pool, wa *webauthn.WebAuthn) *Service {
	return &Service{
		db:         queries.New(pool),
		pool:       pool,
		wa:         wa,
		challenges: newChallengeStore(),
		rekeys:     newRekeyTokenStore(),
	}
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
	AccountID              string `json:"accountId"`
	WrappedMasterKey       []byte `json:"wrappedMasterKey"`       // base64 in JSON
	RecoveryWrappedMaster  []byte `json:"recoveryWrappedMasterKey"` // base64 in JSON
	RecoveryVerifier       []byte `json:"recoveryVerifier"`       // base64 in JSON
	RecoveryCodes          [][]byte `json:"recoveryCodes"`        // 12 × SHA-256 hashes
	PRFSalt                []byte `json:"prfSalt"`
}

func (s *Service) RegisterFinish(ctx context.Context, req *RegisterFinishRequest, r *http.Request) (string, error) {
	sd, ok := s.challenges.take(req.AccountID)
	if !ok {
		return "", fmt.Errorf("challenge not found or expired")
	}

	user := &waUser{id: []byte(req.AccountID), name: req.AccountID, displayName: req.AccountID}
	cred, err := s.wa.FinishRegistration(user, *sd, r)
	if err != nil {
		return "", fmt.Errorf("FinishRegistration: %w", err)
	}

	if len(req.RecoveryCodes) != 12 {
		return "", fmt.Errorf("expected 12 recovery code hashes, got %d", len(req.RecoveryCodes))
	}

	// Write account + recovery codes in a transaction.
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
		return nil
	})
	if err != nil {
		return "", err
	}
	return req.AccountID, nil
}

// ─── Login ────────────────────────────────────────────────────────────────────

type LoginBeginResult struct {
	Assertion *protocol.CredentialAssertion
}

func (s *Service) LoginBegin(ctx context.Context, credentialID []byte) (*LoginBeginResult, error) {
	account, err := s.db.GetAccountByCredentialID(ctx, credentialID)
	if err != nil {
		return nil, ErrNotFound
	}

	user := accountToWAUser(account)
	assertion, sd, err := s.wa.BeginLogin(user,
		webauthn.WithAssertionExtensions(protocol.AuthenticationExtensions{
			"prf": map[string]any{
				"eval": map[string]any{
					"first": protocol.URLEncodedBase64(account.PrfSalt),
				},
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("BeginLogin: %w", err)
	}

	// Key the challenge by base64url(credentialID) so finish can retrieve it.
	key := base64.RawURLEncoding.EncodeToString(credentialID)
	s.challenges.set(key, sd)
	return &LoginBeginResult{Assertion: assertion}, nil
}

type LoginFinishResult struct {
	AccountID        string
	WrappedMasterKey []byte
	Token            string // raw session token (caller sets cookie)
}

func (s *Service) LoginFinish(ctx context.Context, credentialID []byte, r *http.Request) (*LoginFinishResult, error) {
	key := base64.RawURLEncoding.EncodeToString(credentialID)
	sd, ok := s.challenges.take(key)
	if !ok {
		return nil, fmt.Errorf("challenge not found or expired")
	}

	account, err := s.db.GetAccountByCredentialID(ctx, credentialID)
	if err != nil {
		return nil, ErrNotFound
	}

	user := accountToWAUser(account)
	if _, err := s.wa.FinishLogin(user, *sd, r); err != nil {
		return nil, fmt.Errorf("FinishLogin: %w", err)
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
		ID:        sessionID,
		AccountID: account.ID,
		TokenHash: tokenHash,
	}); err != nil {
		return nil, fmt.Errorf("CreateSession: %w", err)
	}

	return &LoginFinishResult{
		AccountID:        account.ID,
		WrappedMasterKey: account.WrappedMasterKey,
		Token:            token,
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

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	return s.db.DeleteSession(ctx, sessionID)
}

func (s *Service) ListSessions(ctx context.Context, accountID string) ([]queries.ListSessionsByAccountRow, error) {
	return s.db.ListSessionsByAccount(ctx, accountID)
}

func (s *Service) DeleteSession(ctx context.Context, accountID, sessionID string) error {
	// Verify session belongs to the account before deleting.
	sessions, err := s.db.ListSessionsByAccount(ctx, accountID)
	if err != nil {
		return err
	}
	for _, sess := range sessions {
		if sess.ID == sessionID {
			return s.db.DeleteSession(ctx, sessionID)
		}
	}
	return ErrNotFound
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
			{ID: a.CredentialID, PublicKey: a.PublicKey},
		},
	}
}
