package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/phantompunk/confide/internal/db/queries"
)

// ─── Rekey Token Store ────────────────────────────────────────────────────────

// rekeyTokenEntry holds a short-lived proof-of-recovery token.
type rekeyTokenEntry struct {
	accountID string
	expires   time.Time
}

// rekeyTokenStore stores short-lived tokens issued after a successful recovery
// code burn. Required to authorize a subsequent rekey registration ceremony.
type rekeyTokenStore struct {
	mu    sync.Mutex
	items map[string]*rekeyTokenEntry // key = base64url(sha256(token))
}

func newRekeyTokenStore() *rekeyTokenStore {
	rs := &rekeyTokenStore{items: make(map[string]*rekeyTokenEntry)}
	go rs.gcLoop()
	return rs
}

// issue mints a new rekey token for accountID and returns the raw token.
func (rs *rekeyTokenStore) issue(accountID string) (string, error) {
	raw, err := randomBase64URL(16)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(raw))
	key := base64.RawURLEncoding.EncodeToString(h[:])
	rs.mu.Lock()
	rs.items[key] = &rekeyTokenEntry{accountID: accountID, expires: time.Now().Add(10 * time.Minute)}
	rs.mu.Unlock()
	return raw, nil
}

// peek validates the token without consuming it (for the begin step).
func (rs *rekeyTokenStore) peek(raw string) (string, bool) {
	h := sha256.Sum256([]byte(raw))
	key := base64.RawURLEncoding.EncodeToString(h[:])
	rs.mu.Lock()
	defer rs.mu.Unlock()
	e, ok := rs.items[key]
	if !ok || time.Now().After(e.expires) {
		delete(rs.items, key)
		return "", false
	}
	return e.accountID, true
}

// consume validates the token and returns accountID, removing the token.
func (rs *rekeyTokenStore) consume(raw string) (string, bool) {
	h := sha256.Sum256([]byte(raw))
	key := base64.RawURLEncoding.EncodeToString(h[:])
	rs.mu.Lock()
	defer rs.mu.Unlock()
	e, ok := rs.items[key]
	if !ok || time.Now().After(e.expires) {
		delete(rs.items, key)
		return "", false
	}
	delete(rs.items, key)
	return e.accountID, true
}

func (rs *rekeyTokenStore) gcLoop() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		rs.mu.Lock()
		for k, e := range rs.items {
			if now.After(e.expires) {
				delete(rs.items, k)
			}
		}
		rs.mu.Unlock()
	}
}

// ─── Rekey Service Methods ────────────────────────────────────────────────────

type RekeyBeginResult struct {
	Creation *protocol.CredentialCreation
	PRFSalt  []byte
}

func (s *Service) RekeyBegin(ctx context.Context, rekeyToken string) (*RekeyBeginResult, error) {
	accountID, ok := s.rekeys.peek(rekeyToken)
	if !ok {
		return nil, fmt.Errorf("invalid or expired rekey token")
	}

	account, err := s.db.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, ErrNotFound
	}

	user := accountToWAUser(account)
	// Generate a fresh PRF salt for the new credential.
	prfSalt, err := randomBytes(32)
	if err != nil {
		return nil, fmt.Errorf("randomBytes: %w", err)
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

	s.challenges.set("rekey:"+accountID, sd)
	return &RekeyBeginResult{Creation: creation, PRFSalt: prfSalt}, nil
}

type RekeyFinishRequest struct {
	RekeyToken            string   `json:"rekeyToken"`
	PRFSalt               []byte   `json:"prfSalt"`
	WrappedMasterKey      []byte   `json:"wrappedMasterKey"`
	RecoveryWrappedMaster []byte   `json:"recoveryWrappedMasterKey"`
	RecoveryVerifier      []byte   `json:"recoveryVerifier"`
	RecoveryCodes         [][]byte `json:"recoveryCodes"` // 12 × SHA-256 hashes
}

type RekeyFinishResult struct {
	CredentialIDBase64 string
	SessionToken       string // raw token; caller sets the session cookie
}

func (s *Service) RekeyFinish(ctx context.Context, req *RekeyFinishRequest, userAgent string, r *http.Request) (*RekeyFinishResult, error) {
	accountID, ok := s.rekeys.consume(req.RekeyToken)
	if !ok {
		return nil, fmt.Errorf("invalid or expired rekey token")
	}

	account, err := s.db.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, ErrNotFound
	}

	sd, ok := s.challenges.take("rekey:" + accountID)
	if !ok {
		return nil, fmt.Errorf("challenge not found or expired")
	}

	user := accountToWAUser(account)
	cred, err := s.wa.FinishRegistration(user, *sd, r)
	if err != nil {
		return nil, fmt.Errorf("FinishRegistration: %w", err)
	}

	if len(req.RecoveryCodes) != 12 {
		return nil, fmt.Errorf("expected 12 recovery code hashes, got %d", len(req.RecoveryCodes))
	}

	credRowID, err := randomBase64URL(16)
	if err != nil {
		return nil, err
	}

	token, tokenHash, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	sessionID, err := randomBase64URL(16)
	if err != nil {
		return nil, err
	}

	// Wipe all existing credentials and sessions, insert the single new credential,
	// update recovery data, replace recovery codes, and create a fresh session —
	// all in one transaction.
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		q := queries.New(tx)

		if err := q.DeleteCredentialsByAccount(ctx, accountID); err != nil {
			return fmt.Errorf("DeleteCredentialsByAccount: %w", err)
		}

		if _, err := q.CreateCredential(ctx, queries.CreateCredentialParams{
			ID:               credRowID,
			AccountID:        accountID,
			CredentialID:     cred.ID,
			PublicKey:        cred.PublicKey,
			PrfSalt:          req.PRFSalt,
			WrappedMasterKey: req.WrappedMasterKey,
			BackupEligible:   cred.Flags.BackupEligible,
			Name:             "",
		}); err != nil {
			return fmt.Errorf("CreateCredential: %w", err)
		}

		if err := q.UpdateAccountRecovery(ctx, queries.UpdateAccountRecoveryParams{
			ID:                    accountID,
			RecoveryWrappedMaster: req.RecoveryWrappedMaster,
			RecoveryVerifier:      req.RecoveryVerifier,
		}); err != nil {
			return fmt.Errorf("UpdateAccountRecovery: %w", err)
		}

		if err := q.DeleteRecoveryCodesByAccount(ctx, accountID); err != nil {
			return fmt.Errorf("DeleteRecoveryCodesByAccount: %w", err)
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
				AccountID: accountID,
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
			AccountID:    accountID,
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

	return &RekeyFinishResult{
		CredentialIDBase64: base64.StdEncoding.EncodeToString(cred.ID),
		SessionToken:       token,
	}, nil
}

// ─── Rotate Recovery Codes ────────────────────────────────────────────────────

type RotateRecoveryCodesRequest struct {
	RecoveryWrappedMaster []byte
	RecoveryVerifier      []byte
	RecoveryCodes         [][]byte // 12 × SHA-256 hashes
}

func (s *Service) RotateRecoveryCodes(ctx context.Context, accountID string, req *RotateRecoveryCodesRequest) error {
	if len(req.RecoveryCodes) != 12 {
		return fmt.Errorf("expected 12 recovery code hashes, got %d", len(req.RecoveryCodes))
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		q := queries.New(tx)

		if err := q.UpdateAccountRecovery(ctx, queries.UpdateAccountRecoveryParams{
			ID:                    accountID,
			RecoveryWrappedMaster: req.RecoveryWrappedMaster,
			RecoveryVerifier:      req.RecoveryVerifier,
		}); err != nil {
			return fmt.Errorf("UpdateAccountRecovery: %w", err)
		}

		if err := q.DeleteRecoveryCodesByAccount(ctx, accountID); err != nil {
			return fmt.Errorf("DeleteRecoveryCodesByAccount: %w", err)
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
				AccountID: accountID,
				CodeHash:  hash,
				Used:      false,
				CreatedAt: now,
			}
		}
		if _, err := q.CreateRecoveryCodes(ctx, codes); err != nil {
			return fmt.Errorf("CreateRecoveryCodes: %w", err)
		}
		return nil
	})
}
