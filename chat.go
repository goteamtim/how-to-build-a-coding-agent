package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
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

	agent := NewAgent(provider, getUserMessage, *verbose)
	err = agent.Run(context.TODO())
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}

func NewAgent(provider Provider, getUserMessage func() (string, bool), verbose bool) *Agent {
	return &Agent{
		provider:       provider,
		getUserMessage: getUserMessage,
		verbose:        verbose,
	}
}

type Agent struct {
	provider       Provider
	getUserMessage func() (string, bool)
	verbose        bool
}

func (a *Agent) Run(ctx context.Context) error {
	conversation := []Message{}

	if a.verbose {
		log.Println("Starting chat session")
	}
	fmt.Println("Chat with Claude (use 'ctrl-c' to quit)")

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
			log.Printf("Sending message to Claude, conversation length: %d", len(conversation))
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

		if a.verbose {
			log.Printf("Received response with %d content blocks", len(response.Content))
		}

		for _, content := range response.Content {
			switch content.Type {
			case "text":
				fmt.Printf("\u001b[93mAssistant\u001b[0m: %s\n", content.Text)
			}
		}
	}

	if a.verbose {
		log.Println("Chat session ended")
	}
	return nil
}

func (a *Agent) runInference(ctx context.Context, conversation []Message) (*ChatCompletionResponse, error) {
	if a.verbose {
		log.Printf("Making API call with %d messages", len(conversation))
	}

	request := ChatCompletionRequest{
		MaxTokens: 1024,
		Messages:  conversation,
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
