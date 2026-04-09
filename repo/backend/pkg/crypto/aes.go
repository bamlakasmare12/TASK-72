package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
)

var encryptionKey []byte

// InitEncryptionKey initializes the AES-256 encryption key from a hex string.
// The key must be 64 hex characters (32 bytes).
func InitEncryptionKey() error {
	return InitEncryptionKeyFromValue("")
}

// InitEncryptionKeyFromValue initializes the AES-256 encryption key from the provided hex string.
func InitEncryptionKeyFromValue(keyHex string) error {
	if keyHex == "" {
		return errors.New("AES_ENCRYPTION_KEY is required")
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return errors.New("AES_ENCRYPTION_KEY must be a valid hex string")
	}
	if len(key) != 32 {
		return errors.New("AES_ENCRYPTION_KEY must be 64 hex characters (32 bytes for AES-256)")
	}
	encryptionKey = key
	return nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns ciphertext with the nonce prepended.
func Encrypt(plaintext []byte) ([]byte, error) {
	if len(encryptionKey) == 0 {
		return nil, errors.New("encryption key not initialized")
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext produced by Encrypt (nonce-prepended AES-256-GCM).
func Decrypt(ciphertext []byte) ([]byte, error) {
	if len(encryptionKey) == 0 {
		return nil, errors.New("encryption key not initialized")
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertextBody := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBody, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// EncryptString is a convenience wrapper for string data.
func EncryptString(plaintext string) ([]byte, error) {
	return Encrypt([]byte(plaintext))
}

// DecryptString is a convenience wrapper that returns a string.
func DecryptString(ciphertext []byte) (string, error) {
	b, err := Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
