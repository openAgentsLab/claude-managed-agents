package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/hkdf"
)

// Resolver resolves vault references at runtime.
// It is satisfied by store.SecretRepository, kept here so the orchestration
// layer can reference the interface without importing the store package.
type Resolver interface {
	Resolve(ctx context.Context, tenantID, userID, ref string) (string, error)
}

// IsVaultRef reports whether ref starts with "vault:".
func IsVaultRef(ref string) bool {
	return strings.HasPrefix(ref, "vault:")
}

// SecretName extracts the secret name from a "vault:name" reference.
// Returns the empty string if ref is not a vault reference.
func SecretName(ref string) string {
	if !IsVaultRef(ref) {
		return ""
	}
	return ref[len("vault:"):]
}

const (
	keyLen  = 32 // AES-256
	nonceLen = 12 // GCM standard nonce
	hkdfInfo = "forge-vault-v1:"
)

// MasterKeyFromEnv reads and decodes the master key from FORGE_VAULT_KEY.
// The env var must be a base64-encoded 32-byte value.
// Returns nil (not an error) when the env var is unset — callers should
// handle the nil case by disabling vault functionality.
func MasterKeyFromEnv() ([]byte, error) {
	raw := os.Getenv("FORGE_VAULT_KEY")
	if raw == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("vault: FORGE_VAULT_KEY is not valid base64: %w", err)
	}
	if len(key) != keyLen {
		return nil, fmt.Errorf("vault: FORGE_VAULT_KEY must be %d bytes after base64 decode, got %d", keyLen, len(key))
	}
	return key, nil
}

// deriveUserKey derives a per-user AES-256 key from the master key using HKDF-SHA256.
// The info string binds the key to this specific user and purpose.
func deriveUserKey(masterKey []byte, userID string) ([]byte, error) {
	r := hkdf.New(sha256.New, masterKey, nil, []byte(hkdfInfo+userID))
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("vault: key derivation failed: %w", err)
	}
	return key, nil
}

// EncryptForUser encrypts plaintext with AES-256-GCM using the derived per-user key.
// Returns (ciphertext, nonce, error).
func EncryptForUser(masterKey []byte, userID, plaintext string) ([]byte, []byte, error) {
	key, err := deriveUserKey(masterKey, userID)
	if err != nil {
		return nil, nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("vault: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("vault: create GCM: %w", err)
	}
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("vault: generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return ciphertext, nonce, nil
}

// DecryptForUser decrypts ciphertext with AES-256-GCM using the derived per-user key.
func DecryptForUser(masterKey []byte, userID string, ciphertext, nonce []byte) (string, error) {
	key, err := deriveUserKey(masterKey, userID)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("vault: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("vault: create GCM: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("vault: decrypt failed: %w", err)
	}
	return string(plaintext), nil
}
