package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/invopop/jsonschema"
)

func main() {
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	flag.Parse()

	if *verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("Verbose logging enabled")
	} else {
		log.SetOutput(os.Stdout)
		log.SetFlags(0)
		log.SetPrefix("")
	}

	provider, err := NewProviderFromEnv()
	if err != nil {
		fmt.Printf("Error initializing provider: %s\n", err.Error())
		os.Exit(1)
	}
	if *verbose {
		log.Println("Provider initialized")
	}

	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	tools := []ToolDefinition{ReadFileDefinition, ListFilesDefinition, BashDefinition, CodeSearchDefinition}
	if *verbose {
		log.Printf("Initialized %d tools", len(tools))
	}
	agent := NewAgent(provider, getUserMessage, tools, *verbose)
	err = agent.Run(context.TODO())
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}

func NewAgent(
	provider Provider,
	getUserMessage func() (string, bool),
	tools []ToolDefinition,
	verbose bool,
) *Agent {
	return &Agent{
		provider:       provider,
		getUserMessage: getUserMessage,
		tools:          tools,
		verbose:        verbose,
	}
}

type Agent struct {
	provider       Provider
	getUserMessage func() (string, bool)
	tools          []ToolDefinition
	verbose        bool
}

func (a *Agent) Run(ctx context.Context) error {
	conversation := []Message{}

	if a.verbose {
		log.Println("Starting chat session with tools enabled")
	}
	fmt.Println("Chat with Assistant (use 'ctrl-c' to quit)")

	for {
		fmt.Print("\u001b[94mYou\u001b[0m: ")
		userInput, ok := a.getUserMessage()
		if !ok {
			if a.verbose {
				log.Println("User input ended, breaking from chat loop")
			}
			break
		}

		// Skip empty messages
		if userInput == "" {
			if a.verbose {
				log.Println("Skipping empty message")
			}
			continue
		}

		if a.verbose {
			log.Printf("User input received: %q", userInput)
		}

		userMessage := Message{
			Role: "user",
			Content: []ContentBlock{
				{Type: "text", Text: userInput},
			},
		}
		conversation = append(conversation, userMessage)

		if a.verbose {
			log.Printf("Sending message, conversation length: %d", len(conversation))
		}

		response, err := a.runInference(ctx, conversation)
		if err != nil {
			if a.verbose {
				log.Printf("Error during inference: %v", err)
			}
			return err
		}
		
		assistantMessage := Message{
			Role:    "assistant",
			Content: response.Content,
		}
		conversation = append(conversation, assistantMessage)

		// Keep processing until assistant stops using tools
		for {
			// Collect all tool uses and their results
			var toolResults []ContentBlock
			var hasToolUse bool

			if a.verbose {
				log.Printf("Processing %d content blocks", len(response.Content))
			}

			for _, content := range response.Content {
				switch content.Type {
				case "text":
					fmt.Printf("\u001b[93mAssistant\u001b[0m: %s\n", content.Text)
				case "tool_use":
					hasToolUse = true
					if a.verbose {
						log.Printf("Tool use detected: %s with input: %s", content.ToolName, string(content.ToolInput))
					}
					fmt.Printf("\u001b[96mtool\u001b[0m: %s(%s)\n", content.ToolName, string(content.ToolInput))

					// Find and execute the tool
					var toolResult string
					var toolError error
					var toolFound bool
					for _, tool := range a.tools {
						if tool.Name == content.ToolName {
							if a.verbose {
								log.Printf("Executing tool: %s", tool.Name)
							}
							toolResult, toolError = tool.Function(content.ToolInput)
							fmt.Printf("\u001b[92mresult\u001b[0m: %s\n", toolResult)
							if toolError != nil {
								fmt.Printf("\u001b[91merror\u001b[0m: %s\n", toolError.Error())
							}
							if a.verbose {
								if toolError != nil {
									log.Printf("Tool execution failed: %v", toolError)
								} else {
									log.Printf("Tool execution successful, result length: %d chars", len(toolResult))
								}
							}
							toolFound = true
							break
						}
					}

					if !toolFound {
						toolError = fmt.Errorf("tool '%s' not found", content.ToolName)
						fmt.Printf("\u001b[91merror\u001b[0m: %s\n", toolError.Error())
					}

					// Add tool result to collection
					if toolError != nil {
						toolResults = append(toolResults, ContentBlock{
							Type:            "tool_result",
							ToolResultID:    content.ToolUseID,
							ToolResultValue: toolError.Error(),
							IsError:         true,
						})
					} else {
						toolResults = append(toolResults, ContentBlock{
							Type:            "tool_result",
							ToolResultID:    content.ToolUseID,
							ToolResultValue: toolResult,
							IsError:         false,
						})
					}
				}
			}

			// If there were no tool uses, we're done
			if !hasToolUse {
				break
			}

			// Send all tool results back and get assistant's response
			if a.verbose {
				log.Printf("Sending %d tool results back", len(toolResults))
			}
			toolResultMessage := Message{
				Role:    "user",
				Content: toolResults,
			}
			conversation = append(conversation, toolResultMessage)

			// Get assistant's response after tool execution
			response, err = a.runInference(ctx, conversation)
			if err != nil {
				if a.verbose {
					log.Printf("Error during followup inference: %v", err)
				}
				return err
			}
			
			assistantMessage := Message{
				Role:    "assistant",
				Content: response.Content,
			}
			conversation = append(conversation, assistantMessage)

			if a.verbose {
				log.Printf("Received followup response with %d content blocks", len(response.Content))
			}

			// Continue loop to process the new message
		}
	}

	if a.verbose {
		log.Println("Chat session ended")
	}
	return nil
}

func (a *Agent) runInference(ctx context.Context, conversation []Message) (*ChatCompletionResponse, error) {
	// Convert tool definitions to generic Tool format
	tools := make([]Tool, 0, len(a.tools))
	for _, tool := range a.tools {
		tools = append(tools, Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	if a.verbose {
		log.Printf("Making API call with %d tools", len(tools))
	}

	request := ChatCompletionRequest{
		MaxTokens: 1024,
		Messages:  conversation,
		Tools:     tools,
	}

	response, err := a.provider.CreateChatCompletion(ctx, request)

	if a.verbose {
		if err != nil {
			log.Printf("API call failed: %v", err)
		} else {
			log.Printf("API call successful, response received")
		}
	}

	return response, err
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema ToolInputSchema `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

var ReadFileDefinition = ToolDefinition{
	Name:        "read_file",
	Description: "Read the contents of a given relative file path. Use this when you want to see what's inside a file. Do not use this with directory names.",
	InputSchema: ReadFileInputSchema,
	Function:    ReadFile,
}

type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"The relative path of a file in the working directory."`
}

var ReadFileInputSchema = GenerateSchema[ReadFileInput]()

func ReadFile(input json.RawMessage) (string, error) {
	readFileInput := ReadFileInput{}
	err := json.Unmarshal(input, &readFileInput)
	if err != nil {
		panic(err)
	}

	log.Printf("Reading file: %s", readFileInput.Path)
	content, err := os.ReadFile(readFileInput.Path)
	if err != nil {
		log.Printf("Failed to read file %s: %v", readFileInput.Path, err)
		return "", err
	}
	log.Printf("Successfully read file %s (%d bytes)", readFileInput.Path, len(content))
	return string(content), nil
}

var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Description: "List files and directories at a given path. If no path is provided, lists files in the current directory.",
	InputSchema: ListFilesInputSchema,
	Function:    ListFiles,
}

type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to current directory if not provided."`
}

var ListFilesInputSchema = GenerateSchema[ListFilesInput]()

func ListFiles(input json.RawMessage) (string, error) {
	listFilesInput := ListFilesInput{}
	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		panic(err)
	}

	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}

	log.Printf("Listing files in directory: %s", dir)
	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// Skip .devenv directory and its contents
		if info.IsDir() && (relPath == ".devenv" || strings.HasPrefix(relPath, ".devenv/")) {
			return filepath.SkipDir
		}

		if relPath != "." {
			if info.IsDir() {
				files = append(files, relPath+"/")
			} else {
				files = append(files, relPath)
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("Failed to list files in %s: %v", dir, err)
		return "", err
	}

	result, err := json.Marshal(files)
	if err != nil {
		return "", err
	}

	log.Printf("Successfully listed %d files/directories in %s", len(files), dir)
	return string(result), nil
}

var BashDefinition = ToolDefinition{
	Name:        "bash",
	Description: "Execute a bash command and return its output. Use this to run shell commands.",
	InputSchema: BashInputSchema,
	Function:    Bash,
}

type BashInput struct {
	Command string `json:"command" jsonschema_description:"The bash command to execute."`
}

var BashInputSchema = GenerateSchema[BashInput]()

func Bash(input json.RawMessage) (string, error) {
	bashInput := BashInput{}
	err := json.Unmarshal(input, &bashInput)
	if err != nil {
		return "", err
	}

	log.Printf("Executing bash command: %s", bashInput.Command)
	cmd := exec.Command("bash", "-c", bashInput.Command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Bash command failed: %s, error: %v", bashInput.Command, err)
		return fmt.Sprintf("Command failed with error: %s\nOutput: %s", err.Error(), string(output)), nil
	}

	log.Printf("Bash command succeeded: %s (output: %d bytes)", bashInput.Command, len(output))
	return strings.TrimSpace(string(output)), nil
}

var CodeSearchDefinition = ToolDefinition{
	Name: "code_search",
	Description: `Search for code patterns using ripgrep (rg).

Use this to find code patterns, function definitions, variable usage, or any text in the codebase.
You can search by pattern, file type, or directory.`,
	InputSchema: CodeSearchInputSchema,
	Function:    CodeSearch,
}

type CodeSearchInput struct {
	Pattern       string `json:"pattern" jsonschema_description:"The search pattern or regex to look for"`
	Path          string `json:"path,omitempty" jsonschema_description:"Optional path to search in (file or directory)"`
	FileType      string `json:"file_type,omitempty" jsonschema_description:"Optional file extension to limit search to (e.g., 'go', 'js', 'py')"`
	CaseSensitive bool   `json:"case_sensitive,omitempty" jsonschema_description:"Whether the search should be case sensitive (default: false)"`
}

var CodeSearchInputSchema = GenerateSchema[CodeSearchInput]()

func CodeSearch(input json.RawMessage) (string, error) {
	codeSearchInput := CodeSearchInput{}
	err := json.Unmarshal(input, &codeSearchInput)
	if err != nil {
		return "", err
	}

	if codeSearchInput.Pattern == "" {
		log.Printf("CodeSearch failed: pattern is required")
		return "", fmt.Errorf("pattern is required")
	}

	log.Printf("Searching for pattern: %s", codeSearchInput.Pattern)

	// Build ripgrep command
	args := []string{"rg", "--line-number", "--with-filename", "--color=never"}

	// Add case sensitivity flag
	if !codeSearchInput.CaseSensitive {
		args = append(args, "--ignore-case")
	}

	// Add file type filter if specified
	if codeSearchInput.FileType != "" {
		args = append(args, "--type", codeSearchInput.FileType)
	}

	// Add pattern
	args = append(args, codeSearchInput.Pattern)

	// Add path if specified
	if codeSearchInput.Path != "" {
		args = append(args, codeSearchInput.Path)
	} else {
		args = append(args, ".")
	}

	if a := false; a { // This is a hack to access verbose mode
		log.Printf("Executing ripgrep with args: %v", args)
	}

	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.Output()

	// ripgrep returns exit code 1 when no matches are found, which is not an error
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			log.Printf("No matches found for pattern: %s", codeSearchInput.Pattern)
			return "No matches found", nil
		}
		log.Printf("Ripgrep command failed: %v", err)
		return "", fmt.Errorf("search failed: %w", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")

	log.Printf("Found %d matches for pattern: %s", len(lines), codeSearchInput.Pattern)

	// Limit output to prevent overwhelming responses
	if len(lines) > 50 {
		result = strings.Join(lines[:50], "\n") + fmt.Sprintf("\n... (showing first 50 of %d matches)", len(lines))
	}

	return result, nil
}

func GenerateSchema[T any]() ToolInputSchema {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T

	schema := reflector.Reflect(v)

	// Convert OrderedMap to regular map
	properties := make(map[string]interface{})
	if schema.Properties != nil {
		for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			properties[pair.Key] = pair.Value
		}
	}

	return ToolInputSchema{
		Type:       "object",
		Properties: properties,
	}
}
