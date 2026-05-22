package brain

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"forge/internal/config"
	anthropicprovider "forge/internal/provider/anthropic"
	"forge/internal/tools"
)

// NewToolCallingModel constructs the chat model for the configured provider.
func NewToolCallingModel(ctx context.Context, c config.ModelConfig) (model.ToolCallingChatModel, error) {
	switch c.Provider {
	case "anthropic":
		return anthropicprovider.NewChatModel(ctx, anthropicprovider.Config{
			APIKey:    c.APIKey,
			BaseURL:   c.BaseURL,
			Model:     c.Model,
			MaxTokens: c.MaxTokens,
		})
	default: // openai (and OpenAI-compatible)
		if c.APIKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is required")
		}
		cfg := &openai.ChatModelConfig{
			APIKey: c.APIKey,
			Model:  c.Model,
		}
		if c.BaseURL != "" {
			cfg.BaseURL = c.BaseURL
		}
		if c.ByAzure {
			cfg.ByAzure = true
			cfg.APIVersion = c.APIVersion
		}
		return openai.NewChatModel(ctx, cfg)
	}
}

// NewFromConfig creates a Brain from a ModelConfig.
func NewFromConfig(ctx context.Context, modelCfg config.ModelConfig, reg tools.ToolRegistry) (*Brain, error) {
	m, err := NewToolCallingModel(ctx, modelCfg)
	if err != nil {
		return nil, fmt.Errorf("create model: %w", err)
	}
	return New(ctx, m, reg, DefaultBrainConfig())
}
