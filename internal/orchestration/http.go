package orchestration

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/gin-gonic/gin"

	"forge/internal/config"
	einocallbacks "forge/internal/eino/callbacks"
	"forge/internal/eventbus"
	"forge/internal/gateway/session"
	appstore "forge/internal/gateway/store"
	"forge/internal/gateway/vault"
	"forge/internal/hands"
	"forge/internal/harness"
	"forge/internal/memory"
	"forge/internal/permission"
	"forge/internal/reqctx"
	"forge/internal/resolver"
	"forge/internal/tools"
)

// engineStore holds the live per-tenant Engine map with RW-safe concurrent access.
// serve.go creates it; HTTPOrchestrator updates it when settings change at runtime.
type engineStore struct {
	mu      sync.RWMutex
	engines map[string]*permission.Engine
}

func (s *engineStore) get(key string) *permission.Engine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.engines[key]
}

func (s *engineStore) set(key string, e *permission.Engine) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.engines[key] = e
}

// resolver returns an EngineResolver that reads from this store under the mutex.
// Viewer role is always forced to plan mode regardless of any request override.
// For other roles, a per-request mode stored in ctx (via reqctx.WithPermissionMode)
// overrides the tenant engine's mode while preserving its allow/deny rules.
func (s *engineStore) resolver() permission.EngineResolver {
	return func(ctx context.Context) *permission.Engine {
		tid := reqctx.TenantIDFromContext(ctx)
		role := reqctx.RoleFromContext(ctx)

		var eng *permission.Engine
		if role == "viewer" {
			eng = s.get(tid + ":viewer")
		}
		if eng == nil {
			eng = s.get(tid)
		}
		if eng == nil {
			eng = s.get("default")
		}
		if eng == nil {
			// No engine found for this tenant — use a clean default rather than
			// borrowing a random tenant's rules, which would be a cross-tenant leak.
			eng = permission.NewEngine(permission.ModeDefault)
		}

		// Viewer is always plan — cannot be overridden by the request.
		if role == "viewer" {
			return eng
		}

		// Apply per-request mode override when present.
		if m := reqctx.PermissionModeFromContext(ctx); m != "" {
			return eng.WithMode(permission.Mode(m))
		}
		return eng
	}
}

// HTTPOrchestrator serves the multi-tenant HTTP API.
//
// All stateful components (Brain, SessionStore, HistoryManager) are shared
// across requests. Memory and sandboxes are per-user, lazily created via their
// respective pools.
//
// Session ID namespace: internally stored as "{tenantID}/{userID}:{clientSessionID}"
// so the flat SessionStore achieves per-tenant-per-user isolation without schema changes.
//
// Authentication: POST /auth/login with username+password returns a signed HS256 JWT.
// All other endpoints require "Authorization: Bearer <token>".
// The JWT "sub" claim holds the scoped internal user ID ("{tenantID}/{username}").
// Additional claims: "tid" (tenantID), "role" (user role).
type HTTPOrchestrator struct {
	harness     *harness.Harness
	memPool     *memory.Pool
	memManager  *memory.StoreManager
	sandboxPool hands.Pool
	auth        authHandler // JWT secret + TTL; owns sign/verify helpers
	tenantStore appstore.Store

	engines   *engineStore       // shared with the permission resolver
	globalCfg *config.Config     // needed to rebuild engines after settings change
	res       *resolver.Resolver // owns the tenant settings cache

	masterKey  []byte                   // nil when FORGE_VAULT_KEY not set
	secretRes  vault.Resolver           // nil when masterKey is nil
	sessionMgr *resolver.SessionManager // per-session brain lifecycle
	workerMgr  *resolver.WorkerManager  // brain cache for sub-agent threads
	caps       Capabilities             // detected once at startup

	eventBus       eventbus.EventBus // queued event delivery to SSE subscribers
	runCancel      sync.Map          // internalSID → context.CancelFunc; used by user.interrupt
	runFromCursors sync.Map          // internalSID → string cursor from MarkRunStart; passed to Subscribe
	pendingStore harness.PendingStore // cross-node delivery for custom tools and HITL gates
	serverCtx    context.Context      // cancelled when the HTTP server shuts down; parent for all runSession goroutines

	// Safety layer
	classifier      *harness.SafetyClassifier // nil when Haiku is not available
	jailbreakLimiter jailbreakRateLimiter      // tracks per-user jailbreak attempts
}

// NewHTTP creates an HTTPOrchestrator.
// It builds all internal components (engines, harness, resolver). Tenant settings
// and permission engines are loaded lazily on first request per tenant.
func NewHTTP(
	ctx context.Context,
	cfg *config.Config,
	memPool *memory.Pool,
	sandboxPool hands.Pool,
	tenantStore appstore.Store,
	baseReg tools.ToolRegistry,
	eb eventbus.EventBus,
	ps harness.PendingStore,
) (*HTTPOrchestrator, error) {
	masterKey, err := vault.MasterKeyFromEnv()
	if err != nil {
		return nil, fmt.Errorf("vault master key: %w", err)
	}

	ttl := time.Duration(cfg.Auth.TokenTTLHours) * time.Hour
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	// engineStore starts empty; entries are populated lazily on first request per tenant.
	engStore := &engineStore{engines: make(map[string]*permission.Engine)}
	// auditedReg wraps baseReg with permission enforcement and audit logging.
	// Every tool call passes through the resolver, which picks the right engine for
	// the calling tenant/role at invocation time.
	auditedReg := permission.WrapRegistryWithResolver(baseReg, engStore.resolver(), tools.ReadOnlyMap(), permission.NewSlogAuditLogger(slog.Default()))

	hns, err := harness.Build(ctx, cfg, auditedReg)
	if err != nil {
		return nil, fmt.Errorf("build harness: %w", err)
	}
	// Register the session-writer callback once at the composition root.
	// Doing this inside harness.Build would re-register on every call (e.g. tests).
	callbacks.AppendGlobalHandlers(einocallbacks.NewToolEventCallback())

	res := resolver.New(tenantStore)
	workerMgr := resolver.NewWorkerManager(res, auditedReg, tenantStore.Secrets(masterKey))
	sessionMgr := resolver.NewSessionManager(res, workerMgr, hns.SessionStore(), auditedReg, tenantStore.Secrets(masterKey))

	var memMgr *memory.StoreManager
	if memPool != nil {
		memMgr = memory.NewStoreManager(memPool)
	}

	var secretRes vault.Resolver
	if masterKey != nil {
		secretRes = tenantStore.Secrets(masterKey)
	}

	return &HTTPOrchestrator{
		harness:          hns,
		classifier:       buildSafetyClassifier(ctx),
		jailbreakLimiter: newJailbreakRateLimiter(),
		memPool:          memPool,
		memManager:   memMgr,
		sandboxPool:  sandboxPool,
		auth:         authHandler{secret: []byte(cfg.Auth.JWTSecret), ttl: ttl},
		tenantStore:  tenantStore,
		engines:      engStore,
		globalCfg:    cfg,
		res:          res,
		masterKey:    masterKey,
		secretRes:    secretRes,
		sessionMgr:   sessionMgr,
		workerMgr:    workerMgr,
		caps:         detectCapabilities(cfg.Sandbox),
		eventBus:     eb,
		pendingStore: ps,
	}, nil
}

// SessionStore exposes the underlying session store for components that need it
// at startup (e.g. the embedded worker pool).
func (o *HTTPOrchestrator) SessionStore() session.SessionStore {
	return o.harness.SessionStore()
}

// ensureTenantEngines lazily builds permission engines for tenantID on first access.
// Settings are loaded from the resolver's cache (which hits DB on first access).
func (o *HTTPOrchestrator) ensureTenantEngines(ctx context.Context, tenantID string) {
	if o.engines.get(tenantID) == nil {
		s := o.res.Settings(ctx, tenantID)
		o.engines.set(tenantID, buildTenantEngine(s, o.globalCfg))
		o.engines.set(tenantID+":viewer", buildViewerEngine(o.globalCfg))
	}
}

const tenantRefreshInterval = 10 * time.Second

// startRefreshLoop runs a background goroutine that polls the DB every
// tenantRefreshInterval and reloads any tenant whose updated_at has advanced.
// This keeps all nodes in a multi-process deployment eventually consistent.
func (o *HTTPOrchestrator) startRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(tenantRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.refreshStaleEngines(ctx)
		}
	}
}

func (o *HTTPOrchestrator) refreshStaleEngines(ctx context.Context) {
	tenants, err := o.tenantStore.Tenants().List(ctx)
	if err != nil {
		slog.WarnContext(ctx, "serve: refresh tenant settings: list tenants", "error", err)
		return
	}
	for _, t := range tenants {
		if !t.UpdatedAt.After(o.res.SettingsUpdatedAt(t.ID)) {
			continue
		}
		s := resolver.SettingsFromStore(t.Settings)
		o.res.PutSettings(t.ID, s, t.UpdatedAt)
		o.engines.set(t.ID, buildTenantEngine(s, o.globalCfg))
		o.engines.set(t.ID+":viewer", buildViewerEngine(o.globalCfg))
	}
}

// Run starts the HTTP server and blocks until ctx is cancelled or a fatal error occurs.
func (o *HTTPOrchestrator) Run(ctx context.Context, addr string) error {
	if len(o.auth.secret) == 0 {
		return fmt.Errorf("serve: auth.jwt_secret is not set; set it in config or FORGE_JWT_SECRET env var")
	}
	// Store the server lifetime context so runSession goroutines are cancelled
	// when the server shuts down, preventing goroutine leaks.
	o.serverCtx = ctx

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/api/health", o.handleHealth)
	r.GET("/api/v1/capabilities", o.handleCapabilities)
	r.POST("/api/auth/login", o.handleLogin)
	r.POST("/api/auth/logout", o.handleLogout)

	authed := r.Group("/api", o.authMiddleware())
	{
		authed.GET("/v1/sessions", o.handleListSessions)
		authed.POST("/v1/sessions", o.handleCreateSession)
		authed.GET("/v1/sessions/:id", o.handleGetSession)
		authed.GET("/v1/sessions/:id/events", o.handleGetSessionEvents)
		authed.GET("/v1/sessions/:id/events/history", o.handleGetSessionEventHistory)
		authed.GET("/v1/sessions/:id/events/:seq/content", o.handleGetSessionEventContent)
		authed.POST("/v1/sessions/:id/events", o.handleSendEvent)
		authed.POST("/v1/sessions/:id/run", o.handleRun)
		authed.DELETE("/v1/sessions/:id", o.handleDeleteSession)
		authed.PATCH("/v1/sessions/:id", o.handleUpdateSessionTitle)
		authed.POST("/v1/sessions/:id/clear", o.handleClearHistory)
		authed.GET("/v1/sessions/:id/resources", o.handleListResources)
		authed.POST("/v1/sessions/:id/resources", o.handleAddResource)
		authed.DELETE("/v1/sessions/:id/resources/:rid", o.handleRemoveResource)
		authed.GET("/v1/sessions/:id/outputs", o.handleListOutputs)
		authed.GET("/v1/sessions/:id/outputs/*path", o.handleReadOutput)

		authed.GET("/v1/tenant", o.handleGetTenant)

		authed.GET("/v1/user/settings", o.handleGetUserSettings)
		authed.PATCH("/v1/user/settings", o.handleUpdateUserSettings)

		authed.GET("/v1/projects", o.handleListProjects)
		authed.POST("/v1/projects", o.handleCreateProject)
		authed.GET("/v1/projects/:id", o.handleGetProject)
		authed.PUT("/v1/projects/:id", o.handleUpdateProject)
		authed.DELETE("/v1/projects/:id", o.handleDeleteProject)
		authed.GET("/v1/projects/:id/sessions", o.handleListProjectSessions)

		authed.GET("/v1/environments", o.handleListEnvironments)

		authed.GET("/v1/vaults", o.handleListVaults)
		authed.POST("/v1/vaults", o.handleSetVault)
		authed.DELETE("/v1/vaults/:name", o.handleDeleteVault)

		// Tenant-level read access: all authenticated users can view MCP servers and skills.
		authed.GET("/v1/tenant/mcp/servers", o.handleListTenantMCPServers)
		authed.GET("/v1/tenant/skills", o.handleListTenantSkills)
		authed.GET("/v1/tenant/skills/:name", o.handleGetTenantSkill)

		authed.GET("/v1/agents", o.handleListAgents)
		authed.GET("/v1/agents/:id", o.handleGetAgent)

		authed.GET("/v1/memory-stores", o.handleListMemoryStores)
		authed.POST("/v1/memory-stores", o.handleCreateMemoryStore)
		authed.GET("/v1/memory-stores/:id", o.handleGetMemoryStore)
		authed.PATCH("/v1/memory-stores/:id", o.handleUpdateMemoryStore)
		authed.DELETE("/v1/memory-stores/:id", o.handleDeleteMemoryStore)
	}

	admin := r.Group("/admin", o.authMiddleware(), o.adminMiddleware())
	{
		admin.PATCH("/v1/tenant/settings", o.handleUpdateTenantSettings)

		admin.GET("/v1/environments", o.handleAdminListEnvironments)
		admin.POST("/v1/environments", o.handleAdminCreateEnvironment)
		admin.PUT("/v1/environments/:id", o.handleAdminUpdateEnvironment)
		admin.DELETE("/v1/environments/:id", o.handleAdminDeleteEnvironment)

		admin.GET("/v1/tenant/users", o.handleListUsers)
		admin.PATCH("/v1/tenant/users/:username", o.handleUpdateUserRole)
		admin.POST("/v1/tenant/users", o.handleCreateUser)

		admin.GET("/v1/tenant/vaults", o.handleListTenantVaults)
		admin.POST("/v1/tenant/vaults", o.handleSetTenantVault)
		admin.DELETE("/v1/tenant/vaults/:name", o.handleDeleteTenantVault)

		admin.POST("/v1/tenant/mcp/servers", o.handleUpsertTenantMCPServer)
		admin.PUT("/v1/tenant/mcp/servers/:name", o.handleUpsertTenantMCPServer)
		admin.DELETE("/v1/tenant/mcp/servers/:name", o.handleDeleteTenantMCPServer)

		admin.POST("/v1/tenant/skills", o.handleUpsertTenantSkill)
		admin.PUT("/v1/tenant/skills/:name", o.handleUpsertTenantSkill)
		admin.DELETE("/v1/tenant/skills/:name", o.handleDeleteTenantSkill)

		admin.POST("/v1/agents", o.handleCreateAgent)
		admin.PATCH("/v1/agents/:id", o.handleUpdateAgent)
		admin.POST("/v1/agents/:id/archive", o.handleArchiveAgent)
	}

	go o.startRefreshLoop(ctx)
	go o.sessionMgr.StartEvictLoop(ctx)
	go o.workerMgr.StartEvictLoop(ctx)

	srv := &http.Server{Addr: addr, Handler: r}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("serve: listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	}
}
