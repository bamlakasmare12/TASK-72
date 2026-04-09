package crypto

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// setupTestKey initializes the encryption key for tests using a deterministic 32-byte key.
func setupTestKey(t *testing.T) {
	t.Helper()
	// 64 hex chars = 32 bytes for AES-256
	key, err := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to decode test key: %v", err)
	}
	encryptionKey = key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	setupTestKey(t)

	plaintext := []byte("Hello, World! This is a test message.")
	ciphertext, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Fatal("ciphertext must differ from plaintext")
	}

	decrypted, err := Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptDecrypt_TOTPSecret(t *testing.T) {
	setupTestKey(t)

	// Simulate a real TOTP secret (base32-encoded, typical length)
	totpSecret := "JBSWY3DPEHPK3PXP"

	ciphertext, err := EncryptString(totpSecret)
	if err != nil {
		t.Fatalf("EncryptString failed: %v", err)
	}

	if len(ciphertext) == 0 {
		t.Fatal("ciphertext must not be empty")
	}

	// Ciphertext must not contain the plaintext
	if strings.Contains(string(ciphertext), totpSecret) {
		t.Fatal("ciphertext must not contain the plaintext TOTP secret")
	}

	decrypted, err := DecryptString(ciphertext)
	if err != nil {
		t.Fatalf("DecryptString failed: %v", err)
	}

	if decrypted != totpSecret {
		t.Fatalf("expected %q, got %q", totpSecret, decrypted)
	}
}

func TestEncryptDecrypt_LongTOTPSecret(t *testing.T) {
	setupTestKey(t)

	// Longer TOTP secret (64 chars, some authenticator apps use these)
	totpSecret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3T"

	ciphertext, err := EncryptString(totpSecret)
	if err != nil {
		t.Fatalf("EncryptString failed: %v", err)
	}

	decrypted, err := DecryptString(ciphertext)
	if err != nil {
		t.Fatalf("DecryptString failed: %v", err)
	}

	if decrypted != totpSecret {
		t.Fatalf("round-trip failed for long secret")
	}
}

func TestEncrypt_ProducesDifferentCiphertextEachTime(t *testing.T) {
	setupTestKey(t)

	plaintext := []byte("JBSWY3DPEHPK3PXP")

	ct1, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("first Encrypt failed: %v", err)
	}

	ct2, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("second Encrypt failed: %v", err)
	}

	// AES-GCM uses a random nonce, so two encryptions of the same plaintext
	// must produce different ciphertexts.
	if bytes.Equal(ct1, ct2) {
		t.Fatal("two encryptions of the same plaintext must produce different ciphertexts (random nonce)")
	}

	// Both must still decrypt to the same value
	d1, _ := Decrypt(ct1)
	d2, _ := Decrypt(ct2)
	if !bytes.Equal(d1, d2) {
		t.Fatal("both ciphertexts must decrypt to the same plaintext")
	}
}

func TestDecrypt_FailsOnTamperedCiphertext(t *testing.T) {
	setupTestKey(t)

	plaintext := []byte("JBSWY3DPEHPK3PXP")
	ciphertext, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with the ciphertext by flipping a byte
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xFF

	_, err = Decrypt(tampered)
	if err == nil {
		t.Fatal("Decrypt should fail on tampered ciphertext")
	}
}

func TestDecrypt_FailsOnTruncatedCiphertext(t *testing.T) {
	setupTestKey(t)

	// A ciphertext shorter than the nonce size should fail
	_, err := Decrypt([]byte{0x01, 0x02, 0x03})
	if err == nil {
		t.Fatal("Decrypt should fail on truncated ciphertext")
	}
}

func TestEncrypt_FailsWithoutKey(t *testing.T) {
	// Clear the key
	encryptionKey = nil

	_, err := Encrypt([]byte("test"))
	if err == nil {
		t.Fatal("Encrypt should fail without an initialized key")
	}

	_, err = Decrypt([]byte("test"))
	if err == nil {
		t.Fatal("Decrypt should fail without an initialized key")
	}

	// Restore for other tests
	setupTestKey(t)
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	setupTestKey(t)

	ciphertext, err := Encrypt([]byte{})
	if err != nil {
		t.Fatalf("Encrypt of empty plaintext failed: %v", err)
	}

	decrypted, err := Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt of empty-plaintext ciphertext failed: %v", err)
	}

	if len(decrypted) != 0 {
		t.Fatalf("expected empty plaintext, got %d bytes", len(decrypted))
	}
}

func TestInitEncryptionKey_InvalidHex(t *testing.T) {
	err := InitEncryptionKeyFromValue("not-valid-hex")
	if err == nil {
		t.Fatal("InitEncryptionKeyFromValue should fail with invalid hex")
	}
}

func TestInitEncryptionKey_WrongLength(t *testing.T) {
	// 32 hex chars = 16 bytes (AES-128, not AES-256)
	err := InitEncryptionKeyFromValue("0123456789abcdef0123456789abcdef")
	if err == nil {
		t.Fatal("InitEncryptionKeyFromValue should fail with 16-byte key (need 32)")
	}
}

func TestInitEncryptionKey_Empty(t *testing.T) {
	err := InitEncryptionKeyFromValue("")
	if err == nil {
		t.Fatal("InitEncryptionKeyFromValue should fail with empty value")
	}
}

func TestInitEncryptionKey_Valid(t *testing.T) {
	err := InitEncryptionKeyFromValue("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("InitEncryptionKeyFromValue should succeed: %v", err)
	}
}
