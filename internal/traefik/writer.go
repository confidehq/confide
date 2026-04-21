package traefik

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Writer maintains the Traefik dynamic config file for verified custom domains.
// It is a no-op when dir is empty (e.g. local dev without Traefik).
type Writer struct {
	dir     string
	mu      sync.Mutex
	domains map[string]struct{}
}

// New creates a Writer rooted at dir and writes an initial config for the
// given verified domains. Pass an empty dir to disable file writes entirely.
func New(dir string, initialDomains []string) (*Writer, error) {
	domains := make(map[string]struct{}, len(initialDomains))
	for _, d := range initialDomains {
		if d != "" {
			domains[d] = struct{}{}
		}
	}
	w := &Writer{dir: dir, domains: domains}
	return w, w.write()
}

// Add marks a domain as verified and rewrites the config file.
func (w *Writer) Add(domain string) error {
	if domain == "" {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.domains[domain] = struct{}{}
	return w.write()
}

// Contains reports whether domain is currently in the verified set.
func (w *Writer) Contains(domain string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, ok := w.domains[domain]
	return ok
}

// Remove drops a domain and rewrites the config file.
func (w *Writer) Remove(domain string) error {
	if domain == "" {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.domains, domain)
	return w.write()
}

func (w *Writer) write() error {
	if w.dir == "" {
		return nil
	}
	path := filepath.Join(w.dir, "confide-custom-domains.yml")
	return os.WriteFile(path, []byte(w.buildYAML()), 0644)
}

func (w *Writer) buildYAML() string {
	if len(w.domains) == 0 {
		return "# confide custom domains — managed automatically\n{}\n"
	}
	var sb strings.Builder
	sb.WriteString("# confide custom domains — managed automatically\nhttp:\n  routers:\n")
	for domain := range w.domains {
		fmt.Fprintf(&sb,
			"    %s:\n      rule: \"Host(`%s`)\"\n      entryPoints: [websecure]\n      service: confide\n      tls:\n        certResolver: letsencrypt\n      priority: 10\n",
			routerKey(domain), domain,
		)
	}
	return sb.String()
}

func routerKey(domain string) string {
	return "confide-" + strings.NewReplacer(".", "-", ":", "-").Replace(domain)
}
