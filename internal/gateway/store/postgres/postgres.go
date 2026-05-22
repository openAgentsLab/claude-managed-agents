package postgres

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"forge/internal/gateway/store"
)

func init() {
	store.Register("postgres", func(opts map[string]string) (store.Store, error) {
		dsn := opts["dsn"]
		if dsn == "" {
			return nil, fmt.Errorf("postgres gateway store: dsn is required")
		}
		return newPostgresStore(dsn)
	})
}

type postgresStore struct {
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

func newPostgresStore(dsn string) (*postgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres gateway store: open: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres gateway store: migrate: %w", err)
	}
	s := &postgresStore{db: db}
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

func (s *postgresStore) Tenants() store.TenantRepository                   { return s.tenants }
func (s *postgresStore) Users() store.UserRepository                       { return s.users }
func (s *postgresStore) MCPServers() store.MCPServerRepository             { return s.mcpServers }
func (s *postgresStore) UserSkills() store.UserSkillRepository             { return s.userSkills }
func (s *postgresStore) Agents() store.AgentRepository                     { return s.agents }
func (s *postgresStore) Sandboxes() store.SandboxRepository                { return s.sandboxes }
func (s *postgresStore) SessionResources() store.SessionResourceRepository { return s.sessionResources }
func (s *postgresStore) Environments() store.EnvironmentRepository         { return s.environments }
func (s *postgresStore) Projects(masterKey []byte) store.ProjectRepository {
	return &projectRepo{db: s.db, masterKey: masterKey}
}
func (s *postgresStore) Secrets(masterKey []byte) store.SecretRepository {
	return &secretRepo{db: s.db, masterKey: masterKey}
}
func (s *postgresStore) Close() error { return s.db.Close() }

var _ store.Store = (*postgresStore)(nil)

// ── versioned migrations ───────────────────────────────────────────────────

type migration struct {
	version int
	apply   func(tx *sql.Tx) error
}

var migrations = []migration{
	{1, migrateV1},
	{2, migrateAgentsV2},
	{3, migrateCustomToolsV3},
	{4, migrateCustomToolsUserV4},
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_versions (
			version    INTEGER PRIMARY KEY,
			applied_at BIGINT  NOT NULL
		)`); err != nil {
		return fmt.Errorf("create schema_versions: %w", err)
	}

	var maxVer int
	_ = db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_versions`).Scan(&maxVer)

	for _, m := range migrations {
		if m.version <= maxVer {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration v%d: %w", m.version, err)
		}
		if err := m.apply(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration v%d: %w", m.version, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO schema_versions (version, applied_at) VALUES ($1, $2)`,
			m.version, time.Now().Unix(),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration v%d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration v%d: %w", m.version, err)
		}
	}
	return nil
}

func migrateV1(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS tenants (
			id            TEXT   PRIMARY KEY,
			name          TEXT   NOT NULL DEFAULT '',
			settings_json JSONB  NOT NULL DEFAULT '{}',
			created_at    BIGINT NOT NULL,
			updated_at    BIGINT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tenant_users (
			tenant_id     TEXT   NOT NULL,
			username      TEXT   NOT NULL,
			password_hash TEXT   NOT NULL DEFAULT '',
			role          TEXT   NOT NULL DEFAULT 'member',
			settings_json JSONB  NOT NULL DEFAULT '{}',
			created_at    BIGINT NOT NULL,
			PRIMARY KEY (tenant_id, username)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_users_username ON tenant_users(username)`,
		`CREATE TABLE IF NOT EXISTS mcp_servers (
			tenant_id    TEXT    NOT NULL,
			user_id      TEXT    NOT NULL DEFAULT '',
			name         TEXT    NOT NULL,
			type         TEXT    NOT NULL DEFAULT 'stdio',
			command      TEXT    NOT NULL DEFAULT '',
			args_json    JSONB   NOT NULL DEFAULT '[]',
			env_json     JSONB   NOT NULL DEFAULT '{}',
			url          TEXT    NOT NULL DEFAULT '',
			headers_json JSONB   NOT NULL DEFAULT '{}',
			disabled     BOOLEAN NOT NULL DEFAULT FALSE,
			created_at   BIGINT  NOT NULL,
			updated_at   BIGINT  NOT NULL,
			PRIMARY KEY (tenant_id, user_id, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mcp_servers_tenant ON mcp_servers(tenant_id)`,
		`CREATE TABLE IF NOT EXISTS user_skills (
			tenant_id  TEXT    NOT NULL,
			user_id    TEXT    NOT NULL,
			name       TEXT    NOT NULL,
			content    TEXT    NOT NULL DEFAULT '',
			enabled    BOOLEAN NOT NULL DEFAULT TRUE,
			created_at BIGINT  NOT NULL,
			updated_at BIGINT  NOT NULL,
			PRIMARY KEY (tenant_id, user_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS vault_secrets (
			id              TEXT   PRIMARY KEY,
			tenant_id       TEXT   NOT NULL DEFAULT '',
			user_id         TEXT   NOT NULL,
			name            TEXT   NOT NULL,
			description     TEXT   NOT NULL DEFAULT '',
			encrypted_value BYTEA  NOT NULL,
			nonce           BYTEA  NOT NULL,
			key_version     INTEGER NOT NULL DEFAULT 1,
			created_at      BIGINT  NOT NULL,
			updated_at      BIGINT  NOT NULL,
			UNIQUE(tenant_id, user_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS environments (
			id              TEXT    PRIMARY KEY,
			tenant_id       TEXT    NOT NULL,
			owner_id        TEXT    NOT NULL DEFAULT '',
			scope           TEXT    NOT NULL DEFAULT 'tenant',
			name            TEXT    NOT NULL DEFAULT '',
			description     TEXT    NOT NULL DEFAULT '',
			packages_json   JSONB   NOT NULL DEFAULT '{}',
			networking_json JSONB   NOT NULL DEFAULT '{}',
			env_json        JSONB   NOT NULL DEFAULT '{}',
			is_default      BOOLEAN NOT NULL DEFAULT FALSE,
			created_at      BIGINT  NOT NULL,
			updated_at      BIGINT  NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_environments_scope  ON environments(tenant_id, scope, owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_environments_tenant ON environments(tenant_id)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id              TEXT   PRIMARY KEY,
			tenant_id       TEXT   NOT NULL,
			owner_id        TEXT   NOT NULL,
			name            TEXT   NOT NULL DEFAULT '',
			description     TEXT   NOT NULL DEFAULT '',
			git_url         TEXT   NOT NULL DEFAULT '',
			git_branch      TEXT   NOT NULL DEFAULT '',
			git_username    TEXT   NOT NULL DEFAULT '',
			git_token       TEXT   NOT NULL DEFAULT '',
			git_token_nonce TEXT   NOT NULL DEFAULT '',
			environment_id  TEXT   NOT NULL DEFAULT '',
			ref_files_json  JSONB  NOT NULL DEFAULT '[]',
			env_json        JSONB  NOT NULL DEFAULT '{}',
			created_at      BIGINT NOT NULL,
			updated_at      BIGINT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_owner  ON projects(tenant_id, owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_tenant ON projects(tenant_id)`,
		`CREATE TABLE IF NOT EXISTS sandboxes (
			session_id       TEXT   PRIMARY KEY,
			tenant_id        TEXT   NOT NULL DEFAULT '',
			sandbox_id       TEXT   NOT NULL DEFAULT '',
			endpoint         TEXT   NOT NULL DEFAULT '',
			token            TEXT   NOT NULL DEFAULT '',
			last_seen        BIGINT NOT NULL DEFAULT 0,
			environment_spec TEXT   NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sandboxes_tenant ON sandboxes(tenant_id)`,
		`CREATE TABLE IF NOT EXISTS session_resources (
			id          TEXT   PRIMARY KEY,
			session_id  TEXT   NOT NULL,
			type        TEXT   NOT NULL,
			target_path TEXT   NOT NULL,
			spec        TEXT   NOT NULL DEFAULT '',
			created_at  BIGINT NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_session_resources_session_id ON session_resources(session_id)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
