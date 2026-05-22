package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"forge/internal/gateway/store"
	"forge/internal/gateway/store/encoding"
)

func migrateProjects(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS projects (
			id             TEXT    PRIMARY KEY,
			tenant_id      TEXT    NOT NULL,
			owner_id       TEXT    NOT NULL,
			name           TEXT    NOT NULL DEFAULT '',
			description    TEXT    NOT NULL DEFAULT '',
			git_url        TEXT    NOT NULL DEFAULT '',
			git_branch     TEXT    NOT NULL DEFAULT '',
			git_token      TEXT    NOT NULL DEFAULT '',
			environment_id TEXT    NOT NULL DEFAULT '',
			created_at     INTEGER NOT NULL,
			updated_at     INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_projects_owner
			ON projects(tenant_id, owner_id);
	`); err != nil {
		return err
	}
	if _, err := db.Exec(
		`ALTER TABLE projects ADD COLUMN ref_files_json TEXT NOT NULL DEFAULT '[]'`,
	); err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("migrate projects.ref_files_json: %w", err)
	}
	// git_token_nonce distinguishes encrypted tokens (hex nonce present) from
	// plaintext legacy tokens (empty nonce). git_token stores hex(ciphertext)
	// when encrypted, or the raw token string when nonce is empty.
	if _, err := db.Exec(
		`ALTER TABLE projects ADD COLUMN git_token_nonce TEXT NOT NULL DEFAULT ''`,
	); err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("migrate projects.git_token_nonce: %w", err)
	}
	if _, err := db.Exec(
		`ALTER TABLE projects ADD COLUMN git_username TEXT NOT NULL DEFAULT ''`,
	); err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("migrate projects.git_username: %w", err)
	}
	if _, err := db.Exec(
		`ALTER TABLE projects ADD COLUMN env_json TEXT NOT NULL DEFAULT '{}'`,
	); err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("migrate projects.env_json: %w", err)
	}
	return nil
}

// projectRepo implements store.ProjectRepository.
// masterKey is nil when FORGE_VAULT_KEY is not configured; in that case tokens
// are stored as plaintext (existing behaviour) with a log-visible warning.
type projectRepo struct {
	db        *sql.DB
	masterKey []byte
}

func (r *projectRepo) Create(_ context.Context, p *store.Project) error {
	rfJSON, err := encoding.MarshalRefFiles(p.RefFiles)
	if err != nil {
		return err
	}
	envJSON, err := encoding.MarshalStringMap(p.Env)
	if err != nil {
		return err
	}
	tokenVal, nonceVal, err := encoding.EncryptToken(r.masterKey, p.TenantID, p.OwnerID, p.GitConfig.Token)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	_, err = r.db.Exec(
		`INSERT INTO projects
			(id, tenant_id, owner_id, name, description, git_url, git_branch, git_username,
			 git_token, git_token_nonce, environment_id, ref_files_json, env_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.TenantID, p.OwnerID, p.Name, p.Description,
		p.GitConfig.URL, p.GitConfig.Branch, p.GitConfig.Username,
		tokenVal, nonceVal,
		p.EnvironmentID, rfJSON, envJSON, now, now,
	)
	return wrapErr("create project", err)
}

func (r *projectRepo) Get(_ context.Context, id string) (*store.Project, error) {
	row := r.db.QueryRow(
		`SELECT id, tenant_id, owner_id, name, description, git_url, git_branch, git_username,
		        git_token, git_token_nonce, environment_id, ref_files_json, env_json, created_at, updated_at
		 FROM projects WHERE id = ?`, id,
	)
	return r.scanProject(row)
}

func (r *projectRepo) List(_ context.Context, tenantID, ownerID string) ([]*store.Project, error) {
	rows, err := r.db.Query(
		`SELECT id, tenant_id, owner_id, name, description, git_url, git_branch, git_username,
		        git_token, git_token_nonce, environment_id, ref_files_json, env_json, created_at, updated_at
		 FROM projects
		 WHERE tenant_id = ? AND owner_id = ?
		 ORDER BY created_at ASC`,
		tenantID, ownerID,
	)
	if err != nil {
		return nil, wrapErr("list projects", err)
	}
	defer rows.Close()

	var out []*store.Project
	for rows.Next() {
		p, err := r.scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *projectRepo) Update(_ context.Context, p *store.Project) error {
	rfJSON, err := encoding.MarshalRefFiles(p.RefFiles)
	if err != nil {
		return err
	}
	envJSON, err := encoding.MarshalStringMap(p.Env)
	if err != nil {
		return err
	}
	tokenVal, nonceVal, err := encoding.EncryptToken(r.masterKey, p.TenantID, p.OwnerID, p.GitConfig.Token)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		`UPDATE projects
		 SET name = ?, description = ?, git_url = ?, git_branch = ?, git_username = ?,
		     git_token = ?, git_token_nonce = ?,
		     environment_id = ?, ref_files_json = ?, env_json = ?, updated_at = ?
		 WHERE id = ?`,
		p.Name, p.Description, p.GitConfig.URL, p.GitConfig.Branch, p.GitConfig.Username,
		tokenVal, nonceVal,
		p.EnvironmentID, rfJSON, envJSON, time.Now().Unix(), p.ID,
	)
	return wrapErr("update project", err)
}

func (r *projectRepo) Delete(_ context.Context, id string) error {
	_, err := r.db.Exec(`DELETE FROM projects WHERE id = ?`, id)
	return wrapErr("delete project", err)
}

// ── scanner helper ─────────────────────────────────────────────────────────

func (r *projectRepo) scanProject(s envScanner) (*store.Project, error) {
	var (
		id, tenantID, ownerID, name, description string
		gitURL, gitBranch, gitUsername            string
		gitToken, gitNonce                        string
		envID, rfJSON, envJSON                    string
		createdAt, updatedAt                      int64
	)
	err := s.Scan(&id, &tenantID, &ownerID, &name, &description,
		&gitURL, &gitBranch, &gitUsername, &gitToken, &gitNonce, &envID, &rfJSON, &envJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, wrapErr("scan project", err)
	}
	plainToken, err := encoding.DecryptToken(r.masterKey, tenantID, ownerID, gitToken, gitNonce)
	if err != nil {
		return nil, err
	}
	refFiles, err := encoding.UnmarshalRefFiles(rfJSON)
	if err != nil {
		return nil, err
	}
	env, err := encoding.UnmarshalStringMap(envJSON)
	if err != nil {
		return nil, err
	}
	return &store.Project{
		ID:          id,
		TenantID:    tenantID,
		OwnerID:     ownerID,
		Name:        name,
		Description: description,
		GitConfig: store.GitConfig{
			URL:      gitURL,
			Branch:   gitBranch,
			Username: gitUsername,
			Token:    plainToken,
		},
		EnvironmentID: envID,
		RefFiles:      refFiles,
		Env:           env,
		CreatedAt:     time.Unix(createdAt, 0),
		UpdatedAt:     time.Unix(updatedAt, 0),
	}, nil
}
