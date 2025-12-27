package main

import (
	"context"
	"encoding/json"
)

// Provider defines the interface that all LLM providers must implement
type Provider interface {
	// CreateChatCompletion sends a chat completion request and returns the response
	CreateChatCompletion(ctx context.Context, request ChatCompletionRequest) (*ChatCompletionResponse, error)
}

// ChatCompletionRequest represents a chat completion request
type ChatCompletionRequest struct {
	Model      string
	MaxTokens  int
	Messages   []Message
	Tools      []Tool
	ToolChoice string // "auto", "required", or "none"
}

// ChatCompletionResponse represents a chat completion response
type ChatCompletionResponse struct {
	Content    []ContentBlock
	StopReason string
}

// Message represents a single message in the conversation
type Message struct {
	Role    string // "user" or "assistant"
	Content []ContentBlock
}

// ContentBlock represents a piece of content (text or tool use/result)
type ContentBlock struct {
	Type string // "text", "tool_use", or "tool_result"

	// For text blocks
	Text string

	// For tool_use blocks
	ToolUseID string
	ToolName  string
	ToolInput json.RawMessage

	// For tool_result blocks
	ToolResultID    string
	ToolResultValue string
	IsError         bool
}

// Tool represents a tool definition
type Tool struct {
	Name        string
	Description string
	InputSchema ToolInputSchema
}

// ToolInputSchema represents the JSON schema for tool inputs
type ToolInputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}
