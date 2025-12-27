package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs
// This includes OpenAI, Ollama, LM Studio, and other compatible services
type OpenAIProvider struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAIProvider creates a new OpenAI-compatible provider
func NewOpenAIProvider(baseURL, apiKey, model string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = os.Getenv("OPENAI_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if model == "" {
		model = os.Getenv("OPENAI_MODEL")
		if model == "" {
			model = "gpt-4"
		}
	}

	return &OpenAIProvider{
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{},
	}
}

// OpenAI API request/response structures
type openAIRequest struct {
	Model      string          `json:"model"`
	Messages   []openAIMessage `json:"messages"`
	MaxTokens  int             `json:"max_tokens,omitempty"`
	Tools      []openAITool    `json:"tools,omitempty"`
	ToolChoice interface{}     `json:"tool_choice,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content,omitempty"` // string or []contentPart
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
}

type openAIChoice struct {
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// CreateChatCompletion implements the Provider interface for OpenAI-compatible APIs
func (p *OpenAIProvider) CreateChatCompletion(ctx context.Context, request ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// Convert our generic messages to OpenAI format
	messages := make([]openAIMessage, 0)

	for _, msg := range request.Messages {
		oaiMsg := openAIMessage{
			Role: msg.Role,
		}

		// Check if this message contains tool calls or tool results
		var hasToolUse, hasToolResult bool
		var toolCalls []openAIToolCall

		for _, content := range msg.Content {
			switch content.Type {
			case "text":
				if oaiMsg.Content == nil {
					oaiMsg.Content = content.Text
				}
			case "tool_use":
				hasToolUse = true
				toolCalls = append(toolCalls, openAIToolCall{
					ID:   content.ToolUseID,
					Type: "function",
					Function: openAIFunctionCall{
						Name:      content.ToolName,
						Arguments: string(content.ToolInput),
					},
				})
			case "tool_result":
				hasToolResult = true
				// Tool results are sent as tool messages in OpenAI format
				messages = append(messages, openAIMessage{
					Role:       "tool",
					Content:    content.ToolResultValue,
					ToolCallID: content.ToolResultID,
				})
			}
		}

		if hasToolUse {
			oaiMsg.ToolCalls = toolCalls
		}

		// Only add the message if it's not just tool results (those were added separately)
		if !hasToolResult {
			messages = append(messages, oaiMsg)
		}
	}

	// Convert tools to OpenAI format
	tools := make([]openAITool, 0, len(request.Tools))
	for _, tool := range request.Tools {
		parameters := map[string]interface{}{
			"type":       "object",
			"properties": tool.InputSchema.Properties,
		}
		if len(tool.InputSchema.Required) > 0 {
			parameters["required"] = tool.InputSchema.Required
		}

		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  parameters,
			},
		})
	}

	// Build the request
	reqBody := openAIRequest{
		Model:     p.model,
		Messages:  messages,
		MaxTokens: request.MaxTokens,
	}

	if len(tools) > 0 {
		reqBody.Tools = tools
		if request.ToolChoice != "" {
			reqBody.ToolChoice = request.ToolChoice
		} else {
			reqBody.ToolChoice = "auto"
		}
	}

	// Marshal request to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))
	}

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var oaiResp openAIResponse
	if err := json.Unmarshal(body, &oaiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	// Convert OpenAI response to our generic format
	choice := oaiResp.Choices[0]
	contentBlocks := make([]ContentBlock, 0)

	// Add text content if present
	if choice.Message.Content != nil {
		if contentStr, ok := choice.Message.Content.(string); ok && contentStr != "" {
			contentBlocks = append(contentBlocks, ContentBlock{
				Type: "text",
				Text: contentStr,
			})
		}
	}

	// Add tool calls if present
	for _, toolCall := range choice.Message.ToolCalls {
		contentBlocks = append(contentBlocks, ContentBlock{
			Type:      "tool_use",
			ToolUseID: toolCall.ID,
			ToolName:  toolCall.Function.Name,
			ToolInput: json.RawMessage(toolCall.Function.Arguments),
		})
	}

	stopReason := choice.FinishReason
	if stopReason == "tool_calls" {
		stopReason = "tool_use"
	}

	return &ChatCompletionResponse{
		Content:    contentBlocks,
		StopReason: stopReason,
	}, nil
}
