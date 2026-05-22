// Package brain implements the Brain layer from the managed-agents architecture.
// Brain depends on tools.ToolRegistry for all tool instances.
// The Sandbox abstraction is reserved for Phase 3 remote/docker sandboxes.
package brain

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"forge/internal/tools"
)


type historyKey struct{}

// Brain depends only on ToolRegistry.
type Brain struct {
	cfg      BrainConfig
	model    model.ToolCallingChatModel
	registry tools.ToolRegistry
	agent    adk.Agent
	runner   *adk.Runner
}

// maxTokensModel is an optional interface implemented by model adapters that
// support per-instance output token caps (e.g. the Anthropic adapter).
type maxTokensModel interface {
	model.ToolCallingChatModel
	WithMaxTokens(n int) model.ToolCallingChatModel
}

// New constructs a Brain.
func New(ctx context.Context, m model.ToolCallingChatModel, registry tools.ToolRegistry, cfg BrainConfig) (*Brain, error) {
	// Apply per-brain token cap if the model adapter supports it.
	if cfg.MaxTokens > 0 {
		if mts, ok := m.(maxTokensModel); ok {
			m = mts.WithMaxTokens(cfg.MaxTokens)
		}
	}
	agent, err := buildAgent(ctx, m, registry, cfg)
	if err != nil {
		return nil, err
	}
	// Use context.Background() so the Runner is not cancelled if the
	// construction-time ctx (e.g. a single HTTP request) is done. Each
	// reasoning turn passes its own ctx via runner.Run().
	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{
		Agent:          agent,
		EnableStreaming: true,
	})
	return &Brain{
		cfg:      cfg,
		model:    m,
		registry: registry,
		agent:    agent,
		runner:   runner,
	}, nil
}

// Model returns the underlying LLM used by this Brain.
// Callers that need to perform LLM operations scoped to the same session
// (e.g. memory extraction) should use this model so they inherit the
// session's effective configuration, including any tenant model overrides.
func (b *Brain) Model() model.BaseChatModel {
	return b.model
}

// GenerateTitle produces a short display title (≤6 words) for a conversation
// based on the user's first message. Uses a single non-agentic model call.
// Returns an empty string on error so callers can silently degrade.
func (b *Brain) GenerateTitle(ctx context.Context, firstMessage string) string {
	// Use a fresh context so Eino session/tool callbacks from the parent run
	// do not fire and accidentally write events into the conversation history.
	titleCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prompt := "Write a short title for the following chat message. " +
		"Rules: plain text only, no markdown, no # symbols, no bullet points, no quotes, no trailing punctuation, output ONLY the title (3–6 words).\n\n" +
		"Message: " + firstMessage
	msgs := []*schema.Message{
		schema.UserMessage(prompt),
	}
	resp, err := b.model.Generate(titleCtx, msgs)
	if err != nil || resp == nil {
		return ""
	}
	return strings.TrimSpace(resp.Content)
}

// WithExtraTools returns a new Brain that includes the provided extra tools in
// addition to the base registry tools. Used to inject per-session client-executed
// (remote) tools without mutating the shared base Brain.
func (b *Brain) WithExtraTools(ctx context.Context, extra []tool.BaseTool) (*Brain, error) {
	combined := tools.Static(append(b.registry.Tools(), extra...))
	return New(ctx, b.model, combined, b.cfg)
}

// WithReadOnlyTools returns a new Brain whose tool list contains only tools
// registered as read-only via tools.RegisterReadOnly. Write tools are removed
// from the LLM's tool definitions entirely, so the model never attempts to call
// them (as opposed to plan mode, which only blocks execution after the call).
// Used by the Grader to prevent accidental state mutation during verification.
func (b *Brain) WithReadOnlyTools(ctx context.Context) (*Brain, error) {
	readOnlyNames := tools.ReadOnlyMap()
	var filtered []tool.BaseTool
	for _, t := range b.registry.Tools() {
		info, err := t.Info(ctx)
		if err != nil || info == nil {
			continue
		}
		if readOnlyNames[info.Name] {
			filtered = append(filtered, t)
		}
	}
	return New(ctx, b.model, tools.Static(filtered), b.cfg)
}

// Run starts a reasoning iteration. history contains prior turns from Session;
// userText is the new user input. Returns an AsyncIterator of AgentEvents
// that Harness drains to collect tokens and detect stop reasons.
func (b *Brain) Run(ctx context.Context, userText string, history []*schema.Message) *adk.AsyncIterator[*adk.AgentEvent] {
	ctx = context.WithValue(ctx, historyKey{}, history)
	return b.runner.Run(ctx, []adk.Message{schema.UserMessage(userText)})
}

// HistoryFromContext retrieves the history injected by Brain.Run.
// Used by buildAgent's GenModelInput to prepend prior turns.
func HistoryFromContext(ctx context.Context) []*schema.Message {
	v, _ := ctx.Value(historyKey{}).([]*schema.Message)
	return v
}

// DefaultSystemPrompt returns the embedded default system prompt text.
// Exposed for output leak detection (see harness.ScanOutputForLeaks).
func DefaultSystemPrompt() string {
	return defaultSystemPrompt
}
