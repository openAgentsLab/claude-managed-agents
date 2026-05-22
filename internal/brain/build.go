package brain

import (
	"context"
	_ "embed"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"forge/internal/memory"
	internaltools "forge/internal/tools"
)

//go:embed prompts/system_prompt.md
var defaultSystemPrompt string

// buildAgent creates an adk.ChatModelAgent.
// All tools come from registry.Tools() — each tool carries its own
// Info() declaration and InvokableRun() execution logic.
func buildAgent(ctx context.Context, m model.ToolCallingChatModel, registry internaltools.ToolRegistry, cfg BrainConfig) (adk.Agent, error) {
	einoTools := registry.Tools()

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "CodingReAct",
		Description: "An AI coding assistant that reads, writes, and executes code to complete software engineering tasks.",
		Model:       m,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: einoTools,
				// ExecuteSequentially: false enables parallel tool calls within
				// the same reasoning turn, reducing total latency.
				ExecuteSequentially: false,
			},
		},
		MaxIterations: cfg.maxIterations(),
		// GenModelInput prepends system prompt and conversation history before
		// the new user message so Brain has full context on each call.
		GenModelInput: func(ctx context.Context, _ string, input *adk.AgentInput) ([]adk.Message, error) {
			var msgs []*schema.Message
			sysPrompt := defaultSystemPrompt
			if cfg.SystemPrompt != "" {
				sysPrompt = cfg.SystemPrompt
			}
			if memCtx := memory.SystemContextFromContext(ctx); memCtx != "" {
				sysPrompt = sysPrompt + "\n\n" + memCtx
			}
			msgs = append(msgs, schema.SystemMessage(sysPrompt))
			msgs = append(msgs, HistoryFromContext(ctx)...)
			msgs = append(msgs, input.Messages...)
			return msgs, nil
		},
	})
}
