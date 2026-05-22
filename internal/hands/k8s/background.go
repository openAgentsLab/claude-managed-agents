package k8s

import (
	"context"
	"log/slog"
	"time"
)

const idleCheckInterval = 5 * time.Minute

func (p *K8sWatchPool) runIdleCleanup(ctx context.Context) {
	ticker := time.NewTicker(idleCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.cleanupIdlePods(ctx)
		}
	}
}

// cleanupIdlePods runs two-tier idle cleanup:
//   - pod tier:     idle > podIdleTimeout     → delete Pod only; Service+PVC kept for reconnect
//   - session tier: idle > sessionIdleTimeout → delete Pod+Service+PVC and remove from cache
func (p *K8sWatchPool) cleanupIdlePods(ctx context.Context) {
	now := time.Now()
	podCutoff := now.Add(-p.podIdleTimeout)
	sessionCutoff := now.Add(-p.sessionIdleTimeout)

	p.mu.Lock()
	var podStale, sessionStale []string
	for key, e := range p.cache {
		if e.lastSeen.IsZero() {
			continue
		}
		if e.lastSeen.Before(sessionCutoff) {
			sessionStale = append(sessionStale, key)
		} else if e.lastSeen.Before(podCutoff) {
			podStale = append(podStale, key)
		}
	}
	p.mu.Unlock()

	for _, key := range sessionStale {
		p.releaseSandboxResources(ctx, key)
		slog.InfoContext(ctx, "k8s idle cleanup: released idle session", "key", key)
		p.mu.Lock()
		delete(p.cache, key)
		p.mu.Unlock()
	}

	for _, key := range podStale {
		if err := p.deletePod(ctx, key); err != nil {
			slog.WarnContext(ctx, "k8s idle cleanup: delete pod", "key", key, "error", err)
			continue
		}
		slog.InfoContext(ctx, "k8s idle cleanup: deleted idle pod", "key", key)
	}
}
