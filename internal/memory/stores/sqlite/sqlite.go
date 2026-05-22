package sqlite

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"forge/internal/memory"

	_ "modernc.org/sqlite"
)

func init() {
	memory.Register("sqlite", func(opts map[string]string) (memory.StoreBackend, error) {
		path := opts["path"]
		if path == "" {
			path = "memory.db"
		}
		return openBackend(path)
	})
}

// backend holds the shared SQLite connection.
type backend struct {
	db *sql.DB
	mu sync.Mutex
}

func openBackend(path string) (*backend, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite memory: open %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
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
    created_at INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY(store_id, filename)
);

CREATE VIRTUAL TABLE IF NOT EXISTS memory_documents_fts USING fts5(
    store_id UNINDEXED,
    filename UNINDEXED,
    content,
    content=memory_documents,
    content_rowid=rowid
);

CREATE TABLE IF NOT EXISTS memory_stores (
    id           TEXT NOT NULL PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    visibility   TEXT NOT NULL DEFAULT 'private',
    write_policy TEXT NOT NULL DEFAULT 'owner_only',
    created_by   TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL DEFAULT 0
);
`)
	return err
}

func (b *backend) NewStore(storeID, name, description string) memory.MemoryStore {
	return &store{b: b, storeID: storeID, name: name, description: description}
}

func (b *backend) CreateMeta(meta memory.StoreMeta) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, err := b.db.Exec(
		`INSERT INTO memory_stores(id,name,description,visibility,write_policy,created_by,created_at)
         VALUES(?,?,?,?,?,?,?)`,
		meta.ID, meta.Name, meta.Description, meta.Visibility, meta.WritePolicy,
		meta.CreatedBy, meta.CreatedAt,
	)
	return err
}

func (b *backend) GetMeta(id string) (memory.StoreMeta, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	row := b.db.QueryRow(
		`SELECT id,name,description,visibility,write_policy,created_by,created_at
         FROM memory_stores WHERE id=?`, id)
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
	b.mu.Lock()
	defer b.mu.Unlock()
	rows, err := b.db.Query(
		`SELECT id,name,description,visibility,write_policy,created_by,created_at
         FROM memory_stores
         WHERE created_by=? OR visibility='shared_tenant'`,
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
		// Filter shared_tenant by tenant prefix.
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
	b.mu.Lock()
	defer b.mu.Unlock()
	_, err := b.db.Exec(
		`UPDATE memory_stores SET name=?,description=?,visibility=?,write_policy=? WHERE id=?`,
		name, description, visibility, writePolicy, id,
	)
	return err
}

func (b *backend) DeleteMeta(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM memory_documents_fts WHERE store_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM memory_documents WHERE store_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM memory_stores WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (b *backend) Close() error {
	return b.db.Close()
}

// store is a MemoryStore scoped to a single storeID.
type store struct {
	b           *backend
	storeID     string
	name        string
	description string
}

func (s *store) Name() string        { return s.name }
func (s *store) Description() string { return s.description }

func (s *store) List() ([]memory.DocumentSummary, error) {
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	rows, err := s.b.db.Query(
		`SELECT filename, content FROM memory_documents WHERE store_id=? ORDER BY filename`,
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
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	row := s.b.db.QueryRow(
		`SELECT filename, content, sha256, version FROM memory_documents WHERE store_id=? AND filename=?`,
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
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	rows, err := s.b.db.Query(
		`SELECT filename, snippet(memory_documents_fts, 2, '', '', '...', 32)
         FROM memory_documents_fts
         WHERE store_id=? AND memory_documents_fts MATCH ?
         ORDER BY rank
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
	s.b.mu.Lock()
	defer s.b.mu.Unlock()

	tx, err := s.b.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// Optimistic concurrency check.
	if sha256Precondition != "" {
		var currentSHA string
		err := tx.QueryRow(
			`SELECT sha256 FROM memory_documents WHERE store_id=? AND filename=?`,
			s.storeID, filename,
		).Scan(&currentSHA)
		if err != nil && err != sql.ErrNoRows {
			return "", err
		}
		if err == nil && currentSHA != sha256Precondition {
			return "", &memory.ConflictError{CurrentSHA256: currentSHA}
		}
	}

	// Remove the stale FTS entry BEFORE updating the content table.
	// FTS5 content tables read from memory_documents when processing a delete
	// to find the tokens that need to be removed from the inverted index. If
	// we update memory_documents first, the delete reads the NEW content and
	// removes the wrong tokens, leaving old tokens permanently in the index.
	if _, err := tx.Exec(`DELETE FROM memory_documents_fts WHERE store_id=? AND filename=?`, s.storeID, filename); err != nil {
		return "", err
	}

	newSHA := hashContent(content)
	now := time.Now().Unix()

	_, err = tx.Exec(
		`INSERT INTO memory_documents(store_id,filename,content,sha256,version,created_at,updated_at)
         VALUES(?,?,?,?,1,?,?)
         ON CONFLICT(store_id,filename) DO UPDATE SET
             content=excluded.content,
             sha256=excluded.sha256,
             version=version+1,
             updated_at=excluded.updated_at`,
		s.storeID, filename, content, newSHA, now, now,
	)
	if err != nil {
		return "", err
	}

	if _, err := tx.Exec(`INSERT INTO memory_documents_fts(store_id,filename,content) VALUES(?,?,?)`, s.storeID, filename, content); err != nil {
		return "", err
	}

	return newSHA, tx.Commit()
}

func (s *store) Edit(filename, oldStr, newStr, sha256Precondition string) (string, error) {
	if err := memory.ValidateFilename(filename); err != nil {
		return "", err
	}
	s.b.mu.Lock()
	defer s.b.mu.Unlock()

	tx, err := s.b.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var currentContent, currentSHA string
	err = tx.QueryRow(
		`SELECT content, sha256 FROM memory_documents WHERE store_id=? AND filename=?`,
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

	// Remove stale FTS entry BEFORE updating the content table (same ordering
	// requirement as Write — see comment there).
	if _, err := tx.Exec(`DELETE FROM memory_documents_fts WHERE store_id=? AND filename=?`, s.storeID, filename); err != nil {
		return "", err
	}

	newSHA := hashContent(newContent)
	now := time.Now().Unix()

	if _, err := tx.Exec(
		`UPDATE memory_documents SET content=?,sha256=?,version=version+1,updated_at=? WHERE store_id=? AND filename=?`,
		newContent, newSHA, now, s.storeID, filename,
	); err != nil {
		return "", err
	}

	if _, err := tx.Exec(`INSERT INTO memory_documents_fts(store_id,filename,content) VALUES(?,?,?)`, s.storeID, filename, newContent); err != nil {
		return "", err
	}

	return newSHA, tx.Commit()
}

func (s *store) Delete(filename string) error {
	s.b.mu.Lock()
	defer s.b.mu.Unlock()

	tx, err := s.b.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM memory_documents_fts WHERE store_id=? AND filename=?`, s.storeID, filename); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM memory_documents WHERE store_id=? AND filename=?`, s.storeID, filename); err != nil {
		return err
	}
	return tx.Commit()
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
