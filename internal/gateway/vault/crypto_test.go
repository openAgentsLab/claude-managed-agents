package vault

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"testing"
)

// ── IsVaultRef ────────────────────────────────────────────────────────────────

func TestIsVaultRef_WithPrefix(t *testing.T) {
	if !IsVaultRef("vault:my_secret") {
		t.Error("'vault:my_secret' should be a vault ref")
	}
}

func TestIsVaultRef_WithoutPrefix(t *testing.T) {
	if IsVaultRef("plaintext") {
		t.Error("'plaintext' should not be a vault ref")
	}
}

func TestIsVaultRef_EmptyString(t *testing.T) {
	if IsVaultRef("") {
		t.Error("empty string should not be a vault ref")
	}
}

func TestIsVaultRef_JustPrefix(t *testing.T) {
	if !IsVaultRef("vault:") {
		t.Error("'vault:' alone is still technically a vault ref")
	}
}

// ── SecretName ────────────────────────────────────────────────────────────────

func TestSecretName_ValidRef(t *testing.T) {
	got := SecretName("vault:my_api_key")
	if got != "my_api_key" {
		t.Errorf("SecretName = %q, want %q", got, "my_api_key")
	}
}

func TestSecretName_NotVaultRef(t *testing.T) {
	if got := SecretName("plain"); got != "" {
		t.Errorf("SecretName for non-ref should be empty, got %q", got)
	}
}

func TestSecretName_EmptyName(t *testing.T) {
	if got := SecretName("vault:"); got != "" {
		t.Errorf("SecretName for 'vault:' should be empty string, got %q", got)
	}
}

// ── MasterKeyFromEnv ──────────────────────────────────────────────────────────

func TestMasterKeyFromEnv_Unset(t *testing.T) {
	t.Setenv("FORGE_VAULT_KEY", "")
	key, err := MasterKeyFromEnv()
	if err != nil {
		t.Fatalf("unset env should return nil error, got: %v", err)
	}
	if key != nil {
		t.Error("unset env should return nil key")
	}
}

func TestMasterKeyFromEnv_Valid(t *testing.T) {
	raw := make([]byte, keyLen)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FORGE_VAULT_KEY", base64.StdEncoding.EncodeToString(raw))

	key, err := MasterKeyFromEnv()
	if err != nil {
		t.Fatalf("valid env: %v", err)
	}
	if len(key) != keyLen {
		t.Errorf("expected %d-byte key, got %d", keyLen, len(key))
	}
}

func TestMasterKeyFromEnv_InvalidBase64(t *testing.T) {
	t.Setenv("FORGE_VAULT_KEY", "not-valid-base64!!!")
	_, err := MasterKeyFromEnv()
	if err == nil {
		t.Error("invalid base64 should return an error")
	}
}

func TestMasterKeyFromEnv_WrongLength(t *testing.T) {
	// 16 bytes encodes to valid base64 but is the wrong key length
	short := make([]byte, 16)
	t.Setenv("FORGE_VAULT_KEY", base64.StdEncoding.EncodeToString(short))
	_, err := MasterKeyFromEnv()
	if err == nil {
		t.Error("wrong-length key should return an error")
	}
}

// Ensure Setenv doesn't leak between tests
func init() {
	os.Unsetenv("FORGE_VAULT_KEY")
}

// ── EncryptForUser / DecryptForUser ───────────────────────────────────────────

func newTestMasterKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, keyLen)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	masterKey := newTestMasterKey(t)
	plaintext := "super secret api key"

	ct, nonce, err := EncryptForUser(masterKey, "user-1", plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	got, err := DecryptForUser(masterKey, "user-1", ct, nonce)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != plaintext {
		t.Errorf("round-trip: got %q, want %q", got, plaintext)
	}
}

func TestEncryptDecrypt_DifferentUserFails(t *testing.T) {
	masterKey := newTestMasterKey(t)
	ct, nonce, err := EncryptForUser(masterKey, "user-1", "secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// Decrypting with a different user should fail (different derived key)
	_, err = DecryptForUser(masterKey, "user-2", ct, nonce)
	if err == nil {
		t.Error("decrypting with wrong user should fail")
	}
}

func TestEncryptDecrypt_TamperedCiphertextFails(t *testing.T) {
	masterKey := newTestMasterKey(t)
	ct, nonce, err := EncryptForUser(masterKey, "user-1", "secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// Flip a byte in the ciphertext
	ct[0] ^= 0xFF
	_, err = DecryptForUser(masterKey, "user-1", ct, nonce)
	if err == nil {
		t.Error("tampered ciphertext should fail authentication")
	}
}

func TestEncryptDecrypt_DifferentMasterKeyFails(t *testing.T) {
	masterKey1 := newTestMasterKey(t)
	masterKey2 := newTestMasterKey(t)

	ct, nonce, err := EncryptForUser(masterKey1, "user-1", "secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	_, err = DecryptForUser(masterKey2, "user-1", ct, nonce)
	if err == nil {
		t.Error("decrypting with different master key should fail")
	}
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	masterKey := newTestMasterKey(t)
	ct, nonce, err := EncryptForUser(masterKey, "user-1", "")
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}
	got, err := DecryptForUser(masterKey, "user-1", ct, nonce)
	if err != nil {
		t.Fatalf("decrypt empty: %v", err)
	}
	if got != "" {
		t.Errorf("round-trip empty: got %q", got)
	}
}

func TestEncryptDecrypt_CiphertextDiffersPerCall(t *testing.T) {
	masterKey := newTestMasterKey(t)
	ct1, _, _ := EncryptForUser(masterKey, "user-1", "same plaintext")
	ct2, _, _ := EncryptForUser(masterKey, "user-1", "same plaintext")
	// Random nonce means ciphertexts differ each call
	if string(ct1) == string(ct2) {
		t.Error("two encryptions of the same plaintext should produce different ciphertexts (random nonce)")
	}
}

func TestEncryptDecrypt_UserIsolation(t *testing.T) {
	masterKey := newTestMasterKey(t)
	ct1, nonce1, _ := EncryptForUser(masterKey, "alice", "alice secret")
	ct2, nonce2, _ := EncryptForUser(masterKey, "bob", "bob secret")

	got1, err := DecryptForUser(masterKey, "alice", ct1, nonce1)
	if err != nil || got1 != "alice secret" {
		t.Errorf("alice decrypt: err=%v, got=%q", err, got1)
	}
	got2, err := DecryptForUser(masterKey, "bob", ct2, nonce2)
	if err != nil || got2 != "bob secret" {
		t.Errorf("bob decrypt: err=%v, got=%q", err, got2)
	}
}
