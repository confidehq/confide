package permission

import (
	"sync"
	"time"
)

const cacheTTL = 60 * time.Second
const purgeInterval = 5 * time.Minute

type cacheKey struct {
	WorkspaceID string
	AccountID   string
}

type cacheEntry struct {
	role      string
	expiresAt time.Time
}

// RoleCache is a short-lived in-memory cache for workspace role lookups.
// A single instance should be shared across all middleware registrations.
type RoleCache struct {
	mu      sync.RWMutex
	entries map[cacheKey]cacheEntry
	done    chan struct{}
}

// NewRoleCache creates a RoleCache and starts a background goroutine to purge
// expired entries every 5 minutes.
func NewRoleCache() *RoleCache {
	c := &RoleCache{
		entries: make(map[cacheKey]cacheEntry),
		done:    make(chan struct{}),
	}
	go c.purgeLoop()
	return c
}

// Get returns the cached role for the given workspace + account pair.
func (c *RoleCache) Get(workspaceID, accountID string) (string, bool) {
	c.mu.RLock()
	entry, ok := c.entries[cacheKey{workspaceID, accountID}]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.role, true
}

// Set stores the role with a 60-second TTL.
func (c *RoleCache) Set(workspaceID, accountID, role string) {
	c.mu.Lock()
	c.entries[cacheKey{workspaceID, accountID}] = cacheEntry{
		role:      role,
		expiresAt: time.Now().Add(cacheTTL),
	}
	c.mu.Unlock()
}

// Invalidate removes the cached entry for a workspace + account pair.
// Call this after a role change or member removal.
func (c *RoleCache) Invalidate(workspaceID, accountID string) {
	c.mu.Lock()
	delete(c.entries, cacheKey{workspaceID, accountID})
	c.mu.Unlock()
}

// Stop terminates the background purge goroutine. Used in tests.
func (c *RoleCache) Stop() {
	close(c.done)
}

func (c *RoleCache) purgeLoop() {
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.purgeExpired()
		case <-c.done:
			return
		}
	}
}

func (c *RoleCache) purgeExpired() {
	now := time.Now()
	c.mu.Lock()
	for k, v := range c.entries {
		if now.After(v.expiresAt) {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}
