package kv

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// AESEncryptor implements the Encryptor interface using AES-256-GCM.
// It is safe for concurrent use.
//
// AES-GCM provides both confidentiality and authenticity:
//   - AES-256: Industry-standard symmetric encryption
//   - GCM: Galois/Counter Mode provides authenticated encryption
//   - Prevents tampering and ensures data integrity
//
// Security notes:
//   - Uses a random nonce for each encryption (never reused)
//   - Key must be exactly 32 bytes for AES-256
//   - In production, use proper key management (KMS, Vault, etc.)
//   - Consider key rotation strategies
type AESEncryptor struct {
	gcm cipher.AEAD
}

// NewAESEncryptor creates a new AES-256-GCM encryptor.
//
// The key must be exactly 32 bytes. For production use:
//   - Generate keys using crypto/rand
//   - Store keys securely (environment variables, KMS, Vault)
//   - Never hardcode keys in source code
//   - Implement key rotation
//
// Example key generation:
//
//	key := make([]byte, 32)
//	if _, err := io.ReadFull(rand.Reader, key); err != nil {
//	    log.Fatal(err)
//	}
func NewAESEncryptor(key []byte) (*AESEncryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be exactly 32 bytes for AES-256, got %d bytes", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &AESEncryptor{gcm: gcm}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
//
// Format: [nonce][ciphertext+authentication_tag]
//
// The nonce (12 bytes) is prepended to the ciphertext. Each encryption
// uses a unique random nonce, ensuring the same plaintext produces
// different ciphertexts.
//
// The authentication tag (16 bytes) is appended by GCM to verify
// data integrity and authenticity during decryption.
func (e *AESEncryptor) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	// Generate a random nonce for this encryption
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and append authentication tag
	// Result: nonce + ciphertext + tag
	ciphertext := e.gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM.
//
// Verifies the authentication tag before returning plaintext.
// Returns an error if:
//   - Ciphertext is too short (< nonce size)
//   - Authentication tag verification fails (data was tampered with)
//   - Decryption fails for any other reason
func (e *AESEncryptor) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short: %d bytes (minimum: %d bytes)", len(ciphertext), nonceSize)
	}

	// Extract nonce and ciphertext
	nonce := ciphertext[:nonceSize]
	ciphertextData := ciphertext[nonceSize:]

	// Decrypt and verify authentication tag
	plaintext, err := e.gcm.Open(nil, nonce, ciphertextData, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (authentication check failed or invalid data): %w", err)
	}

	return plaintext, nil
}
