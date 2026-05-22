package docker

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"forge/internal/hands"
	"forge/internal/reqctx"
)

const outputsSubdir = "outputs"

// ListOutputs lists all files under the session outputs directory.
// Requires shared storage (volumes_root configured and writable).
func (p *DockerPool) ListOutputs(ctx context.Context, sessionID string) ([]hands.OutputEntry, error) {
	if !p.sharedStorageAvailable() {
		return nil, hands.ErrSharedStorageUnavailable
	}
	dir := filepath.Join(p.volumeDir(sessionID, reqctx.TenantIDFromContext(ctx)), outputsSubdir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil // outputs dir not created yet — return empty list
	}

	var entries []hands.OutputEntry
	if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		info, err := d.Info()
		if err != nil {
			return nil // skip unreadable entries
		}
		entries = append(entries, hands.OutputEntry{Path: rel, Size: info.Size()})
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list outputs: %w", err)
	}
	return entries, nil
}

// ReadOutput returns the contents of path (relative to the outputs directory).
func (p *DockerPool) ReadOutput(ctx context.Context, sessionID, path string) ([]byte, error) {
	if !p.sharedStorageAvailable() {
		return nil, hands.ErrSharedStorageUnavailable
	}
	if err := validateOutputPath(path); err != nil {
		return nil, err
	}
	dir := filepath.Join(p.volumeDir(sessionID, reqctx.TenantIDFromContext(ctx)), outputsSubdir)
	full := filepath.Join(dir, path)
	// Ensure the resolved path stays within the outputs directory.
	if !strings.HasPrefix(full, dir+string(filepath.Separator)) {
		return nil, fmt.Errorf("path %q escapes outputs directory", path)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("read output %q: %w", path, err)
	}
	return data, nil
}

func validateOutputPath(path string) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("output path must be relative, got %q", path)
	}
	if strings.HasPrefix(filepath.Clean(path), "..") {
		return fmt.Errorf("output path %q escapes outputs directory", path)
	}
	return nil
}
