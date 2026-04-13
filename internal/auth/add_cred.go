package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"sync"
	"time"
)

// ─── Add-Credential Token Store ───────────────────────────────────────────────

// addCredTokenEntry holds a short-lived proof-of-reauth token used to authorize
// adding a new passkey without going through the full recovery flow.
type addCredTokenEntry struct {
	accountID string
	expires   time.Time
}

// addCredTokenStore stores short-lived tokens issued after a successful reauth
// with purpose "add-credential". Required to authorize a subsequent add-credential
// registration ceremony.
type addCredTokenStore struct {
	mu    sync.Mutex
	items map[string]*addCredTokenEntry // key = base64url(sha256(token))
}

func newAddCredTokenStore() *addCredTokenStore {
	s := &addCredTokenStore{items: make(map[string]*addCredTokenEntry)}
	go s.gcLoop()
	return s
}

// issue mints a new add-credential token for accountID and returns the raw token.
func (s *addCredTokenStore) issue(accountID string) (string, error) {
	raw, err := randomBase64URL(16)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(raw))
	key := base64.RawURLEncoding.EncodeToString(h[:])
	s.mu.Lock()
	s.items[key] = &addCredTokenEntry{accountID: accountID, expires: time.Now().Add(10 * time.Minute)}
	s.mu.Unlock()
	return raw, nil
}

// peek validates the token without consuming it (for the begin step).
func (s *addCredTokenStore) peek(raw string) (string, bool) {
	h := sha256.Sum256([]byte(raw))
	key := base64.RawURLEncoding.EncodeToString(h[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[key]
	if !ok || time.Now().After(e.expires) {
		delete(s.items, key)
		return "", false
	}
	return e.accountID, true
}

// consume validates the token and returns accountID, removing the token.
func (s *addCredTokenStore) consume(raw string) (string, bool) {
	h := sha256.Sum256([]byte(raw))
	key := base64.RawURLEncoding.EncodeToString(h[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[key]
	if !ok || time.Now().After(e.expires) {
		delete(s.items, key)
		return "", false
	}
	delete(s.items, key)
	return e.accountID, true
}

func (s *addCredTokenStore) gcLoop() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		s.mu.Lock()
		for k, e := range s.items {
			if now.After(e.expires) {
				delete(s.items, k)
			}
		}
		s.mu.Unlock()
	}
}
