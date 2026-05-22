package orchestration

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/callbacks"

	"forge/internal/config"
	einocallbacks "forge/internal/eino/callbacks"
	"forge/internal/eventbus"
	appstore "forge/internal/gateway/store"
	"forge/internal/hands"
	"forge/internal/harness"
	"forge/internal/memory"
	"forge/internal/observability"
	"forge/internal/permission"
)

// Serve assembles all serve-mode components and runs the HTTP API until ctx
// is cancelled. It is the sole entry-point for the "forge serve" command;
// main.go is responsible only for flag parsing and calling this function.
func Serve(ctx context.Context, cfg *config.Config, addr string) error {
	// ── 1. Observability ──────────────────────────────────────────────────────
	observability.InitLogger(cfg.Log)
	callbacks.AppendGlobalHandlers(einocallbacks.NewLogHandler(slog.Default()))

	// ── 3. App store — tenant / user / session / sandbox metadata ─────────────
	appStore, err := appstore.Open(cfg.Store.DriverOrDefault(), cfg.Store.Options)
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}
	defer appStore.Close()

	// ── 4. Memory pool — optional external memory backend (nil = disabled) ─────
	memPool, err := memory.NewPool(cfg.Memory)
	if err != nil {
		return fmt.Errorf("init memory: %w", err)
	}
	if memPool != nil {
		defer memPool.Close()
	}

	// ── 6. Sandbox layer — tool registry + sandbox pool + platform MCP ─────────
	// toolReg is the base registry shared by all sessions; per-session custom
	// MCP servers are merged in at session creation time by the resolver.
	toolReg, sandboxPool, cleanup, err := hands.BuildSandboxLayer(ctx, cfg.Sandbox, cfg.Tools, appStore.Sandboxes(), appStore.SessionResources())
	if err != nil {
		return err
	}
	defer cleanup()
	defer sandboxPool.CloseAll()
	if !sandboxPool.Isolated() {
		slog.Warn("serve mode running with local sandbox — tool execution is NOT isolated; set sandbox.driver: docker for production")
	}

	// ── 7. Event bus — pub/sub for SSE event distribution ────────────────────
	eb, err := buildEventBus(cfg)
	if err != nil {
		return fmt.Errorf("init event bus: %w", err)
	}

	// ── 7b. Pending store — cross-node custom-tool / HITL result delivery ─────
	ps, err := buildPendingStore(cfg)
	if err != nil {
		return fmt.Errorf("init pending store: %w", err)
	}

	// ── 8. HTTP orchestrator — harness, session store, permission engines ──────
	orch, err := NewHTTP(ctx, cfg, memPool, sandboxPool, appStore, toolReg, eb, ps)
	if err != nil {
		return err
	}

	// ── 9. HTTP server ────────────────────────────────────────────────────────
	return orch.Run(ctx, addr)
}

// buildEventBus constructs the EventBus selected by cfg.EventBus.
// "memory" (default) uses an in-process implementation; "redis" uses Redis pub/sub
// for cross-node SSE event distribution and interrupt propagation.
func buildEventBus(cfg *config.Config) (eventbus.EventBus, error) {
	switch cfg.EventBus.DriverOrDefault() {
	case "redis":
		name := cfg.EventBus.RedisNameOrDefault()
		inst, ok := cfg.Storage.Redis[name]
		if !ok {
			return nil, fmt.Errorf("event_bus.redis: instance %q not found in storage.redis", name)
		}
		return eventbus.NewRedis(inst.Addr, inst.Password, inst.DB)
	default:
		return eventbus.NewMemory(), nil
	}
}

// buildPendingStore constructs the PendingStore selected by cfg.EventBus.
// Uses Redis when the event bus driver is "redis" (same instance), otherwise
// falls back to the single-node in-memory implementation.
func buildPendingStore(cfg *config.Config) (harness.PendingStore, error) {
	switch cfg.EventBus.DriverOrDefault() {
	case "redis":
		name := cfg.EventBus.RedisNameOrDefault()
		inst, ok := cfg.Storage.Redis[name]
		if !ok {
			return nil, fmt.Errorf("pending_store: redis instance %q not found in storage.redis", name)
		}
		return eventbus.NewRedisPending(inst.Addr, inst.Password, inst.DB)
	default:
		return eventbus.NewMemoryPending(), nil
	}
}

// buildTenantEngine creates an Engine loaded with the tenant's rules layered on
// top of the global PermissionConfig rules. The engine always starts in
// ModeDefault; per-request mode is applied at run time via the resolver.
func buildTenantEngine(settings config.TenantSettings, cfg *config.Config) *permission.Engine {
	eng := permission.NewEngine(permission.ModeDefault)

	// Global platform-level rules.
	for _, s := range cfg.Permission.AllowRules {
		if r, err := permission.ParseRuleString(s, permission.BehaviorAllow, permission.SourceProject); err == nil {
			eng.AddRule(r)
		}
	}
	for _, s := range cfg.Permission.DenyRules {
		if r, err := permission.ParseRuleString(s, permission.BehaviorDeny, permission.SourceProject); err == nil {
			eng.AddRule(r)
		}
	}

	// Tenant-level rules (higher priority than global rules).
	for _, s := range settings.AllowRules {
		if r, err := permission.ParseRuleString(s, permission.BehaviorAllow, permission.SourceCLIArg); err == nil {
			eng.AddRule(r)
		}
	}
	for _, s := range settings.DenyRules {
		if r, err := permission.ParseRuleString(s, permission.BehaviorDeny, permission.SourceCLIArg); err == nil {
			eng.AddRule(r)
		}
	}

	return eng
}

// buildViewerEngine creates a plan-mode (read-only) Engine for the viewer role.
// Viewers can never execute write operations regardless of tenant settings.
func buildViewerEngine(cfg *config.Config) *permission.Engine {
	eng := permission.NewEngine(permission.ModePlan)

	// Still respect global deny rules (belt-and-suspenders).
	for _, s := range cfg.Permission.DenyRules {
		if r, err := permission.ParseRuleString(s, permission.BehaviorDeny, permission.SourceProject); err == nil {
			eng.AddRule(r)
		}
	}
	return eng
}
