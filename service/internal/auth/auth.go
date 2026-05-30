// Package auth provides password hashing, API-token generation, and stateless
// signed session tokens for Marginalia's multi-user authentication.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// SessionTTL is how long an issued session cookie remains valid.
const SessionTTL = 30 * 24 * time.Hour

// ErrInvalidSession is returned when a session token is malformed, tampered
// with, or expired.
var ErrInvalidSession = errors.New("invalid session")

// HashPassword returns a bcrypt hash of the given plaintext password.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword reports whether password matches the stored bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateAPIToken returns a new random API token. It returns the plaintext
// token (shown to the user once), its SHA-256 hash (stored in the DB), and a
// short prefix used for display in the UI.
func GenerateAPIToken() (plaintext, hash, prefix string, err error) {
	b := make([]byte, 24)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", err
	}
	plaintext = "mrg_" + base64.RawURLEncoding.EncodeToString(b)
	hash = HashAPIToken(plaintext)
	prefix = plaintext[:12]
	return plaintext, hash, prefix, nil
}

// HashAPIToken returns the hex-encoded SHA-256 hash of a token. Lookups are
// done by hash so plaintext tokens are never persisted.
func HashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SignSession produces a stateless session token of the form
// "<userID>.<expiryUnix>.<hmac>" signed with secret. It carries no server-side
// state, so revoking an individual session means rotating the secret.
func SignSession(secret []byte, userID string, now time.Time) string {
	exp := now.Add(SessionTTL).Unix()
	payload := userID + "." + strconv.FormatInt(exp, 10)
	return payload + "." + sign(secret, payload)
}

// ParseSession validates a session token against secret and returns the userID
// if the signature is valid and the token has not expired.
func ParseSession(secret []byte, token string, now time.Time) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", ErrInvalidSession
	}
	userID, expStr, gotSig := parts[0], parts[1], parts[2]
	payload := userID + "." + expStr

	wantSig := sign(secret, payload)
	if subtle.ConstantTimeCompare([]byte(gotSig), []byte(wantSig)) != 1 {
		return "", ErrInvalidSession
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return "", ErrInvalidSession
	}
	if now.Unix() > exp {
		return "", ErrInvalidSession
	}
	if userID == "" {
		return "", ErrInvalidSession
	}
	return userID, nil
}

func sign(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// RandomSecret returns a hex-encoded cryptographically random secret, used as a
// fallback session-signing key when none is configured.
func RandomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}
