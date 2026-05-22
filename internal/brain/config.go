package brain

// BrainConfig controls Brain inference behaviour.
type BrainConfig struct {
	// Effort controls the total token budget across text + tools + thinking.
	// Options: max | high | medium | low. Default: "high".
	// Docs: Effort parameter (build-with-claude/effort)
	Effort string

	// Thinking is the adaptive-thinking configuration.
	// Type="adaptive" lets Claude decide thinking depth automatically (recommended).
	// Type="disabled" turns off thinking (low-latency scenarios).
	// The old type="enabled" + budget_tokens is deprecated in Claude 4.6.
	// Docs: Adaptive Thinking (build-with-claude/adaptive-thinking)
	Thinking ThinkingConfig

	// MaxRetries is the maximum number of retries for 429/529 API errors.
	// Default: 5.
	MaxRetries int

	// MaxIterations is the maximum number of tool-call/model cycles per turn.
	// Prevents runaway agentic loops. Default: 48.
	MaxIterations int

	// MaxTokens caps the model's output token budget. When 0 the provider
	// default is used (4096 for Anthropic). Set a lower value for
	// latency-sensitive single-turn use cases such as safety classifiers or
	// title generation.
	// Docs: Reduce Latency (build-with-claude/reduce-latency)
	MaxTokens int

	// SystemPrompt overrides the default embedded system prompt when non-empty.
	// Set by the Agent layer so different agents can have distinct personas.
	SystemPrompt string
}

// ThinkingConfig specifies how Claude uses its thinking capability.
type ThinkingConfig struct {
	// "adaptive" (recommended) or "disabled"
	Type string
}

// DefaultBrainConfig returns the recommended Phase 1 configuration.
func DefaultBrainConfig() BrainConfig {
	return BrainConfig{
		Effort:        "high",
		Thinking:      ThinkingConfig{Type: "adaptive"},
		MaxRetries:    5,
		MaxIterations: 48,
	}
}

// maxIterations returns MaxIterations, defaulting to 48 when zero.
func (c BrainConfig) maxIterations() int {
	if c.MaxIterations <= 0 {
		return 48
	}
	return c.MaxIterations
}
