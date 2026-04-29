package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const traefikServiceName = "confide"

// writeTraefikConfig writes a per-domain Traefik dynamic config file so
// Traefik can obtain a Let's Encrypt cert for the domain via HTTP challenge.
func writeTraefikConfig(dir, domain string) error {
	content := fmt.Sprintf(`# This file is managed automatically by Confide. Do not edit manually.
http:
  routers:
    domain-%s:
      rule: "Host(` + "`%s`" + `)"
      entryPoints: [websecure]
      service: %s
      tls:
        certResolver: letsencrypt
      priority: 100
`, domainToSlug(domain), domain, traefikServiceName)

	subdir := filepath.Join(dir, "confide")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		return err
	}

	tmp := filepath.Join(subdir, ".domain-"+domainToSlug(domain)+".yml.tmp")
	dst := filepath.Join(subdir, "domain-"+domainToSlug(domain)+".yml")

	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// DeleteTraefikConfig removes the per-domain Traefik config file for the given
// domain. Safe to call when the file does not exist.
func DeleteTraefikConfig(dir, domain string) error {
	path := filepath.Join(dir, "confide", "domain-"+domainToSlug(domain)+".yml")
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func domainToSlug(domain string) string {
	return strings.NewReplacer(".", "-", "*", "wildcard").Replace(domain)
}
