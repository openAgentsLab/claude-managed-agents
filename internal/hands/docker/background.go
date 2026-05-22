package docker

import (
	"context"
	"log/slog"
	"maps"
	"os/exec"
	"sync"
	"time"
)

func (p *DockerPool) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.heartbeat(ctx)
		}
	}
}

// heartbeat health-checks every cached container concurrently. Unhealthy
// entries are evicted. last_seen is updated exclusively in Acquire.
func (p *DockerPool) heartbeat(ctx context.Context) {
	p.mu.Lock()
	snapshot := maps.Clone(p.entries)
	p.mu.Unlock()

	var wg sync.WaitGroup
	for sessionID, e := range snapshot {
		wg.Add(1)
		go func(sid string, entry *dockerEntry) {
			defer wg.Done()
			if !p.healthy(ctx, entry.sandbox) {
				slog.InfoContext(ctx, "docker heartbeat: evicting unhealthy sandbox", "session_id", sid)
				p.mu.Lock()
				if cur, ok := p.entries[sid]; ok && cur == entry {
					p.evictLocked(ctx, sid, entry)
				}
				p.mu.Unlock()
			}
		}(sessionID, e)
	}
	wg.Wait()
}

func (p *DockerPool) idleTimeoutLoop(ctx context.Context) {
	ticker := time.NewTicker(idleCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.stopIdleContainers(ctx)
		}
	}
}

// stopIdleContainers stops containers whose last_seen is older than idleTimeout.
// The workspace directory and DB record are preserved so that the next Acquire
// for the same session can rebuild the container transparently.
func (p *DockerPool) stopIdleContainers(ctx context.Context) {
	p.mu.Lock()
	snapshot := maps.Clone(p.entries)
	p.mu.Unlock()

	cutoff := time.Now().Add(-idleTimeout).Unix()
	for sessionID, e := range snapshot {
		if p.repo == nil {
			continue
		}
		rec, err := p.repo.Get(ctx, sessionID)
		if err != nil || rec == nil {
			continue
		}
		if rec.LastSeen >= cutoff {
			continue // still active
		}
		slog.InfoContext(ctx, "docker idle-timeout: stopping idle container",
			"session_id", sessionID, "last_seen", rec.LastSeen)
		p.mu.Lock()
		if cur, ok := p.entries[sessionID]; ok && cur == e {
			// Stop container but keep DB record and workspace for rebuild.
			_ = executeDockerRemove(e.id)
			delete(p.entries, sessionID)
		}
		p.mu.Unlock()
	}
}

func (p *DockerPool) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.cleanupStale(ctx)
		}
	}
}

// cleanupStale removes DB records that are older than staleTTL. These
// correspond to sandboxes where the worker process died without calling
// ReleaseSession, leaving orphaned records.
func (p *DockerPool) cleanupStale(ctx context.Context) {
	if p.repo == nil {
		return
	}
	cutoff := time.Now().Add(-staleTTL)
	n, err := p.repo.DeleteStaleBefore(ctx, cutoff)
	if err != nil {
		slog.WarnContext(ctx, "docker cleanup: delete stale failed", "error", err)
		return
	}
	if n > 0 {
		slog.InfoContext(ctx, "docker cleanup: removed stale records", "count", n)
	}
}

// executeDockerRemove is a thin wrapper so it can be overridden in tests.
var executeDockerRemove = func(containerID string) error {
	return exec.Command("docker", "rm", "-f", containerID).Run()
}
