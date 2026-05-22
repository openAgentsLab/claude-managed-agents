package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"forge/internal/brain"
	einocallbacks "forge/internal/eino/callbacks"
	"forge/internal/gateway/session"
	"forge/internal/harness"
	"forge/internal/reqctx"
)

// CallableAgent is a minimal summary of an agent that this session may dispatch tasks to.
type CallableAgent struct {
	ID          string
	Name        string
	Description string
}

// BrainAcquirer abstracts WorkerManager.AcquireBrain to avoid a circular import
// between subagent and resolver. The returned release func MUST be called (e.g.
// via defer) when the Brain is no longer needed; it decrements the ref count that
// prevents idle eviction of the underlying MCP connections.
type BrainAcquirer func(ctx context.Context, tenantID, agentID string) (*brain.Brain, func(), error)

// AgentTool is the Eino InvokableTool that dispatches sub-agent Threads.
// Calling dispatch_agent_task runs the sub-agent synchronously in the same
// process and returns the result directly — no task queue or worker pool.
//
// Parallel tool calls from the Orchestrator Brain run concurrently; the
// semaphore limits the number of live Threads to maxConcurrentThreads.
type AgentTool struct {
	callableAgents []CallableAgent
	acquireBrain   BrainAcquirer
	sessionStore   session.SessionStore
	semaphore      chan struct{}
}

const maxConcurrentThreads = 25

// NewAgentTool creates an AgentTool.
// acquireBrain is a closure over WorkerManager.AcquireBrain to avoid circular imports.
func NewAgentTool(
	agents []CallableAgent,
	acquireBrain BrainAcquirer,
	sessionStore session.SessionStore,
) *AgentTool {
	sem := make(chan struct{}, maxConcurrentThreads)
	for i := 0; i < maxConcurrentThreads; i++ {
		sem <- struct{}{}
	}
	return &AgentTool{
		callableAgents: agents,
		acquireBrain:   acquireBrain,
		sessionStore:   sessionStore,
		semaphore:      sem,
	}
}

type agentInput struct {
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	AgentID     string `json:"agent_id"`
}

type agentOutput struct {
	Result string `json:"result"`
	Status string `json:"status"`
}

// Info returns the dispatch_agent_task tool schema.
func (t *AgentTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	agentListSection := buildAgentListSection(t.callableAgents)

	desc := `Dispatch a sub-agent task to handle complex, multi-step work autonomously.

The sub-agent runs immediately in the same session and returns its result directly. Multiple dispatch_agent_task calls in the same turn run concurrently.

` + agentListSection + `

When using the Agent tool, specify an agent_id parameter to select which agent to use.

Usage notes:
- Always include a short description (3-5 words) summarizing what the agent will do
- Dispatch multiple agents concurrently whenever possible by calling this tool multiple times in a single turn
- The result is returned directly in the tool response — no need to wait for a notification
- The agent is not visible to the user — you must relay its findings in a text message when relevant

## Writing the prompt

Brief the agent like a smart colleague who just walked into the room — it hasn't seen this conversation, doesn't know what you've tried, doesn't understand why this task matters.
- Explain what you're trying to accomplish and why.
- Describe what you've already learned or ruled out.
- Give enough context that the agent can make judgment calls rather than just following a narrow instruction.
- If you need a short response, say so ("report in under 200 words").

Terse command-style prompts produce shallow, generic work.

**Never delegate understanding.** Don't write "based on your findings, fix the bug." Write prompts that prove you understood: include file paths, line numbers, what specifically to change.`

	agentIDDesc := "ID of the agent to dispatch."
	if len(t.callableAgents) > 0 {
		names := make([]string, 0, len(t.callableAgents))
		for _, ca := range t.callableAgents {
			names = append(names, ca.ID+" ("+ca.Name+")")
		}
		sort.Strings(names)
		agentIDDesc = "ID of the agent to dispatch. Available: " + strings.Join(names, ", ") + "."
	}

	return &schema.ToolInfo{
		Name: "dispatch_agent_task",
		Desc: desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"description": {
				Type:     schema.String,
				Desc:     "Short (3-5 word) description of the task — shown in logs.",
				Required: true,
			},
			"prompt": {
				Type:     schema.String,
				Desc:     "Full task prompt for the agent. Be specific: include file paths, line numbers, and precise requirements.",
				Required: true,
			},
			"agent_id": {
				Type:     schema.String,
				Desc:     agentIDDesc,
				Required: true,
			},
		}),
	}, nil
}

// InvokableRun runs the sub-agent synchronously in the same process (Thread model).
// It blocks until the sub-agent completes and returns the result directly.
func (t *AgentTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var in agentInput
	if err := json.Unmarshal([]byte(argsJSON), &in); err != nil {
		return "", fmt.Errorf("dispatch_agent_task: invalid arguments: %w", err)
	}
	if in.Description == "" {
		return "", fmt.Errorf("dispatch_agent_task: 'description' is required")
	}
	if in.Prompt == "" {
		return "", fmt.Errorf("dispatch_agent_task: 'prompt' is required")
	}
	if in.AgentID == "" {
		return "", fmt.Errorf("dispatch_agent_task: 'agent_id' is required")
	}

	found := false
	for _, ca := range t.callableAgents {
		if ca.ID == in.AgentID {
			found = true
			break
		}
	}
	if !found {
		available := make([]string, 0, len(t.callableAgents))
		for _, ca := range t.callableAgents {
			available = append(available, ca.ID)
		}
		sort.Strings(available)
		return "", fmt.Errorf("dispatch_agent_task: unknown agent_id %q. Callable agents: %s",
			in.AgentID, strings.Join(available, ", "))
	}

	// Acquire a Thread slot — blocks if maxConcurrentThreads are already running.
	select {
	case <-t.semaphore:
		defer func() { t.semaphore <- struct{}{} }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	tenantID := reqctx.TenantIDFromContext(ctx)
	b, release, err := t.acquireBrain(ctx, tenantID, in.AgentID)
	if err != nil {
		return "", fmt.Errorf("dispatch_agent_task: acquire brain: %w", err)
	}
	defer release()

	// Create an isolated Thread session (independent conversation history).
	parentSID := sessionIDFromContext(ctx)
	threadID := "thread-" + parentSID + "-" + uuid.NewString()[:8]
	if err := t.sessionStore.CreateSession(session.Session{ID: threadID}); err != nil {
		return "", fmt.Errorf("dispatch_agent_task: create thread session: %w", err)
	}
	// Clean up the ephemeral thread session when the sub-agent finishes.
	defer func() { _ = t.sessionStore.ClearSession(threadID) }()

	// Run the sub-agent synchronously — blocks until complete.
	h := harness.NewStateless(t.sessionStore)
	outCh, errCh := h.RunStateless(ctx, threadID, b, in.Prompt)

	var sb strings.Builder
	for tok := range outCh {
		sb.WriteString(tok)
	}
	if runErr := <-errCh; runErr != nil {
		return "", fmt.Errorf("dispatch_agent_task: sub-agent failed: %w", runErr)
	}

	out, _ := json.Marshal(agentOutput{Result: sb.String(), Status: "completed"})
	return string(out), nil
}

// sessionIDFromContext reads the session ID injected by Harness via callbacks.SessionIDKey{}.
func sessionIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(einocallbacks.SessionIDKey{}).(string); ok && id != "" {
		return id
	}
	return "default"
}

// buildAgentListSection returns the callable agents section for the tool description.
func buildAgentListSection(agents []CallableAgent) string {
	if len(agents) == 0 {
		return "Callable agents are listed in <system-reminder> messages in the conversation."
	}
	sorted := make([]CallableAgent, len(agents))
	copy(sorted, agents)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	lines := make([]string, 0, len(sorted))
	for _, ca := range sorted {
		lines = append(lines, "- "+ca.ID+" ("+ca.Name+"): "+ca.Description)
	}
	return "Callable agents:\n" + strings.Join(lines, "\n")
}
