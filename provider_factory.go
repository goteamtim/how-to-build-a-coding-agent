package main

import (
	"fmt"
	"os"
	"strings"
)

// ProviderType represents the type of LLM provider
type ProviderType string

const (
	ProviderAnthropic ProviderType = "anthropic"
	ProviderOpenAI    ProviderType = "openai"
)

// ProviderConfig holds configuration for creating a provider
type ProviderConfig struct {
	Type    ProviderType
	BaseURL string
	APIKey  string
	Model   string
}

// NewProvider creates a new provider based on the configuration
func NewProvider(config ProviderConfig) (Provider, error) {
	switch config.Type {
	case ProviderAnthropic:
		return NewAnthropicProvider(), nil
	case ProviderOpenAI:
		return NewOpenAIProvider(config.BaseURL, config.APIKey, config.Model), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", config.Type)
	}
}

// NewProviderFromEnv creates a provider based on environment variables
func NewProviderFromEnv() (Provider, error) {
	providerType := os.Getenv("PROVIDER")
	if providerType == "" {
		// Default to Anthropic for backward compatibility
		if os.Getenv("ANTHROPIC_API_KEY") != "" {
			providerType = "anthropic"
		} else if os.Getenv("OPENAI_API_KEY") != "" || os.Getenv("OPENAI_BASE_URL") != "" {
			providerType = "openai"
		} else {
			providerType = "anthropic"
		}
	}

	config := ProviderConfig{
		Type:    ProviderType(strings.ToLower(providerType)),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("OPENAI_MODEL"),
	}

	return NewProvider(config)
}
