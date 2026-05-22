package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	einocallbacks "forge/internal/eino/callbacks"
	"forge/internal/brain"
	"forge/internal/entity"
	"forge/internal/gateway/session"
	appstore "forge/internal/gateway/store"
	"forge/internal/hands"
	"forge/internal/harness"
	"forge/internal/permission"
	"forge/internal/reqctx"
)

// projectConfigSnapshot captures the effective project configuration at session
// creation time for historical traceability. Token is intentionally excluded.
type projectConfigSnapshot struct {
	GitURL    string            `json:"git_url,omitempty"`
	GitBranch string            `json:"git_branch,omitempty"`
	GitUser   string            `json:"git_username,omitempty"`
	RefFiles  []appstore.RefFile `json:"ref_files,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

// outcomeEvalContent is the JSON structure persisted in session events of type
// RoleOutcomeEval. It is also used when deserialising those events back into
// harness.Event for history / SSE replay.
type outcomeEvalContent struct {
	Result    string                      `json:"outcome_result"`
	Iteration int                         `json:"outcome_iteration"`
	MaxIter   int                         `json:"outcome_max_iterations"`
	Criteria  []harness.CriterionFeedback `json:"criteria_feedback,omitempty"`
}

func (o *HTTPOrchestrator) handleListSessions(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	filterProjectID := c.Query("project_id")

	sessions, err := o.harness.ListSessions(id.UserID)
	if err != nil {
		c.String(http.StatusInternalServerError, "list sessions: %s", err.Error())
		return
	}

	resp := make([]entity.SessionListItem, 0, len(sessions))
	for _, s := range sessions {
		if filterProjectID != "" && s.ProjectID != filterProjectID {
			continue
		}
		resp = append(resp, entity.SessionListItem{
			SessionID: s.ID,
			ProjectID: s.ProjectID,
			Title:     s.Title,
			Status:    string(s.Status),
			InitError: s.InitError,
			CreatedAt: s.CreatedAt.UTC().Format(entity.TimeFormatISO8601),
		})
	}
	c.JSON(http.StatusOK, resp)
}

// handleListProjectSessions lists sessions belonging to a specific project.
func (o *HTTPOrchestrator) handleListProjectSessions(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	username := strings.TrimPrefix(id.UserID, id.TenantID+"/")
	projectID := c.Param("id")

	// Validate project ownership.
	p, err := o.tenantStore.Projects(o.masterKey).Get(c.Request.Context(), projectID)
	if err != nil {
		c.String(http.StatusInternalServerError, "get project: %s", err.Error())
		return
	}
	if p == nil || p.TenantID != id.TenantID || p.OwnerID != username {
		c.String(http.StatusNotFound, "project not found")
		return
	}

	sessions, err := o.harness.ListProjectSessions(id.UserID, projectID)
	if err != nil {
		c.String(http.StatusInternalServerError, "list sessions: %s", err.Error())
		return
	}

	resp := make([]entity.SessionListItem, 0, len(sessions))
	for _, s := range sessions {
		resp = append(resp, entity.SessionListItem{
			SessionID: s.ID,
			ProjectID: s.ProjectID,
			Title:     s.Title,
			Status:    string(s.Status),
			InitError: s.InitError,
			CreatedAt: s.CreatedAt.UTC().Format(entity.TimeFormatISO8601),
		})
	}
	c.JSON(http.StatusOK, resp)
}

func (o *HTTPOrchestrator) handleCreateSession(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	var req entity.CreateSessionRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.String(http.StatusBadRequest, "bad request: %s", err.Error())
			return
		}
	}

	clientID := req.SessionID
	if clientID == "" {
		clientID = newSessionID()
	}

	var linkedProject *entity.ProjectResponse // non-nil when session is bound to a real Project
	var rawProject *appstore.Project          // held for resource initialisation below
	projectID := req.ProjectID
	if projectID != "" {
		username := strings.TrimPrefix(id.UserID, id.TenantID+"/")
		p, err := o.tenantStore.Projects(o.masterKey).Get(c.Request.Context(), projectID)
		if err != nil {
			c.String(http.StatusInternalServerError, "get project: %s", err.Error())
			return
		}
		if p == nil || p.TenantID != id.TenantID || p.OwnerID != username {
			c.String(http.StatusNotFound, "project not found")
			return
		}
		linkedProject = projectToResponse(p)
		rawProject = p
	} else {
		projectID = newSessionID()
	}

	// Validate custom memory stores before creating the session so we can
	// reject invalid IDs before any state is written.
	if len(req.MemoryStores) > 0 {
		if err := o.validateMemoryStores(c.Request.Context(), req.MemoryStores, id.UserID, id.TenantID); err != nil {
			c.String(http.StatusBadRequest, "%s", err.Error())
			return
		}
	}

	// Resolve agent: use the requested ID, fall back to the tenant default.
	agentID := req.AgentID
	if agentID == "" {
		if def, err := o.tenantStore.Agents().GetDefault(c.Request.Context(), id.TenantID); err == nil && def != nil {
			agentID = def.ID
		}
	}
	if agentID == "" {
		c.String(http.StatusBadRequest, "agent_id is required: no agent specified and no default agent configured for this tenant")
		return
	}

	// Resolve environment before creating the session record so we can capture
	// the snapshot and determine whether async init is needed.
	envUsername := strings.TrimPrefix(id.UserID, id.TenantID+"/")
	envCtx := reqctx.WithTenantID(c.Request.Context(), id.TenantID)
	merged := o.resolveSession(envCtx, id.TenantID, envUsername, rawProject)

	// Build a token-free snapshot of the effective project config.
	var snapshotJSON string
	if rawProject != nil {
		snap := projectConfigSnapshot{
			GitURL:    merged.GitConfig.URL,
			GitBranch: merged.GitConfig.Branch,
			GitUser:   merged.GitConfig.Username,
			RefFiles:  merged.RefFiles,
			Env:       merged.Env,
		}
		if b, err := json.Marshal(snap); err == nil {
			snapshotJSON = string(b)
		}
	}

	needsInit := merged.GitConfig.URL != "" || len(merged.RefFiles) > 0
	initStatus := session.SessionIdle
	if needsInit {
		initStatus = session.SessionInitializing
	}

	customTools := make([]session.CustomToolDef, len(req.CustomTools))
	for i, t := range req.CustomTools {
		customTools[i] = session.CustomToolDef{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	internalSID := scopedSessionID(id.UserID, clientID)
	if err := o.harness.CreateSession(session.Session{
		ID:              internalSID,
		ProjectID:       projectID,
		AgentID:         agentID,
		MemoryStoreIDs:  req.MemoryStores,
		CustomTools:     customTools,
		Status:          initStatus,
		ProjectSnapshot: snapshotJSON,
	}); err != nil {
		c.String(http.StatusInternalServerError, "create session: %s", err.Error())
		return
	}

	o.ensureTenantEngines(c.Request.Context(), id.TenantID)
	o.sessionMgr.Ensure(c.Request.Context(), internalSID, id.TenantID, id.UserID, agentID)

	if es, ok := o.sandboxPool.(hands.SessionEnvSetter); ok {
		if err := es.SetSessionEnvironment(envCtx, internalSID, toEnvironment(merged)); err != nil {
			slog.WarnContext(envCtx, "create session: set session environment", "error", err)
		}
	}

	if needsInit {
		go func() {
			// Use serverCtx so this goroutine is cancelled on graceful shutdown,
			// preventing git-clone / resource-init tasks from outliving the server.
			initErr := o.initSessionResources(o.serverCtx, internalSID, id, merged)
			status := session.SessionIdle
			errMsg := ""
			if initErr != nil {
				status = session.SessionInitFailed
				errMsg = initErr.Error()
				slog.WarnContext(o.serverCtx, "session init failed", "session_id", clientID, "error", initErr)
			}
			if err := o.harness.SessionStore().UpdateSessionInitStatus(internalSID, status, errMsg); err != nil {
				slog.WarnContext(o.serverCtx, "session: update init status", "error", err)
			}
		}()
	}

	resp := entity.CreateSessionResponse{
		SessionID: clientID,
		ProjectID: projectID,
		Status:    string(initStatus),
	}
	if linkedProject != nil {
		resp.ProjectName = linkedProject.Name
		resp.EnvironmentID = linkedProject.EnvironmentID
	}
	c.JSON(http.StatusOK, resp)
}

// handleGetSession returns session metadata including the current status.
func (o *HTTPOrchestrator) handleGetSession(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}

	internalSID := scopedSessionID(id.UserID, clientID)
	sess, err := o.harness.GetSessionMeta(internalSID)
	if err != nil {
		c.String(http.StatusNotFound, "session not found")
		return
	}

	c.JSON(http.StatusOK, entity.SessionResponse{
		SessionID: clientID,
		ProjectID: sess.ProjectID,
		Title:     sess.Title,
		Status:    string(sess.Status),
		InitError: sess.InitError,
		CreatedAt: sess.CreatedAt.UTC().Format(entity.TimeFormatISO8601),
	})
}

// handleRun accepts a user message and launches an async agent turn.
// Returns 202 immediately; the client subscribes to GET /events for the stream.
func (o *HTTPOrchestrator) handleRun(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}

	var req entity.RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		c.String(http.StatusBadRequest, "message is required")
		return
	}

	// ── Safety layer (Layer 1 — code-level rate limit) ───────────────────────
	if o.jailbreakLimiter.IsBlocked(id.UserID) {
		c.JSON(http.StatusTooManyRequests, entity.ErrorResponse{Error: "too many policy violations — please contact support"})
		return
	}
	// ── Safety layer (Layer 2 — regex jailbreak patterns) ────────────────────
	if safe, reason := permission.ScanInput(req.Message); !safe {
		slog.WarnContext(c.Request.Context(), "handleRun: jailbreak pattern blocked",
			"user", id.UserID, "reason", reason)
		blocked := o.jailbreakLimiter.Record(id.UserID)
		if blocked {
			slog.WarnContext(c.Request.Context(), "handleRun: user rate-limited after repeated violations", "user", id.UserID)
		}
		c.JSON(http.StatusBadRequest, entity.ErrorResponse{Error: "message rejected by safety filter"})
		return
	}

	// Validate and resolve the requested mode.
	// viewer is always forced to plan; other roles may choose default or plan.
	mode := req.Mode
	if id.Role == entity.RoleViewer {
		mode = entity.PermissionModePlan
	} else if mode != "" && mode != entity.PermissionModeDefault && mode != entity.PermissionModePlan {
		c.String(http.StatusBadRequest, "mode must be 'default' or 'plan'")
		return
	}

	internalSID := scopedSessionID(id.UserID, clientID)

	sessMeta, err := o.harness.GetSessionMeta(internalSID)
	if err != nil {
		c.String(http.StatusNotFound, "session not found")
		return
	}

	// Atomically register this run using LoadOrStore as the sole concurrency gate.
	// This eliminates the TOCTOU race between a status read and the cancel store:
	// if another goroutine already holds the cancel for this session, loaded=true
	// and we return 409 without spawning a second goroutine.
	// The context is rooted at serverCtx so all in-flight runs are cancelled on
	// server shutdown, preventing goroutine leaks.
	runCtx, cancel := context.WithCancel(o.serverCtx)
	if _, loaded := o.runCancel.LoadOrStore(internalSID, cancel); loaded {
		cancel() // discard the unused cancel function
		c.JSON(http.StatusConflict, entity.ErrorResponse{Error: "session is already running; send user.interrupt to stop it"})
		return
	}

	// Set session status to running synchronously before spawning the goroutine.
	// This ensures that an SSE connection opening concurrently with the agent
	// goroutine (between the 202 response and the first harness event) sees the
	// correct running status and does not receive a premature idle.
	_ = o.harness.UpdateSessionStatus(internalSID, session.SessionRunning)

	// Capture the current tail of the event stream. streamSessionEvents passes
	// this cursor to Subscribe so events published by the agent goroutine before
	// the SSE connection is established are replayed from the queue.
	cursor := o.eventBus.MarkRunStart(internalSID)
	o.runFromCursors.Store(internalSID, cursor)

	go o.runSession(runCtx, internalSID, id, req.Message, mode, sessMeta)

	c.JSON(http.StatusAccepted, entity.RunResponse{Status: "running"})
}

// runSession executes one agent turn as a background goroutine.
func (o *HTTPOrchestrator) runSession(
	ctx context.Context,
	internalSID string,
	id Identity,
	message, mode string,
	sessMeta *session.Session,
) {
	defer o.runCancel.Delete(internalSID)

	// Honour cross-node interrupts delivered via EventBus (Redis mode).
	// Creates a child context that is cancelled when Interrupt(internalSID) fires
	// on any node, in addition to the local CancelFunc stored in runCancel.
	ctx, cancelInterrupt := context.WithCancel(ctx)
	defer cancelInterrupt()
	go func() {
		select {
		case <-o.eventBus.WatchInterrupt(ctx, internalSID):
			cancelInterrupt()
		case <-ctx.Done():
		}
	}()

	// ── Safety layer (Layer 3 — Haiku LLM classifier) ───────────────────────
	// Runs async (after 202 was returned) so it doesn't block the HTTP response.
	// Novel jailbreak phrasings not caught by regex may be caught here.
	if o.classifier != nil {
		result, err := o.classifier.Classify(ctx, message)
		if err != nil {
			slog.WarnContext(ctx, "run: safety classifier error (proceeding)", "error", err)
		} else if !result.IsSafe {
			slog.WarnContext(ctx, "run: Haiku classifier blocked message", "user", id.UserID, "reason", result.Reason)
			o.jailbreakLimiter.Record(id.UserID)
			errSeq, _ := o.harness.SessionStore().EmitEvent(internalSID, session.Event{Role: session.RoleError, Content: "message rejected by safety filter"})
			o.eventBus.Publish(internalSID, harness.Event{Type: harness.EventSessionError, Content: "message rejected by safety filter", Seq: errSeq})
			_ = o.harness.UpdateSessionStatus(internalSID, session.SessionIdle)
			return
		}
	}

	tenantSettings := o.res.Settings(ctx, id.TenantID)
	o.ensureTenantEngines(ctx, id.TenantID)

	// Status ownership: runSession owns the Idle reset only for pre-harness failures
	// (sandbox acquire). Once h.Run is called, harness.Run owns all status transitions
	// (Running → Idle/Interrupted) via its internal defer.
	sb, err := o.sandboxPool.Acquire(ctx, internalSID, hands.AcquireRequest{
		Quota: hands.ResourceQuota{
			MemoryBytes: tenantSettings.ResourceQuota.MemoryBytes,
			NanoCPUs:    tenantSettings.ResourceQuota.NanoCPUs,
		},
	})
	if err != nil {
		slog.ErrorContext(ctx, "run: acquire sandbox", "session", internalSID, "error", err)
		errSeq, _ := o.harness.SessionStore().EmitEvent(internalSID, session.Event{Role: session.RoleError, Content: err.Error()})
		o.eventBus.Publish(internalSID, harness.Event{Type: harness.EventSessionError, Content: err.Error(), Seq: errSeq})
		_ = o.harness.UpdateSessionStatus(internalSID, session.SessionIdle)
		return
	}

	ctx = o.buildRunCtx(ctx, id, mode, sessMeta.ProjectID, sb)

	entry := o.sessionMgr.Ensure(ctx, internalSID, id.TenantID, id.UserID, sessMeta.AgentID)
	if entry == nil {
		errSeq, _ := o.harness.SessionStore().EmitEvent(internalSID, session.Event{Role: session.RoleError, Content: "failed to initialize session brain"})
		o.eventBus.Publish(internalSID, harness.Event{Type: harness.EventSessionError, Content: "failed to initialize session brain", Seq: errSeq})
		_ = o.harness.UpdateSessionStatus(internalSID, session.SessionIdle)
		return
	}
	h := o.harness.WithBrain(entry.Brain).WithHistory(entry.History)
	if o.memPool != nil {
		ss := o.mountSessionMemoryStores(id.UserID, sessMeta.ProjectID, id.TenantID, id.Role == entity.RoleAdmin, sessMeta.MemoryStoreIDs)
		h = h.WithMemoryStores(ss)
	}

	outCh, errCh := h.Run(ctx, internalSID, message)
	// ── Safety layer (Layer 4 — output monitoring) ───────────────────────────
	// Collect reply text to scan for system-prompt leakage after the turn ends.
	var replyBuilder strings.Builder
	for ev := range outCh {
		if ev.Type == harness.EventAgentMessage {
			replyBuilder.WriteString(ev.Content)
		} else {
			slog.InfoContext(ctx, fmt.Sprintf("sse: event type=%s tool=%q", ev.Type, ev.Tool))
		}
		o.eventBus.Publish(internalSID, ev)
	}
	if err := <-errCh; err != nil {
		slog.ErrorContext(ctx, "run: harness error", "session", internalSID, "error", err)
		_, _ = o.harness.SessionStore().EmitEvent(internalSID, session.Event{Role: session.RoleError, Content: err.Error()})
	}

	// Scan completed reply for verbatim system-prompt fragments.
	if reply := replyBuilder.String(); reply != "" {
		if leaked, fragment := harness.ScanOutputForLeaks(reply, brain.DefaultSystemPrompt()); leaked {
			slog.WarnContext(ctx, "run: agent leaked system prompt fragment",
				"session", internalSID,
				"fragment_prefix", fragment[:min(60, len(fragment))],
			)
		}
	}
}

// handleGetSessionEvents serves SSE streams (Accept: text/event-stream).
// For conversation history use GET /v1/sessions/:id/events/history.
func (o *HTTPOrchestrator) handleGetSessionEvents(c *gin.Context) {
	if c.GetHeader("Accept") != entity.ContentTypeSSE {
		c.String(http.StatusBadRequest, "use Accept: text/event-stream for SSE, or GET /events/history for history")
		return
	}

	id := c.MustGet(identityKey).(Identity)
	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}
	internalSID := scopedSessionID(id.UserID, clientID)
	o.streamSessionEvents(c, internalSID)
}

// streamSessionEvents handles the SSE streaming branch of handleGetSessionEvents.
// Events are delivered via the EventBus queue: handleRun calls MarkRunStart
// before spawning the agent goroutine, so events published before the SSE
// connection is established are buffered in the queue and replayed here.
func (o *HTTPOrchestrator) streamSessionEvents(c *gin.Context, internalSID string) {
	// Retrieve the cursor captured by handleRun at run-start time.
	// Subscribe with this cursor so all events from the current run are delivered,
	// including those published before this SSE connection was established.
	cursor := ""
	if v, ok := o.runFromCursors.Load(internalSID); ok {
		cursor = v.(string)
	}
	liveCh := o.eventBus.Subscribe(c.Request.Context(), internalSID, cursor)

	w := c.Writer
	w.Header().Set("Content-Type", entity.ContentTypeSSE)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// If the session is not running and the queue is empty, the run already
	// completed and its events were consumed by a previous connection.
	// Emit a synthetic idle so the client closes cleanly.
	status, _ := o.harness.GetSessionStatus(internalSID)
	if status != session.SessionRunning {
		// Non-blocking drain: forward any events still buffered in the queue
		// (e.g. run completed faster than this SSE connection was established).
		gotAny := false
		for {
			select {
			case ev, ok := <-liveCh:
				if !ok {
					return
				}
				gotAny = true
				writeSSEEventWithID(w, ev, ev.Seq)
				w.Flush()
				if ev.Type == harness.EventSessionIdle {
					return
				}
			default:
				goto afterDrain
			}
		}
	afterDrain:
		if !gotAny {
			writeSSEEventWithID(w, harness.Event{Type: harness.EventSessionIdle, StopReason: "end_turn"}, 0)
			w.Flush()
			return
		}
		// Fell through: drained some events but not EventSessionIdle.
		// The run is finishing — fall through to the live loop below.
	}

	w.Flush()

	for {
		select {
		case ev, ok := <-liveCh:
			if !ok {
				return
			}
			writeSSEEventWithID(w, ev, ev.Seq)
			w.Flush()
			if ev.Type == harness.EventSessionIdle {
				// During an outcome cycle, intermediate idles appear between iterations.
				// Stay connected while the cycle goroutine still holds runCancel.
				if _, active := o.runCancel.Load(internalSID); !active {
					return
				}
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}

// handleGetSessionEventHistory returns the full persisted event stream for a session
// as a JSON array. All event types are included (agent.thinking, agent.tool_use,
// span.model_request_end, etc.), unlike the legacy conversation-history endpoint.
// Supports ?after_seq=N for incremental pagination after a disconnect.
func (o *HTTPOrchestrator) handleGetSessionEventHistory(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}
	internalSID := scopedSessionID(id.UserID, clientID)

	afterSeq := int64(0)
	if s := c.Query("after_seq"); s != "" {
		afterSeq, _ = strconv.ParseInt(s, 10, 64)
	}

	events, err := o.harness.GetEventsSince(internalSID, afterSeq)
	if err != nil {
		c.String(http.StatusInternalServerError, "get events: %s", err.Error())
		return
	}

	resp := make([]harness.Event, 0, len(events))
	for _, e := range events {
		resp = append(resp, storeEventToHarnessEvent(e))
	}
	c.JSON(http.StatusOK, resp)
}

// handleGetSessionEventContent returns the raw Content of a single persisted event
// identified by its sequence number. Use this to fetch full tool call arguments or
// tool result payloads that are omitted from the history/SSE responses.
func (o *HTTPOrchestrator) handleGetSessionEventContent(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)
	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}
	seq, err := strconv.ParseInt(c.Param("seq"), 10, 64)
	if err != nil || seq <= 0 {
		c.String(http.StatusBadRequest, "invalid seq")
		return
	}
	internalSID := scopedSessionID(id.UserID, clientID)

	events, err := o.harness.GetEventsSince(internalSID, seq-1)
	if err != nil {
		c.String(http.StatusInternalServerError, "get event: %s", err.Error())
		return
	}
	if len(events) == 0 || events[0].Seq != seq {
		c.String(http.StatusNotFound, "event not found")
		return
	}
	c.JSON(http.StatusOK, entity.EventContentResponse{Seq: seq, Content: events[0].Content})
}

// storeEventToHarnessEvent converts a persisted session.Event to a harness.Event
// for the history endpoint and SSE replay. Role-based store events are mapped to
// Managed Agents event types.
func storeEventToHarnessEvent(e session.Event) harness.Event {
	switch e.Role {
	case session.RoleAssistant:
		return harness.Event{Type: harness.EventAgentMessage, Content: e.Content, Seq: e.Seq}
	case session.RoleToolCall:
		ev := harness.Event{Type: harness.EventAgentToolUse, Tool: e.ToolName, Seq: e.Seq}
		var tc struct {
			Function struct {
				Arguments string `json:"arguments"`
			} `json:"function"`
		}
		if json.Unmarshal([]byte(e.Content), &tc) == nil {
			ev.Description = einocallbacks.BuildSummary(e.ToolName, tc.Function.Arguments)
		}
		return ev
	case session.RoleToolResult:
		return harness.Event{Type: harness.EventAgentToolResult, Tool: e.ToolName, ToolUseID: e.ToolCallID, Seq: e.Seq}
	case session.RoleThinking:
		return harness.Event{Type: harness.EventAgentThinking, Content: e.Content, Seq: e.Seq}
	case session.RoleCustomToolCall:
		return harness.Event{Type: harness.EventAgentCustomToolUse, Tool: e.ToolName, ToolUseID: e.ToolCallID, ToolInput: e.Content, Seq: e.Seq}
	case session.RoleRequiresAction:
		return harness.Event{Type: harness.EventSessionRequiresAction, Tool: e.ToolName, ToolUseID: e.ToolCallID, Content: e.Content, Seq: e.Seq}
	case session.RoleError:
		return harness.Event{Type: harness.EventSessionError, Content: e.Content, Seq: e.Seq}
	case session.RoleSpan:
		ev := harness.Event{Type: harness.EventSpanModelRequestEnd, Seq: e.Seq}
		var usage harness.ModelUsage
		if err := json.Unmarshal([]byte(e.Content), &usage); err == nil {
			ev.ModelUsage = &usage
		}
		return ev
	case session.RoleOutcomeEval:
		ev := harness.Event{Type: harness.EventAgentOutcomeEvaluation, Seq: e.Seq}
		var payload outcomeEvalContent
		if err := json.Unmarshal([]byte(e.Content), &payload); err == nil {
			ev.OutcomeResult = payload.Result
			ev.OutcomeIteration = payload.Iteration
			ev.OutcomeMaxIter = payload.MaxIter
			ev.CriteriaFeedback = payload.Criteria
		}
		return ev
	case session.RoleSystem:
		if strings.HasPrefix(e.Content, "idle:") {
			return harness.Event{Type: harness.EventSessionIdle, StopReason: strings.TrimPrefix(e.Content, "idle:"), Seq: e.Seq}
		}
		return harness.Event{Type: harness.EventSessionRunning, Seq: e.Seq}
	default: // user — not forwarded to SSE (client sent these)
		return harness.Event{Type: "user.message", Content: e.Content, Seq: e.Seq}
	}
}

// writeSSEEventWithID writes a single SSE event with an optional id: line.
func writeSSEEventWithID(w http.ResponseWriter, ev harness.Event, seq int64) {
	if seq > 0 {
		fmt.Fprintf(w, "id: %d\n", seq)
	}
	writeSSEJSON(w, ev)
}

// handleSendEvent processes client-to-server events:
//   - user.interrupt: cancel the running turn
//   - user.custom_tool_result: deliver result to a waiting RemoteToolExecutor
//   - user.tool_confirmation: deliver HITL confirm/deny to a waiting tool gate
//   - user.define_outcome: start an outcome-driven iteration cycle
func (o *HTTPOrchestrator) handleSendEvent(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}

	var req entity.SendEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}

	internalSID := scopedSessionID(id.UserID, clientID)

	switch harness.EventType(req.Type) {
	case harness.EventUserInterrupt:
		// Use DB status so the check works across nodes (the session may be running
		// on a different node than this request).
		if status, _ := o.harness.GetSessionStatus(internalSID); status != session.SessionRunning {
			c.JSON(http.StatusConflict, entity.ErrorResponse{Error: "session is not running"})
			return
		}
		// Broadcast interrupt via EventBus (cross-node in Redis mode).
		o.eventBus.Interrupt(internalSID)
		// Also cancel locally as a fast path if the run is on this node.
		if v, ok := o.runCancel.Load(internalSID); ok {
			v.(context.CancelFunc)()
		}
		c.Status(http.StatusAccepted)

	case harness.EventUserCustomToolResult:
		if req.ToolUseID == "" {
			c.String(http.StatusBadRequest, "tool_use_id is required")
			return
		}
		if !o.pendingStore.Deliver(req.ToolUseID, harness.SuspendedResult{Content: req.Content, Error: req.Error}) {
			c.JSON(http.StatusNotFound, entity.ErrorResponse{Error: "no pending tool call with id " + req.ToolUseID})
			return
		}
		c.Status(http.StatusAccepted)

	case harness.EventUserToolConfirmation:
		if req.ToolUseID == "" {
			c.String(http.StatusBadRequest, "tool_use_id is required")
			return
		}
		if !o.pendingStore.Deliver(req.ToolUseID, harness.SuspendedResult{Confirmed: req.Confirmed}) {
			c.JSON(http.StatusNotFound, entity.ErrorResponse{Error: "no pending action with id " + req.ToolUseID})
			return
		}
		c.Status(http.StatusAccepted)

	case harness.EventUserDefineOutcome:
		if req.Rubric == "" {
			c.String(http.StatusBadRequest, "rubric is required")
			return
		}
		sessMeta, err := o.harness.GetSessionMeta(internalSID)
		if err != nil {
			c.String(http.StatusNotFound, "session not found")
			return
		}
		runCtx, cancel := context.WithCancel(o.serverCtx)
		if _, loaded := o.runCancel.LoadOrStore(internalSID, cancel); loaded {
			cancel()
			c.JSON(http.StatusConflict, entity.ErrorResponse{Error: "session is already running; send user.interrupt to stop it"})
			return
		}
		_ = o.harness.UpdateSessionStatus(internalSID, session.SessionRunning)
		cursor := o.eventBus.MarkRunStart(internalSID)
		o.runFromCursors.Store(internalSID, cursor)
		go o.runOutcomeCycle(runCtx, internalSID, id, req.Description, req.Rubric, req.MaxIterations, sessMeta)
		c.JSON(http.StatusAccepted, entity.RunResponse{Status: "running"})

	default:
		c.String(http.StatusBadRequest, "unsupported event type %q", req.Type)
	}
}

func (o *HTTPOrchestrator) handleDeleteSession(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}

	internalSID := scopedSessionID(id.UserID, clientID)

	// Guard: cannot delete a running session without interrupting it first.
	if status, _ := o.harness.GetSessionStatus(internalSID); status == session.SessionRunning {
		c.JSON(http.StatusConflict, entity.ErrorResponse{Error: "send user.interrupt before deleting a running session"})
		return
	}

	if err := o.harness.ClearSession(c.Request.Context(), internalSID); err != nil {
		c.String(http.StatusInternalServerError, "failed to clear session: %s", err.Error())
		return
	}
	o.sessionMgr.Release(internalSID)
	_ = o.sandboxPool.ReleaseSession(c.Request.Context(), internalSID)
	o.eventBus.PurgeSession(internalSID)
	c.Status(http.StatusNoContent)
}

func (o *HTTPOrchestrator) handleClearHistory(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}

	internalSID := scopedSessionID(id.UserID, clientID)
	if err := o.harness.ResetHistory(internalSID); err != nil {
		c.String(http.StatusInternalServerError, "failed to clear history: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (o *HTTPOrchestrator) handleUpdateSessionTitle(c *gin.Context) {
	id := c.MustGet(identityKey).(Identity)

	clientID := c.Param("id")
	if clientID == "" {
		c.String(http.StatusBadRequest, "missing session id")
		return
	}

	var req entity.UpdateSessionTitleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "bad request: %s", err.Error())
		return
	}

	internalSID := scopedSessionID(id.UserID, clientID)
	if err := o.harness.UpdateSessionTitle(internalSID, strings.TrimSpace(req.Title)); err != nil {
		c.String(http.StatusInternalServerError, "update title: %s", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// buildRunCtx enriches ctx with all per-request values needed by tools,
// sandbox, and sub-components: identity, project, quota, sandbox, and workspace.
func (o *HTTPOrchestrator) buildRunCtx(ctx context.Context, id Identity, mode, projectID string, sb hands.Sandbox) context.Context {
	if mode != "" {
		ctx = reqctx.WithPermissionMode(ctx, mode)
	}
	ctx = reqctx.WithUserID(ctx, id.UserID)
	ctx = reqctx.WithTenantID(ctx, id.TenantID)
	ctx = reqctx.WithRole(ctx, id.Role)
	ctx = reqctx.WithProjectID(ctx, projectID)
	ctx = hands.WithSandbox(ctx, sb)
	ctx = context.WithValue(ctx, harness.PendingActionsKey{}, o.pendingStore)
	return ctx
}

// runOutcomeCycle executes an outcome-driven iteration loop:
// 1. Run the agent with description as the initial message.
// 2. Evaluate output against rubric with an independent Grader model call.
// 3. If needs_revision and max_iterations not reached, inject feedback and repeat.
// Emits agent.outcome_evaluation at each iteration, then a final session.status_idle.
func (o *HTTPOrchestrator) runOutcomeCycle(
	ctx context.Context,
	internalSID string,
	id Identity,
	description, rubric string,
	maxIterations int,
	sessMeta *session.Session,
) {
	defer o.runCancel.Delete(internalSID)

	// Honour cross-node interrupts (same pattern as runSession).
	ctx, cancelInterrupt := context.WithCancel(ctx)
	defer cancelInterrupt()
	go func() {
		select {
		case <-o.eventBus.WatchInterrupt(ctx, internalSID):
			cancelInterrupt()
		case <-ctx.Done():
		}
	}()

	if maxIterations <= 0 {
		maxIterations = entity.DefaultOutcomeMaxIterations
	}
	if maxIterations > entity.MaxOutcomeMaxIterations {
		maxIterations = entity.MaxOutcomeMaxIterations
	}

	tenantSettings := o.res.Settings(ctx, id.TenantID)
	o.ensureTenantEngines(ctx, id.TenantID)
	sb, err := o.sandboxPool.Acquire(ctx, internalSID, hands.AcquireRequest{
		Quota: hands.ResourceQuota{
			MemoryBytes: tenantSettings.ResourceQuota.MemoryBytes,
			NanoCPUs:    tenantSettings.ResourceQuota.NanoCPUs,
		},
	})
	if err != nil {
		slog.ErrorContext(ctx, "outcome: acquire sandbox", "session", internalSID, "error", err)
		errSeq, _ := o.harness.SessionStore().EmitEvent(internalSID, session.Event{Role: session.RoleError, Content: err.Error()})
		o.eventBus.Publish(internalSID, harness.Event{Type: harness.EventSessionError, Content: err.Error(), Seq: errSeq})
		_ = o.harness.UpdateSessionStatus(internalSID, session.SessionIdle)
		return
	}

	runCtx := o.buildRunCtx(ctx, id, "", sessMeta.ProjectID, sb)

	outcomeEntry := o.sessionMgr.Ensure(runCtx, internalSID, id.TenantID, id.UserID, sessMeta.AgentID)
	if outcomeEntry == nil {
		errSeq, _ := o.harness.SessionStore().EmitEvent(internalSID, session.Event{Role: session.RoleError, Content: "failed to initialize session brain"})
		o.eventBus.Publish(internalSID, harness.Event{Type: harness.EventSessionError, Content: "failed to initialize session brain", Seq: errSeq})
		_ = o.harness.UpdateSessionStatus(internalSID, session.SessionIdle)
		return
	}
	h := o.harness.WithBrain(outcomeEntry.Brain).WithHistory(outcomeEntry.History)
	if o.memPool != nil {
		ss := o.mountSessionMemoryStores(id.UserID, sessMeta.ProjectID, id.TenantID, id.Role == entity.RoleAdmin, sessMeta.MemoryStoreIDs)
		h = h.WithMemoryStores(ss)
	}

	// Build read-only grader harness once — reused across all iterations.
	// WithReadOnlyBrain strips write tools from the LLM's definitions so the
	// grader model never attempts to call them.
	graderH, err := h.WithReadOnlyBrain(runCtx)
	if err != nil {
		slog.WarnContext(runCtx, "outcome: build grader brain failed, falling back to full brain", "error", err)
		graderH = h
	}

	message := description
	if message == "" {
		message = "Please complete the task according to the provided acceptance criteria."
	}

	for iteration := 1; iteration <= maxIterations; iteration++ {
		if ctx.Err() != nil {
			o.publishOutcomeEval(internalSID, "interrupted", iteration, maxIterations, nil)
			return
		}

		// Run one agent turn; relay all events except intermediate idles to SSE subscribers.
		// harness.Run emits session.status_idle at the end of every turn, but during an
		// outcome cycle that idle is premature — runOutcomeCycle emits the terminal one.
		outCh, errCh := h.Run(runCtx, internalSID, message)
		for ev := range outCh {
			if ev.Type == harness.EventSessionIdle {
				continue
			}
			o.eventBus.Publish(internalSID, ev)
		}
		if runErr := <-errCh; runErr != nil {
			slog.ErrorContext(runCtx, "outcome: agent turn failed", "iteration", iteration, "error", runErr)
			o.publishOutcomeEval(internalSID, "failed", iteration, maxIterations, nil)
			return
		}

		if ctx.Err() != nil {
			o.publishOutcomeEval(internalSID, "interrupted", iteration, maxIterations, nil)
			return
		}

		// Evaluate with an independent Grader that uses tools to verify results directly.
		lastTurn := o.buildLastTurnContext(internalSID)
		graderResult, err := graderH.RunGraderWithBrain(runCtx, rubric, lastTurn)
		if err != nil {
			slog.ErrorContext(runCtx, "outcome: grader error", "iteration", iteration, "error", err)
			o.publishOutcomeEval(internalSID, "failed", iteration, maxIterations, nil)
			idleSeq, _ := o.harness.SessionStore().EmitEvent(internalSID, session.Event{Role: session.RoleSystem, Content: "idle:failed"})
			o.eventBus.Publish(internalSID, harness.Event{Type: harness.EventSessionIdle, StopReason: "failed", Seq: idleSeq})
			return
		}

		if graderResult.Overall == "satisfied" {
			var criteria []harness.CriterionFeedback
			if graderResult != nil {
				criteria = graderResult.Criteria
			}
			o.publishOutcomeEval(internalSID, "satisfied", iteration, maxIterations, criteria)
			// Emit a terminal idle so SSE clients know to disconnect.
			idleSeq, _ := o.harness.SessionStore().EmitEvent(internalSID, session.Event{Role: session.RoleSystem, Content: "idle:satisfied"})
			o.eventBus.Publish(internalSID, harness.Event{Type: harness.EventSessionIdle, StopReason: "satisfied", Seq: idleSeq})
			return
		}

		if iteration == maxIterations {
			o.publishOutcomeEval(internalSID, "max_iterations_reached", iteration, maxIterations, graderResult.Criteria)
			idleSeq, _ := o.harness.SessionStore().EmitEvent(internalSID, session.Event{Role: session.RoleSystem, Content: "idle:max_iterations_reached"})
			o.eventBus.Publish(internalSID, harness.Event{Type: harness.EventSessionIdle, StopReason: "max_iterations_reached", Seq: idleSeq})
			return
		}

		o.publishOutcomeEval(internalSID, "needs_revision", iteration, maxIterations, graderResult.Criteria)
		// Build next agent message from grader feedback.
		message = harness.BuildRevisionMessage(rubric, graderResult)
	}
}

// publishOutcomeEval persists and broadcasts an agent.outcome_evaluation event.
func (o *HTTPOrchestrator) publishOutcomeEval(
	internalSID, result string,
	iteration, maxIterations int,
	criteria []harness.CriterionFeedback,
) {
	contentBytes, _ := json.Marshal(outcomeEvalContent{
		Result:    result,
		Iteration: iteration,
		MaxIter:   maxIterations,
		Criteria:  criteria,
	})
	seq, _ := o.harness.SessionStore().EmitEvent(internalSID, session.Event{
		Role:    session.RoleOutcomeEval,
		Content: string(contentBytes),
	})
	o.eventBus.Publish(internalSID, harness.Event{
		Type:             harness.EventAgentOutcomeEvaluation,
		OutcomeResult:    result,
		OutcomeIteration: iteration,
		OutcomeMaxIter:   maxIterations,
		CriteriaFeedback: criteria,
		Seq:              seq,
	})
}

// buildLastTurnContext returns the most recent assistant reply for the Grader.
// The Grader verifies the latest output only; passing the full history would
// grow unboundedly across iterations and dilute focus on the current attempt.
func (o *HTTPOrchestrator) buildLastTurnContext(internalSID string) string {
	events, err := o.harness.GetEvents(internalSID)
	if err != nil {
		return ""
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Role == session.RoleAssistant {
			return "[Assistant]\n" + events[i].Content + "\n\n"
		}
	}
	return ""
}
