package subagent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"forge/internal/brain"
	"forge/internal/config"
	"forge/internal/gateway/session"
	_ "forge/internal/gateway/session/memory" // register memory driver
)

// stubAcquire returns a nil brain — sufficient for tests that only check
// Info() or pre-execution validation (which happens before the brain is used).
func stubAcquire(_ context.Context, _, _ string) (*brain.Brain, func(), error) {
	return nil, func() {}, nil
}

// errorAcquire always fails — used for tests that should not reach acquireBrain.
func errorAcquire(_ context.Context, _, _ string) (*brain.Brain, func(), error) {
	return nil, func() {}, context.DeadlineExceeded
}

func newTestStore(t *testing.T) session.SessionStore {
	t.Helper()
	store, err := session.Open(config.SessionConfig{Driver: "memory"})
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	return store
}

// ── NewAgentTool ──────────────────────────────────────────────────────────────

func TestNewAgentTool_SemaphoreCapacity(t *testing.T) {
	at := NewAgentTool(nil, stubAcquire, newTestStore(t))
	if cap(at.semaphore) != maxConcurrentThreads {
		t.Errorf("semaphore capacity: got %d, want %d", cap(at.semaphore), maxConcurrentThreads)
	}
	if len(at.semaphore) != maxConcurrentThreads {
		t.Errorf("semaphore initial fill: got %d, want %d", len(at.semaphore), maxConcurrentThreads)
	}
}

// ── Info ──────────────────────────────────────────────────────────────────────

func TestAgentTool_Info_ToolName(t *testing.T) {
	at := NewAgentTool(nil, stubAcquire, newTestStore(t))
	info, err := at.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "dispatch_agent_task" {
		t.Errorf("Name: got %q, want %q", info.Name, "dispatch_agent_task")
	}
}

func TestAgentTool_Info_RequiredParams(t *testing.T) {
	at := NewAgentTool(nil, stubAcquire, newTestStore(t))
	info, err := at.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	js, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatalf("ToJSONSchema: %v", err)
	}
	if js.Properties == nil {
		t.Fatal("expected non-nil Properties in tool schema")
	}
	for _, required := range []string{"description", "prompt", "agent_id"} {
		if _, ok := js.Properties.Get(required); !ok {
			t.Errorf("expected parameter %q in tool info", required)
		}
	}
}

func TestAgentTool_Info_AgentListInDescription(t *testing.T) {
	agents := []CallableAgent{
		{ID: "agent-1", Name: "Alpha", Description: "does alpha things"},
		{ID: "agent-2", Name: "Beta", Description: "does beta things"},
	}
	at := NewAgentTool(agents, stubAcquire, newTestStore(t))
	info, err := at.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if !strings.Contains(info.Desc, "agent-1") || !strings.Contains(info.Desc, "agent-2") {
		t.Error("Info.Desc should list available agent IDs")
	}
}

func TestAgentTool_Info_NoAgents_FallbackMessage(t *testing.T) {
	at := NewAgentTool(nil, stubAcquire, newTestStore(t))
	info, err := at.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if !strings.Contains(info.Desc, "system-reminder") {
		t.Error("Info.Desc should mention system-reminder when no agents configured")
	}
}

// ── buildAgentListSection ─────────────────────────────────────────────────────

func TestBuildAgentListSection_Empty(t *testing.T) {
	got := buildAgentListSection(nil)
	if !strings.Contains(got, "system-reminder") {
		t.Error("empty agent list should return fallback message")
	}
}

func TestBuildAgentListSection_SortedByName(t *testing.T) {
	agents := []CallableAgent{
		{ID: "b-id", Name: "Zeta"},
		{ID: "a-id", Name: "Alpha"},
	}
	got := buildAgentListSection(agents)
	alphaIdx := strings.Index(got, "Alpha")
	zetaIdx := strings.Index(got, "Zeta")
	if alphaIdx == -1 || zetaIdx == -1 {
		t.Fatalf("both agent names should appear; got:\n%s", got)
	}
	if alphaIdx > zetaIdx {
		t.Error("buildAgentListSection should sort agents by name alphabetically")
	}
}

func TestBuildAgentListSection_ContainsIDAndDescription(t *testing.T) {
	agents := []CallableAgent{
		{ID: "my-agent", Name: "My Agent", Description: "does awesome work"},
	}
	got := buildAgentListSection(agents)
	if !strings.Contains(got, "my-agent") {
		t.Error("agent list should contain agent ID")
	}
	if !strings.Contains(got, "does awesome work") {
		t.Error("agent list should contain agent description")
	}
}

// ── InvokableRun validation ───────────────────────────────────────────────────

func TestInvokableRun_MissingDescription(t *testing.T) {
	at := NewAgentTool(
		[]CallableAgent{{ID: "a", Name: "A"}},
		errorAcquire,
		newTestStore(t),
	)
	args, _ := json.Marshal(map[string]string{"prompt": "do stuff", "agent_id": "a"})
	_, err := at.InvokableRun(context.Background(), string(args))
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Errorf("expected 'description' error, got %v", err)
	}
}

func TestInvokableRun_MissingPrompt(t *testing.T) {
	at := NewAgentTool(
		[]CallableAgent{{ID: "a", Name: "A"}},
		errorAcquire,
		newTestStore(t),
	)
	args, _ := json.Marshal(map[string]string{"description": "desc", "agent_id": "a"})
	_, err := at.InvokableRun(context.Background(), string(args))
	if err == nil || !strings.Contains(err.Error(), "prompt") {
		t.Errorf("expected 'prompt' error, got %v", err)
	}
}

func TestInvokableRun_MissingAgentID(t *testing.T) {
	at := NewAgentTool(
		[]CallableAgent{{ID: "a", Name: "A"}},
		errorAcquire,
		newTestStore(t),
	)
	args, _ := json.Marshal(map[string]string{"description": "desc", "prompt": "do stuff"})
	_, err := at.InvokableRun(context.Background(), string(args))
	if err == nil || !strings.Contains(err.Error(), "agent_id") {
		t.Errorf("expected 'agent_id' error, got %v", err)
	}
}

func TestInvokableRun_UnknownAgentID(t *testing.T) {
	at := NewAgentTool(
		[]CallableAgent{{ID: "known", Name: "Known"}},
		errorAcquire,
		newTestStore(t),
	)
	args, _ := json.Marshal(map[string]string{
		"description": "desc",
		"prompt":      "do stuff",
		"agent_id":    "unknown-id",
	})
	_, err := at.InvokableRun(context.Background(), string(args))
	if err == nil || !strings.Contains(err.Error(), "unknown agent_id") {
		t.Errorf("expected 'unknown agent_id' error, got %v", err)
	}
}

func TestInvokableRun_InvalidJSON(t *testing.T) {
	at := NewAgentTool(nil, errorAcquire, newTestStore(t))
	_, err := at.InvokableRun(context.Background(), "not-json")
	if err == nil {
		t.Error("expected error for invalid JSON input, got nil")
	}
}

func TestInvokableRun_ContextCanceled_BeforeAcquire(t *testing.T) {
	// Fill all semaphore slots so the acquire blocks.
	at := NewAgentTool(
		[]CallableAgent{{ID: "a", Name: "A"}},
		stubAcquire,
		newTestStore(t),
	)
	// Drain the semaphore so the next call blocks.
	for i := 0; i < maxConcurrentThreads; i++ {
		<-at.semaphore
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	args, _ := json.Marshal(map[string]string{
		"description": "desc",
		"prompt":      "do stuff",
		"agent_id":    "a",
	})
	_, err := at.InvokableRun(ctx, string(args))
	if err == nil {
		t.Error("expected error when context is already canceled, got nil")
	}
}
