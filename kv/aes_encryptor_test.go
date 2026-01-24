package kv_test

import (
	"context"
	"crypto/rand"
	"io"
	"testing"

	"github.com/erlorenz/go-toolbox/kv"
)

func TestAESEncryptor(t *testing.T) {
	ctx := context.Background()

	t.Run("ValidKeySize", func(t *testing.T) {
		key := make([]byte, 32)
		io.ReadFull(rand.Reader, key)

		encryptor, err := kv.NewAESEncryptor(key)
		if err != nil {
			t.Fatalf("NewAESEncryptor failed with valid key: %v", err)
		}
		if encryptor == nil {
			t.Fatal("NewAESEncryptor returned nil encryptor")
		}
	})

	t.Run("InvalidKeySize", func(t *testing.T) {
		testCases := []struct {
			name    string
			keySize int
		}{
			{"16 bytes (AES-128)", 16},
			{"24 bytes (AES-192)", 24},
			{"31 bytes (too short)", 31},
			{"33 bytes (too long)", 33},
			{"0 bytes (empty)", 0},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				key := make([]byte, tc.keySize)
				_, err := kv.NewAESEncryptor(key)
				if err == nil {
					t.Errorf("NewAESEncryptor should fail with %d-byte key", tc.keySize)
				}
			})
		}
	})

	t.Run("EncryptDecrypt", func(t *testing.T) {
		key := make([]byte, 32)
		io.ReadFull(rand.Reader, key)

		encryptor, err := kv.NewAESEncryptor(key)
		if err != nil {
			t.Fatalf("NewAESEncryptor failed: %v", err)
		}

		plaintext := []byte("Hello, World!")

		// Encrypt
		ciphertext, err := encryptor.Encrypt(ctx, plaintext)
		if err != nil {
			t.Fatalf("Encrypt failed: %v", err)
		}

		// Ciphertext should be larger than plaintext (nonce + tag)
		if len(ciphertext) <= len(plaintext) {
			t.Errorf("Ciphertext length (%d) should be > plaintext length (%d)", len(ciphertext), len(plaintext))
		}

		// Decrypt
		decrypted, err := encryptor.Decrypt(ctx, ciphertext)
		if err != nil {
			t.Fatalf("Decrypt failed: %v", err)
		}

		if string(decrypted) != string(plaintext) {
			t.Errorf("Decrypted = %q, want %q", decrypted, plaintext)
		}
	})

	t.Run("DifferentCiphertexts", func(t *testing.T) {
		key := make([]byte, 32)
		io.ReadFull(rand.Reader, key)

		encryptor, err := kv.NewAESEncryptor(key)
		if err != nil {
			t.Fatalf("NewAESEncryptor failed: %v", err)
		}

		plaintext := []byte("Same plaintext")

		// Encrypt the same plaintext twice
		ciphertext1, _ := encryptor.Encrypt(ctx, plaintext)
		ciphertext2, _ := encryptor.Encrypt(ctx, plaintext)

		// Ciphertexts should be different (different nonces)
		if string(ciphertext1) == string(ciphertext2) {
			t.Error("Encrypting same plaintext twice should produce different ciphertexts")
		}

		// Both should decrypt to the same plaintext
		decrypted1, _ := encryptor.Decrypt(ctx, ciphertext1)
		decrypted2, _ := encryptor.Decrypt(ctx, ciphertext2)

		if string(decrypted1) != string(plaintext) || string(decrypted2) != string(plaintext) {
			t.Error("Both ciphertexts should decrypt to original plaintext")
		}
	})

	t.Run("EmptyPlaintext", func(t *testing.T) {
		key := make([]byte, 32)
		io.ReadFull(rand.Reader, key)

		encryptor, err := kv.NewAESEncryptor(key)
		if err != nil {
			t.Fatalf("NewAESEncryptor failed: %v", err)
		}

		plaintext := []byte("")

		ciphertext, err := encryptor.Encrypt(ctx, plaintext)
		if err != nil {
			t.Fatalf("Encrypt failed with empty plaintext: %v", err)
		}

		decrypted, err := encryptor.Decrypt(ctx, ciphertext)
		if err != nil {
			t.Fatalf("Decrypt failed: %v", err)
		}

		if string(decrypted) != "" {
			t.Errorf("Decrypted = %q, want empty string", decrypted)
		}
	})

	t.Run("LargePlaintext", func(t *testing.T) {
		key := make([]byte, 32)
		io.ReadFull(rand.Reader, key)

		encryptor, err := kv.NewAESEncryptor(key)
		if err != nil {
			t.Fatalf("NewAESEncryptor failed: %v", err)
		}

		// 1MB plaintext
		plaintext := make([]byte, 1024*1024)
		io.ReadFull(rand.Reader, plaintext)

		ciphertext, err := encryptor.Encrypt(ctx, plaintext)
		if err != nil {
			t.Fatalf("Encrypt failed with large plaintext: %v", err)
		}

		decrypted, err := encryptor.Decrypt(ctx, ciphertext)
		if err != nil {
			t.Fatalf("Decrypt failed: %v", err)
		}

		if len(decrypted) != len(plaintext) {
			t.Errorf("Decrypted length = %d, want %d", len(decrypted), len(plaintext))
		}
	})

	t.Run("TamperedCiphertext", func(t *testing.T) {
		key := make([]byte, 32)
		io.ReadFull(rand.Reader, key)

		encryptor, err := kv.NewAESEncryptor(key)
		if err != nil {
			t.Fatalf("NewAESEncryptor failed: %v", err)
		}

		plaintext := []byte("Secret message")

		ciphertext, err := encryptor.Encrypt(ctx, plaintext)
		if err != nil {
			t.Fatalf("Encrypt failed: %v", err)
		}

		// Tamper with ciphertext
		tamperedCiphertext := make([]byte, len(ciphertext))
		copy(tamperedCiphertext, ciphertext)
		tamperedCiphertext[len(tamperedCiphertext)-1] ^= 0xFF // Flip bits in last byte

		// Decrypt should fail
		_, err = encryptor.Decrypt(ctx, tamperedCiphertext)
		if err == nil {
			t.Error("Decrypt should fail with tampered ciphertext")
		}
	})

	t.Run("ShortCiphertext", func(t *testing.T) {
		key := make([]byte, 32)
		io.ReadFull(rand.Reader, key)

		encryptor, err := kv.NewAESEncryptor(key)
		if err != nil {
			t.Fatalf("NewAESEncryptor failed: %v", err)
		}

		// Ciphertext shorter than nonce size
		shortCiphertext := []byte("too short")

		_, err = encryptor.Decrypt(ctx, shortCiphertext)
		if err == nil {
			t.Error("Decrypt should fail with ciphertext shorter than nonce size")
		}
	})

	t.Run("WrongKey", func(t *testing.T) {
		key1 := make([]byte, 32)
		key2 := make([]byte, 32)
		io.ReadFull(rand.Reader, key1)
		io.ReadFull(rand.Reader, key2)

		encryptor1, _ := kv.NewAESEncryptor(key1)
		encryptor2, _ := kv.NewAESEncryptor(key2)

		plaintext := []byte("Secret message")

		// Encrypt with key1
		ciphertext, err := encryptor1.Encrypt(ctx, plaintext)
		if err != nil {
			t.Fatalf("Encrypt failed: %v", err)
		}

		// Try to decrypt with key2 (wrong key)
		_, err = encryptor2.Decrypt(ctx, ciphertext)
		if err == nil {
			t.Error("Decrypt should fail with wrong key")
		}
	})
}
