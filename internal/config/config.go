package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
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
}

func Load() (*Config, error) {
	// Load .env file if it exists (don't error if missing)
	_ = godotenv.Load()

	flushInterval := 60 * time.Second
	if v := os.Getenv("GHOSTFORM_RELAY_FLUSH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			flushInterval = d
		}
	}

	cfg := &Config{
		BindAddr:           getEnv("GHOSTFORM_BIND_ADDR", ":8080"),
		CORSOrigin:         getEnv("GHOSTFORM_CORS_ORIGIN", "http://localhost:3000"),
		RPID:               getEnv("GHOSTFORM_RP_ID", "localhost"),
		RPOrigin:           getEnv("GHOSTFORM_RP_ORIGIN", "http://localhost:3000"),
		RPDisplayName:      getEnv("GHOSTFORM_RP_DISPLAY_NAME", "GhostForm"),
		Env:                getEnv("GHOSTFORM_ENV", "development"),
		RelayFlushInterval: flushInterval,
	}

	var errs []error

	dbURL := os.Getenv("GHOSTFORM_DATABASE_URL")
	if dbURL == "" {
		errs = append(errs, errors.New("GHOSTFORM_DATABASE_URL is required"))
	}
	cfg.DatabaseURL = dbURL

	hmacRaw := os.Getenv("GHOSTFORM_HMAC_KEY")
	if hmacRaw == "" {
		errs = append(errs, errors.New("GHOSTFORM_HMAC_KEY is required"))
	} else {
		key, err := base64.URLEncoding.DecodeString(hmacRaw)
		if err != nil || len(key) != 32 {
			errs = append(errs, fmt.Errorf("GHOSTFORM_HMAC_KEY must be base64url-encoded 32 bytes"))
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
