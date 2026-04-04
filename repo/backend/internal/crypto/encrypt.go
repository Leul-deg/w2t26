package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

const nonceSize = 12 // GCM standard nonce size is 12 bytes

// Encrypt encrypts plaintext using AES-256-GCM with the provided 32-byte key.
// Returns a base64-encoded string of (12-byte nonce || ciphertext || GCM tag).
// The nonce is randomly generated for each call.
func Encrypt(key []byte, plaintext string) (string, error) {
	if len(key) != keyLength {
		return "", fmt.Errorf("crypto: key must be %d bytes, got %d", keyLength, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: failed to generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce.
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext produced by Encrypt.
// The ciphertext must be base64(nonce || ciphertext || GCM tag).
func Decrypt(key []byte, b64ciphertext string) (string, error) {
	if len(key) != keyLength {
		return "", fmt.Errorf("crypto: key must be %d bytes, got %d", keyLength, len(key))
	}

	data, err := base64.StdEncoding.DecodeString(b64ciphertext)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to base64-decode ciphertext: %w", err)
	}

	if len(data) < nonceSize {
		return "", fmt.Errorf("crypto: ciphertext too short")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decryption failed (authentication error): %w", err)
	}

	return string(plaintext), nil
}
