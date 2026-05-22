package compact

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Config controls the compression pipeline layers.
type Config struct {
	// ContextWindowTokens is the total context window of the model in tokens.
	// Default: 128_000.
	ContextWindowTokens int
	// MaxOutputTokens is headroom reserved for the model's reply.
	// Default: 20_000.
	MaxOutputTokens int
	// AutoCompactBuffer is the additional safety margin below which compaction fires.
	// Default: 13_000.  Threshold = ContextWindowTokens - MaxOutputTokens - AutoCompactBuffer.
	AutoCompactBuffer int
	// KeepRecentTurns is the number of recent user+assistant pairs protected from MicroCompact.
	// Default: 4.
	KeepRecentTurns int
	// MicroCompactMaxChars is the per-message character limit for MicroCompact.
	// Default: 8_000.
	MicroCompactMaxChars int
	// StripMaxChars is the per-message character limit for tool-result Strip (Layer 3, runs before GlobalCompact).
	// Default: 2_000.
	StripMaxChars int
	// MaxPTLRetries is the maximum number of PTL retry attempts inside GlobalCompact.
	// Default: 3.
	MaxPTLRetries int
	// MaxConsecFailures is the circuit-breaker threshold for consecutive compaction failures.
	// Default: 3.
	MaxConsecFailures int
}

// DefaultConfig returns sensible defaults for a 128K-token model.
func DefaultConfig() Config {
	return Config{
		ContextWindowTokens:  128_000,
		MaxOutputTokens:      20_000,
		AutoCompactBuffer:    13_000,
		KeepRecentTurns:      4,
		MicroCompactMaxChars: 8_000,
		StripMaxChars:        2_000,
		MaxPTLRetries:        3,
		MaxConsecFailures:    3,
	}
}

func (c Config) autoCompactThreshold() int {
	return c.ContextWindowTokens - c.MaxOutputTokens - c.AutoCompactBuffer
}

// Service orchestrates the compression pipeline.
type Service struct {
	cfg   Config
	model model.BaseChatModel

	mu                  sync.Mutex
	consecutiveFailures int
}

// New creates a Service. m is used only for the GlobalCompact LLM call.
func New(cfg Config, m model.BaseChatModel) *Service {
	return &Service{cfg: cfg, model: m}
}

// CircuitBreakerTripped reports whether too many consecutive failures have disabled auto-compaction.
func (s *Service) CircuitBreakerTripped() bool {
	if s.cfg.MaxConsecFailures <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.consecutiveFailures >= s.cfg.MaxConsecFailures
}

// ShouldAutoCompact reports whether the current history warrants automatic compaction.
func (s *Service) ShouldAutoCompact(messages []*schema.Message) bool {
	if s.cfg.ContextWindowTokens == 0 {
		return false
	}
	if s.CircuitBreakerTripped() {
		return false
	}
	return EstimateMessagesTokens(messages) >= s.cfg.autoCompactThreshold()
}

// Result is the outcome of Compact.
type Result struct {
	Messages   []*schema.Message
	PreTokens  int
	PostTokens int
	Summary    string
	GlobalUsed bool
}

// Compact runs all applicable layers unconditionally.
func (s *Service) Compact(ctx context.Context, messages []*schema.Message) (*Result, error) {
	preTokens := EstimateMessagesTokens(messages)

	// Layer 1: MicroCompact — shorten oversized old turns, no LLM call.
	after1 := MicroCompact(messages, s.cfg.KeepRecentTurns, s.cfg.MicroCompactMaxChars)

	tokensAfterMicro := EstimateMessagesTokens(after1)
	if tokensAfterMicro < s.cfg.autoCompactThreshold() {
		return &Result{
			Messages:   after1,
			PreTokens:  preTokens,
			PostTokens: tokensAfterMicro,
		}, nil
	}

	// Layer 2: Strip — truncate large tool-result content before the LLM call.
	after2 := StripToolResults(after1, s.cfg.StripMaxChars)

	// Layer 3: GlobalCompact (summarise → PTL retry).
	gr, err := GlobalCompact(ctx, s.model, after2, "", s.cfg.MaxPTLRetries)
	if err != nil {
		s.mu.Lock()
		s.consecutiveFailures++
		s.mu.Unlock()
		return nil, fmt.Errorf("compact: %w", err)
	}

	s.mu.Lock()
	s.consecutiveFailures = 0
	s.mu.Unlock()

	return &Result{
		Messages:   gr.Messages,
		PreTokens:  preTokens,
		PostTokens: gr.PostTokens,
		Summary:    gr.Summary,
		GlobalUsed: true,
	}, nil
}
