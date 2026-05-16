package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	"github.com/phantompunk/confide/internal/workspace"
)

// Sentinel errors returned from service methods.
var (
	ErrNotFound         = errors.New("not found")
	ErrDuplicateAccount = errors.New("credential already registered")
	ErrInvalidCode      = errors.New("invalid or expired recovery code")
	ErrLastCredential   = errors.New("cannot delete the last passkey")
)

// DB is the subset of queries.Queries used by the service.
// Extracted as an interface to allow mock injection in unit tests.
type DB interface {
	CreateAccount(ctx context.Context, arg queries.CreateAccountParams) (queries.Account, error)
	GetAccountByID(ctx context.Context, id string) (queries.Account, error)
	GetAccountByUsername(ctx context.Context, username pgtype.Text) (queries.Account, error)
	UpdateAccountRecovery(ctx context.Context, arg queries.UpdateAccountRecoveryParams) error
	CreateSession(ctx context.Context, arg queries.CreateSessionParams) (queries.Session, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (queries.GetSessionByTokenHashRow, error)
	TouchSession(ctx context.Context, id string) error
	DeleteSession(ctx context.Context, arg queries.DeleteSessionParams) error
	DeleteOtherSessions(ctx context.Context, arg queries.DeleteOtherSessionsParams) error
	DeleteStaleSessions(ctx context.Context) error
	ListSessionsByAccount(ctx context.Context, accountID string) ([]queries.ListSessionsByAccountRow, error)
	CreateRecoveryCodes(ctx context.Context, arg []queries.CreateRecoveryCodesParams) (int64, error)
	GetUnusedRecoveryCode(ctx context.Context, arg queries.GetUnusedRecoveryCodeParams) (queries.RecoveryCode, error)
	BurnRecoveryCode(ctx context.Context, id string) error
	CountUnusedRecoveryCodes(ctx context.Context, accountID string) (int64, error)
	DeleteRecoveryCodesByAccount(ctx context.Context, accountID string) error
	CreateCredential(ctx context.Context, arg queries.CreateCredentialParams) (queries.Credential, error)
	GetCredentialByWebAuthnID(ctx context.Context, credentialID []byte) (queries.GetCredentialByWebAuthnIDRow, error)
	GetCredentialForLogin(ctx context.Context, credentialID []byte) (queries.GetCredentialForLoginRow, error)
	GetPrimaryCredentialByAccount(ctx context.Context, accountID string) ([]byte, error)
	GetPrimaryCredentialIDByAccount(ctx context.Context, accountID string) ([]byte, error)
	GetAllPrfSalts(ctx context.Context) ([]queries.GetAllPrfSaltsRow, error)
	GetPrfSaltsByAccount(ctx context.Context, accountID string) ([]queries.GetPrfSaltsByAccountRow, error)
	ListCredentialsByAccount(ctx context.Context, accountID string) ([]queries.ListCredentialsByAccountRow, error)
	UpdateCredentialName(ctx context.Context, arg queries.UpdateCredentialNameParams) error
	UpdateCredentialBackupEligible(ctx context.Context, arg queries.UpdateCredentialBackupEligibleParams) error
	DeleteCredential(ctx context.Context, arg queries.DeleteCredentialParams) error
	CountCredentialsByAccount(ctx context.Context, accountID string) (int64, error)
	DeleteCredentialsByAccount(ctx context.Context, accountID string) error
	DeleteAccount(ctx context.Context, id string) error
	ListOwnedWorkspacesForDeletion(ctx context.Context, accountID string) ([]queries.ListOwnedWorkspacesForDeletionRow, error)
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

func (cs *challengeStore) has(key string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	e, ok := cs.items[key]
	if !ok || time.Now().After(e.expires) {
		delete(cs.items, key)
		return false
	}
	return true
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
				s.log.Error().Err(err).Msg("session cleanup failed")
			} else {
				s.log.Info().Msg("session cleanup complete")
			}
		}
	}()
}

// Service handles auth business logic.
type Service struct {
	log         zerolog.Logger
	db          DB
	pool        *pgxpool.Pool
	wa          *webauthn.WebAuthn
	challenges  *challengeStore
	rekeys      *rekeyTokenStore
	addCredToks *addCredTokenStore
	pairings    *pairingStore
}

func NewService(pool *pgxpool.Pool, wa *webauthn.WebAuthn) *Service {
	svc := &Service{
		log:         log.With().Str("module", "auth").Logger(),
		db:          queries.New(pool),
		pool:        pool,
		wa:          wa,
		challenges:  newChallengeStore(),
		rekeys:      newRekeyTokenStore(),
		addCredToks: newAddCredTokenStore(),
		pairings:    newPairingStore(),
	}
	svc.startSessionCleanup()
	return svc
}

// newServiceWithDB is used by unit tests to inject a mock DB.
func newServiceWithDB(db DB, wa *webauthn.WebAuthn) *Service {
	return &Service{
		db:          db,
		wa:          wa,
		challenges:  newChallengeStore(),
		rekeys:      newRekeyTokenStore(),
		addCredToks: newAddCredTokenStore(),
		pairings:    newPairingStore(),
	}
}

// ─── Registration ────────────────────────────────────────────────────────────

type RegisterBeginResult struct {
	AccountID string
	PRFSalt   []byte
	Creation  *protocol.CredentialCreation
}

func (s *Service) RegisterBegin(ctx context.Context, username string) (*RegisterBeginResult, error) {
	accountID, err := randomBase64URL(16) // 22-char base64url from 16 bytes
	if err != nil {
		return nil, err
	}
	prfSalt, err := randomBytes(32)
	if err != nil {
		return nil, err
	}

	displayName := username
	if displayName == "" {
		displayName = accountID
	}
	user := &waUser{id: []byte(accountID), name: displayName, displayName: displayName}

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
	Username              string   `json:"username"`
	Name                  string   `json:"name"`
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

	user := &waUser{id: []byte(req.AccountID)}
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
	credRowID, err := randomBase64URL(16)
	if err != nil {
		return nil, err
	}

	// Write account, credential, personal workspace, recovery codes, and initial session in a transaction.
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		q := queries.New(tx)

		_, err := q.CreateAccount(ctx, queries.CreateAccountParams{
			ID:                    req.AccountID,
			RecoveryWrappedMaster: req.RecoveryWrappedMaster,
			RecoveryVerifier:      req.RecoveryVerifier,
			Username:              pgtype.Text{String: req.Username, Valid: req.Username != ""},
		})
		if err != nil {
			if isDuplicateKey(err) {
				return ErrDuplicateAccount
			}
			return fmt.Errorf("CreateAccount: %w", err)
		}

		if _, err := q.CreateCredential(ctx, queries.CreateCredentialParams{
			ID:               credRowID,
			AccountID:        req.AccountID,
			CredentialID:     cred.ID,
			PublicKey:        cred.PublicKey,
			PrfSalt:          req.PRFSalt,
			WrappedMasterKey: req.WrappedMasterKey,
			BackupEligible:   cred.Flags.BackupEligible,
			Name:             req.Name,
		}); err != nil {
			if isDuplicateKey(err) {
				return ErrDuplicateAccount
			}
			return fmt.Errorf("CreateCredential: %w", err)
		}

		if _, err := workspace.CreatePersonalWorkspace(ctx, q, req.AccountID); err != nil {
			return fmt.Errorf("CreatePersonalWorkspace: %w", err)
		}

		now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
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
				CreatedAt: now,
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
// If username is non-empty, looks up the account by username and uses targeted
// mode with the correct PRF salt (preferred path).
//
// If credentialID is non-nil, targeted mode: embeds the account's PRF salt via
// prf.eval.first (Chrome 116+). The challenge is keyed by base64url(credentialID).
//
// If both are absent, discoverable mode: queries all credentials and embeds
// their PRF salts via prf.evalByCredential (Chrome 132+, 1Password). The
// challenge is keyed by a random nonce returned as ChallengeKey.
func (s *Service) LoginBegin(ctx context.Context, credentialID []byte, username string) (*LoginBeginResult, error) {
	if username != "" {
		return s.loginBeginByUsername(ctx, username)
	}
	if len(credentialID) > 0 {
		return s.loginBeginTargeted(ctx, credentialID)
	}
	return s.loginBeginDiscoverable(ctx)
}

// HasRegistrationChallenge reports whether accountID has an active registration
// challenge in the store, without consuming it.
func (s *Service) HasRegistrationChallenge(accountID string) bool {
	return s.challenges.has(accountID)
}

// loginBeginByUsername looks up an account by username, finds its primary
// credential, and delegates to targeted mode. All errors map to ErrNotFound so
// the response does not reveal whether the username exists.
func (s *Service) loginBeginByUsername(ctx context.Context, username string) (*LoginBeginResult, error) {
	account, err := s.db.GetAccountByUsername(ctx, pgtype.Text{String: username, Valid: true})
	if err != nil {
		return nil, ErrNotFound
	}
	credID, err := s.db.GetPrimaryCredentialIDByAccount(ctx, account.ID)
	if err != nil {
		return nil, ErrNotFound
	}
	res, err := s.loginBeginTargeted(ctx, credID)
	if err != nil {
		return nil, ErrNotFound
	}
	return res, nil
}

// loginBeginTargeted uses prf.eval.first with the known credential's PRF salt.
// Uses BeginLogin with allowCredentials set so the browser routes directly to
// the credential provider (e.g. 1Password) instead of showing the full picker.
func (s *Service) loginBeginTargeted(ctx context.Context, credentialID []byte) (*LoginBeginResult, error) {
	credRow, err := s.db.GetCredentialByWebAuthnID(ctx, credentialID)
	if err != nil {
		return nil, ErrNotFound
	}
	user := credRowToWAUser(credRow)
	assertion, sd, err := s.wa.BeginLogin(user,
		webauthn.WithAssertionExtensions(protocol.AuthenticationExtensions{
			"prf": map[string]any{
				"eval": map[string]any{
					"first": protocol.URLEncodedBase64(credRow.PrfSalt),
				},
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("BeginLogin: %w", err)
	}
	challengeKey := base64.RawURLEncoding.EncodeToString(credentialID)
	s.challenges.set(challengeKey, sd)
	return &LoginBeginResult{Assertion: assertion, ChallengeKey: challengeKey}, nil
}

// loginBeginDiscoverable starts a discoverable login ceremony but patches the
// assertion options to include all registered credentials in allowCredentials and
// evalByCredential. The browser only honours evalByCredential when allowCredentials
// is non-empty, so sending both lets the passkey picker appear while ensuring the
// correct per-credential PRF salt is applied to whichever passkey the user selects.
// The session remains discoverable (BeginDiscoverableLogin sets Discoverable: true)
// so FinishDiscoverableLogin works on the server side.
func (s *Service) loginBeginDiscoverable(ctx context.Context) (*LoginBeginResult, error) {
	salts, err := s.db.GetAllPrfSalts(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetAllPrfSalts: %w", err)
	}
	if len(salts) == 0 {
		return nil, fmt.Errorf("no registered accounts: please sign up first")
	}

	assertion, sd, err := s.wa.BeginDiscoverableLogin()
	if err != nil {
		return nil, fmt.Errorf("BeginDiscoverableLogin: %w", err)
	}

	// Patch allowCredentials so the browser activates evalByCredential.
	allowedCreds := make([]protocol.CredentialDescriptor, len(salts))
	evalByCredential := make(map[string]any, len(salts))
	for i, salt := range salts {
		allowedCreds[i] = protocol.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: salt.CredentialID,
		}
		// Key must match the base64url (no-padding) encoding go-webauthn uses for
		// allowCredentials[i].id in JSON so the browser can correlate them.
		credKey := base64.RawURLEncoding.EncodeToString(salt.CredentialID)
		evalByCredential[credKey] = map[string]any{
			"first": protocol.URLEncodedBase64(salt.PrfSalt),
		}
	}
	assertion.Response.AllowedCredentials = allowedCreds
	if assertion.Response.Extensions == nil {
		assertion.Response.Extensions = protocol.AuthenticationExtensions{}
	}
	assertion.Response.Extensions["prf"] = map[string]any{
		"evalByCredential": evalByCredential,
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

	// Peek at the assertion body to read the BackupEligible flag before the
	// library validates it. go-webauthn rejects assertions where BE differs from
	// the stored credential, but BE can legitimately change (e.g. a credential
	// migrates to a synced keychain). By setting the flag on the credential struct
	// before handing it to the library, we avoid a pre-write to the DB on every
	// login; the post-update below persists any real change after verification.
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	assertionBE, hasBE := parseAssertionBackupEligible(body)

	var usedCredID []byte
	var accountID string

	if len(sd.AllowedCredentialIDs) == 1 {
		// Targeted login: session was started with BeginLogin for a single credential.
		credRow, err := s.db.GetCredentialByWebAuthnID(ctx, sd.AllowedCredentialIDs[0])
		if err != nil {
			return nil, ErrNotFound
		}
		if hasBE {
			credRow.BackupEligible = assertionBE
		}
		user := credRowToWAUser(credRow)
		cred, err := s.wa.FinishLogin(user, *sd, r)
		if err != nil {
			return nil, fmt.Errorf("FinishLogin: %w", err)
		}
		usedCredID = cred.ID
		accountID = credRow.AccountID

		if credRow.BackupEligible != cred.Flags.BackupEligible {
			_ = s.db.UpdateCredentialBackupEligible(ctx, queries.UpdateCredentialBackupEligibleParams{
				CredentialID:   cred.ID,
				BackupEligible: cred.Flags.BackupEligible,
			})
		}
	} else {
		// Discoverable login (loginBeginDiscoverable path): look up the credential by
		// rawID — that is the one that actually signed.
		handler := func(rawID, userHandle []byte) (webauthn.User, error) {
			credRow, err := s.db.GetCredentialByWebAuthnID(ctx, rawID)
			if err != nil {
				return nil, ErrNotFound
			}
			if hasBE {
				credRow.BackupEligible = assertionBE
			}
			accountID = credRow.AccountID
			return credRowToWAUser(credRow), nil
		}
		cred, err := s.wa.FinishDiscoverableLogin(handler, *sd, r)
		if err != nil {
			return nil, fmt.Errorf("FinishDiscoverableLogin: %w", err)
		}
		usedCredID = cred.ID

		if credRow, cerr := s.db.GetCredentialByWebAuthnID(ctx, cred.ID); cerr == nil {
			if credRow.BackupEligible != cred.Flags.BackupEligible {
				_ = s.db.UpdateCredentialBackupEligible(ctx, queries.UpdateCredentialBackupEligibleParams{
					CredentialID:   cred.ID,
					BackupEligible: cred.Flags.BackupEligible,
				})
			}
		}
	}

	// Get the wrapped master key for the credential that actually signed.
	credLogin, err := s.db.GetCredentialForLogin(ctx, usedCredID)
	if err != nil {
		return nil, fmt.Errorf("GetCredentialForLogin: %w", err)
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
		AccountID:    accountID,
		TokenHash:    tokenHash,
		CredentialID: usedCredID,
		UserAgent:    userAgent,
	}); err != nil {
		return nil, fmt.Errorf("CreateSession: %w", err)
	}

	return &LoginFinishResult{
		AccountID:        accountID,
		WrappedMasterKey: credLogin.WrappedMasterKey,
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
// Populates allowCredentials with all of the account's registered credentials and
// uses evalByCredential so the browser only offers valid keys for this account and
// applies the correct per-credential PRF salt regardless of which key is used.
func (s *Service) ReauthBegin(ctx context.Context, accountID string) (*ReauthBeginResult, error) {
	salts, err := s.db.GetPrfSaltsByAccount(ctx, accountID)
	if err != nil || len(salts) == 0 {
		return nil, ErrNotFound
	}

	assertion, sd, err := s.wa.BeginDiscoverableLogin()
	if err != nil {
		return nil, fmt.Errorf("BeginDiscoverableLogin: %w", err)
	}

	allowedCreds := make([]protocol.CredentialDescriptor, len(salts))
	evalByCredential := make(map[string]any, len(salts))
	for i, salt := range salts {
		allowedCreds[i] = protocol.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: salt.CredentialID,
		}
		credKey := base64.RawURLEncoding.EncodeToString(salt.CredentialID)
		evalByCredential[credKey] = map[string]any{
			"first": protocol.URLEncodedBase64(salt.PrfSalt),
		}
	}
	assertion.Response.AllowedCredentials = allowedCreds
	if assertion.Response.Extensions == nil {
		assertion.Response.Extensions = protocol.AuthenticationExtensions{}
	}
	assertion.Response.Extensions["prf"] = map[string]any{
		"evalByCredential": evalByCredential,
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
	CredentialID     []byte // raw WebAuthn credential ID of the credential used
	WrappedMasterKey []byte
	AddCredToken     string // non-empty when purpose == "add-credential"
}

// ReauthFinish verifies the WebAuthn assertion and returns the wrapped master
// key for the existing session. No new session row is created.
func (s *Service) ReauthFinish(ctx context.Context, challengeKey string, accountID string, purpose string, r *http.Request) (*ReauthFinishResult, error) {
	sd, ok := s.challenges.take(challengeKey)
	if !ok {
		return nil, fmt.Errorf("challenge not found or expired")
	}

	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	assertionBE, hasBE := parseAssertionBackupEligible(body)

	var usedCredID []byte
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		credRow, err := s.db.GetCredentialByWebAuthnID(ctx, rawID)
		if err != nil {
			return nil, ErrNotFound
		}
		if hasBE {
			credRow.BackupEligible = assertionBE
		}
		accountID = credRow.AccountID
		return credRowToWAUser(credRow), nil
	}

	cred, err := s.wa.FinishDiscoverableLogin(handler, *sd, r)
	if err != nil {
		return nil, fmt.Errorf("FinishDiscoverableLogin: %w", err)
	}
	usedCredID = cred.ID

	if credRow, cerr := s.db.GetCredentialByWebAuthnID(ctx, cred.ID); cerr == nil {
		if credRow.BackupEligible != cred.Flags.BackupEligible {
			_ = s.db.UpdateCredentialBackupEligible(ctx, queries.UpdateCredentialBackupEligibleParams{
				CredentialID:   cred.ID,
				BackupEligible: cred.Flags.BackupEligible,
			})
		}
	}

	credLogin, err := s.db.GetCredentialForLogin(ctx, usedCredID)
	if err != nil {
		return nil, fmt.Errorf("GetCredentialForLogin: %w", err)
	}

	result := &ReauthFinishResult{
		AccountID:        accountID,
		CredentialID:     usedCredID,
		WrappedMasterKey: credLogin.WrappedMasterKey,
	}

	if purpose == "add-credential" {
		tok, err := s.addCredToks.issue(accountID)
		if err != nil {
			return nil, fmt.Errorf("issue add-credential token: %w", err)
		}
		result.AddCredToken = tok
	}

	return result, nil
}

// ─── Add Credential ───────────────────────────────────────────────────────────

type AddCredentialBeginResult struct {
	PRFSalt  []byte
	Creation *protocol.CredentialCreation
}

// AddCredentialBegin starts a registration ceremony to add a new passkey to an
// existing account. Requires a valid addCredToken from a successful reauth.
func (s *Service) AddCredentialBegin(ctx context.Context, accountID, addCredToken string) (*AddCredentialBeginResult, error) {
	if _, ok := s.addCredToks.peek(addCredToken); !ok {
		return nil, fmt.Errorf("invalid or expired add-credential token")
	}

	prfSalt, err := randomBytes(32)
	if err != nil {
		return nil, fmt.Errorf("randomBytes: %w", err)
	}

	user, err := s.getWAUser(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("getWAUser: %w", err)
	}
	creation, sd, err := s.wa.BeginRegistration(user,
		webauthn.WithExtensions(protocol.AuthenticationExtensions{
			"prf": map[string]any{
				"eval": map[string]any{
					"first": protocol.URLEncodedBase64(prfSalt),
				},
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("BeginRegistration: %w", err)
	}

	s.challenges.set("add-cred:"+accountID, sd)
	return &AddCredentialBeginResult{PRFSalt: prfSalt, Creation: creation}, nil
}

type AddCredentialFinishRequest struct {
	AddCredToken     string
	WrappedMasterKey []byte
	PRFSalt          []byte
	Name             string
}

type AddCredentialFinishResult struct {
	ID        string
	Name      string
	CreatedAt string
}

// AddCredentialFinish verifies the registration and stores the new credential.
func (s *Service) AddCredentialFinish(ctx context.Context, accountID string, req *AddCredentialFinishRequest, r *http.Request) (*AddCredentialFinishResult, error) {
	if _, ok := s.addCredToks.consume(req.AddCredToken); !ok {
		return nil, fmt.Errorf("invalid or expired add-credential token")
	}

	sd, ok := s.challenges.take("add-cred:" + accountID)
	if !ok {
		return nil, fmt.Errorf("challenge not found or expired")
	}

	user := &waUser{id: []byte(accountID)}
	cred, err := s.wa.FinishRegistration(user, *sd, r)
	if err != nil {
		return nil, fmt.Errorf("FinishRegistration: %w", err)
	}

	row, err := s.insertCredential(ctx, accountID, cred, req.PRFSalt, req.WrappedMasterKey, req.Name)
	if err != nil {
		return nil, err
	}

	return &AddCredentialFinishResult{
		ID:        row.ID,
		Name:      row.Name,
		CreatedAt: row.CreatedAt.Time.Format(time.RFC3339),
	}, nil
}

// insertCredential creates a credential row. Used by AddCredentialFinish and PairingComplete.
func (s *Service) insertCredential(ctx context.Context, accountID string, cred *webauthn.Credential, prfSalt, wrappedMasterKey []byte, name string) (queries.Credential, error) {
	credRowID, err := randomBase64URL(16)
	if err != nil {
		return queries.Credential{}, err
	}
	row, err := s.db.CreateCredential(ctx, queries.CreateCredentialParams{
		ID:               credRowID,
		AccountID:        accountID,
		CredentialID:     cred.ID,
		PublicKey:        cred.PublicKey,
		PrfSalt:          prfSalt,
		WrappedMasterKey: wrappedMasterKey,
		BackupEligible:   cred.Flags.BackupEligible,
		Name:             name,
	})
	if err != nil {
		if isDuplicateKey(err) {
			return queries.Credential{}, ErrDuplicateAccount
		}
		return queries.Credential{}, fmt.Errorf("CreateCredential: %w", err)
	}
	return row, nil
}

// ─── Device Pairing ───────────────────────────────────────────────────────────

type PairingCreateResult struct {
	Token     string
	ShortCode string
	ExpiresAt time.Time
}

// PairingCreate issues a new pairing session for an authenticated user.
func (s *Service) PairingCreate(ctx context.Context, accountID string) (*PairingCreateResult, error) {
	sess, err := s.pairings.create(accountID)
	if err != nil {
		return nil, err
	}
	return &PairingCreateResult{
		Token:     sess.token,
		ShortCode: sess.shortCode,
		ExpiresAt: sess.expiresAt,
	}, nil
}

// PairingByCode looks up a full pairing token by its short code.
func (s *Service) PairingByCode(code string) (string, bool) {
	return s.pairings.getByCode(code)
}

type PairingRequestResult struct {
	PRFSalt  []byte
	Creation *protocol.CredentialCreation
}

// PairingRequest stores the new device's ephemeral public key, transitions the
// session to "requested", and begins a WebAuthn registration ceremony so the
// new device can start the passkey creation while waiting for approval.
func (s *Service) PairingRequest(ctx context.Context, token string, newDevicePubKey []byte) (*PairingRequestResult, error) {
	accountID, err := s.pairings.request(token, newDevicePubKey)
	if err != nil {
		return nil, err
	}

	prfSalt, err := randomBytes(32)
	if err != nil {
		return nil, fmt.Errorf("randomBytes: %w", err)
	}

	user, err := s.getWAUser(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("getWAUser: %w", err)
	}
	creation, sd, err := s.wa.BeginRegistration(user,
		webauthn.WithExtensions(protocol.AuthenticationExtensions{
			"prf": map[string]any{
				"eval": map[string]any{
					"first": protocol.URLEncodedBase64(prfSalt),
				},
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("BeginRegistration: %w", err)
	}

	s.challenges.set("pairing:"+token, sd)
	return &PairingRequestResult{PRFSalt: prfSalt, Creation: creation}, nil
}

// PairingFulfill stores the wrapped master key and transitions the session to
// "fulfilled". Only the account that created the pairing session can fulfill it.
func (s *Service) PairingFulfill(ctx context.Context, accountID, token string, wrappedMasterKey []byte) error {
	return s.pairings.fulfill(token, accountID, wrappedMasterKey)
}

// PairingPoll returns the current state of a pairing session.
func (s *Service) PairingPoll(ctx context.Context, token string) (*PairingPollResult, bool) {
	return s.pairings.poll(token)
}

type PairingCompleteResult struct {
	AccountID        string
	SessionToken     string
	SessionID        string
	CredentialID     []byte
	WrappedMasterKey []byte
}

// PairingComplete finishes the WebAuthn registration on the new device,
// inserts the credential, and issues a session.
func (s *Service) PairingComplete(ctx context.Context, token string, prfSalt, wrappedMasterKey []byte, name, userAgent string, r *http.Request) (*PairingCompleteResult, error) {
	sess, err := s.pairings.complete(token)
	if err != nil {
		return nil, err
	}

	sd, ok := s.challenges.take("pairing:" + token)
	if !ok {
		return nil, fmt.Errorf("registration challenge not found or expired")
	}

	user := &waUser{id: []byte(sess.accountID)}
	cred, err := s.wa.FinishRegistration(user, *sd, r)
	if err != nil {
		return nil, fmt.Errorf("FinishRegistration: %w", err)
	}

	sessionToken, tokenHash, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	sessionID, err := randomBase64URL(16)
	if err != nil {
		return nil, err
	}
	credRowID, err := randomBase64URL(16)
	if err != nil {
		return nil, err
	}

	var credRow queries.Credential
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		q := queries.New(tx)

		var txErr error
		credRow, txErr = q.CreateCredential(ctx, queries.CreateCredentialParams{
			ID:               credRowID,
			AccountID:        sess.accountID,
			CredentialID:     cred.ID,
			PublicKey:        cred.PublicKey,
			PrfSalt:          prfSalt,
			WrappedMasterKey: wrappedMasterKey,
			BackupEligible:   cred.Flags.BackupEligible,
			Name:             name,
		})
		if txErr != nil {
			if isDuplicateKey(txErr) {
				return ErrDuplicateAccount
			}
			return fmt.Errorf("CreateCredential: %w", txErr)
		}

		if _, txErr = q.CreateSession(ctx, queries.CreateSessionParams{
			ID:           sessionID,
			AccountID:    sess.accountID,
			TokenHash:    tokenHash,
			CredentialID: cred.ID,
			UserAgent:    userAgent,
		}); txErr != nil {
			return fmt.Errorf("CreateSession: %w", txErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &PairingCompleteResult{
		AccountID:        sess.accountID,
		SessionToken:     sessionToken,
		SessionID:        sessionID,
		CredentialID:     credRow.CredentialID,
		WrappedMasterKey: wrappedMasterKey,
	}, nil
}

// ─── Credential Management ────────────────────────────────────────────────────

type CredentialSummary struct {
	ID             string
	Name           string
	CreatedAt      string
	BackupEligible bool
	CredentialID   []byte // raw WebAuthn credential ID (for isCurrentSession check)
}

func (s *Service) ListCredentials(ctx context.Context, accountID string) ([]CredentialSummary, error) {
	rows, err := s.db.ListCredentialsByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ListCredentialsByAccount: %w", err)
	}
	out := make([]CredentialSummary, len(rows))
	for i, r := range rows {
		out[i] = CredentialSummary{
			ID:             r.ID,
			Name:           r.Name,
			CreatedAt:      r.CreatedAt.Time.Format(time.RFC3339),
			BackupEligible: r.BackupEligible,
			CredentialID:   r.CredentialID,
		}
	}
	return out, nil
}

func (s *Service) RenameCredential(ctx context.Context, accountID, credID, name string) error {
	return s.db.UpdateCredentialName(ctx, queries.UpdateCredentialNameParams{
		ID:        credID,
		AccountID: accountID,
		Name:      name,
	})
}

func (s *Service) DeleteCredential(ctx context.Context, accountID, credID string) error {
	count, err := s.db.CountCredentialsByAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("CountCredentialsByAccount: %w", err)
	}
	if count <= 1 {
		return ErrLastCredential
	}
	return s.db.DeleteCredential(ctx, queries.DeleteCredentialParams{
		ID:        credID,
		AccountID: accountID,
	})
}

// ─── Recovery ─────────────────────────────────────────────────────────────────

type RecoverResult struct {
	AccountID             string
	RecoveryWrappedMaster []byte
	RekeyToken            string // short-lived proof-of-recovery token
}

func (s *Service) Recover(ctx context.Context, username string, codeHash []byte) (*RecoverResult, error) {
	account, err := s.db.GetAccountByUsername(ctx, pgtype.Text{String: username, Valid: true})
	if err != nil {
		return nil, ErrInvalidCode // don't leak whether the username exists
	}

	rc, err := s.db.GetUnusedRecoveryCode(ctx, queries.GetUnusedRecoveryCodeParams{
		AccountID: account.ID,
		CodeHash:  codeHash,
	})
	if err != nil {
		return nil, ErrInvalidCode
	}

	if err := s.db.BurnRecoveryCode(ctx, rc.ID); err != nil {
		return nil, fmt.Errorf("BurnRecoveryCode: %w", err)
	}

	rekeyToken, err := s.rekeys.issue(account.ID)
	if err != nil {
		return nil, fmt.Errorf("issue rekey token: %w", err)
	}

	return &RecoverResult{
		AccountID:             account.ID,
		RecoveryWrappedMaster: account.RecoveryWrappedMaster,
		RekeyToken:            rekeyToken,
	}, nil
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

func (s *Service) DeleteOtherSessions(ctx context.Context, accountID, currentSessionID string) error {
	return s.db.DeleteOtherSessions(ctx, queries.DeleteOtherSessionsParams{AccountID: accountID, ID: currentSessionID})
}

// ─── Account deletion ─────────────────────────────────────────────────────────

// WorkspaceSubscription holds the Stripe subscription ID for a workspace that
// had an active subscription at deletion time.
type WorkspaceSubscription struct {
	WorkspaceID        string
	StripeSubscriptionID string
}

// DeleteAccount deletes the account and all its owned workspaces in a single
// transaction. Returns the Stripe subscription IDs from owned workspaces so
// the caller can cancel them with Stripe after the DB changes commit.
func (s *Service) DeleteAccount(ctx context.Context, accountID string) ([]WorkspaceSubscription, error) {
	var subs []WorkspaceSubscription
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		q := queries.New(tx)

		owned, err := q.ListOwnedWorkspacesForDeletion(ctx, accountID)
		if err != nil {
			return fmt.Errorf("ListOwnedWorkspacesForDeletion: %w", err)
		}
		for _, ws := range owned {
			if ws.StripeSubscriptionID.Valid && ws.StripeSubscriptionID.String != "" {
				subs = append(subs, WorkspaceSubscription{
					WorkspaceID:        ws.ID,
					StripeSubscriptionID: ws.StripeSubscriptionID.String,
				})
			}
			if err := q.DeleteWorkspace(ctx, ws.ID); err != nil {
				return fmt.Errorf("DeleteWorkspace(%s): %w", ws.ID, err)
			}
		}

		if err := q.DeleteAccount(ctx, accountID); err != nil {
			return fmt.Errorf("DeleteAccount: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return subs, nil
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

// parseAssertionBackupEligible extracts the BackupEligible flag (bit 3 of the
// authenticator flags byte) from a raw WebAuthn assertion JSON. Returns the flag
// value and true if parsing succeeded; false if the body is malformed or short.
func parseAssertionBackupEligible(body []byte) (be bool, ok bool) {
	var parsed struct {
		Response struct {
			AuthenticatorData string `json:"authenticatorData"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.Response.AuthenticatorData == "" {
		return false, false
	}
	authData, err := base64.RawURLEncoding.DecodeString(parsed.Response.AuthenticatorData)
	if err != nil || len(authData) < 33 {
		return false, false
	}
	return authData[32]&0x08 != 0, true
}

// credRowToWAUser builds a waUser from a GetCredentialByWebAuthnID row.
func credRowToWAUser(row queries.GetCredentialByWebAuthnIDRow) *waUser {
	name := row.AccountID
	if row.Username.Valid && row.Username.String != "" {
		name = row.Username.String
	}
	return &waUser{
		id:          []byte(row.AccountID),
		name:        name,
		displayName: name,
		credentials: []webauthn.Credential{
			{
				ID:        row.CredentialID,
				PublicKey: row.PublicKey,
				Flags: webauthn.CredentialFlags{
					BackupEligible: row.BackupEligible,
				},
			},
		},
	}
}

// getWAUser fetches the account and all its registered credential IDs, then
// returns a fully-populated waUser. Use this for registration ceremonies
// (BeginRegistration) so go-webauthn can build a correct excludeCredentials list.
// Only credential IDs are needed; public keys are not required at registration time.
func (s *Service) getWAUser(ctx context.Context, accountID string) (*waUser, error) {
	a, err := s.db.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.ListCredentialsByAccount(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	name := a.ID
	if a.Username.Valid && a.Username.String != "" {
		name = a.Username.String
	}
	creds := make([]webauthn.Credential, len(rows))
	for i, row := range rows {
		creds[i] = webauthn.Credential{ID: row.CredentialID}
	}
	return &waUser{
		id:          []byte(a.ID),
		name:        name,
		displayName: name,
		credentials: creds,
	}, nil
}
