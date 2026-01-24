package kv_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// ExampleEncryptor demonstrates a simple AES-GCM encryptor implementation.
// WARNING: This is for demonstration purposes only. Production systems should use
// proper key management (KMS, Vault, etc.) and consider key rotation.
type ExampleEncryptor struct {
	gcm cipher.AEAD
}

// NewExampleEncryptor creates an encryptor using AES-256-GCM.
// The key must be 32 bytes for AES-256.
func NewExampleEncryptor(key []byte) (*ExampleEncryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes for AES-256, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &ExampleEncryptor{gcm: gcm}, nil
}

// Encrypt encrypts plaintext using AES-GCM.
// Format: [nonce][ciphertext+tag]
func (e *ExampleEncryptor) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := e.gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-GCM.
func (e *ExampleEncryptor) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
