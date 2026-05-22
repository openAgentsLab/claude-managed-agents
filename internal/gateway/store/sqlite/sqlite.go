package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"forge/internal/gateway/store"
)

const defaultPath = ".forge/store.db"

func init() {
	store.Register("sqlite", func(opts map[string]string) (store.Store, error) {
		path := opts["path"]
		if path == "" {
			path = defaultPath
		}
		return newSQLiteStore(path)
	})
}

type sqliteStore struct {
	db               *sql.DB
	tenants          *tenantRepo
	users            *userRepo
	mcpServers       *mcpServerRepo
	userSkills       *userSkillRepo
	agents           *agentRepo
	sandboxes        *sandboxRepo
	sessionResources *sessionResourceRepo
	environments     *environmentRepo
}

func newSQLiteStore(path string) (*sqliteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("sqlite store: mkdir %q: %w", filepath.Dir(path), err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: open %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite store: migrate: %w", err)
	}
	s := &sqliteStore{db: db}
	s.tenants = &tenantRepo{db: db}
	s.users = &userRepo{db: db}
	s.mcpServers = &mcpServerRepo{db: db}
	s.userSkills = &userSkillRepo{db: db}
	s.agents = &agentRepo{db: db}
	s.sandboxes = &sandboxRepo{db: db}
	s.sessionResources = &sessionResourceRepo{db: db}
	s.environments = &environmentRepo{db: db}
	return s, nil
}

func (s *sqliteStore) Tenants() store.TenantRepository                   { return s.tenants }
func (s *sqliteStore) Users() store.UserRepository                       { return s.users }
func (s *sqliteStore) MCPServers() store.MCPServerRepository             { return s.mcpServers }
func (s *sqliteStore) UserSkills() store.UserSkillRepository             { return s.userSkills }
func (s *sqliteStore) Agents() store.AgentRepository                     { return s.agents }
func (s *sqliteStore) Sandboxes() store.SandboxRepository                { return s.sandboxes }
func (s *sqliteStore) SessionResources() store.SessionResourceRepository { return s.sessionResources }
func (s *sqliteStore) Environments() store.EnvironmentRepository         { return s.environments }
func (s *sqliteStore) Projects(masterKey []byte) store.ProjectRepository {
	return &projectRepo{db: s.db, masterKey: masterKey}
}
func (s *sqliteStore) Secrets(masterKey []byte) store.SecretRepository {
	return &secretRepo{db: s.db, masterKey: masterKey}
}
func (s *sqliteStore) Close() error { return s.db.Close() }

var _ store.Store = (*sqliteStore)(nil)

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tenants (
			id            TEXT    PRIMARY KEY,
			name          TEXT    NOT NULL DEFAULT '',
			settings_json TEXT    NOT NULL DEFAULT '{}',
			created_at    INTEGER NOT NULL,
			updated_at    INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS tenant_users (
			tenant_id     TEXT    NOT NULL,
			username      TEXT    NOT NULL,
			password_hash TEXT    NOT NULL DEFAULT '',
			role          TEXT    NOT NULL DEFAULT 'member',
			created_at    INTEGER NOT NULL,
			PRIMARY KEY (tenant_id, username)
		);
		CREATE INDEX IF NOT EXISTS idx_tenant_users_username ON tenant_users(username);
		CREATE TABLE IF NOT EXISTS mcp_servers (
			tenant_id    TEXT    NOT NULL,
			user_id      TEXT    NOT NULL,
			name         TEXT    NOT NULL,
			type         TEXT    NOT NULL DEFAULT 'stdio',
			command      TEXT    NOT NULL DEFAULT '',
			args_json    TEXT    NOT NULL DEFAULT '[]',
			env_json     TEXT    NOT NULL DEFAULT '{}',
			url          TEXT    NOT NULL DEFAULT '',
			headers_json TEXT    NOT NULL DEFAULT '{}',
			disabled     INTEGER NOT NULL DEFAULT 0,
			created_at   INTEGER NOT NULL,
			updated_at   INTEGER NOT NULL,
			PRIMARY KEY (tenant_id, user_id, name)
		);
		CREATE TABLE IF NOT EXISTS user_skills (
			tenant_id  TEXT    NOT NULL,
			user_id    TEXT    NOT NULL,
			name       TEXT    NOT NULL,
			content    TEXT    NOT NULL DEFAULT '',
			enabled    INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (tenant_id, user_id, name)
		);
	`)
	if err != nil {
		return err
	}
	// Additive migrations — ignore "duplicate column" errors for existing DBs.
	if _, err := db.Exec(
		`ALTER TABLE tenant_users ADD COLUMN settings_json TEXT NOT NULL DEFAULT '{}'`,
	); err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("migrate tenant_users.settings_json: %w", err)
	}
	if err := migrateVaultSecrets(db); err != nil {
		return fmt.Errorf("migrate vault_secrets: %w", err)
	}
	if err := migrateSandboxes(db); err != nil {
		return fmt.Errorf("migrate sandboxes: %w", err)
	}
	if err := migrateEnvironments(db); err != nil {
		return fmt.Errorf("migrate environments: %w", err)
	}
	if err := migrateProjects(db); err != nil {
		return fmt.Errorf("migrate projects: %w", err)
	}
	if err := migrateSessionResources(db); err != nil {
		return fmt.Errorf("migrate session_resources: %w", err)
	}
	if err := migrateAgents(db); err != nil {
		return fmt.Errorf("migrate agents: %w", err)
	}
	// Remove legacy user-level MCP records; MCP config is now tenant-scoped only.
	if _, err := db.Exec(`DELETE FROM mcp_servers WHERE user_id != ''`); err != nil {
		return fmt.Errorf("migrate mcp_servers cleanup: %w", err)
	}
	// Remove legacy user-level skill preference rows; skills are tenant-scoped only.
	if _, err := db.Exec(`DELETE FROM user_skills WHERE user_id != ''`); err != nil {
		return fmt.Errorf("migrate user_skills cleanup: %w", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS custom_tools (
			tenant_id    TEXT    NOT NULL,
			user_id      TEXT    NOT NULL DEFAULT '',
			name         TEXT    NOT NULL,
			description  TEXT    NOT NULL DEFAULT '',
			input_schema TEXT    NOT NULL DEFAULT '{}',
			created_at   INTEGER NOT NULL,
			updated_at   INTEGER NOT NULL,
			PRIMARY KEY (tenant_id, user_id, name)
		)
	`); err != nil {
		return fmt.Errorf("migrate custom_tools: %w", err)
	}
	// Additive: add user_id for DBs created before this column existed.
	if _, err := db.Exec(
		`ALTER TABLE custom_tools ADD COLUMN user_id TEXT NOT NULL DEFAULT ''`,
	); err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("migrate custom_tools.user_id: %w", err)
	}
	return nil
}

// migrateVaultSecrets creates vault_secrets with the tenant_id column (fresh DB),
// or rebuilds the table to add tenant_id when upgrading from an older schema that
// only had UNIQUE(user_id, name). Existing rows get tenant_id=''.
func migrateVaultSecrets(db *sql.DB) error {
	const createSQL = `CREATE TABLE IF NOT EXISTS vault_secrets (
		id               TEXT    PRIMARY KEY,
		tenant_id        TEXT    NOT NULL DEFAULT '',
		user_id          TEXT    NOT NULL,
		name             TEXT    NOT NULL,
		description      TEXT    NOT NULL DEFAULT '',
		encrypted_value  BLOB    NOT NULL,
		nonce            BLOB    NOT NULL,
		key_version      INTEGER NOT NULL DEFAULT 1,
		created_at       INTEGER NOT NULL,
		updated_at       INTEGER NOT NULL,
		UNIQUE(tenant_id, user_id, name)
	)`

	// Check if table already exists with tenant_id column.
	var colName string
	err := db.QueryRow(
		`SELECT name FROM pragma_table_info('vault_secrets') WHERE name='tenant_id'`,
	).Scan(&colName)

	switch {
	case err == nil:
		// tenant_id already present — nothing to do.
		return nil
	case err != sql.ErrNoRows:
		return fmt.Errorf("check vault_secrets schema: %w", err)
	}

	// No tenant_id column. Two cases:
	//   a) table does not exist yet → CREATE TABLE handles it.
	//   b) table exists with old schema → rename, recreate, copy, drop.
	var tblName string
	existsErr := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='vault_secrets'`).Scan(&tblName)

	if existsErr == sql.ErrNoRows {
		// Fresh DB: just create with correct schema.
		_, err := db.Exec(createSQL)
		return err
	}
	if existsErr != nil {
		return fmt.Errorf("check vault_secrets existence: %w", existsErr)
	}

	// Existing table lacks tenant_id: rebuild.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin rebuild tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	steps := []string{
		`ALTER TABLE vault_secrets RENAME TO vault_secrets_old`,
		createSQL,
		`INSERT INTO vault_secrets (id, tenant_id, user_id, name, description, encrypted_value, nonce, key_version, created_at, updated_at)
		 SELECT id, '', user_id, name, description, encrypted_value, nonce, key_version, created_at, updated_at
		 FROM vault_secrets_old`,
		`DROP TABLE vault_secrets_old`,
	}
	for _, s := range steps {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("rebuild vault_secrets: %w", err)
		}
	}
	return tx.Commit()
}

// isDuplicateColumn reports whether err is a SQLite "duplicate column name" error,
// which is expected when re-running ADD COLUMN on an already-migrated database.
func isDuplicateColumn(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column name")
}
