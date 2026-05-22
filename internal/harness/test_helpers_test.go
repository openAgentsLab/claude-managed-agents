package harness_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"forge/internal/brain"
	"forge/internal/config"
	"forge/internal/gateway/session"
	_ "forge/internal/gateway/session/memory" // register "memory" driver
	"forge/internal/harness"
	"forge/internal/history"
	"forge/internal/tools"
)

// newTestStore opens an in-memory session store for tests.
func newTestStore(t *testing.T) session.SessionStore {
	t.Helper()
	store, err := session.Open(config.SessionConfig{Driver: "memory"})
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	return store
}

// newTestBrain creates a Brain from model config.
func newTestBrain(t *testing.T, modelCfg config.ModelConfig) *brain.Brain {
	t.Helper()
	b, err := brain.NewFromConfig(context.Background(), modelCfg, tools.Static(nil))
	if err != nil {
		t.Fatalf("build brain: %v", err)
	}
	return b
}

// newTestHarness creates a Harness backed by an in-memory store + the given brain.
func newTestHarness(t *testing.T, b *brain.Brain) *harness.Harness {
	t.Helper()
	store := newTestStore(t)
	mgr := history.NewManager(store, nil)
	return harness.New(b, store, mgr)
}

// skipIfAPIUnaccessible probes the API with a minimal brain call and skips the
// test when the endpoint returns a 403 / "Access denied" response. The harness
// silently drops such errors, so callers would otherwise just see empty replies.
func skipIfAPIUnaccessible(t *testing.T, b *brain.Brain) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	iter := b.Run(ctx, "hi", nil)
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev != nil && ev.Err != nil {
			s := ev.Err.Error()
			if strings.Contains(s, "403") || strings.Contains(s, "Access denied") {
				t.Skipf("API endpoint returned access denied — skipping integration test: %v", ev.Err)
			}
		}
	}
}

// anthropicKey returns ANTHROPIC_API_KEY or skips the test.
func anthropicKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set — skipping integration test")
	}
	return key
}

// haikuCfg returns a ModelConfig for the Haiku model using ANTHROPIC_API_KEY.
func haikuCfg(t *testing.T) config.ModelConfig {
	return config.ModelConfig{
		Provider: "anthropic",
		APIKey:   anthropicKey(t),
		Model:    "claude-haiku-4-5-20251001",
	}
}
