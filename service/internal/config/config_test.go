package config

import (
	"testing"
)

func TestLoad_NoTokenRequired(t *testing.T) {
	clearEnv(t)
	// Multi-user auth means the legacy shared token is optional; Load must
	// succeed without it.
	if _, err := Load(); err != nil {
		t.Fatalf("expected no error without MARGINALIA_API_TOKEN, got %v", err)
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("MARGINALIA_API_TOKEN", "test-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DatabaseURL != "./marginalia.db" {
		t.Errorf("expected default DatabaseURL, got %q", cfg.DatabaseURL)
	}
	if cfg.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.APIToken != "test-token" {
		t.Errorf("expected APIToken 'test-token', got %q", cfg.APIToken)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	clearEnv(t)
	t.Setenv("MARGINALIA_API_TOKEN", "my-token")
	t.Setenv("DATABASE_URL", "postgres://localhost/marginalia")
	t.Setenv("MARGINALIA_PORT", "9090")
	t.Setenv("MARGINALIA_READECK_URL", "https://readeck.example.com")
	t.Setenv("MARGINALIA_READECK_TOKEN", "rdk-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://localhost/marginalia" {
		t.Errorf("unexpected DatabaseURL: %q", cfg.DatabaseURL)
	}
	if cfg.Port != 9090 {
		t.Errorf("unexpected Port: %d", cfg.Port)
	}
	if !cfg.IsReadeckConfigured() {
		t.Error("expected Readeck to be configured")
	}
}

func TestIsReadeckConfigured(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		token    string
		expected bool
	}{
		{"both set", "https://readeck.example.com", "token", true},
		{"missing url", "", "token", false},
		{"missing token", "https://readeck.example.com", "", false},
		{"both empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{ReadeckURL: tt.url, ReadeckToken: tt.token}
			if got := c.IsReadeckConfigured(); got != tt.expected {
				t.Errorf("IsReadeckConfigured() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DATABASE_URL", "MARGINALIA_API_TOKEN", "MARGINALIA_PORT",
		"MARGINALIA_READECK_URL", "MARGINALIA_READECK_TOKEN",
	} {
		t.Setenv(key, "")
	}
}
