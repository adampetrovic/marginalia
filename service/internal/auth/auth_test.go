package auth

import (
	"testing"
	"time"
)

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hashing failed: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password stored in plaintext")
	}
	if !CheckPassword(hash, "correct horse battery staple") {
		t.Error("correct password rejected")
	}
	if CheckPassword(hash, "wrong password") {
		t.Error("wrong password accepted")
	}
}

func TestAPITokenGenerationAndHashing(t *testing.T) {
	plaintext, hash, prefix, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("generating token: %v", err)
	}
	if plaintext == "" || hash == "" || prefix == "" {
		t.Fatal("empty token fields")
	}
	if hash == plaintext {
		t.Fatal("token stored without hashing")
	}
	if HashAPIToken(plaintext) != hash {
		t.Error("hash is not reproducible from plaintext")
	}
	if HashAPIToken("different") == hash {
		t.Error("distinct tokens collide")
	}
}

func TestSessionSignAndParse(t *testing.T) {
	secret := []byte("super-secret-key")
	now := time.Now()

	token := SignSession(secret, "user-123", now)
	uid, err := ParseSession(secret, token, now)
	if err != nil {
		t.Fatalf("parsing valid session: %v", err)
	}
	if uid != "user-123" {
		t.Errorf("expected user-123, got %q", uid)
	}
}

func TestSessionRejectsTampering(t *testing.T) {
	secret := []byte("super-secret-key")
	now := time.Now()
	token := SignSession(secret, "user-123", now)

	// Wrong secret.
	if _, err := ParseSession([]byte("other-key"), token, now); err == nil {
		t.Error("accepted session signed with a different secret")
	}

	// Tampered payload (different user, original signature).
	if _, err := ParseSession(secret, "user-999."+token[len("user-123."):], now); err == nil {
		t.Error("accepted session with tampered user id")
	}

	// Malformed token.
	if _, err := ParseSession(secret, "garbage", now); err == nil {
		t.Error("accepted malformed session")
	}
}

func TestSessionExpiry(t *testing.T) {
	secret := []byte("super-secret-key")
	issued := time.Now()
	token := SignSession(secret, "user-123", issued)

	future := issued.Add(SessionTTL + time.Minute)
	if _, err := ParseSession(secret, token, future); err == nil {
		t.Error("accepted an expired session")
	}
}
