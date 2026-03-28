package config

import (
	"fmt"
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

	// APIToken is the bearer token required for API authentication.
	APIToken string

	// Readeck source configuration.
	ReadeckURL   string
	ReadeckToken string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	c := &Config{
		DatabaseURL: envOrDefault("DATABASE_URL", "./marginalia.db"),
		Port:        envIntOrDefault("MARGINALIA_PORT", 8080),
		APIToken:    os.Getenv("MARGINALIA_API_TOKEN"),
		ReadeckURL:  os.Getenv("MARGINALIA_READECK_URL"),
		ReadeckToken: os.Getenv("MARGINALIA_READECK_TOKEN"),
	}

	if c.APIToken == "" {
		return nil, fmt.Errorf("MARGINALIA_API_TOKEN is required")
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
