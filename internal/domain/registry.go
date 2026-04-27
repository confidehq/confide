package domain

import "sync"

// Registry is an in-memory set of enabled custom domains. It is safe for
// concurrent use and requires no database or file I/O at query time.
type Registry struct {
	mu      sync.RWMutex
	domains map[string]struct{}
}

func NewRegistry(initial []string) *Registry {
	m := make(map[string]struct{}, len(initial))
	for _, d := range initial {
		if d != "" {
			m[d] = struct{}{}
		}
	}
	return &Registry{domains: m}
}

func (r *Registry) Enable(d string) {
	r.mu.Lock()
	r.domains[d] = struct{}{}
	r.mu.Unlock()
}

func (r *Registry) Disable(d string) {
	r.mu.Lock()
	delete(r.domains, d)
	r.mu.Unlock()
}

func (r *Registry) IsEnabled(d string) bool {
	r.mu.RLock()
	_, ok := r.domains[d]
	r.mu.RUnlock()
	return ok
}
