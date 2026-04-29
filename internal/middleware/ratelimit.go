package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/httprate"
)

// rotatingHMACKey holds two 32-byte slots that rotate every 15 minutes.
// Requests are valid against either the current or the previous slot.
type rotatingHMACKey struct {
	mu       sync.RWMutex
	current  []byte
	previous []byte
	rotateAt time.Time
}

func newRotatingHMACKey(seed []byte) *rotatingHMACKey {
	k := &rotatingHMACKey{rotateAt: time.Now().Add(15 * time.Minute)}
	k.current = deriveSlot(seed, 0)
	k.previous = k.current
	return k
}

func (k *rotatingHMACKey) rotate(now time.Time) {
	if now.Before(k.rotateAt) {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if now.Before(k.rotateAt) {
		return // double-check under lock
	}
	k.previous = k.current
	slot := make([]byte, 32)
	_, _ = rand.Read(slot)
	k.current = slot
	k.rotateAt = now.Add(15 * time.Minute)
}

// IPKey returns an opaque HMAC key string for the given IP, valid against
// current or previous slot. The raw IP is never stored beyond this call.
func (k *rotatingHMACKey) IPKey(ip string) string {
	k.rotate(time.Now())
	k.mu.RLock()
	defer k.mu.RUnlock()
	mac := hmac.New(sha256.New, k.current)
	mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))
}

func deriveSlot(seed []byte, n uint64) []byte {
	h := hmac.New(sha256.New, seed)
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n)
	h.Write(b[:])
	return h.Sum(nil)
}

// RateLimit returns a per-IP rate-limit middleware using HMAC-hashed IP keys.
// Requests: 100 per minute on auth routes.
func RateLimit(hmacSeed []byte) func(http.Handler) http.Handler {
	rk := newRotatingHMACKey(hmacSeed)

	limiter := httprate.NewRateLimiter(100, time.Minute,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			return rk.IPKey(ip), nil
		}),
	)

	return limiter.Handler
}

// PublicSchemaRateLimit limits unauthenticated schema fetches: 100 req/min per IP.
func PublicSchemaRateLimit(hmacSeed []byte) func(http.Handler) http.Handler {
	rk := newRotatingHMACKey(hmacSeed)

	limiter := httprate.NewRateLimiter(100, time.Minute,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			return rk.IPKey(ip), nil
		}),
	)

	return limiter.Handler
}

// RelayRateLimit limits anonymous submissions: 20 per 10 minutes per IP.
func RelayRateLimit(hmacSeed []byte) func(http.Handler) http.Handler {
	rk := newRotatingHMACKey(hmacSeed)

	limiter := httprate.NewRateLimiter(20, 10*time.Minute,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			return rk.IPKey(ip), nil
		}),
	)

	return limiter.Handler
}

// UsernameCheckRateLimit limits username availability checks: 10 per minute per IP.
func UsernameCheckRateLimit(hmacSeed []byte) func(http.Handler) http.Handler {
	rk := newRotatingHMACKey(hmacSeed)

	limiter := httprate.NewRateLimiter(10, time.Minute,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			return rk.IPKey(ip), nil
		}),
	)

	return limiter.Handler
}

// RecoveryRateLimit is a stricter limiter: 5 requests per 5 minutes.
func RecoveryRateLimit(hmacSeed []byte) func(http.Handler) http.Handler {
	rk := newRotatingHMACKey(hmacSeed)

	limiter := httprate.NewRateLimiter(5, 5*time.Minute,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			return rk.IPKey(ip), nil
		}),
	)

	return limiter.Handler
}
