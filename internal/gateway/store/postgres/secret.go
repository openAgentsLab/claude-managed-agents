package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"forge/internal/gateway/store"
	"forge/internal/gateway/vault"
)

type secretRepo struct {
	db        *sql.DB
	masterKey []byte
}

// cryptoID returns the identity string used for key derivation.
func cryptoID(tenantID, userID string) string {
	if userID == "" {
		return "tenant:" + tenantID
	}
	return userID
}

func (r *secretRepo) Set(_ context.Context, tenantID, userID, name, description, plaintext string) error {
	if r.masterKey == nil {
		return fmt.Errorf("vault: master key not configured (set FORGE_VAULT_KEY)")
	}
	ciphertext, nonce, err := vault.EncryptForUser(r.masterKey, cryptoID(tenantID, userID), plaintext)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	id := newSecretID()
	_, err = r.db.Exec(`
		INSERT INTO vault_secrets (id, tenant_id, user_id, name, description, encrypted_value, nonce, key_version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8, $9)
		ON CONFLICT (tenant_id, user_id, name) DO UPDATE SET
			description     = EXCLUDED.description,
			encrypted_value = EXCLUDED.encrypted_value,
			nonce           = EXCLUDED.nonce,
			key_version     = 1,
			updated_at      = EXCLUDED.updated_at
	`, id, tenantID, userID, name, description, ciphertext, nonce, now, now)
	return wrapSecretErr("set", err)
}

func (r *secretRepo) List(_ context.Context, tenantID, userID string) ([]store.SecretMeta, error) {
	rows, err := r.db.Query(
		`SELECT name, description, updated_at FROM vault_secrets WHERE tenant_id = $1 AND user_id = $2 ORDER BY name ASC`,
		tenantID, userID,
	)
	if err != nil {
		return nil, wrapSecretErr("list", err)
	}
	defer rows.Close()
	out := []store.SecretMeta{}
	for rows.Next() {
		var m store.SecretMeta
		var updatedAt int64
		if err := rows.Scan(&m.Name, &m.Description, &updatedAt); err != nil {
			return nil, wrapSecretErr("scan", err)
		}
		m.UpdatedAt = time.Unix(updatedAt, 0)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *secretRepo) Delete(_ context.Context, tenantID, userID, name string) error {
	_, err := r.db.Exec(
		`DELETE FROM vault_secrets WHERE tenant_id = $1 AND user_id = $2 AND name = $3`,
		tenantID, userID, name,
	)
	return wrapSecretErr("delete", err)
}

func (r *secretRepo) Resolve(_ context.Context, tenantID, userID, ref string) (string, error) {
	if !vault.IsVaultRef(ref) {
		return ref, nil
	}
	if r.masterKey == nil {
		return "", fmt.Errorf("vault: master key not configured (set FORGE_VAULT_KEY)")
	}
	name := vault.SecretName(ref)

	// Try user-level secret first.
	cid, ciphertext, nonce, err := r.fetchSecret(tenantID, userID, name)
	if err == sql.ErrNoRows && tenantID != "" && userID != "" {
		// Fall back to tenant-level secret.
		cid, ciphertext, nonce, err = r.fetchSecret(tenantID, "", name)
	}
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("vault: secret %q not found", name)
	}
	if err != nil {
		return "", wrapSecretErr("resolve", err)
	}
	return vault.DecryptForUser(r.masterKey, cid, ciphertext, nonce)
}

func (r *secretRepo) fetchSecret(tenantID, userID, name string) (cid string, ciphertext, nonce []byte, err error) {
	err = r.db.QueryRow(
		`SELECT encrypted_value, nonce FROM vault_secrets WHERE tenant_id = $1 AND user_id = $2 AND name = $3`,
		tenantID, userID, name,
	).Scan(&ciphertext, &nonce)
	if err != nil {
		return "", nil, nil, err
	}
	return cryptoID(tenantID, userID), ciphertext, nonce, nil
}

func newSecretID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var _ store.SecretRepository = (*secretRepo)(nil)
