package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/phantompunk/wisp/internal/db/queries"
)

// ─── Mock DB ──────────────────────────────────────────────────────────────────

type mockDB struct {
	accounts       map[string]queries.Account
	sessions       map[string]queries.Session
	sessionsByHash map[string]queries.GetSessionByTokenHashRow
	recoveryCodes  map[string]queries.RecoveryCode
}

func newMockDB() *mockDB {
	return &mockDB{
		accounts:       make(map[string]queries.Account),
		sessions:       make(map[string]queries.Session),
		sessionsByHash: make(map[string]queries.GetSessionByTokenHashRow),
		recoveryCodes:  make(map[string]queries.RecoveryCode),
	}
}

func (m *mockDB) CreateAccount(_ context.Context, arg queries.CreateAccountParams) (queries.Account, error) {
	if _, exists := m.accounts[arg.ID]; exists {
		return queries.Account{}, errors.New("duplicate key value violates unique constraint")
	}
	a := queries.Account{
		ID:                    arg.ID,
		CredentialID:          arg.CredentialID,
		PublicKey:             arg.PublicKey,
		PrfSalt:               arg.PrfSalt,
		WrappedMasterKey:      arg.WrappedMasterKey,
		RecoveryWrappedMaster: arg.RecoveryWrappedMaster,
		RecoveryVerifier:      arg.RecoveryVerifier,
	}
	m.accounts[arg.ID] = a
	return a, nil
}

func (m *mockDB) GetAccountByCredentialID(_ context.Context, credentialID []byte) (queries.Account, error) {
	for _, a := range m.accounts {
		if string(a.CredentialID) == string(credentialID) {
			return a, nil
		}
	}
	return queries.Account{}, errors.New("no rows")
}

func (m *mockDB) GetAccountByID(_ context.Context, id string) (queries.Account, error) {
	a, ok := m.accounts[id]
	if !ok {
		return queries.Account{}, errors.New("no rows")
	}
	return a, nil
}

func (m *mockDB) CreateSession(_ context.Context, arg queries.CreateSessionParams) (queries.Session, error) {
	s := queries.Session{ID: arg.ID, AccountID: arg.AccountID, TokenHash: arg.TokenHash}
	m.sessions[arg.ID] = s
	return s, nil
}

func (m *mockDB) GetSessionByTokenHash(_ context.Context, tokenHash []byte) (queries.GetSessionByTokenHashRow, error) {
	row, ok := m.sessionsByHash[string(tokenHash)]
	if !ok {
		return queries.GetSessionByTokenHashRow{}, errors.New("no rows")
	}
	return row, nil
}

func (m *mockDB) TouchSession(_ context.Context, id string) error { return nil }

func (m *mockDB) DeleteSession(_ context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockDB) ListSessionsByAccount(_ context.Context, accountID string) ([]queries.ListSessionsByAccountRow, error) {
	var out []queries.ListSessionsByAccountRow
	for _, s := range m.sessions {
		if s.AccountID == accountID {
			out = append(out, queries.ListSessionsByAccountRow{ID: s.ID, CreatedAt: s.CreatedAt, LastSeen: s.LastSeen})
		}
	}
	return out, nil
}

func (m *mockDB) CreateRecoveryCodes(_ context.Context, arg []queries.CreateRecoveryCodesParams) (int64, error) {
	for _, p := range arg {
		m.recoveryCodes[p.ID] = queries.RecoveryCode{
			ID:        p.ID,
			AccountID: p.AccountID,
			CodeHash:  p.CodeHash,
			Used:      p.Used,
			CreatedAt: p.CreatedAt,
		}
	}
	return int64(len(arg)), nil
}

func (m *mockDB) GetUnusedRecoveryCode(_ context.Context, arg queries.GetUnusedRecoveryCodeParams) (queries.RecoveryCode, error) {
	for _, rc := range m.recoveryCodes {
		if rc.AccountID == arg.AccountID && string(rc.CodeHash) == string(arg.CodeHash) && !rc.Used {
			return rc, nil
		}
	}
	return queries.RecoveryCode{}, errors.New("no rows")
}

func (m *mockDB) BurnRecoveryCode(_ context.Context, id string) error {
	if rc, ok := m.recoveryCodes[id]; ok {
		rc.Used = true
		m.recoveryCodes[id] = rc
	}
	return nil
}

func (m *mockDB) CountUnusedRecoveryCodes(_ context.Context, accountID string) (int64, error) {
	var n int64
	for _, rc := range m.recoveryCodes {
		if rc.AccountID == accountID && !rc.Used {
			n++
		}
	}
	return n, nil
}

func (m *mockDB) UpdateAccountCredential(_ context.Context, arg queries.UpdateAccountCredentialParams) error {
	a, ok := m.accounts[arg.ID]
	if !ok {
		return errors.New("no rows")
	}
	a.CredentialID = arg.CredentialID
	a.PublicKey = arg.PublicKey
	a.PrfSalt = arg.PrfSalt
	a.WrappedMasterKey = arg.WrappedMasterKey
	a.RecoveryWrappedMaster = arg.RecoveryWrappedMaster
	a.RecoveryVerifier = arg.RecoveryVerifier
	a.BackupEligible = arg.BackupEligible
	m.accounts[arg.ID] = a
	return nil
}

func (m *mockDB) ListCredentials(_ context.Context) ([]queries.ListCredentialsRow, error) {
	rows := make([]queries.ListCredentialsRow, 0, len(m.accounts))
	for _, a := range m.accounts {
		rows = append(rows, queries.ListCredentialsRow{CredentialID: a.CredentialID, PrfSalt: a.PrfSalt})
	}
	return rows, nil
}

func (m *mockDB) UpdateAccountBackupEligible(_ context.Context, arg queries.UpdateAccountBackupEligibleParams) error {
	for id, a := range m.accounts {
		if string(a.CredentialID) == string(arg.CredentialID) {
			a.BackupEligible = arg.BackupEligible
			m.accounts[id] = a
			return nil
		}
	}
	return nil
}

func (m *mockDB) DeleteRecoveryCodesByAccount(_ context.Context, accountID string) error {
	for id, rc := range m.recoveryCodes {
		if rc.AccountID == accountID {
			delete(m.recoveryCodes, id)
		}
	}
	return nil
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func newTestWA(t *testing.T) *webauthn.WebAuthn {
	t.Helper()
	wa, err := webauthn.New(&webauthn.Config{
		RPID:          "localhost",
		RPDisplayName: "Test",
		RPOrigins:     []string{"http://localhost:3000"},
	})
	if err != nil {
		t.Fatalf("webauthn.New: %v", err)
	}
	return wa
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestRegisterBegin_ReturnsAccountID(t *testing.T) {
	svc := newServiceWithDB(newMockDB(), newTestWA(t))
	res, err := svc.RegisterBegin(context.Background())
	if err != nil {
		t.Fatalf("RegisterBegin: %v", err)
	}
	if res.AccountID == "" {
		t.Error("expected non-empty AccountID")
	}
	if len(res.PRFSalt) != 32 {
		t.Errorf("expected 32-byte PRFSalt, got %d", len(res.PRFSalt))
	}
	if res.Creation == nil {
		t.Error("expected non-nil CredentialCreation")
	}
}

func TestRecover_InvalidCode_Returns401(t *testing.T) {
	db := newMockDB()
	db.accounts["acc1"] = queries.Account{
		ID:                    "acc1",
		RecoveryWrappedMaster: []byte("key"),
	}
	svc := newServiceWithDB(db, newTestWA(t))

	_, err := svc.Recover(context.Background(), "acc1", "WRONG-CODE")
	if err != ErrInvalidCode {
		t.Errorf("expected ErrInvalidCode, got %v", err)
	}
}

func TestRecover_BurnsCode(t *testing.T) {
	db := newMockDB()
	db.accounts["acc1"] = queries.Account{
		ID:                    "acc1",
		RecoveryWrappedMaster: []byte("recoverykey"),
	}

	// Pre-insert a valid recovery code hash for "TESTCODE".
	normalised := "TESTCODE"
	hash := sha256Sum([]byte(normalised))
	db.recoveryCodes["rc1"] = queries.RecoveryCode{
		ID:        "rc1",
		AccountID: "acc1",
		CodeHash:  hash,
		Used:      false,
	}

	svc := newServiceWithDB(db, newTestWA(t))
	res, err := svc.Recover(context.Background(), "acc1", "TESTCODE")
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if string(res.RecoveryWrappedMaster) != "recoverykey" {
		t.Error("wrong recovery wrapped master key")
	}

	// Code should now be burned.
	if !db.recoveryCodes["rc1"].Used {
		t.Error("expected recovery code to be burned")
	}
}

func TestDeleteSession_WrongAccount_ReturnsNotFound(t *testing.T) {
	db := newMockDB()
	db.sessions["sess1"] = queries.Session{ID: "sess1", AccountID: "acc2"}
	svc := newServiceWithDB(db, newTestWA(t))

	err := svc.DeleteSession(context.Background(), "acc1", "sess1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
