package crypto_test

import (
	"strings"
	"testing"

	"wlpr-portal/pkg/crypto"
)

func TestHashPassword_ProducesValidHash(t *testing.T) {
	hash, err := crypto.HashPassword("mysecretpassword")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if !strings.HasPrefix(hash, "$2a$") {
		t.Errorf("expected bcrypt hash starting with $2a$, got %q", hash)
	}
}

func TestHashPassword_DifferentHashesPerCall(t *testing.T) {
	password := "samepassword"
	hash1, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("first HashPassword call returned error: %v", err)
	}
	hash2, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("second HashPassword call returned error: %v", err)
	}
	if hash1 == hash2 {
		t.Error("expected different hashes for the same password due to bcrypt salt, but got identical hashes")
	}
}

func TestCheckPassword_CorrectPassword(t *testing.T) {
	password := "correcthorsebatterystaple"
	hash, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if !crypto.CheckPassword(password, hash) {
		t.Error("CheckPassword returned false for correct password")
	}
}

func TestCheckPassword_WrongPassword(t *testing.T) {
	hash, err := crypto.HashPassword("realpassword")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if crypto.CheckPassword("wrongpassword", hash) {
		t.Error("CheckPassword returned true for wrong password")
	}
}

func TestCheckPassword_EmptyPassword(t *testing.T) {
	hash, err := crypto.HashPassword("nonemptypassword")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if crypto.CheckPassword("", hash) {
		t.Error("CheckPassword returned true for empty password")
	}
}

func TestHashPassword_LongPassword(t *testing.T) {
	// bcrypt rejects passwords > 72 bytes
	longPassword := strings.Repeat("a", 100)
	_, err := crypto.HashPassword(longPassword)
	if err == nil {
		t.Fatal("expected error for password > 72 bytes, but got nil")
	}

	// Exactly 72 bytes should work
	exactPassword := strings.Repeat("b", 72)
	hash, err := crypto.HashPassword(exactPassword)
	if err != nil {
		t.Fatalf("72-byte password should succeed: %v", err)
	}
	if !crypto.CheckPassword(exactPassword, hash) {
		t.Error("CheckPassword failed for 72-byte password")
	}
}
