package postgres

import "database/sql"

func migrateCustomToolsV3(tx *sql.Tx) error {
	stmts := []string{
		// Remove legacy user-level skill preference rows; skills are tenant-scoped only.
		`DELETE FROM user_skills WHERE user_id != ''`,
		`CREATE TABLE IF NOT EXISTS custom_tools (
			tenant_id    TEXT   NOT NULL,
			user_id      TEXT   NOT NULL DEFAULT '',
			name         TEXT   NOT NULL,
			description  TEXT   NOT NULL DEFAULT '',
			input_schema JSONB  NOT NULL DEFAULT '{}',
			created_at   BIGINT NOT NULL,
			updated_at   BIGINT NOT NULL,
			PRIMARY KEY (tenant_id, user_id, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_custom_tools_user ON custom_tools(tenant_id, user_id)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

func migrateCustomToolsUserV4(tx *sql.Tx) error {
	// Add user_id to custom_tools for DBs that had the old (tenant_id, name) PK.
	// Postgres supports ADD COLUMN IF NOT EXISTS so this is idempotent.
	stmts := []string{
		`ALTER TABLE custom_tools ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT ''`,
		`DROP INDEX IF EXISTS idx_custom_tools_tenant`,
		`CREATE INDEX IF NOT EXISTS idx_custom_tools_user ON custom_tools(tenant_id, user_id)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
