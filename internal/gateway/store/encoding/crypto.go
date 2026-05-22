package encoding

import (
	"encoding/hex"
	"fmt"

	"forge/internal/gateway/vault"
)

// EncryptToken encrypts a git token using AES-256-GCM when masterKey is set.
// Returns (hex(ciphertext), hex(nonce)) when encrypted, (plaintext, "") when not.
// An empty token always returns ("", ""). Vault references are stored as-is.
func EncryptToken(masterKey []byte, tenantID, ownerID, token string) (tokenVal, nonceVal string, err error) {
	if token == "" {
		return "", "", nil
	}
	if vault.IsVaultRef(token) {
		return token, "", nil
	}
	if masterKey == nil {
		return token, "", nil
	}
	cryptoID := tenantID + "/" + ownerID
	ciphertext, nonce, err := vault.EncryptForUser(masterKey, cryptoID, token)
	if err != nil {
		return "", "", fmt.Errorf("store: encrypt git token: %w", err)
	}
	return hex.EncodeToString(ciphertext), hex.EncodeToString(nonce), nil
}

// DecryptToken reverses EncryptToken. An empty nonceHex means the value is plaintext.
func DecryptToken(masterKey []byte, tenantID, ownerID, tokenVal, nonceHex string) (string, error) {
	if tokenVal == "" || nonceHex == "" {
		return tokenVal, nil
	}
	ciphertext, err := hex.DecodeString(tokenVal)
	if err != nil {
		return "", fmt.Errorf("store: decode git token ciphertext: %w", err)
	}
	nonce, err := hex.DecodeString(nonceHex)
	if err != nil {
		return "", fmt.Errorf("store: decode git token nonce: %w", err)
	}
	if masterKey == nil {
		return "", fmt.Errorf("store: git token is encrypted but FORGE_VAULT_KEY is not set")
	}
	cryptoID := tenantID + "/" + ownerID
	return vault.DecryptForUser(masterKey, cryptoID, ciphertext, nonce)
}
