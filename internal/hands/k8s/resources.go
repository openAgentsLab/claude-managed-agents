package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"forge/internal/config"
	"forge/internal/gateway/store"
	"forge/internal/hands"
	"forge/internal/hands/remote"
	"forge/internal/resources"
)

const resourceExecTimeout = 120_000 // milliseconds

// AddFileResource writes r.Content to the session workspace by executing a
// bash command inside the sandbox pod. The pod is started if it is idle.
// Only metadata is persisted to DB; content is not stored because the PVC
// survives pod deletion and files remain accessible on reconnect.
func (p *K8sWatchPool) AddFileResource(ctx context.Context, sessionID string, r resources.FileResource) error {
	targetAbs, err := resources.SafeJoin(config.ContainerWorkspaceRoot, r.TargetPath)
	if err != nil {
		return err
	}

	sb, err := p.acquireForResource(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("add file resource: acquire sandbox: %w", err)
	}
	var cmd string
	if r.SourceURL != "" {
		cmd = fmt.Sprintf(
			"mkdir -p %q && wget -q -O %q %q",
			filepath.Dir(targetAbs), targetAbs, r.SourceURL,
		)
	} else {
		encoded := base64.StdEncoding.EncodeToString(r.Content)
		cmd = fmt.Sprintf(
			"mkdir -p %q && printf '%%s' %q | base64 -d > %q",
			filepath.Dir(targetAbs), encoded, targetAbs,
		)
	}
	if _, err := sb.Execute(ctx, "bash", bashInput(cmd)); err != nil {
		return fmt.Errorf("add file resource: write to pod: %w", err)
	}

	if p.resRepo != nil {
		_ = p.resRepo.Upsert(ctx, store.SessionResourceRecord{
			ID:         r.ID,
			SessionID:  sessionID,
			Type:       "file",
			TargetPath: r.TargetPath,
			Spec:       "{}",
			CreatedAt:  time.Now().Unix(),
		})
	}
	return nil
}

// AddGitResource clones a repository into the session workspace by running
// git clone inside the sandbox pod. Token is not stored in DB.
func (p *K8sWatchPool) AddGitResource(ctx context.Context, sessionID string, r resources.GitResource) error {
	targetAbs, err := resources.SafeJoin(config.ContainerWorkspaceRoot, r.TargetPath)
	if err != nil {
		return err
	}

	sb, err := p.acquireForResource(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("add git resource: acquire sandbox: %w", err)
	}

	cloneURL := r.URL
	if r.Token != "" {
		cloneURL = resources.EmbedToken(r.URL, r.Token)
	}
	args := fmt.Sprintf("--depth=1")
	if r.Branch != "" {
		args += fmt.Sprintf(" --branch %q", r.Branch)
	}
	cmd := fmt.Sprintf(
		"mkdir -p %q && git clone %s %q %q && git -C %q remote remove origin",
		filepath.Dir(targetAbs), args, cloneURL, targetAbs, targetAbs,
	)
	if _, err := sb.Execute(ctx, "bash", bashInput(cmd)); err != nil {
		return fmt.Errorf("add git resource: clone in pod: %w", err)
	}

	if p.resRepo != nil {
		spec, _ := json.Marshal(struct {
			URL    string `json:"url"`
			Branch string `json:"branch"`
		}{URL: r.URL, Branch: r.Branch})
		_ = p.resRepo.Upsert(ctx, store.SessionResourceRecord{
			ID:         r.ID,
			SessionID:  sessionID,
			Type:       "git",
			TargetPath: r.TargetPath,
			Spec:       string(spec),
			CreatedAt:  time.Now().Unix(),
		})
	}
	return nil
}

// RemoveResource deletes the resource path from the workspace and removes the
// DB record.
func (p *K8sWatchPool) RemoveResource(ctx context.Context, sessionID, resourceID string) error {
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
		if targetAbs, pathErr := resources.SafeJoin(config.ContainerWorkspaceRoot, rec.TargetPath); pathErr == nil {
			if sb, sbErr := p.acquireForResource(ctx, sessionID); sbErr == nil {
				cmd := fmt.Sprintf("rm -rf %q", targetAbs)
				_, _ = sb.Execute(ctx, "bash", bashInput(cmd)) // best-effort
			}
		}
		return p.resRepo.Delete(ctx, resourceID)
	}
	return nil
}

// acquireForResource returns a RemoteSandbox for the session, starting the pod
// if it is currently idle. Using a short timeout context avoids blocking
// long-running requests when the pod is slow to start.
func (p *K8sWatchPool) acquireForResource(ctx context.Context, sessionID string) (hands.Sandbox, error) {
	key := sanitizeName(sessionID)

	// Fast path: pod is already up and in the cache.
	p.mu.Lock()
	if e, ok := p.cache[key]; ok && e.ready {
		sb := remote.New(e.serviceURL, e.token)
		p.mu.Unlock()
		return sb, nil
	}
	p.mu.Unlock()

	// Slow path: delegate to Acquire which will start the pod if needed.
	return p.Acquire(ctx, sessionID, hands.AcquireRequest{})
}

type bashRequest struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

func bashInput(cmd string) json.RawMessage {
	b, _ := json.Marshal(bashRequest{Command: cmd, Timeout: resourceExecTimeout})
	return b
}
