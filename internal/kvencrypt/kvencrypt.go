package kvencrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const prefix = "enc:v1:"

// Encrypt encrypts plaintext using AES-256-GCM and returns an "enc:v1:<base64url>" string.
// key must be exactly 32 bytes.
func Encrypt(key [32]byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("kvencrypt: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("kvencrypt: create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("kvencrypt: generate nonce: %w", err)
	}

	// Seal appends ciphertext and 16-byte tag after nonce.
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts an "enc:v1:<base64url>" string produced by Encrypt.
// Returns the original plaintext. Returns an error if the value is not a
// recognized encrypted format, the key is wrong, or the data is corrupted.
func Decrypt(key [32]byte, value string) (string, error) {
	if !strings.HasPrefix(value, prefix) {
		return "", fmt.Errorf("kvencrypt: value is not in encrypted format")
	}

	data, err := base64.RawURLEncoding.DecodeString(value[len(prefix):])
	if err != nil {
		return "", fmt.Errorf("kvencrypt: base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("kvencrypt: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("kvencrypt: create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize+gcm.Overhead() {
		return "", fmt.Errorf("kvencrypt: ciphertext too short")
	}

	nonce, ciphertextAndTag := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextAndTag, nil)
	if err != nil {
		// Do not wrap the raw GCM error; return a user-safe message.
		return "", fmt.Errorf("decryption failed: invalid key or corrupted value")
	}

	return string(plaintext), nil
}

// IsEncrypted reports whether value was produced by Encrypt.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, prefix)
}

// ParseKey parses a 64-character hex string into a 32-byte AES key.
func ParseKey(s string) ([32]byte, error) {
	if len(s) != 64 {
		return [32]byte{}, fmt.Errorf("encryption key must be 64 hex characters (got %d)", len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return [32]byte{}, fmt.Errorf("encryption key must be a valid hex string: %w", err)
	}
	return [32]byte(b), nil
}
