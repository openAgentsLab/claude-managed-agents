// Package docker implements the Docker-backed per-session sandbox pool for forge.
//
// DockerPool manages per-session containers, binding
// {volumesRoot}/{sessionHash} at the fixed container path /workspace.
//
// Import this package with a blank identifier to activate the driver:
//
//	import _ "forge/internal/hands/docker"
package docker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"forge/internal/config"
	"forge/internal/gateway/store"
	"forge/internal/hands"
	"forge/internal/hands/remote"
	"forge/internal/reqctx"
	"forge/internal/resources"
)

const (
	defaultImage            = "alpine:3.20"
	forgeBinDest            = "/forge"
	toolServerContainerPort = "7777"

	readinessAttempts  = 40
	readinessInterval  = 200 * time.Millisecond
	healthCheckTimeout = 2 * time.Second

	heartbeatInterval = 30 * time.Second
	idleTimeout       = 5 * time.Minute
	idleCheckInterval = 1 * time.Minute
	// staleTTL must exceed idleTimeout so that cleanupStale does not delete
	// records before stopIdleContainers has a chance to fire. Orphaned records
	// from crashed workers (no ReleaseSession call) are cleaned up after this
	// window of inactivity.
	staleTTL        = 10 * time.Minute
	cleanupInterval = 1 * time.Minute
)

func init() {
	hands.RegisterPool(config.SandboxDriverDocker, func(_ context.Context, cfg config.SandboxConfig, deps hands.PoolDeps) (hands.Pool, error) {
		return newDockerPool(cfg, deps.Sandbox, deps.Resources)
	})
}

type dockerEntry struct {
	sandbox hands.Sandbox
	id      string // container ID
	token   string
}

// DockerPool manages per-session Docker containers and implements hands.Pool.
type DockerPool struct {
	// container config
	image    string
	network  string // docker network name used when networking is unrestricted
	forgeBin string

	// pool management
	cfg        config.SandboxConfig
	repo       store.SandboxRepository
	resRepo    store.SessionResourceRepository // may be nil when shared storage disabled
	mu         sync.Mutex
	entries    map[string]*dockerEntry // key: sessionID
	startGroup singleflight.Group     // deduplicates concurrent container starts per session
}

func newDockerPool(cfg config.SandboxConfig, repo store.SandboxRepository, resRepo store.SessionResourceRepository) (*DockerPool, error) {
	if cfg.VolumesRoot == "" {
		return nil, fmt.Errorf("docker pool requires sandbox.volumes_root; dynamic mounting and workspace persistence are unavailable without shared storage")
	}
	if !filepath.IsAbs(cfg.VolumesRoot) {
		return nil, fmt.Errorf("docker pool: sandbox.volumes_root %q must be an absolute path", cfg.VolumesRoot)
	}
	if err := os.MkdirAll(cfg.VolumesRoot, 0o755); err != nil {
		return nil, fmt.Errorf("docker pool: cannot access volumes_root %q: %w", cfg.VolumesRoot, err)
	}
	opts := cfg.Options
	return &DockerPool{
		image:    optOr(opts, "image", defaultImage),
		network:  optOr(opts, "network", "bridge"),
		forgeBin: resolveForgeBin(opts),
		cfg:      cfg,
		repo:     repo,
		resRepo:  resRepo,
		entries:  make(map[string]*dockerEntry),
	}, nil
}

func (p *DockerPool) Isolated() bool { return true }

func (p *DockerPool) StartBackground(ctx context.Context) {
	go p.heartbeatLoop(ctx)
	go p.idleTimeoutLoop(ctx)
	go p.cleanupLoop(ctx)
}

// Acquire returns a ready Sandbox for sessionID.
//
// Lookup order:
//  1. In-memory cache — healthy → return immediately (short lock + Touch).
//  2. DB record (cross-worker) — healthy → cache locally and return.
//  3. Neither found or healthy → start a new container.
//
// Steps 2-3 run outside the global lock via singleflight so that concurrent
// Acquire calls for the same session do not start duplicate containers, and
// concurrent calls for different sessions proceed in parallel.
func (p *DockerPool) Acquire(ctx context.Context, sessionID string, req hands.AcquireRequest) (hands.Sandbox, error) {
	// Fast path: in-memory cache hit under a short lock.
	p.mu.Lock()
	if e, ok := p.entries[sessionID]; ok {
		if p.healthy(ctx, e.sandbox) {
			p.mu.Unlock()
			if p.repo != nil {
				_ = p.repo.Touch(ctx, sessionID) // update last_seen; best-effort, outside lock
			}
			return e.sandbox, nil
		}
		p.evictLocked(ctx, sessionID, e)
	}
	p.mu.Unlock()

	// Slow path: DB lookup + possible container start.
	v, err, _ := p.startGroup.Do(sessionID, func() (any, error) {
		// Re-check cache: another goroutine may have just started the container.
		p.mu.Lock()
		if e, ok := p.entries[sessionID]; ok {
			if p.healthy(ctx, e.sandbox) {
				p.mu.Unlock()
				if p.repo != nil {
					_ = p.repo.Touch(ctx, sessionID)
				}
				return e.sandbox, nil
			}
			p.evictLocked(ctx, sessionID, e)
		}
		p.mu.Unlock()

		// Single DB read: get both existing container info and env spec.
		var storedEnv resources.Environment
		var storedEnvSpec string
		if p.repo != nil {
			if rec, err := p.repo.Get(ctx, sessionID); err == nil && rec != nil {
				if rec.EnvironmentSpec != "" {
					_ = json.Unmarshal([]byte(rec.EnvironmentSpec), &storedEnv)
					storedEnvSpec = rec.EnvironmentSpec
				}
				if rec.SandboxID != "" {
					// Real container record — check if it's still healthy.
					sb := remote.New(rec.Endpoint, rec.Token)
					if p.healthy(ctx, sb) {
						p.mu.Lock()
						p.entries[sessionID] = &dockerEntry{sandbox: sb, id: rec.SandboxID, token: rec.Token}
						p.mu.Unlock()
						_ = p.repo.Touch(ctx, sessionID)
						return sb, nil
					}
				}
				// Stale or stub record — remove so Upsert below writes a fresh one.
				if delErr := p.repo.Delete(ctx, sessionID); delErr != nil {
					slog.WarnContext(ctx, "docker pool: failed to delete stale DB record",
						"session_id", sessionID, "error", delErr)
				}
			}
		}

		// Start a new container with the stored environment.
		token := generateToken()
		sb, containerID, err := p.start(ctx, sessionID, token, req.Quota, storedEnv)
		if err != nil {
			return nil, err
		}
		p.mu.Lock()
		p.entries[sessionID] = &dockerEntry{sandbox: sb, id: containerID, token: token}
		p.mu.Unlock()

		if p.repo != nil {
			endpoint := ""
			if h, ok := sb.(hands.HealthEndpointer); ok {
				endpoint = h.HealthEndpoint()
			}
			if upsertErr := p.repo.Upsert(ctx, store.SandboxRecord{
				SessionID:       sessionID,
				TenantID:        reqctx.TenantIDFromContext(ctx),
				SandboxID:       containerID,
				Endpoint:        endpoint,
				Token:           token,
				LastSeen:        time.Now().Unix(),
				EnvironmentSpec: storedEnvSpec,
			}); upsertErr != nil {
				slog.WarnContext(ctx, "docker pool: failed to persist sandbox record",
					"session_id", sessionID, "error", upsertErr)
			}
		}

		// Re-materialise any dynamic resources that may be missing after a rebuild.
		p.ensureSessionResources(ctx, sessionID)
		return sb, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(hands.Sandbox), nil
}

// ReleaseSession tears down the container and workspace for sessionID.
func (p *DockerPool) ReleaseSession(ctx context.Context, sessionID string) error {
	p.mu.Lock()
	if e, ok := p.entries[sessionID]; ok {
		p.evictLocked(ctx, sessionID, e)
	}
	p.mu.Unlock()

	if p.repo != nil {
		_ = p.repo.Delete(ctx, sessionID) // best-effort
	}
	if p.resRepo != nil {
		_ = p.resRepo.DeleteBySession(ctx, sessionID) // best-effort
	}

	// Remove the workspace directory from shared or fallback storage.
	dir := p.volumeDir(sessionID, reqctx.TenantIDFromContext(ctx))
	if err := os.RemoveAll(dir); err != nil {
		slog.WarnContext(ctx, "docker pool: failed to remove workspace dir",
			"session_id", sessionID, "dir", dir, "error", err)
	}
	return nil
}

// SetSessionEnvironment stores the resolved Environment for sessionID so it is
// applied the next time a container starts. Implements hands.SessionEnvSetter.
func (p *DockerPool) SetSessionEnvironment(ctx context.Context, sessionID string, env resources.Environment) error {
	return hands.SetEnvSpec(ctx, p.repo, sessionID, env)
}

// CloseAll stops every container in the pool.
func (p *DockerPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	ctx := context.Background()
	for sessionID, e := range p.entries {
		_ = executeDockerRemove(e.id)
		if p.repo != nil {
			_ = p.repo.Delete(ctx, sessionID) // best-effort
		}
		delete(p.entries, sessionID)
	}
}

func (p *DockerPool) evictLocked(ctx context.Context, sessionID string, e *dockerEntry) {
	_ = executeDockerRemove(e.id)
	if p.repo != nil {
		_ = p.repo.Delete(ctx, sessionID) // best-effort
	}
	delete(p.entries, sessionID)
}

// sessionHash returns a short filesystem-safe identifier for sessionID.
// SHA-256 truncated to 16 hex characters (64 bits) is collision-resistant
// enough for workspace directory naming.
func sessionHash(sessionID string) string {
	h := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(h[:8])
}

// volumeDir returns the host-side workspace directory for sessionID under VolumesRoot.
func (p *DockerPool) volumeDir(sessionID, tenantID string) string {
	if tenantID != "" {
		return filepath.Join(p.cfg.VolumesRoot, tenantID, sessionHash(sessionID))
	}
	return filepath.Join(p.cfg.VolumesRoot, sessionHash(sessionID))
}

// ensureSessionResources re-materialises any dynamic resource declarations
// stored in DB that are missing from the workspace directory. Called after a
// new container is created so that container rebuilds are transparent.
func (p *DockerPool) ensureSessionResources(ctx context.Context, sessionID string) {
	if p.resRepo == nil {
		return
	}
	recs, err := p.resRepo.List(ctx, sessionID)
	if err != nil {
		slog.WarnContext(ctx, "docker pool: list session resources failed",
			"session_id", sessionID, "error", err)
		return
	}
	dir := p.volumeDir(sessionID, reqctx.TenantIDFromContext(ctx))
	for _, rec := range recs {
		target, err := resources.SafeJoin(dir, rec.TargetPath)
		if err != nil {
			slog.WarnContext(ctx, "docker pool: skip resource with unsafe path",
				"session_id", sessionID, "target_path", rec.TargetPath)
			continue
		}
		if _, err := os.Stat(target); err == nil {
			continue // already present
		}
		switch rec.Type {
		case "file":
			p.restoreFileResource(ctx, sessionID, rec, target)
		case "git":
			p.restoreGitResource(ctx, sessionID, rec, target)
		}
	}
}

func (p *DockerPool) restoreFileResource(ctx context.Context, sessionID string, rec store.SessionResourceRecord, target string) {
	// Volume is a host bind-mount and survives container restarts; reaching here
	// means the workspace directory was lost (e.g. host migration). Content is
	// not stored in DB, so the file must be re-added via the resources API.
	slog.WarnContext(ctx, "docker pool: file resource missing after rebuild — re-add via API",
		"session_id", sessionID, "id", rec.ID, "target_path", rec.TargetPath)
}

func (p *DockerPool) restoreGitResource(ctx context.Context, sessionID string, rec store.SessionResourceRecord, target string) {
	// Token is not stored in DB (security). Without it we cannot re-clone
	// private repos on rebuild. Log a warning so operators know they need to
	// re-add the resource manually if the workspace was lost.
	var spec struct {
		URL    string `json:"url"`
		Branch string `json:"branch"`
	}
	if err := json.Unmarshal([]byte(rec.Spec), &spec); err != nil {
		slog.WarnContext(ctx, "docker pool: cannot restore git resource — bad spec",
			"session_id", sessionID, "id", rec.ID, "error", err)
		return
	}
	slog.WarnContext(ctx, "docker pool: git resource missing after rebuild — token not stored, re-add via API",
		"session_id", sessionID, "url", spec.URL, "target", target)
}

// sharedStorageAvailable reports whether volumes_root is configured and the
// directory is still accessible on the host filesystem.
func (p *DockerPool) sharedStorageAvailable() bool {
	if p.cfg.VolumesRoot == "" {
		return false
	}
	_, err := os.Stat(p.cfg.VolumesRoot)
	return err == nil
}

