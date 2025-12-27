package main

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// AnthropicProvider implements the Provider interface for Anthropic's Claude API
type AnthropicProvider struct {
	client *anthropic.Client
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider() *AnthropicProvider {
	client := anthropic.NewClient()
	return &AnthropicProvider{
		client: &client,
	}
}

// CreateChatCompletion implements the Provider interface for Anthropic
func (p *AnthropicProvider) CreateChatCompletion(ctx context.Context, request ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// Convert our generic messages to Anthropic format
	messages := make([]anthropic.MessageParam, 0, len(request.Messages))
	for _, msg := range request.Messages {
		contentBlocks := make([]anthropic.ContentBlockParamUnion, 0, len(msg.Content))
		for _, content := range msg.Content {
			switch content.Type {
			case "text":
				contentBlocks = append(contentBlocks, anthropic.NewTextBlock(content.Text))
			case "tool_use":
				// Tool use is part of assistant messages, handled by Anthropic SDK
			case "tool_result":
				contentBlocks = append(contentBlocks, anthropic.NewToolResultBlock(
					content.ToolResultID,
					content.ToolResultValue,
					content.IsError,
				))
			}
		}

		if msg.Role == "user" {
			messages = append(messages, anthropic.NewUserMessage(contentBlocks...))
		}
		// Assistant messages are added by the SDK's response handling
	}

	// Convert tools to Anthropic format
	anthropicTools := make([]anthropic.ToolUnionParam, 0, len(request.Tools))
	for _, tool := range request.Tools {
		// Convert our ToolInputSchema to Anthropic's format
		// Need to convert map[string]interface{} to OrderedMap
		properties := orderedmap.New[string, interface{}]()
		for key, value := range tool.InputSchema.Properties {
			properties.Set(key, value)
		}
		
		inputSchema := anthropic.ToolInputSchemaParam{
			Properties: properties,
		}

		anthropicTools = append(anthropicTools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: inputSchema,
			},
		})
	}

	// Make the API call
	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude3_7SonnetLatest,
		MaxTokens: int64(request.MaxTokens),
		Messages:  messages,
	}

	if len(anthropicTools) > 0 {
		params.Tools = anthropicTools
	}

	response, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, err
	}

	// Convert Anthropic response to our generic format
	contentBlocks := make([]ContentBlock, 0, len(response.Content))
	for _, content := range response.Content {
		switch content.Type {
		case "text":
			contentBlocks = append(contentBlocks, ContentBlock{
				Type: "text",
				Text: content.Text,
			})
		case "tool_use":
			toolUse := content.AsToolUse()
			contentBlocks = append(contentBlocks, ContentBlock{
				Type:        "tool_use",
				ToolUseID:   toolUse.ID,
				ToolName:    toolUse.Name,
				ToolInput:   toolUse.Input,
			})
		}
	}

	return &ChatCompletionResponse{
		Content:    contentBlocks,
		StopReason: string(response.StopReason),
	}, nil
}
