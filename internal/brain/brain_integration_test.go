package brain

import (
	"context"
	"os"
	"strings"
	"testing"

	"forge/internal/config"
	"forge/internal/tools"
)

// TestBrainRun_ReturnsModelResponse is an integration test that hits the real
// API. Skipped when no API key is found in the environment.
func TestBrainRun_ReturnsModelResponse(t *testing.T) {
	var modelCfg config.ModelConfig
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		modelCfg = config.ModelConfig{Provider: "anthropic", APIKey: key, Model: "claude-sonnet-4-6"}
	} else if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		modelCfg = config.ModelConfig{Provider: "openai", APIKey: key, Model: "gpt-4o-mini"}
	} else {
		t.Skip("ANTHROPIC_API_KEY or OPENAI_API_KEY not set")
	}

	ctx := context.Background()

	b, err := NewFromConfig(ctx, modelCfg, tools.Static(nil))
	if err != nil {
		t.Fatalf("NewFromConfig: %v", err)
	}

	iter := b.Run(ctx, "Reply with exactly: hello", nil)

	var sb strings.Builder
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil || ev.Err != nil {
			if ev != nil {
				errStr := ev.Err.Error()
				if strings.Contains(errStr, "403") || strings.Contains(errStr, "Access denied") {
					t.Skipf("API endpoint returned access denied — skipping integration test: %v", ev.Err)
				}
				t.Logf("event err: %v", ev.Err)
			}
			continue
		}
		if ev.Output != nil && ev.Output.MessageOutput != nil {
			if msg, err := ev.Output.MessageOutput.GetMessage(); err == nil && msg != nil {
				sb.WriteString(msg.Content)
			}
		}
	}

	got := strings.TrimSpace(sb.String())
	if got == "" {
		t.Errorf("expected non-empty response from model, got empty string")
	}
	t.Logf("model response: %q", got)
}
