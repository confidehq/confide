package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// shortCodeAlphabet excludes 0, 1, I, O to reduce transcription errors.
const shortCodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
const shortCodeLen = 8

const maxPairingRequestAttempts = 5
const maxPairingCompleteAttempts = 5

var (
	ErrConflict        = fmt.Errorf("pairing already claimed by another device")
	ErrTooManyAttempts = fmt.Errorf("too many attempts")
)

type pairingState string

const (
	pairingCreated   pairingState = "created"
	pairingRequested pairingState = "requested"
	pairingFulfilled pairingState = "fulfilled"
	pairingCompleted pairingState = "completed"
)

type pairingSession struct {
	token            string
	shortCode        string
	accountID        string
	state            pairingState
	newDevicePubKey  []byte
	wrappedMasterKey []byte
	requestAttempts  int
	completeAttempts int
	expiresAt        time.Time
}

type pairingStore struct {
	mu      sync.Mutex
	byToken map[string]*pairingSession
	byCode  map[string]*pairingSession
}

func newPairingStore() *pairingStore {
	s := &pairingStore{
		byToken: make(map[string]*pairingSession),
		byCode:  make(map[string]*pairingSession),
	}
	go s.gcLoop()
	return s
}

func (s *pairingStore) create(accountID string) (*pairingSession, error) {
	tokenBytes, err := randomBytes(32)
	if err != nil {
		return nil, fmt.Errorf("generate pairing token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	code, err := generateShortCode()
	if err != nil {
		return nil, fmt.Errorf("generate short code: %w", err)
	}

	sess := &pairingSession{
		token:     token,
		shortCode: code,
		accountID: accountID,
		state:     pairingCreated,
		expiresAt: time.Now().Add(5 * time.Minute),
	}

	s.mu.Lock()
	s.byToken[token] = sess
	s.byCode[code] = sess
	s.mu.Unlock()
	return sess, nil
}

// request transitions created → requested and returns the session's accountID.
// Returns ErrConflict if already claimed, ErrTooManyAttempts if the cap is exceeded.
func (s *pairingStore) request(token string, newDevicePubKey []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.byToken[token]
	if !ok || time.Now().After(sess.expiresAt) {
		return "", ErrNotFound
	}

	sess.requestAttempts++
	if sess.requestAttempts > maxPairingRequestAttempts {
		delete(s.byToken, sess.token)
		delete(s.byCode, sess.shortCode)
		return "", ErrTooManyAttempts
	}

	if sess.state != pairingCreated {
		return "", ErrConflict
	}

	sess.state = pairingRequested
	sess.newDevicePubKey = newDevicePubKey
	return sess.accountID, nil
}

func (s *pairingStore) fulfill(token, accountID string, wrappedMasterKey []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.byToken[token]
	if !ok || time.Now().After(sess.expiresAt) {
		return ErrNotFound
	}
	if sess.accountID != accountID {
		return ErrNotFound
	}
	if sess.state != pairingRequested {
		return ErrConflict
	}

	sess.state = pairingFulfilled
	sess.wrappedMasterKey = wrappedMasterKey
	return nil
}

// PairingPollResult is the response shape for GET /pairing/{token}.
type PairingPollResult struct {
	State            string
	NewDevicePubKey  []byte // non-nil when state = requested or fulfilled
	WrappedMasterKey []byte // non-nil when state = fulfilled
}

func (s *pairingStore) poll(token string) (*PairingPollResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.byToken[token]
	if !ok {
		return nil, false
	}
	if time.Now().After(sess.expiresAt) {
		return &PairingPollResult{State: "expired"}, true
	}

	res := &PairingPollResult{State: string(sess.state)}
	if sess.state == pairingRequested || sess.state == pairingFulfilled {
		res.NewDevicePubKey = sess.newDevicePubKey
	}
	if sess.state == pairingFulfilled {
		res.WrappedMasterKey = sess.wrappedMasterKey
	}
	return res, true
}

// complete transitions fulfilled → completed, removes the session, and returns it.
func (s *pairingStore) complete(token string) (*pairingSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.byToken[token]
	if !ok || time.Now().After(sess.expiresAt) {
		return nil, ErrNotFound
	}

	sess.completeAttempts++
	if sess.completeAttempts > maxPairingCompleteAttempts {
		delete(s.byToken, sess.token)
		delete(s.byCode, sess.shortCode)
		return nil, ErrTooManyAttempts
	}

	if sess.state != pairingFulfilled {
		return nil, ErrConflict
	}

	sess.state = pairingCompleted
	delete(s.byToken, sess.token)
	delete(s.byCode, sess.shortCode)
	return sess, nil
}

func (s *pairingStore) getByCode(code string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.byCode[code]
	if !ok || time.Now().After(sess.expiresAt) {
		return "", false
	}
	return sess.token, true
}

func (s *pairingStore) gcLoop() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		s.mu.Lock()
		for token, sess := range s.byToken {
			if now.After(sess.expiresAt) {
				delete(s.byToken, token)
				delete(s.byCode, sess.shortCode)
			}
		}
		s.mu.Unlock()
	}
}

func generateShortCode() (string, error) {
	b := make([]byte, shortCodeLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := make([]byte, shortCodeLen)
	for i, byt := range b {
		code[i] = shortCodeAlphabet[int(byt)%len(shortCodeAlphabet)]
	}
	return string(code), nil
}
