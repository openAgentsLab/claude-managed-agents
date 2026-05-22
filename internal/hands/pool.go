package hands

import "context"

// ResourceQuota sets CPU and memory limits for a sandbox container.
// Zero values mean unlimited.
type ResourceQuota struct {
	MemoryBytes int64
	NanoCPUs    int64 // 1 CPU = 1_000_000_000
}

// AcquireRequest carries options for Pool.Acquire.
// Using a struct allows adding fields without breaking existing callers.
type AcquireRequest struct {
	Quota ResourceQuota
}

// Pool is the abstraction used by the orchestration and worker layers.
// Each driver implements it directly: LocalPool, DockerPool, K8sWatchPool.
type Pool interface {
	// Acquire returns a ready Sandbox for sessionID, creating one if necessary.
	Acquire(ctx context.Context, sessionID string, req AcquireRequest) (Sandbox, error)

	// ReleaseSession tears down the sandbox for sessionID and removes all
	// associated storage (container, workspace directory). Best-effort; errors
	// are logged but do not prevent further cleanup. No-op if not found.
	ReleaseSession(ctx context.Context, sessionID string) error

	// Isolated reports true when the pool provides execution isolation
	// (i.e. each session runs in a separate container / pod).
	Isolated() bool

	// StartBackground launches any background goroutines (heartbeat, cleanup,
	// Watch loops). Call once after pool creation with the service's root context.
	StartBackground(ctx context.Context)

	// CloseAll stops all active sandboxes and releases manager-level resources.
	CloseAll()
}
