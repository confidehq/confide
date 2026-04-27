package domain

import (
	"context"
	"net"
	"strings"
)

// Verifier checks DNS records for a custom domain. It uses the system resolver
// and has no external dependencies.
type Verifier struct {
	// CNAMETarget is the hostname users must CNAME their domain to,
	// e.g. "forms.confide.com".
	CNAMETarget string
}

// CheckCNAME returns true if domain has a CNAME record that resolves to CNAMETarget.
func (v *Verifier) CheckCNAME(ctx context.Context, domain string) (bool, error) {
	cname, err := net.DefaultResolver.LookupCNAME(ctx, domain)
	if err != nil {
		return false, err
	}
	// LookupCNAME always returns a fully-qualified name ending with ".".
	target := strings.TrimSuffix(v.CNAMETarget, ".") + "."
	return strings.EqualFold(cname, target), nil
}

// CheckTXT returns true if "_confide-verify.<domain>" has a TXT record containing
// "confide-verification=<token>".
func (v *Verifier) CheckTXT(ctx context.Context, domain, token string) (bool, error) {
	records, err := net.DefaultResolver.LookupTXT(ctx, "_confide-verify."+domain)
	if err != nil {
		// NXDOMAIN / no records — not an error from the caller's perspective.
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			return false, nil
		}
		return false, err
	}
	want := "confide-verification=" + token
	for _, rec := range records {
		if rec == want {
			return true, nil
		}
	}
	return false, nil
}
