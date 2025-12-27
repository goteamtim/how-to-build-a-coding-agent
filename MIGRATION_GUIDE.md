# Migration Guide: Provider-Agnostic Updates

This guide helps existing users understand the changes made to support multiple LLM providers.

## What Changed?

The repository has been refactored to support multiple AI providers through a clean interface abstraction. This means you can now use:
- Anthropic Claude (existing default)
- OpenAI (GPT-4, GPT-3.5, etc.)
- Ollama (local models)
- LM Studio (local models)
- Any OpenAI-compatible API

## Breaking Changes

**None!** The changes are fully backward compatible. If you don't change anything, the agents will continue to use Anthropic Claude as before.

## What You Need to Know

### For Existing Users (Anthropic)

No action required! Your existing setup will continue to work:

```bash
export ANTHROPIC_API_KEY="your-key"
go run chat.go  # Still uses Anthropic by default
```

### For New Provider Users

Set the appropriate environment variables before running the agents:

**OpenAI:**
```bash
export PROVIDER="openai"
export OPENAI_API_KEY="your-key"
export OPENAI_MODEL="gpt-4"  # optional
go run chat.go
```

**Ollama (local):**
```bash
# Start Ollama first: ollama serve
export PROVIDER="openai"
export OPENAI_BASE_URL="http://localhost:11434/v1"
export OPENAI_MODEL="llama2"
go run chat.go
```

**LM Studio (local):**
```bash
# Start LM Studio and load a model first
export PROVIDER="openai"
export OPENAI_BASE_URL="http://localhost:1234/v1"
export OPENAI_MODEL="your-model-name"
go run chat.go
```

## Architecture Changes

### Before:
```
Agent → Anthropic SDK → Claude API
```

### After:
```
Agent → Provider Interface → [Anthropic Adapter → Claude API]
                           → [OpenAI Adapter → OpenAI/Ollama/LM Studio API]
```

The provider interface abstracts away the details of each provider, making the agents provider-agnostic.

## Code Changes

If you've built custom agents based on this codebase:

1. **Replace direct Anthropic client usage** with the provider interface:
   ```go
   // Old
   client := anthropic.NewClient()
   
   // New
   provider, err := NewProviderFromEnv()
   ```

2. **Update message handling** to use generic types:
   ```go
   // Old
   conversation := []anthropic.MessageParam{}
   
   // New
   conversation := []Message{}
   ```

3. **Include provider files** in your builds:
   ```bash
   go build myagent.go provider.go provider_anthropic.go provider_openai.go provider_factory.go
   ```

## New Files

- `provider.go` - Core interface and types
- `provider_anthropic.go` - Anthropic adapter
- `provider_openai.go` - OpenAI-compatible adapter
- `provider_factory.go` - Provider selection logic
- `PROVIDERS.md` - Configuration examples

## Benefits

1. **Flexibility**: Switch between providers easily
2. **Local Development**: Use Ollama or LM Studio for free local testing
3. **Cost Optimization**: Choose the most cost-effective provider for your use case
4. **Vendor Independence**: Not locked into a single provider
5. **Future-Proof**: Easy to add new providers (Azure OpenAI, Google Vertex AI, etc.)

## Questions?

Check out:
- `README.md` for comprehensive documentation
- `PROVIDERS.md` for configuration examples
- The code itself - the provider interface is well-documented

## Contributing

Want to add support for a new provider? Implement the `Provider` interface in a new file (e.g., `provider_azure.go`) and submit a PR!
