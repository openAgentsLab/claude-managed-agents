package encoding

import (
	"testing"

	"forge/internal/gateway/store"
	"forge/internal/gateway/vault"
)

// ── MarshalSettings / UnmarshalSettings ───────────────────────────────────────

func TestMarshalUnmarshalSettings_RoundTrip(t *testing.T) {
	model := "claude-haiku-4-5-20251001"
	s := store.Settings{
		AllowRules:  []string{"allow bash:*"},
		DenyRules:   []string{"deny rm -rf"},
		MemoryBytes: 512 * 1024 * 1024,
		NanoCPUs:    1000000000,
		ModelOverride: &store.ModelSettings{
			Provider: "anthropic",
			Model:    model,
		},
	}
	raw, err := MarshalSettings(s)
	if err != nil {
		t.Fatalf("MarshalSettings: %v", err)
	}
	got, err := UnmarshalSettings(raw)
	if err != nil {
		t.Fatalf("UnmarshalSettings: %v", err)
	}
	if got.MemoryBytes != s.MemoryBytes {
		t.Errorf("MemoryBytes: got %d, want %d", got.MemoryBytes, s.MemoryBytes)
	}
	if got.ModelOverride == nil || got.ModelOverride.Model != model {
		t.Errorf("ModelOverride.Model: got %v", got.ModelOverride)
	}
	if len(got.AllowRules) != 1 || got.AllowRules[0] != "allow bash:*" {
		t.Errorf("AllowRules: got %v", got.AllowRules)
	}
}

func TestUnmarshalSettings_EmptyString(t *testing.T) {
	got, err := UnmarshalSettings("")
	if err != nil {
		t.Fatalf("unexpected error for empty string: %v", err)
	}
	if got.MemoryBytes != 0 || got.ModelOverride != nil {
		t.Errorf("expected zero Settings for empty string, got %+v", got)
	}
}

func TestUnmarshalSettings_EmptyObject(t *testing.T) {
	got, err := UnmarshalSettings("{}")
	if err != nil {
		t.Fatalf("unexpected error for '{}': %v", err)
	}
	if got.MemoryBytes != 0 {
		t.Errorf("expected zero Settings for '{}', got %+v", got)
	}
}

// ── MarshalUserSettings / UnmarshalUserSettings ────────────────────────────────

func TestMarshalUnmarshalUserSettings_RoundTrip(t *testing.T) {
	s := store.UserSettings{
		ModelOverride: &store.ModelSettings{
			Provider: "openai",
			Model:    "gpt-4o",
			APIKey:   "sk-test",
		},
	}
	raw, err := MarshalUserSettings(s)
	if err != nil {
		t.Fatalf("MarshalUserSettings: %v", err)
	}
	got, err := UnmarshalUserSettings(raw)
	if err != nil {
		t.Fatalf("UnmarshalUserSettings: %v", err)
	}
	if got.ModelOverride == nil || got.ModelOverride.Model != "gpt-4o" {
		t.Errorf("ModelOverride.Model: got %v", got.ModelOverride)
	}
}

func TestUnmarshalUserSettings_EmptyString(t *testing.T) {
	got, err := UnmarshalUserSettings("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ModelOverride != nil {
		t.Errorf("expected zero UserSettings, got %+v", got)
	}
}

// ── MarshalStringMap / UnmarshalStringMap ─────────────────────────────────────

func TestMarshalStringMap_RoundTrip(t *testing.T) {
	m := map[string]string{"KEY1": "val1", "KEY2": "val2"}
	raw, err := MarshalStringMap(m)
	if err != nil {
		t.Fatalf("MarshalStringMap: %v", err)
	}
	got, err := UnmarshalStringMap(raw)
	if err != nil {
		t.Fatalf("UnmarshalStringMap: %v", err)
	}
	if got["KEY1"] != "val1" || got["KEY2"] != "val2" {
		t.Errorf("round-trip mismatch: got %v", got)
	}
}

func TestMarshalStringMap_EmptyReturnsBraces(t *testing.T) {
	raw, err := MarshalStringMap(nil)
	if err != nil {
		t.Fatalf("MarshalStringMap(nil): %v", err)
	}
	if raw != "{}" {
		t.Errorf("expected '{}', got %q", raw)
	}
}

func TestUnmarshalStringMap_NullReturnsNil(t *testing.T) {
	got, err := UnmarshalStringMap("null")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for 'null' input, got %v", got)
	}
}

// ── MarshalPackageList / UnmarshalPackageList ─────────────────────────────────

func TestMarshalPackageList_RoundTrip(t *testing.T) {
	pl := store.PackageList{
		Pip:  []string{"requests", "flask"},
		Npm:  []string{"lodash"},
		Cargo: []string{"serde"},
	}
	raw, err := MarshalPackageList(pl)
	if err != nil {
		t.Fatalf("MarshalPackageList: %v", err)
	}
	got, err := UnmarshalPackageList(raw)
	if err != nil {
		t.Fatalf("UnmarshalPackageList: %v", err)
	}
	if len(got.Pip) != 2 || got.Pip[0] != "requests" {
		t.Errorf("Pip: got %v", got.Pip)
	}
	if len(got.Npm) != 1 || got.Npm[0] != "lodash" {
		t.Errorf("Npm: got %v", got.Npm)
	}
}

func TestUnmarshalPackageList_Empty(t *testing.T) {
	got, err := UnmarshalPackageList("{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Pip)+len(got.Npm)+len(got.Apt)+len(got.Cargo) != 0 {
		t.Errorf("expected empty PackageList, got %+v", got)
	}
}

// ── MarshalNetworking / UnmarshalNetworking ───────────────────────────────────

func TestMarshalNetworking_RoundTrip(t *testing.T) {
	n := store.NetworkingConfig{
		Mode:         store.NetworkingLimited,
		AllowedHosts: []string{"api.example.com", "cdn.example.com"},
	}
	raw, err := MarshalNetworking(n)
	if err != nil {
		t.Fatalf("MarshalNetworking: %v", err)
	}
	got, err := UnmarshalNetworking(raw)
	if err != nil {
		t.Fatalf("UnmarshalNetworking: %v", err)
	}
	if got.Mode != store.NetworkingLimited {
		t.Errorf("Mode: got %q", got.Mode)
	}
	if len(got.AllowedHosts) != 2 {
		t.Errorf("AllowedHosts: got %v", got.AllowedHosts)
	}
}

// ── MarshalRefFiles / UnmarshalRefFiles ───────────────────────────────────────

func TestMarshalRefFiles_RoundTrip(t *testing.T) {
	rf := []store.RefFile{{Path: "README.md", URL: "https://example.com/readme"}}
	raw, err := MarshalRefFiles(rf)
	if err != nil {
		t.Fatalf("MarshalRefFiles: %v", err)
	}
	got, err := UnmarshalRefFiles(raw)
	if err != nil {
		t.Fatalf("UnmarshalRefFiles: %v", err)
	}
	if len(got) != 1 || got[0].Path != "README.md" {
		t.Errorf("round-trip mismatch: got %v", got)
	}
}

func TestMarshalRefFiles_EmptyReturnsArray(t *testing.T) {
	raw, err := MarshalRefFiles(nil)
	if err != nil {
		t.Fatalf("MarshalRefFiles(nil): %v", err)
	}
	if raw != "[]" {
		t.Errorf("expected '[]', got %q", raw)
	}
}

func TestUnmarshalRefFiles_EmptyArray(t *testing.T) {
	got, err := UnmarshalRefFiles("[]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for '[]', got %v", got)
	}
}

// ── EncryptToken / DecryptToken ───────────────────────────────────────────────

func vaultMasterKey(t *testing.T) []byte {
	t.Helper()
	key, err := vault.MasterKeyFromEnv()
	if err != nil || key == nil {
		// Use a static test key (32 random bytes expressed as base64)
		import_key := make([]byte, 32)
		for i := range import_key {
			import_key[i] = byte(i + 1)
		}
		return import_key
	}
	return key
}

func TestEncryptDecryptToken_RoundTrip(t *testing.T) {
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i + 1)
	}
	tenantID := "tenant1"
	ownerID := "alice"
	plaintext := "ghp_mysupersecrettoken"

	tokenVal, nonceVal, err := EncryptToken(masterKey, tenantID, ownerID, plaintext)
	if err != nil {
		t.Fatalf("EncryptToken: %v", err)
	}
	if tokenVal == plaintext {
		t.Error("encrypted token should not equal plaintext")
	}
	if nonceVal == "" {
		t.Error("nonce should be non-empty when encrypting")
	}

	decrypted, err := DecryptToken(masterKey, tenantID, ownerID, tokenVal, nonceVal)
	if err != nil {
		t.Fatalf("DecryptToken: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("decrypted %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptToken_EmptyToken(t *testing.T) {
	masterKey := make([]byte, 32)
	tokenVal, nonceVal, err := EncryptToken(masterKey, "t1", "u1", "")
	if err != nil {
		t.Fatalf("unexpected error for empty token: %v", err)
	}
	if tokenVal != "" || nonceVal != "" {
		t.Errorf("empty token should return empty values; got %q, %q", tokenVal, nonceVal)
	}
}

func TestEncryptToken_NilMasterKey(t *testing.T) {
	// When masterKey is nil, token is stored as plaintext.
	tokenVal, nonceVal, err := EncryptToken(nil, "t1", "u1", "mytoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokenVal != "mytoken" {
		t.Errorf("nil master key should return plaintext; got %q", tokenVal)
	}
	if nonceVal != "" {
		t.Errorf("nil master key should return empty nonce; got %q", nonceVal)
	}
}

func TestEncryptToken_VaultRef_Passthrough(t *testing.T) {
	masterKey := make([]byte, 32)
	ref := "vault:my-secret"
	tokenVal, nonceVal, err := EncryptToken(masterKey, "t1", "u1", ref)
	if err != nil {
		t.Fatalf("unexpected error for vault ref: %v", err)
	}
	if tokenVal != ref {
		t.Errorf("vault ref should be stored as-is; got %q", tokenVal)
	}
	if nonceVal != "" {
		t.Errorf("vault ref should produce empty nonce; got %q", nonceVal)
	}
}

func TestDecryptToken_EmptyNonce(t *testing.T) {
	// When nonceHex is empty, DecryptToken should return tokenVal as-is.
	masterKey := make([]byte, 32)
	got, err := DecryptToken(masterKey, "t1", "u1", "plaintext-token", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "plaintext-token" {
		t.Errorf("empty nonce should return token as-is; got %q", got)
	}
}

func TestDecryptToken_NilMasterKeyWithNonce(t *testing.T) {
	// When masterKey is nil but nonce is set, DecryptToken should error.
	_, err := DecryptToken(nil, "t1", "u1", "sometoken", "aabbcc")
	if err == nil {
		t.Error("expected error when masterKey is nil but nonce is present")
	}
}
