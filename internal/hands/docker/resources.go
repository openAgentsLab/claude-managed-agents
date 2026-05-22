package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"forge/internal/gateway/store"
	"forge/internal/hands"
	"forge/internal/reqctx"
	"forge/internal/resources"
)

// AddFileResource writes r.Content to the session workspace and persists the
// resource declaration (metadata only, no content) to DB. Container-side
// visibility is immediate because the workspace directory is bind-mounted.
func (p *DockerPool) AddFileResource(ctx context.Context, sessionID string, r resources.FileResource) error {
	if !p.sharedStorageAvailable() {
		return hands.ErrSharedStorageUnavailable
	}

	dst, err := resources.SafeJoin(p.volumeDir(sessionID, reqctx.TenantIDFromContext(ctx)), r.TargetPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir for file resource: %w", err)
	}
	if r.SourceURL != "" {
		if err := downloadToFile(ctx, r.SourceURL, dst); err != nil {
			return fmt.Errorf("fetch file resource: %w", err)
		}
	} else {
		if err := os.WriteFile(dst, r.Content, 0o644); err != nil {
			return fmt.Errorf("write file resource: %w", err)
		}
	}

	if p.resRepo != nil {
		if err := p.resRepo.Upsert(ctx, store.SessionResourceRecord{
			ID:         r.ID,
			SessionID:  sessionID,
			Type:       "file",
			TargetPath: r.TargetPath,
			Spec:       "{}",
			CreatedAt:  time.Now().Unix(),
		}); err != nil {
			return fmt.Errorf("persist file resource: %w", err)
		}
	}
	return nil
}

// AddGitResource clones the repository into the session workspace on the
// orchestration side. The token is used for cloning but never stored in DB.
func (p *DockerPool) AddGitResource(ctx context.Context, sessionID string, r resources.GitResource) error {
	if !p.sharedStorageAvailable() {
		return hands.ErrSharedStorageUnavailable
	}

	dst, err := resources.SafeJoin(p.volumeDir(sessionID, reqctx.TenantIDFromContext(ctx)), r.TargetPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir for git resource: %w", err)
	}

	if err := cloneRepo(ctx, r, dst); err != nil {
		return err
	}

	if p.resRepo != nil {
		// Token is intentionally omitted from the persisted spec.
		spec, err := json.Marshal(struct {
			URL    string `json:"url"`
			Branch string `json:"branch"`
		}{URL: r.URL, Branch: r.Branch})
		if err != nil {
			return fmt.Errorf("marshal git resource spec: %w", err)
		}
		if err := p.resRepo.Upsert(ctx, store.SessionResourceRecord{
			ID:         r.ID,
			SessionID:  sessionID,
			Type:       "git",
			TargetPath: r.TargetPath,
			Spec:       string(spec),
			CreatedAt:  time.Now().Unix(),
		}); err != nil {
			return fmt.Errorf("persist git resource: %w", err)
		}
	}
	return nil
}

// RemoveResource removes the resource with the given ID from the workspace
// and from the DB.
func (p *DockerPool) RemoveResource(ctx context.Context, sessionID, resourceID string) error {
	if p.resRepo == nil {
		return nil
	}

	recs, err := p.resRepo.List(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("list resources: %w", err)
	}

	for _, rec := range recs {
		if rec.ID != resourceID {
			continue
		}
		// Best-effort filesystem removal.
		if target, err := resources.SafeJoin(p.volumeDir(sessionID, reqctx.TenantIDFromContext(ctx)), rec.TargetPath); err == nil {
			_ = os.RemoveAll(target)
		}

		return p.resRepo.Delete(ctx, resourceID)
	}
	return nil // no-op if not found
}

func downloadToFile(ctx context.Context, rawURL, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// cloneRepo clones r.URL at r.Branch into dst. The token is embedded in the
// URL so it never enters the container environment. The remote origin is
// removed after cloning to scrub the token from git config.
func cloneRepo(ctx context.Context, r resources.GitResource, dst string) error {
	cloneURL := r.URL
	if r.Token != "" {
		cloneURL = resources.EmbedToken(r.URL, r.Token)
	}

	args := []string{"clone", "--depth=1"}
	if r.Branch != "" {
		args = append(args, "--branch", r.Branch)
	}
	args = append(args, cloneURL, dst)

	cmd := exec.CommandContext(ctx, "git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, out)
	}

	// Scrub token: remove origin so it's not accessible from inside the container.
	scrub := exec.CommandContext(ctx, "git", "-C", dst, "remote", "remove", "origin")
	_ = scrub.Run() // best-effort
	return nil
}
