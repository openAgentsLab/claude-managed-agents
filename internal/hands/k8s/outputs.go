package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"forge/internal/config"
	"forge/internal/hands"
)

func outputsContainerPath() string {
	return filepath.Join(config.ContainerWorkspaceRoot, "outputs")
}

// ListOutputs lists files in /workspace/outputs/ by running find inside the pod.
// The pod is started if it is idle.
func (p *K8sWatchPool) ListOutputs(ctx context.Context, sessionID string) ([]hands.OutputEntry, error) {
	sb, err := p.acquireForResource(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list outputs: %w", err)
	}
	cmd := fmt.Sprintf("find %q -type f -printf '%%P\t%%s\n' 2>/dev/null || true", outputsContainerPath())
	out, err := sb.Execute(ctx, "bash", bashInput(cmd))
	if err != nil {
		return nil, fmt.Errorf("list outputs: execute: %w", err)
	}
	return parseOutputListing(out), nil
}

// ReadOutput reads the contents of path (relative to outputs/) from inside the pod.
func (p *K8sWatchPool) ReadOutput(ctx context.Context, sessionID, path string) ([]byte, error) {
	if err := validateOutputPath(path); err != nil {
		return nil, err
	}
	sb, err := p.acquireForResource(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("read output: %w", err)
	}
	absPath := filepath.Join(outputsContainerPath(), path)
	out, err := sb.Execute(ctx, "bash", bashInput(fmt.Sprintf("base64 %q", absPath)))
	if err != nil {
		return nil, fmt.Errorf("read output %q: %w", path, err)
	}
	data, decErr := base64.StdEncoding.DecodeString(strings.TrimSpace(out))
	if decErr != nil {
		return nil, fmt.Errorf("read output %q: base64 decode: %w", path, decErr)
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

func parseOutputListing(raw string) []hands.OutputEntry {
	var entries []hands.OutputEntry
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 || parts[0] == "" {
			continue
		}
		size, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		entries = append(entries, hands.OutputEntry{Path: parts[0], Size: size})
	}
	return entries
}
