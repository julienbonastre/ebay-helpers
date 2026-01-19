package database

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

// GetEncryptionKey loads the encryption key from the EBAY_ENCRYPTION_KEY environment variable
// The key must be base64-encoded and decode to exactly 32 bytes (256 bits) for AES-256
func GetEncryptionKey() ([]byte, error) {
	keyStr := os.Getenv("EBAY_ENCRYPTION_KEY")
	if keyStr == "" {
		return nil, errors.New("EBAY_ENCRYPTION_KEY environment variable not set")
	}

	// Decode from base64
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encryption key from base64: %w", err)
	}

	// Verify key length (must be 32 bytes for AES-256)
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid encryption key length: got %d bytes, expected 32 bytes for AES-256", len(key))
	}

	return key, nil
}

// EncryptSecret encrypts a plaintext string using AES-256-GCM
// Returns the encrypted data as a byte slice (nonce + ciphertext)
// The nonce is prepended to the ciphertext for storage
func EncryptSecret(plaintext string, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: got %d bytes, expected 32", len(key))
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM (Galois/Counter Mode) cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce (number used once)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the plaintext
	// GCM authentication tag is automatically appended by Seal()
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Prepend nonce to ciphertext for storage (nonce is not secret)
	// Format: [nonce][ciphertext+tag]
	encrypted := append(nonce, ciphertext...)

	return encrypted, nil
}

// DecryptSecret decrypts an encrypted byte slice back to plaintext
// Expects the input to be in the format: [nonce][ciphertext+tag]
func DecryptSecret(encrypted []byte, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("invalid key length: got %d bytes, expected 32", len(key))
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce from beginning of encrypted data
	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return "", errors.New("encrypted data too short - missing nonce")
	}

	nonce := encrypted[:nonceSize]
	ciphertext := encrypted[nonceSize:]

	// Decrypt and verify authentication tag
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (authentication tag verification failed): %w", err)
	}

	return string(plaintext), nil
}
