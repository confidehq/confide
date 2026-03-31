package botguard

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

const (
	maxTokenAge   = 24 * time.Hour
	minElapsed    = 2 * time.Second
	tokenByteLen  = 8 + 32 // timestamp (8) + HMAC (32)
	honeypotCount = 2
	honeypotSlice = 5 // bytes per name → 8 base32 chars
)

// Guard performs stateless bot detection using HMAC-derived honeypot field
// names and signed timing tokens. All operations are derived from the same
// HMAC key used elsewhere in the application.
type Guard struct {
	key []byte
}

// New creates a Guard backed by the given HMAC key.
func New(key []byte) *Guard {
	return &Guard{key: key}
}

// HoneypotNames returns the two honeypot field names for the given form and
// daily time window. windowDay is unix seconds / 86400.
func (g *Guard) honeypotNames(formID string, windowDay int64) []string {
	mac := hmac.New(sha256.New, g.key)
	fmt.Fprintf(mac, "confide.hp.v1:%s:%d", formID, windowDay) //nolint:errcheck
	digest := mac.Sum(nil)

	names := make([]string, honeypotCount)
	for i := range honeypotCount {
		chunk := digest[i*honeypotSlice : (i+1)*honeypotSlice]
		names[i] = strings.ToLower(base32.StdEncoding.EncodeToString(chunk))
	}
	return names
}

// HoneypotNames returns the current day's honeypot field names for a form.
func (g *Guard) HoneypotNames(formID string) []string {
	day := time.Now().UTC().Unix() / 86400
	return g.honeypotNames(formID, day)
}

// IsHoneypotTriggered reports whether any honeypot field (for today or
// yesterday, to handle midnight boundary) is present and non-empty.
func (g *Guard) IsHoneypotTriggered(formID string, fields map[string]string) bool {
	if len(fields) == 0 {
		return false
	}
	now := time.Now().UTC().Unix() / 86400
	for _, day := range []int64{now, now - 1} {
		for _, name := range g.honeypotNames(formID, day) {
			if v, ok := fields[name]; ok && v != "" {
				return true
			}
		}
	}
	return false
}

// IssueToken returns a base64-encoded token containing the current unix
// timestamp and an HMAC that binds it to the given form ID.
func (g *Guard) IssueToken(formID string) string {
	now := time.Now().Unix()
	mac := hmac.New(sha256.New, g.key)
	fmt.Fprintf(mac, "confide.timing.v1:%s:%d", formID, now) //nolint:errcheck
	sig := mac.Sum(nil)

	buf := make([]byte, tokenByteLen)
	binary.BigEndian.PutUint64(buf[:8], uint64(now))
	copy(buf[8:], sig)
	return base64.StdEncoding.EncodeToString(buf)
}

// VelocityTooFast returns true if the token was issued fewer than 2 seconds
// before now, indicating an automated submission. Returns false (pass through)
// if the token is absent, malformed, has an invalid signature, or is expired
// (> 24 h old) — this keeps old clients working during rollout.
func (g *Guard) VelocityTooFast(formID, token string) bool {
	if token == "" {
		return false
	}
	buf, err := base64.StdEncoding.DecodeString(token)
	if err != nil || len(buf) != tokenByteLen {
		return false
	}

	issuedAt := int64(binary.BigEndian.Uint64(buf[:8]))
	sig := buf[8:]

	// Verify signature.
	mac := hmac.New(sha256.New, g.key)
	fmt.Fprintf(mac, "confide.timing.v1:%s:%d", formID, issuedAt) //nolint:errcheck
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return false
	}

	now := time.Now().Unix()
	age := time.Duration(now-issuedAt) * time.Second

	// Expired token — pass through rather than block.
	if age > maxTokenAge {
		return false
	}

	return age < minElapsed
}
