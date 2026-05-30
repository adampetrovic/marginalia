package config

import (
	"os"
	"strconv"
)

// Config holds all application configuration, sourced from environment variables.
type Config struct {
	// DatabaseURL is a SQLite file path or postgres:// connection string.
	// Default: ./marginalia.db
	DatabaseURL string

	// Port is the HTTP listen port. Default: 8080
	Port int

	// SessionSecret signs web-UI session cookies. If unset, a random secret is
	// generated at startup (sessions then reset on every restart).
	SessionSecret string

	// DisableRegistration turns off public sign-up; accounts must be created by
	// an existing admin.
	DisableRegistration bool

	// Bootstrap admin account, created on first run when no users exist.
	AdminEmail    string
	AdminPassword string

	// APIToken is a legacy single shared bearer token. When set on first run it
	// is migrated into a named API token belonging to the bootstrap admin so
	// existing KOReader/Readest devices keep working. Optional.
	APIToken string

	// Readeck source configuration. Used to seed the bootstrap admin's Readeck
	// integration; per-user config is stored in the database thereafter.
	ReadeckURL   string
	ReadeckToken string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	c := &Config{
		DatabaseURL:         envOrDefault("DATABASE_URL", "./marginalia.db"),
		Port:                envIntOrDefault("MARGINALIA_PORT", 8080),
		SessionSecret:       os.Getenv("MARGINALIA_SESSION_SECRET"),
		DisableRegistration: os.Getenv("MARGINALIA_DISABLE_REGISTRATION") != "",
		AdminEmail:          os.Getenv("MARGINALIA_ADMIN_EMAIL"),
		AdminPassword:       os.Getenv("MARGINALIA_ADMIN_PASSWORD"),
		APIToken:            os.Getenv("MARGINALIA_API_TOKEN"),
		ReadeckURL:          os.Getenv("MARGINALIA_READECK_URL"),
		ReadeckToken:        os.Getenv("MARGINALIA_READECK_TOKEN"),
	}

	return c, nil
}

// IsReadeckConfigured returns true if Readeck source credentials are set.
func (c *Config) IsReadeckConfigured() bool {
	return c.ReadeckURL != "" && c.ReadeckToken != ""
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
