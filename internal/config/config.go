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
	CORSOrigin         string
	HMACKey            []byte
	RPID               string
	RPOrigin           string
	RPDisplayName      string
	Env                string
	RelayFlushInterval time.Duration
	RegistrationOpen   bool
}

func Load() (*Config, error) {
	// Load .env file if it exists (don't error if missing)
	_ = godotenv.Load()

	flushInterval := 60 * time.Second
	if v := os.Getenv("WISP_RELAY_FLUSH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			flushInterval = d
		}
	}

	cfg := &Config{
		BindAddr:           getEnv("WISP_BIND_ADDR", ":8080"),
		CORSOrigin:         getEnv("WISP_CORS_ORIGIN", "http://localhost:3000"),
		RPID:               getEnv("WISP_RP_ID", "localhost"),
		RPOrigin:           getEnv("WISP_RP_ORIGIN", "http://localhost:3000"),
		RPDisplayName:      getEnv("WISP_RP_DISPLAY_NAME", "Ghostform"),
		Env:                getEnv("WISP_ENV", "development"),
		RelayFlushInterval: flushInterval,
		RegistrationOpen:   parseBool(os.Getenv("WISP_REGISTRATION_OPEN"), true),
	}

	var errs []error

	dbURL := os.Getenv("WISP_DATABASE_URL")
	if dbURL == "" {
		errs = append(errs, errors.New("WISP_DATABASE_URL is required"))
	}
	cfg.DatabaseURL = dbURL

	hmacRaw := os.Getenv("WISP_HMAC_KEY")
	if hmacRaw == "" {
		errs = append(errs, errors.New("WISP_HMAC_KEY is required"))
	} else {
		key, err := base64.URLEncoding.DecodeString(hmacRaw)
		if err != nil || len(key) != 32 {
			errs = append(errs, fmt.Errorf("WISP_HMAC_KEY must be base64url-encoded 32 bytes"))
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
