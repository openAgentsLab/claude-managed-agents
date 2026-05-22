package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"forge/internal/config"
	"forge/internal/gateway/store"
	"forge/internal/hands"
	"forge/internal/hands/remote"
	"forge/internal/resources"
)

const (
	watchReconnectDelay = 5 * time.Second
	acquireReadyTimeout = 90 * time.Second
	acquirePollInterval = 2 * time.Second

	sandboxLabelSelector = labelApp + "=" + labelAppVal
)

// k8sSandboxEntry holds the stable metadata for a session's sandbox.
// Token and serviceURL are fixed at creation and survive pod restarts.
type k8sSandboxEntry struct {
	token      string
	serviceURL string
	ready      bool      // updated by the Pod Watch goroutine
	lastSeen   time.Time // updated by Acquire; used by idle cleanup
}

// K8sWatchPool manages per-session k8s sandboxes using Pod + Service + PVC.
// Each session gets a dedicated Pod (RestartPolicy: Never), a stable ClusterIP
// Service, and a PVC. Background cleanup runs in two tiers:
//   - podIdleTimeout:     idle pod deleted, Service+PVC kept for reconnect
//   - sessionIdleTimeout: Service+PVC deleted, session fully released
type K8sWatchPool struct {
	client             kubernetes.Interface
	namespace          string
	image              string
	storageClass       string
	workspaceSize      string
	serviceAccount     string
	podIdleTimeout     time.Duration
	sessionIdleTimeout time.Duration
	repo               store.SandboxRepository        // optional; packages spec + networking spec persistence
	resRepo            store.SessionResourceRepository // optional; dynamic resource declarations

	mu    sync.Mutex
	cache map[string]*k8sSandboxEntry // key = sanitizeName(sessionID)

	stopBg context.CancelFunc // cancels background goroutines; set by StartBackground
}

func newK8sWatchPool(cfg config.SandboxConfig, repo store.SandboxRepository, resRepo store.SessionResourceRepository) (*K8sWatchPool, error) {
	opts := cfg.Options
	client, err := buildClient(opts)
	if err != nil {
		return nil, fmt.Errorf("k8s watch pool: build client: %w", err)
	}
	podIdleTimeout := defaultPodIdleTimeout
	if v := opts["pod_idle_timeout"]; v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			podIdleTimeout = d
		}
	}
	sessionIdleTimeout := defaultSessionIdleTimeout
	if v := opts["session_idle_timeout"]; v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			sessionIdleTimeout = d
		}
	}
	return &K8sWatchPool{
		client:             client,
		namespace:          optOr(opts, "namespace", defaultNamespace),
		image:              optOr(opts, "image", defaultImage),
		storageClass:       opts["storage_class"],
		workspaceSize:      optOr(opts, "workspace_size", defaultWorkspaceSize),
		serviceAccount:     opts["service_account"],
		podIdleTimeout:     podIdleTimeout,
		sessionIdleTimeout: sessionIdleTimeout,
		repo:               repo,
		resRepo:            resRepo,
		cache:          make(map[string]*k8sSandboxEntry),
	}, nil
}

// ── Pool interface ─────────────────────────────────────────────────────────

func (p *K8sWatchPool) Isolated() bool { return true }

// StartBackground runs the initial list-sync then launches Pod Watch and idle
// cleanup goroutines. Call once with the service's root context.
func (p *K8sWatchPool) StartBackground(ctx context.Context) {
	bgCtx, cancel := context.WithCancel(ctx)
	p.stopBg = cancel
	p.initialSync(bgCtx)
	go p.runPodWatch(bgCtx)
	go p.runIdleCleanup(bgCtx)
}

// CloseAll stops background goroutines. Pods, Services, and PVCs are left
// intact so that sandboxes survive worker restarts.
func (p *K8sWatchPool) CloseAll() {
	if p.stopBg != nil {
		p.stopBg()
	}
}

// ReleaseSession deletes the Pod, Service, NetworkPolicy (if any), and PVC for
// sessionID. Best-effort: errors are logged but do not block the caller.
func (p *K8sWatchPool) ReleaseSession(ctx context.Context, sessionID string) error {
	key := sanitizeName(sessionID)

	p.mu.Lock()
	delete(p.cache, key)
	p.mu.Unlock()

	p.releaseSandboxResources(ctx, key)
	if p.repo != nil {
		_ = p.repo.Delete(ctx, sessionID) // best-effort
	}
	return nil
}

// releaseSandboxResources deletes the Pod, NetworkPolicy, Service, and PVC for
// key. Best-effort: each deletion is attempted independently and errors are logged.
func (p *K8sWatchPool) releaseSandboxResources(ctx context.Context, key string) {
	if err := p.deletePod(ctx, key); err != nil {
		slog.WarnContext(ctx, "k8s release: delete pod", "key", key, "error", err)
	}
	p.deleteNetworkPolicy(ctx, key)

	svcName := sandboxName(key)
	if err := p.client.CoreV1().Services(p.namespace).Delete(ctx, svcName, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		slog.WarnContext(ctx, "k8s release: delete service", "key", key, "error", err)
	}
	pvcName := workspacePVCName(key)
	if err := p.client.CoreV1().PersistentVolumeClaims(p.namespace).Delete(ctx, pvcName, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		slog.WarnContext(ctx, "k8s release: delete pvc", "key", key, "error", err)
	}
}

// Acquire returns a ready Sandbox for sessionID. If no pod exists it creates
// Pod + Service + PVC, then waits for the pod to become ready.
func (p *K8sWatchPool) Acquire(ctx context.Context, sessionID string, req hands.AcquireRequest) (hands.Sandbox, error) {
	key := sanitizeName(sessionID)

	p.mu.Lock()
	if e, ok := p.cache[key]; ok && e.ready {
		e.lastSeen = time.Now()
		sb := remote.New(e.serviceURL, e.token)
		p.mu.Unlock()
		return sb, nil
	}
	_, exists := p.cache[key]
	p.mu.Unlock()

	if !exists {
		env := p.loadEnvironment(ctx, sessionID)

		token := generateToken()
		e := &k8sSandboxEntry{
			token:      token,
			serviceURL: p.serviceURL(key),
			ready:      false,
		}

		p.mu.Lock()
		// Re-check after acquiring lock to avoid double-create.
		if _, alreadyIn := p.cache[key]; !alreadyIn {
			p.cache[key] = e
			p.mu.Unlock()

			if err := p.createSandbox(ctx, key, token, env, req.Quota); err != nil {
				p.mu.Lock()
				if cur, ok := p.cache[key]; ok && cur == e {
					delete(p.cache, key)
				}
				p.mu.Unlock()
				return nil, fmt.Errorf("acquire sandbox for %q: %w", sessionID, err)
			}
		} else {
			p.mu.Unlock()
		}
	}

	return p.waitReady(ctx, key)
}

// SetSessionEnvironment stores the resolved Environment for sessionID so it is
// applied the next time a pod starts. Implements hands.SessionEnvSetter.
func (p *K8sWatchPool) SetSessionEnvironment(ctx context.Context, sessionID string, env resources.Environment) error {
	return hands.SetEnvSpec(ctx, p.repo, sessionID, env)
}

func (p *K8sWatchPool) loadEnvironment(ctx context.Context, sessionID string) resources.Environment {
	env, _ := hands.LoadEnvSpec(ctx, p.repo, sessionID)
	return env
}

func (p *K8sWatchPool) waitReady(ctx context.Context, key string) (hands.Sandbox, error) {
	deadline := time.Now().Add(acquireReadyTimeout)
	for {
		p.mu.Lock()
		e, ok := p.cache[key]
		if ok && e.ready {
			e.lastSeen = time.Now()
			sb := remote.New(e.serviceURL, e.token)
			p.mu.Unlock()
			return sb, nil
		}
		p.mu.Unlock()

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("sandbox %q not ready after %s", key, acquireReadyTimeout)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(acquirePollInterval):
		}
	}
}


// deletePod force-deletes the pod for key. No-op if not found.
func (p *K8sWatchPool) deletePod(ctx context.Context, key string) error {
	grace := int64(0)
	err := p.client.CoreV1().Pods(p.namespace).Delete(ctx, sandboxName(key), metav1.DeleteOptions{
		GracePeriodSeconds: &grace,
	})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (p *K8sWatchPool) serviceURL(key string) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
		sandboxName(key), p.namespace, toolServerPort)
}

