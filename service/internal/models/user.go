package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// User is an account that owns sources, documents, highlights, templates and
// API tokens. All user-facing data is scoped to a User via UserID.
type User struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	Email        string    `gorm:"not null;uniqueIndex" json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `gorm:"not null" json:"-"`
	IsAdmin      bool      `gorm:"not null;default:false" json:"is_admin"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// APIToken is a per-user bearer token used by machine clients (KOReader,
// Readest, Readeck, the Logseq plugin). Only the SHA-256 hash of the token is
// stored; the plaintext value is shown to the user exactly once at creation.
type APIToken struct {
	ID         string     `gorm:"primaryKey" json:"id"`
	UserID     string     `gorm:"not null;index" json:"user_id"`
	Name       string     `gorm:"not null" json:"name"`
	TokenHash  string     `gorm:"not null;uniqueIndex" json:"-"`
	Prefix     string     `json:"prefix"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

// NewID returns a random, URL-safe identifier with the given prefix, e.g.
// NewID("user") -> "user-3f9a1c...". Used for primary keys that are not
// derived from an upstream source.
func NewID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// rand.Read never fails on supported platforms; fall back to time-based.
		return prefix + "-" + hex.EncodeToString([]byte(time.Now().UTC().String()))
	}
	return prefix + "-" + hex.EncodeToString(b)
}
