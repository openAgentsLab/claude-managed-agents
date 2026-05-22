package fs

import (
	"fmt"
	"path/filepath"
)

// safeJoinAny accepts either an absolute or relative path. Relative paths are resolved against root.
func safeJoinAny(root, pathInput string) (string, error) {
	p := filepath.Clean(pathInput)
	if p == "" {
		p = root
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	return filepath.Clean(p), nil
}

// safeJoinAbsolute accepts only absolute paths.
func safeJoinAbsolute(root, abs string) (string, error) {
	if !filepath.IsAbs(abs) {
		return "", fmt.Errorf("path must be absolute, got: %s", abs)
	}
	return filepath.Clean(abs), nil
}
