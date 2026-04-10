package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL        string
	BindAddr           string
	CORSOrigin         string
	HMACKey            []byte
	RPID               string
	RPOrigin           string
	RPDisplayName      string
	Env                string
	RelayFlushInterval time.Duration
	ReaperInterval     time.Duration
	RegistrationOpen   bool

	// SMTP — all optional; if SMTPHost is empty, invitation emails are skipped.
	SMTPHost    string
	SMTPPort    string
	SMTPUser    string
	SMTPPass    string
	FromEmail   string
	AppDomain   string
}

func Load() (*Config, error) {
	// Load .env file if it exists (don't error if missing)
	_ = godotenv.Load()

	flushInterval := 60 * time.Second
	if v := os.Getenv("CONFIDE_RELAY_FLUSH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			flushInterval = d
		}
	}

	reaperInterval := 1 * time.Hour
	if v := os.Getenv("CONFIDE_REAPER_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			reaperInterval = d
		}
	}

	cfg := &Config{
		BindAddr:           getEnv("CONFIDE_BIND_ADDR", ":8080"),
		CORSOrigin:         getEnv("CONFIDE_CORS_ORIGIN", "http://localhost:3000"),
		RPID:               getEnv("CONFIDE_RP_ID", "localhost"),
		RPOrigin:           getEnv("CONFIDE_RP_ORIGIN", "http://localhost:3000"),
		RPDisplayName:      getEnv("CONFIDE_RP_DISPLAY_NAME", "Confide"),
		Env:                getEnv("CONFIDE_ENV", "development"),
		RelayFlushInterval: flushInterval,
		ReaperInterval:     reaperInterval,
		RegistrationOpen:   parseBool(os.Getenv("CONFIDE_REGISTRATION_OPEN"), true),
		SMTPHost:           os.Getenv("CONFIDE_SMTP_HOST"),
		SMTPPort:           getEnv("CONFIDE_SMTP_PORT", "587"),
		SMTPUser:           os.Getenv("CONFIDE_SMTP_USER"),
		SMTPPass:           os.Getenv("CONFIDE_SMTP_PASS"),
		FromEmail:          os.Getenv("CONFIDE_FROM_EMAIL"),
		AppDomain:          getEnv("CONFIDE_APP_DOMAIN", "http://localhost:3000"),
	}

	log.Println(cfg.RPOrigin)

	var errs []error

	dbURL := os.Getenv("CONFIDE_DATABASE_URL")
	if dbURL == "" {
		errs = append(errs, errors.New("CONFIDE_DATABASE_URL is required"))
	}
	cfg.DatabaseURL = dbURL

	hmacRaw := os.Getenv("CONFIDE_HMAC_KEY")
	if hmacRaw == "" {
		errs = append(errs, errors.New("CONFIDE_HMAC_KEY is required"))
	} else {
		key, err := base64.URLEncoding.DecodeString(hmacRaw)
		if err != nil || len(key) != 32 {
			errs = append(errs, fmt.Errorf("CONFIDE_HMAC_KEY must be base64url-encoded 32 bytes"))
		} else {
			cfg.HMACKey = key
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return cfg, nil
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
