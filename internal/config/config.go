package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL        string
	BindAddr           string
	CORSOrigins        []string
	HMACKey            []byte
	RPID               string
	RPOrigin           string
	RPDisplayName      string
	Env                string
	ReaperInterval time.Duration
	RegistrationOpen   bool

	// SMTP — all optional; if SMTPHost is empty, invitation emails are skipped.
	SMTPHost    string
	SMTPPort    string
	SMTPUser    string
	SMTPPass    string
	FromEmail   string
	AppDomain   string

	// ResendAPIKey — when set, emails are sent via the Resend REST API instead of SMTP.
	// This allows sending RFC 3156 PGP/MIME messages which Resend's SMTP relay rejects.
	ResendAPIKey string

	// Stripe — all optional; if StripeSecretKey is empty, billing endpoints return 503.
	StripeSecretKey     string
	StripeWebhookSecret string
	StripePriceIDPro    string
	StripePriceIDOrg    string // reserved for Organization tier

	// FormsDomain is the public hostname for form hosting (e.g. forms.example.com).
	// When set, form share URLs point here and requests from this host are restricted
	// to public form-serving routes only. Leave empty to serve forms from AppDomain.
	FormsDomain string

	// CustomDomainTarget is the CNAME hostname users must point their custom domain to.
	// Defaults to the forms subdomain (or AppDomain if FormsDomain is unset).
	CustomDomainTarget string

	// DomainVerifyInterval controls how often the background worker polls unverified
	// custom domains for CNAME and TXT record changes. Defaults to 2 minutes.
	DomainVerifyInterval time.Duration

	// TraefikConfigDir is the directory where per-domain Traefik config files are
	// written so Traefik can obtain Let's Encrypt certs for custom domains.
	// Leave empty to disable file writing (e.g. in local dev).
	TraefikConfigDir string
}

func Load() (*Config, error) {
	// Load .env file if it exists (don't error if missing)
	_ = godotenv.Load()

	reaperInterval := 1 * time.Hour
	if v := os.Getenv("CONFIDE_REAPER_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			reaperInterval = d
		}
	}

	appDomain := getEnv("CONFIDE_APP_DOMAIN", "http://localhost:3000")

	cfg := &Config{
		BindAddr:            ":" + getEnv("CONFIDE_PORT", "8080"),
		RPID:                getEnv("CONFIDE_RP_ID", "localhost"),
		RPOrigin:            getEnv("CONFIDE_RP_ORIGIN", appDomain),
		RPDisplayName:       getEnv("CONFIDE_RP_DISPLAY_NAME", "Confide"),
		Env:                 getEnv("CONFIDE_ENV", "development"),
		ReaperInterval:      reaperInterval,
		RegistrationOpen:    parseBool(os.Getenv("CONFIDE_REGISTRATION_OPEN"), true),
		SMTPHost:            os.Getenv("CONFIDE_SMTP_HOST"),
		SMTPPort:            getEnv("CONFIDE_SMTP_PORT", "587"),
		SMTPUser:            os.Getenv("CONFIDE_SMTP_USER"),
		SMTPPass:            os.Getenv("CONFIDE_SMTP_PASS"),
		FromEmail:           os.Getenv("CONFIDE_FROM_EMAIL"),
		ResendAPIKey:        os.Getenv("CONFIDE_RESEND_API_KEY"),
		AppDomain:           appDomain,
		StripeSecretKey:     os.Getenv("CONFIDE_STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("CONFIDE_STRIPE_WEBHOOK_SECRET"),
		StripePriceIDPro:    os.Getenv("CONFIDE_STRIPE_PRICE_PRO"),
		StripePriceIDOrg:    os.Getenv("CONFIDE_STRIPE_PRICE_ORG"),
	}

	// Strip scheme so FormsDomain is always a bare hostname internally,
	// while the env var is set with https:// like every other domain var.
	if fd := os.Getenv("CONFIDE_FORMS_DOMAIN"); fd != "" {
		cfg.FormsDomain = stripScheme(fd)
	}

	// Build CORS origins: explicit list > derived from AppDomain + FormsDomain
	if v := os.Getenv("CONFIDE_CORS_ORIGIN"); v != "" {
		cfg.CORSOrigins = strings.Split(v, ",")
	} else {
		cfg.CORSOrigins = []string{appDomain}
		if cfg.FormsDomain != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, "https://"+cfg.FormsDomain)
		}
	}

	if t := os.Getenv("CONFIDE_CUSTOM_DOMAIN_TARGET"); t != "" {
		cfg.CustomDomainTarget = t
	} else if cfg.FormsDomain != "" {
		cfg.CustomDomainTarget = cfg.FormsDomain
	} else {
		cfg.CustomDomainTarget = cfg.AppDomain
	}

	cfg.DomainVerifyInterval = 2 * time.Minute
	if v := os.Getenv("CONFIDE_DOMAIN_VERIFY_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.DomainVerifyInterval = d
		}
	}

	cfg.TraefikConfigDir = os.Getenv("CONFIDE_TRAEFIK_DYNAMIC_DIR")

	var errs []error

	dbURL := os.Getenv("CONFIDE_DATABASE_URL")
	if dbURL == "" {
		errs = append(errs, errors.New("CONFIDE_DATABASE_URL is required"))
	}
	cfg.DatabaseURL = dbURL

	hmacRaw := os.Getenv("CONFIDE_SECRET_KEY")
	if hmacRaw == "" {
		errs = append(errs, errors.New("CONFIDE_SECRET_KEY is required"))
	} else {
		key, err := base64.URLEncoding.DecodeString(hmacRaw)
		if err != nil || len(key) != 32 {
			errs = append(errs, fmt.Errorf("CONFIDE_SECRET_KEY must be base64url-encoded 32 bytes"))
		} else {
			cfg.HMACKey = key
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return cfg, nil
}

func stripScheme(s string) string {
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	return s
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parseBool parses "true"/"1"/"yes" as true, "false"/"0"/"no" as false.
// If the value is empty or unrecognized, defaultVal is returned.
func parseBool(s string, defaultVal bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return defaultVal
	}
}
