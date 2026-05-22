package postgres

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"forge/internal/memory"

	_ "github.com/lib/pq"
)

func init() {
	memory.Register("postgres", func(opts map[string]string) (memory.StoreBackend, error) {
		dsn := opts["dsn"]
		if dsn == "" {
			return nil, fmt.Errorf("postgres memory: opts.dsn is required")
		}
		return openBackend(dsn)
	})
}

type backend struct {
	db *sql.DB
}

func openBackend(dsn string) (*backend, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres memory: open: %w", err)
	}
	b := &backend{db: db}
	if err := b.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return b, nil
}

func (b *backend) migrate() error {
	_, err := b.db.Exec(`
CREATE TABLE IF NOT EXISTS memory_documents (
    store_id   TEXT    NOT NULL,
    filename   TEXT    NOT NULL,
    content    TEXT    NOT NULL DEFAULT '',
    sha256     TEXT    NOT NULL DEFAULT '',
    version    INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT  NOT NULL DEFAULT 0,
    updated_at BIGINT  NOT NULL DEFAULT 0,
    PRIMARY KEY(store_id, filename)
);

CREATE INDEX IF NOT EXISTS memory_documents_fts_idx
ON memory_documents USING GIN (to_tsvector('english', content));

CREATE TABLE IF NOT EXISTS memory_stores (
    id           TEXT NOT NULL PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    visibility   TEXT NOT NULL DEFAULT 'private',
    write_policy TEXT NOT NULL DEFAULT 'owner_only',
    created_by   TEXT NOT NULL DEFAULT '',
    created_at   BIGINT NOT NULL DEFAULT 0
);
`)
	return err
}

func (b *backend) NewStore(storeID, name, description string) memory.MemoryStore {
	return &store{b: b, storeID: storeID, name: name, description: description}
}

func (b *backend) CreateMeta(meta memory.StoreMeta) error {
	_, err := b.db.Exec(
		`INSERT INTO memory_stores(id,name,description,visibility,write_policy,created_by,created_at)
         VALUES($1,$2,$3,$4,$5,$6,$7)`,
		meta.ID, meta.Name, meta.Description, meta.Visibility, meta.WritePolicy,
		meta.CreatedBy, meta.CreatedAt,
	)
	return err
}

func (b *backend) GetMeta(id string) (memory.StoreMeta, bool, error) {
	row := b.db.QueryRow(
		`SELECT id,name,description,visibility,write_policy,created_by,created_at
         FROM memory_stores WHERE id=$1`, id)
	var m memory.StoreMeta
	err := row.Scan(&m.ID, &m.Name, &m.Description, &m.Visibility, &m.WritePolicy, &m.CreatedBy, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return memory.StoreMeta{}, false, nil
	}
	if err != nil {
		return memory.StoreMeta{}, false, err
	}
	return m, true, nil
}

func (b *backend) ListMeta(callerUserID, tenantID string) ([]memory.StoreMeta, error) {
	rows, err := b.db.Query(
		`SELECT id,name,description,visibility,write_policy,created_by,created_at
         FROM memory_stores
         WHERE created_by=$1 OR visibility='shared_tenant'`,
		callerUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []memory.StoreMeta
	for rows.Next() {
		var m memory.StoreMeta
		if err := rows.Scan(&m.ID, &m.Name, &m.Description, &m.Visibility, &m.WritePolicy, &m.CreatedBy, &m.CreatedAt); err != nil {
			return nil, err
		}
		if m.Visibility == "shared_tenant" && m.CreatedBy != callerUserID {
			creatorTenant := strings.SplitN(m.CreatedBy, "/", 2)[0]
			if creatorTenant != tenantID {
				continue
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (b *backend) UpdateMeta(id, name, description, visibility, writePolicy string) error {
	_, err := b.db.Exec(
		`UPDATE memory_stores SET name=$1,description=$2,visibility=$3,write_policy=$4 WHERE id=$5`,
		name, description, visibility, writePolicy, id,
	)
	return err
}

func (b *backend) DeleteMeta(id string) error {
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM memory_documents WHERE store_id=$1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM memory_stores WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (b *backend) Close() error {
	return b.db.Close()
}

type store struct {
	b           *backend
	storeID     string
	name        string
	description string
}

func (s *store) Name() string        { return s.name }
func (s *store) Description() string { return s.description }

func (s *store) List() ([]memory.DocumentSummary, error) {
	rows, err := s.b.db.Query(
		`SELECT filename, content FROM memory_documents WHERE store_id=$1 ORDER BY filename`,
		s.storeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []memory.DocumentSummary
	for rows.Next() {
		var filename, content string
		if err := rows.Scan(&filename, &content); err != nil {
			return nil, err
		}
		out = append(out, memory.DocumentSummary{
			Filename: filename,
			Excerpt:  firstLine(content),
		})
	}
	return out, rows.Err()
}

func (s *store) Read(filename string) (memory.Document, bool, error) {
	row := s.b.db.QueryRow(
		`SELECT filename, content, sha256, version FROM memory_documents WHERE store_id=$1 AND filename=$2`,
		s.storeID, filename,
	)
	var d memory.Document
	err := row.Scan(&d.Filename, &d.Content, &d.SHA256, &d.Version)
	if err == sql.ErrNoRows {
		return memory.Document{}, false, nil
	}
	if err != nil {
		return memory.Document{}, false, err
	}
	return d, true, nil
}

func (s *store) Search(query string) ([]memory.SearchResult, error) {
	rows, err := s.b.db.Query(
		`SELECT filename,
                ts_headline('english', content, plainto_tsquery('english', $2), 'MaxWords=32,MinWords=8') AS excerpt
         FROM memory_documents
         WHERE store_id=$1
           AND to_tsvector('english', content) @@ plainto_tsquery('english', $2)
         ORDER BY ts_rank(to_tsvector('english', content), plainto_tsquery('english', $2)) DESC
         LIMIT 5`,
		s.storeID, query,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []memory.SearchResult
	for rows.Next() {
		var r memory.SearchResult
		r.StoreName = s.name
		if err := rows.Scan(&r.Filename, &r.Excerpt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *store) Write(filename, content, sha256Precondition string) (string, error) {
	if err := memory.ValidateFilename(filename); err != nil {
		return "", err
	}
	if err := memory.ValidateContent(content); err != nil {
		return "", err
	}

	tx, err := s.b.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if sha256Precondition != "" {
		var currentSHA string
		err := tx.QueryRow(
			`SELECT sha256 FROM memory_documents WHERE store_id=$1 AND filename=$2 FOR UPDATE`,
			s.storeID, filename,
		).Scan(&currentSHA)
		if err != nil && err != sql.ErrNoRows {
			return "", err
		}
		if err == nil && currentSHA != sha256Precondition {
			return "", &memory.ConflictError{CurrentSHA256: currentSHA}
		}
	}

	newSHA := hashContent(content)
	now := time.Now().Unix()

	_, err = tx.Exec(
		`INSERT INTO memory_documents(store_id,filename,content,sha256,version,created_at,updated_at)
         VALUES($1,$2,$3,$4,1,$5,$6)
         ON CONFLICT(store_id,filename) DO UPDATE SET
             content=EXCLUDED.content,
             sha256=EXCLUDED.sha256,
             version=memory_documents.version+1,
             updated_at=EXCLUDED.updated_at`,
		s.storeID, filename, content, newSHA, now, now,
	)
	if err != nil {
		return "", err
	}
	return newSHA, tx.Commit()
}

func (s *store) Edit(filename, oldStr, newStr, sha256Precondition string) (string, error) {
	if err := memory.ValidateFilename(filename); err != nil {
		return "", err
	}

	tx, err := s.b.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var currentContent, currentSHA string
	err = tx.QueryRow(
		`SELECT content, sha256 FROM memory_documents WHERE store_id=$1 AND filename=$2 FOR UPDATE`,
		s.storeID, filename,
	).Scan(&currentContent, &currentSHA)
	if err == sql.ErrNoRows {
		return "", memory.ErrNotFound
	}
	if err != nil {
		return "", err
	}

	if sha256Precondition != "" && currentSHA != sha256Precondition {
		return "", &memory.ConflictError{CurrentSHA256: currentSHA}
	}

	count := strings.Count(currentContent, oldStr)
	if count == 0 {
		return "", fmt.Errorf("memory: old_str not found in %q", filename)
	}
	if count > 1 {
		return "", fmt.Errorf("memory: old_str appears %d times in %q, must be unique", count, filename)
	}

	newContent := strings.Replace(currentContent, oldStr, newStr, 1)
	if err := memory.ValidateContent(newContent); err != nil {
		return "", err
	}
	newSHA := hashContent(newContent)
	now := time.Now().Unix()

	if _, err := tx.Exec(
		`UPDATE memory_documents SET content=$1,sha256=$2,version=version+1,updated_at=$3 WHERE store_id=$4 AND filename=$5`,
		newContent, newSHA, now, s.storeID, filename,
	); err != nil {
		return "", err
	}
	return newSHA, tx.Commit()
}

func (s *store) Delete(filename string) error {
	_, err := s.b.db.Exec(
		`DELETE FROM memory_documents WHERE store_id=$1 AND filename=$2`,
		s.storeID, filename,
	)
	return err
}

func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func firstLine(s string) string {
	for _, line := range strings.SplitN(s, "\n", -1) {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
