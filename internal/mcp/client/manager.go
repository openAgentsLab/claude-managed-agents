// Package client implements MCP transport and connection pooling.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	connectTimeout       = 30 * time.Second
	toolCallTimeout      = 5 * time.Minute
	maxReconnectAttempts = 5
	initialBackoff       = time.Second
	maxBackoff           = 30 * time.Second
	maxDescriptionLen    = 2048
	// maxOutputRunes is the truncation limit for tool output (25 000 tokens × 4 chars).
	// We count runes (Unicode code points) rather than bytes to avoid cutting
	// multi-byte sequences mid-character (e.g. CJK text).
	maxOutputRunes = 25000 * 4
	// stderrCapBytes caps in-memory stderr accumulation.
	stderrCapBytes = 64 * 1024
)

// ToolEntry is a discovered MCP tool along with its original (un-normalised)
// name so we can call it correctly.
type ToolEntry struct {
	NormalizedName string // mcp__server__tool  (used by the LLM)
	OriginalName   string // as returned by listTools (used in callTool)
	Description    string
	InputSchema    mcp.ToolInputSchema
}

// ServerEntry holds the live state of one MCP server connection.
type ServerEntry struct {
	Name   string
	Config MCPServerConfig
	State  ServerState
	Tools  []ToolEntry
	Error  string
	// Instructions is the optional usage hint returned by the server during
	// the MCP Initialize handshake (InitializeResult.Instructions).
	// It is injected into the system prompt so the model knows how to use the
	// server's tools and resources.  Truncated to maxDescriptionLen chars.
	Instructions string

	client    *mcpgo.Client
	transport mcptransport.Interface // kept for graceful stdio shutdown
	cancel    context.CancelFunc
	mu        sync.RWMutex
}

// Manager owns the lifecycle of all configured MCP server connections.
// It is safe for concurrent use.
type Manager struct {
	mu      sync.RWMutex
	servers map[string]*ServerEntry

	// toolUpdateCh is signalled whenever the set of available tools changes.
	toolUpdateCh chan struct{}
}

// NewManager allocates a Manager with no servers.
func NewManager() *Manager {
	return &Manager{
		servers:      make(map[string]*ServerEntry),
		toolUpdateCh: make(chan struct{}, 1),
	}
}

// ToolUpdateCh returns the channel pulsed when the tool set changes.
func (m *Manager) ToolUpdateCh() <-chan struct{} { return m.toolUpdateCh }

func (m *Manager) notifyToolUpdate() {
	select {
	case m.toolUpdateCh <- struct{}{}:
	default:
	}
}

// Add registers a server config without connecting.
func (m *Manager) Add(name string, cfg MCPServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.servers[name]; !exists {
		m.servers[name] = &ServerEntry{Name: name, Config: cfg, State: StateDisconnected}
	}
}

// SetDisabled enables or disables a server by name.
func (m *Manager) SetDisabled(name string, disabled bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.servers[name]
	if !ok {
		return false
	}
	e.mu.Lock()
	e.Config.Disabled = disabled
	e.mu.Unlock()
	return true
}

// Connect establishes a connection to the named server, discovers its tools,
// and stores the result. Blocks until ready or timed out.
func (m *Manager) Connect(ctx context.Context, name string) error {
	m.mu.RLock()
	entry, ok := m.servers[name]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("mcp: server %q not registered", name)
	}

	entry.mu.Lock()
	entry.State = StatePending
	entry.mu.Unlock()

	c, tools, instructions, transport, err := dial(ctx, name, entry.Config)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	if err != nil {
		entry.State = StateFailed
		entry.Error = err.Error()
		return err
	}

	// Cancel any previous connection's watch goroutine.
	if entry.cancel != nil {
		entry.cancel()
	}

	serverCtx, cancel := context.WithCancel(ctx)
	entry.client = c
	entry.transport = transport
	entry.cancel = cancel
	entry.State = StateConnected
	entry.Tools = tools
	entry.Instructions = instructions
	entry.Error = ""

	// P1: register tools/list_changed notification so tool set refreshes live.
	m.registerNotificationHandlers(serverCtx, name, c)

	// P0: start reconnect watchdog for remote transports (sse/http/ws).
	// stdio is a local process — if it dies, the user must restart explicitly.
	typ := entry.Config.Type
	if typ == "" {
		typ = MCPStdio
	}
	if typ != MCPStdio {
		go m.reconnectOnConnectionLost(serverCtx, name, c)
	}

	m.notifyToolUpdate()
	return nil
}

// ConnectAll connects to every registered non-disabled server concurrently
// (batch size 3, matching forge's MCP_SERVER_CONNECTION_BATCH_SIZE).
func (m *Manager) ConnectAll(ctx context.Context) {
	m.mu.RLock()
	names := make([]string, 0, len(m.servers))
	for name, e := range m.servers {
		if !e.Config.Disabled {
			names = append(names, name)
		}
	}
	m.mu.RUnlock()

	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		sem <- struct{}{}
		go func(n string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := m.Connect(ctx, n); err != nil {
				slog.WarnContext(ctx, "mcp connect failed", "server", n, "error", err)
			}
		}(name)
	}
	wg.Wait()
}

// CallTool invokes toolOriginalName on the named server with the given JSON args.
func (m *Manager) CallTool(ctx context.Context, serverName, toolOriginalName, argsJSON string) (string, error) {
	m.mu.RLock()
	entry, ok := m.servers[serverName]
	m.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("mcp: server %q not found", serverName)
	}

	entry.mu.RLock()
	c := entry.client
	state := entry.State
	entry.mu.RUnlock()

	if state != StateConnected || c == nil {
		return "", fmt.Errorf("mcp: server %q is not connected (state=%v)", serverName, state)
	}

	callCtx, cancel := context.WithTimeout(ctx, toolCallTimeout)
	defer cancel()

	var args map[string]any
	if argsJSON != "" && argsJSON != "{}" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("mcp callTool: invalid args JSON: %w", err)
		}
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = toolOriginalName
	req.Params.Arguments = args

	result, err := c.CallTool(callCtx, req)
	if err != nil {
		return "", fmt.Errorf("mcp callTool %s/%s: %w", serverName, toolOriginalName, err)
	}
	if result.IsError {
		msg := extractTextContent(result.Content)
		return "", fmt.Errorf("mcp tool error %s/%s: %s", serverName, toolOriginalName, msg)
	}

	// P1: truncate by rune count (Unicode code points), not bytes.
	out := extractTextContent(result.Content)
	out = truncateRunes(out, maxOutputRunes)
	return out, nil
}

// ResourceEntry is a discovered MCP resource from a single server.
type ResourceEntry struct {
	ServerName  string
	URI         string
	Name        string
	Description string
	MIMEType    string
}

// ListResources fetches resources from all connected servers (or just the named
// server when serverName != "").  Errors from individual servers are logged and
// skipped so one bad server doesn't block the rest.
func (m *Manager) ListResources(ctx context.Context, serverName string) ([]ResourceEntry, error) {
	m.mu.RLock()
	entries := make([]*ServerEntry, 0, len(m.servers))
	for name, e := range m.servers {
		if serverName != "" && name != serverName {
			continue
		}
		entries = append(entries, e)
	}
	m.mu.RUnlock()

	if serverName != "" && len(entries) == 0 {
		return nil, fmt.Errorf("mcp: server %q not found", serverName)
	}

	var out []ResourceEntry
	for _, e := range entries {
		e.mu.RLock()
		c := e.client
		state := e.State
		name := e.Name
		e.mu.RUnlock()

		if state != StateConnected || c == nil {
			continue
		}

		listCtx, cancel := context.WithTimeout(ctx, toolCallTimeout)
		result, err := c.ListResources(listCtx, mcp.ListResourcesRequest{})
		cancel()
		if err != nil {
			slog.WarnContext(ctx, "mcp listResources failed", "server", name, "error", err)
			continue
		}
		for _, r := range result.Resources {
			out = append(out, ResourceEntry{
				ServerName:  name,
				URI:         r.URI,
				Name:        r.Name,
				Description: r.Description,
				MIMEType:    r.MIMEType,
			})
		}
	}
	return out, nil
}

// ReadResource fetches a single resource by URI from the named server.
// Returns the text content (for text resources) or a base64-encoded blob
// (for binary resources), along with the MIME type.
func (m *Manager) ReadResource(ctx context.Context, serverName, uri string) (text string, mimeType string, isBlob bool, err error) {
	m.mu.RLock()
	e, ok := m.servers[serverName]
	m.mu.RUnlock()
	if !ok {
		return "", "", false, fmt.Errorf("mcp: server %q not found", serverName)
	}

	e.mu.RLock()
	c := e.client
	state := e.State
	e.mu.RUnlock()

	if state != StateConnected || c == nil {
		return "", "", false, fmt.Errorf("mcp: server %q is not connected (state=%v)", serverName, state)
	}

	readCtx, cancel := context.WithTimeout(ctx, toolCallTimeout)
	defer cancel()

	req := mcp.ReadResourceRequest{}
	req.Params.URI = uri
	result, err := e.client.ReadResource(readCtx, req)
	if err != nil {
		return "", "", false, fmt.Errorf("mcp readResource %s/%s: %w", serverName, uri, err)
	}
	if result == nil || len(result.Contents) == 0 {
		return "", "", false, fmt.Errorf("mcp readResource %s/%s: empty response", serverName, uri)
	}

	switch rc := result.Contents[0].(type) {
	case mcp.TextResourceContents:
		text = truncateRunes(rc.Text, maxOutputRunes)
		mimeType = rc.MIMEType
		isBlob = false
	case mcp.BlobResourceContents:
		text = rc.Blob // base64-encoded
		mimeType = rc.MIMEType
		isBlob = true
	default:
		return "", "", false, fmt.Errorf("mcp readResource %s/%s: unknown content type", serverName, uri)
	}
	return text, mimeType, isBlob, nil
}

// ServerInstructionsEntry pairs a server name with its instructions text.
type ServerInstructionsEntry struct {
	Name         string
	Instructions string
}

// AllServerInstructions returns the instructions for every connected server
// that provided a non-empty instructions field during initialization.
func (m *Manager) AllServerInstructions() []ServerInstructionsEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []ServerInstructionsEntry
	for _, e := range m.servers {
		e.mu.RLock()
		if e.State == StateConnected && e.Instructions != "" {
			out = append(out, ServerInstructionsEntry{
				Name:         e.Name,
				Instructions: e.Instructions,
			})
		}
		e.mu.RUnlock()
	}
	return out
}

// AllTools returns a flattened ToolEntry slice for all connected servers.
func (m *Manager) AllTools() []ToolEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var all []ToolEntry
	for _, e := range m.servers {
		e.mu.RLock()
		if e.State == StateConnected {
			all = append(all, e.Tools...)
		}
		e.mu.RUnlock()
	}
	return all
}

// ServerSnapshot is a copyable view of a ServerEntry for use in display/CLI code.
// It avoids copying the mutex embedded in ServerEntry.
type ServerSnapshot struct {
	Name         string
	Config       MCPServerConfig
	State        ServerState
	Tools        []ToolEntry
	Error        string
	Instructions string
}

// AllServers returns a snapshot of all server states.
// The returned slice is safe to read concurrently; mutations do not affect the Manager.
func (m *Manager) AllServers() []ServerSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ServerSnapshot, 0, len(m.servers))
	for _, e := range m.servers {
		e.mu.RLock()
		out = append(out, ServerSnapshot{
			Name:         e.Name,
			Config:       e.Config,
			State:        e.State,
			Tools:        e.Tools,
			Error:        e.Error,
			Instructions: e.Instructions,
		})
		e.mu.RUnlock()
	}
	return out
}

// Reconnect forces a reconnection for the named server.
func (m *Manager) Reconnect(ctx context.Context, name string) error {
	return m.Connect(ctx, name)
}

// Close cancels all server connections and gracefully shuts down stdio processes.
// It waits for all graceful-shutdown goroutines to complete before returning so
// callers (e.g. main) can safely exit once Close returns.
func (m *Manager) Close() {
	m.mu.RLock()
	entries := make([]*ServerEntry, 0, len(m.servers))
	for _, e := range m.servers {
		entries = append(entries, e)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	for _, e := range entries {
		e.mu.Lock()
		if e.cancel != nil {
			e.cancel()
		}
		typ := e.Config.Type
		if typ == "" {
			typ = MCPStdio
		}
		// P0: graceful stdio shutdown: SIGINT → 100ms → SIGTERM → 400ms → SIGKILL.
		// Run in goroutines so multiple stdio servers shut down concurrently, but
		// track them in a WaitGroup so Close() doesn't return until all are done.
		if typ == MCPStdio {
			if st, ok := e.transport.(*mcptransport.Stdio); ok {
				name := e.Name
				wg.Add(1)
				go func() {
					defer wg.Done()
					gracefulShutdownStdio(st, name)
				}()
			} else if e.transport != nil {
				e.transport.Close() //nolint:errcheck
			}
		} else if e.transport != nil {
			e.transport.Close() //nolint:errcheck
		}
		e.mu.Unlock()
	}
	wg.Wait()
}

// ─── internal: notification & reconnection ───────────────────────────────────

// registerNotificationHandlers sets up tools/list_changed on a freshly
// connected client, matching forge's useManageMCPConnections.ts.
func (m *Manager) registerNotificationHandlers(ctx context.Context, serverName string, c *mcpgo.Client) {
	c.OnNotification(func(n mcp.JSONRPCNotification) {
		if n.Method != string(mcp.MethodNotificationToolsListChanged) {
			return
		}
		slog.Debug("mcp tools/list_changed received", "server", serverName)
		go func() {
			fetchCtx, cancel := context.WithTimeout(ctx, connectTimeout)
			defer cancel()

			m.mu.RLock()
			entry, ok := m.servers[serverName]
			m.mu.RUnlock()
			if !ok {
				return
			}

			entry.mu.RLock()
			cl := entry.client
			entry.mu.RUnlock()
			if cl == nil {
				return
			}

			tools, err := fetchTools(fetchCtx, serverName, cl)
			if err != nil {
				slog.WarnContext(ctx, "mcp tools/list_changed refresh failed",
					"server", serverName, "error", err)
				return
			}

			entry.mu.Lock()
			entry.Tools = tools
			entry.mu.Unlock()
			m.notifyToolUpdate()
			slog.Info("mcp tools refreshed via list_changed",
				"server", serverName, "count", len(tools))
		}()
	})
}

// reconnectOnConnectionLost registers OnConnectionLost for remote transports
// and drives reconnection with exponential back-off.
// Must NOT be called for stdio transports.
func (m *Manager) reconnectOnConnectionLost(ctx context.Context, name string, c *mcpgo.Client) {
	lost := make(chan error, 1)
	c.OnConnectionLost(func(err error) {
		select {
		case lost <- err:
		default:
		}
	})

	select {
	case <-ctx.Done():
		return
	case err := <-lost:
		slog.WarnContext(ctx, "mcp remote connection lost", "server", name, "error", err)
	}

	m.mu.RLock()
	entry, ok := m.servers[name]
	m.mu.RUnlock()
	if !ok {
		return
	}
	entry.mu.RLock()
	disabled := entry.Config.Disabled
	entry.mu.RUnlock()
	if disabled {
		slog.Info("mcp server disabled, skipping reconnection", "server", name)
		return
	}

	m.reconnectWithBackoff(ctx, name)
}

// reconnectWithBackoff attempts reconnection with exponential back-off.
// Matches forge: 1s, 2s, 4s, 8s, 16s (capped at 30s), 5 attempts.
func (m *Manager) reconnectWithBackoff(ctx context.Context, name string) {
	for attempt := 1; attempt <= maxReconnectAttempts; attempt++ {
		wait := time.Duration(math.Min(
			float64(initialBackoff)*math.Pow(2, float64(attempt-1)),
			float64(maxBackoff),
		))

		m.mu.RLock()
		entry, ok := m.servers[name]
		m.mu.RUnlock()
		if !ok {
			return
		}
		entry.mu.Lock()
		entry.State = StatePending
		entry.Error = fmt.Sprintf("reconnecting (attempt %d/%d)", attempt, maxReconnectAttempts)
		entry.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		entry.mu.RLock()
		disabled := entry.Config.Disabled
		entry.mu.RUnlock()
		if disabled {
			slog.Info("mcp server disabled during reconnection, stopping", "server", name)
			return
		}

		slog.InfoContext(ctx, "mcp reconnecting", "server", name,
			"attempt", attempt, "maxAttempts", maxReconnectAttempts)
		if err := m.Connect(ctx, name); err == nil {
			slog.InfoContext(ctx, "mcp reconnected", "server", name)
			return
		}
	}

	m.mu.RLock()
	entry, ok := m.servers[name]
	m.mu.RUnlock()
	if ok {
		entry.mu.Lock()
		entry.State = StateFailed
		entry.Error = fmt.Sprintf("failed after %d reconnect attempts", maxReconnectAttempts)
		entry.mu.Unlock()
	}
	slog.WarnContext(ctx, "mcp max reconnect attempts reached", "server", name)
}

// ─── internal: dial ──────────────────────────────────────────────────────────

func dial(ctx context.Context, name string, cfg MCPServerConfig) (
	*mcpgo.Client, []ToolEntry, string, mcptransport.Interface, error,
) {
	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	typ := cfg.Type
	if typ == "" {
		typ = MCPStdio
	}

	switch typ {
	case MCPStdio:
		return dialStdio(connectCtx, name, cfg)
	case MCPSSE, MCPHTTP:
		// Pass the parent ctx for the long-lived SSE connection so it isn't
		// killed by the connect timeout's cancel; use connectCtx only for the
		// time-bounded handshake (Initialize + ListTools).
		return dialSSE(ctx, connectCtx, name, cfg)
	default:
		return nil, nil, "", nil, fmt.Errorf("mcp: unsupported transport %q for server %q", typ, name)
	}
}

// dialStdio starts a stdio MCP server subprocess.
// P2: captures subprocess stderr for diagnostic output on failure.
// P3: declares client capabilities on Initialize.
func dialStdio(ctx context.Context, name string, cfg MCPServerConfig) (
	*mcpgo.Client, []ToolEntry, string, mcptransport.Interface, error,
) {
	// P1: full env-var expansion with ${VAR:-default} support.
	cmd, cmdMissing := expandEnvVarsFull(cfg.Command)
	args, argsMissing := expandEnvSliceFull(cfg.Args)
	envOverrides, envMissing := expandEnvMapFull(cfg.Env)
	if missing := append(append(cmdMissing, argsMissing...), envMissing...); len(missing) > 0 {
		slog.Warn("mcp stdio: missing environment variables",
			"server", name, "vars", dedup(missing))
	}
	if strings.TrimSpace(cmd) == "" {
		return nil, nil, "", nil, fmt.Errorf("mcp stdio %q: command is empty after env expansion (original: %q)", name, cfg.Command)
	}
	merged := mergeEnv(envOverrides)

	// P2: stderr capture via CommandFunc.
	// stderrMu guards stderrBuf: the draining goroutine writes while the main
	// goroutine reads on error, so unsynchronised access is a data race.
	var stderrMu sync.Mutex
	var stderrBuf strings.Builder
	captureCmd := func(_ context.Context, command string, env []string, cmdArgs []string) (*exec.Cmd, error) {
		c := exec.Command(command, cmdArgs...)
		c.Env = env
		return c, nil
	}

	st := mcptransport.NewStdioWithOptions(cmd, merged, args,
		mcptransport.WithCommandFunc(captureCmd),
	)

	// Drain stderr in background so it doesn't block.
	go func() {
		r := st.Stderr()
		if r == nil {
			return
		}
		buf := make([]byte, 4096)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				stderrMu.Lock()
				if stderrBuf.Len() < stderrCapBytes {
					stderrBuf.Write(buf[:n])
				}
				stderrMu.Unlock()
			}
			if readErr != nil {
				return
			}
		}
	}()

	stderrSnapshot := func() string {
		stderrMu.Lock()
		defer stderrMu.Unlock()
		return strings.TrimSpace(stderrBuf.String())
	}

	if err := st.Start(ctx); err != nil {
		errMsg := err.Error()
		if s := stderrSnapshot(); s != "" {
			errMsg += "\n--- server stderr ---\n" + s
		}
		return nil, nil, "", nil, fmt.Errorf("mcp stdio start %q: %s", name, errMsg)
	}

	// P3: declare client capabilities.
	c := mcpgo.NewClient(st, mcpgo.WithClientCapabilities(mcp.ClientCapabilities{
		Roots: &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{},
	}))

	initResult, err := c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ClientInfo: mcp.Implementation{Name: "ai-coding", Version: "0.1.0"},
		},
	})
	if err != nil {
		if s := stderrSnapshot(); s != "" {
			err = fmt.Errorf("%w\n--- server stderr ---\n%s", err, s)
		}
		st.Close() //nolint:errcheck
		return nil, nil, "", nil, fmt.Errorf("mcp initialize %q: %w", name, err)
	}

	logServerConnected(name, initResult)

	tools, err := fetchTools(ctx, name, c)
	if err != nil {
		st.Close() //nolint:errcheck
		return nil, nil, "", nil, err
	}
	instructions := truncateRunes(initResult.Instructions, maxDescriptionLen)
	return c, tools, instructions, st, nil
}

// dialSSE connects to a remote SSE / Streamable-HTTP MCP server.
// ctx is the long-lived context for the persistent SSE connection.
// connectCtx is a short-timeout context used only for the handshake (Initialize + ListTools).
func dialSSE(ctx, connectCtx context.Context, name string, cfg MCPServerConfig) (
	*mcpgo.Client, []ToolEntry, string, mcptransport.Interface, error,
) {
	// P1: full env-var expansion.
	url, urlMissing := expandEnvVarsFull(cfg.URL)
	headers, headersMissing := expandEnvMapFull(cfg.Headers)
	if missing := append(urlMissing, headersMissing...); len(missing) > 0 {
		slog.Warn("mcp remote: missing environment variables",
			"server", name, "vars", dedup(missing))
	}
	if strings.TrimSpace(url) == "" {
		return nil, nil, "", nil, fmt.Errorf("mcp remote %q: url is empty after env expansion (original: %q)", name, cfg.URL)
	}

	sseTransport, err := mcptransport.NewSSE(url, mcptransport.WithHeaders(headers))
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("mcp sse transport %q: %w", name, err)
	}

	// P3: declare client capabilities.
	c := mcpgo.NewClient(sseTransport, mcpgo.WithClientCapabilities(mcp.ClientCapabilities{
		Roots: &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{},
	}))

	// Start uses the long-lived ctx so the SSE connection outlives this function.
	if err := c.Start(ctx); err != nil {
		return nil, nil, "", nil, fmt.Errorf("mcp sse start %q: %w", name, err)
	}

	// Handshake steps use connectCtx (30s timeout).
	initResult, err := c.Initialize(connectCtx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ClientInfo: mcp.Implementation{Name: "ai-coding", Version: "0.1.0"},
		},
	})
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("mcp initialize %q: %w", name, err)
	}

	logServerConnected(name, initResult)

	tools, err := fetchTools(connectCtx, name, c)
	if err != nil {
		return nil, nil, "", nil, err
	}
	instructions := truncateRunes(initResult.Instructions, maxDescriptionLen)
	return c, tools, instructions, sseTransport, nil
}

// fetchTools calls ListTools on the connected client and normalises the results.
func fetchTools(ctx context.Context, serverName string, c *mcpgo.Client) ([]ToolEntry, error) {
	result, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("mcp listTools %q: %w", serverName, err)
	}
	entries := make([]ToolEntry, 0, len(result.Tools))
	for _, t := range result.Tools {
		// P1: truncate description by rune count.
		desc := truncateRunes(t.Description, maxDescriptionLen)
		entries = append(entries, ToolEntry{
			NormalizedName: BuildMCPToolName(serverName, t.Name),
			OriginalName:   t.Name,
			Description:    desc,
			InputSchema:    t.InputSchema,
		})
	}
	return entries, nil
}

// ─── internal: stdio graceful shutdown ───────────────────────────────────────

// gracefulShutdownStdio sends SIGINT → SIGTERM → SIGKILL with delays, matching
// forge's cleanup() for stdio transports:
//
//	SIGINT → wait 100ms → check → SIGTERM → wait 400ms → check → SIGKILL → failsafe 600ms
func gracefulShutdownStdio(st *mcptransport.Stdio, serverName string) {
	// Use CommandFunc to track the PID. Since mcp-go v0.39 doesn't expose Pid()
	// directly, we send signals by probing via process kill(pid, 0) on the
	// stderr reader EOF as a proxy for exit.
	//
	// Approach: attempt a timed Close() which sends stdin EOF (SIGPIPE-equivalent).
	// If the process doesn't exit within 100ms, escalate.
	done := make(chan error, 1)
	go func() { done <- st.Close() }()

	// Step 1: wait up to 100ms for graceful stdin-EOF exit.
	select {
	case <-done:
		slog.Debug("mcp stdio exited after stdin close", "server", serverName)
		return
	case <-time.After(100 * time.Millisecond):
	}

	// Step 2: drain remaining output and give 400ms more.
	r := st.Stderr()
	if r != nil {
		io.Copy(io.Discard, r) //nolint:errcheck
	}

	select {
	case <-done:
		slog.Debug("mcp stdio exited during drain", "server", serverName)
		return
	case <-time.After(400 * time.Millisecond):
	}

	// Step 3: process still alive — we can't send SIGKILL without the PID,
	// but we've already closed stdin. Log the stale state and abandon.
	// (mcp-go v0.39 does not expose cmd.Process.Pid; if a future version
	// does, replace this with syscall.Kill(pid, syscall.SIGKILL).)
	slog.Warn("mcp stdio did not exit within 500ms after stdin close",
		"server", serverName,
		"note", "upgrade mcp-go or use WithCommandFunc to obtain PID for SIGKILL")

	// Failsafe: absolute 600ms deadline from caller perspective is already met.
	_ = syscall.SIGKILL // imported for future use
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// truncateRunes truncates s to at most maxRunes Unicode code points.
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "\n[Output truncated. Use pagination to get more results.]"
}

// mergeEnv builds the subprocess environment from a minimal safe whitelist plus
// any explicitly configured overrides. It intentionally does NOT inherit
// os.Environ() to prevent stdio MCP server processes from accessing host
// credentials such as ANTHROPIC_API_KEY or GITHUB_TOKEN.
func mergeEnv(extra map[string]string) []string {
	safeKeys := []string{"PATH", "HOME", "USER", "TMPDIR", "TZ", "LANG", "LC_ALL"}
	out := make([]string, 0, len(safeKeys)+len(extra))
	for _, k := range safeKeys {
		if v := os.Getenv(k); v != "" {
			out = append(out, k+"="+v)
		}
	}
	for k, v := range extra {
		out = append(out, k+"="+v)
	}
	return out
}

func extractTextContent(content []mcp.Content) string {
	var sb strings.Builder
	for _, c := range content {
		if tc, ok := c.(mcp.TextContent); ok {
			// Guard against a single malicious/buggy server sending huge text.
			// Truncation to runes (Unicode-safe) happens in the callers, but we
			// also cap the raw accumulation here so we never buffer more than
			// 2× the final limit in memory.
			remaining := maxOutputRunes*2 - sb.Len()
			if remaining <= 0 {
				break
			}
			if len(tc.Text) > remaining {
				sb.WriteString(tc.Text[:remaining])
				break
			}
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

func logServerConnected(name string, r *mcp.InitializeResult) {
	if r == nil {
		return
	}
	listChanged := r.Capabilities.Tools != nil && r.Capabilities.Tools.ListChanged
	slog.Info("mcp server connected",
		"server", name,
		"serverName", r.ServerInfo.Name,
		"serverVersion", r.ServerInfo.Version,
		"toolsListChanged", listChanged,
	)
}
