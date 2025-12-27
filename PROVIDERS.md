# Provider Configuration Examples

This file provides example configurations for different AI providers.
Copy the relevant section to your shell profile or export before running the agents.

## Anthropic Claude (Default)
# This is the default provider - no PROVIDER variable needed
export ANTHROPIC_API_KEY="sk-ant-..."

## OpenAI
export PROVIDER="openai"
export OPENAI_API_KEY="sk-..."
export OPENAI_MODEL="gpt-4"  # Optional: defaults to gpt-4

## Ollama (Local)
# Make sure Ollama is running: ollama serve
# Check available models: ollama list
export PROVIDER="openai"
export OPENAI_BASE_URL="http://localhost:11434/v1"
export OPENAI_MODEL="llama2"  # Or any model you have installed
# No API key needed for local Ollama

## LM Studio (Local)
# Start LM Studio and load a model
# The base URL may vary depending on your LM Studio configuration
export PROVIDER="openai"
export OPENAI_BASE_URL="http://localhost:1234/v1"
export OPENAI_MODEL="local-model"  # Check LM Studio for the exact model name
# No API key needed for local LM Studio

## Notes:
# - Only one provider configuration should be active at a time
# - The PROVIDER variable determines which provider adapter to use
# - When PROVIDER is not set, Anthropic is used by default
# - OpenAI-compatible providers (Ollama, LM Studio) use PROVIDER="openai"
