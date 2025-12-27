.PHONY: build fmt check clean all

# Go binaries to build
BINARIES := bash_tool chat code_search_tool edit_tool list_files read

# Build all binaries
build:
	@echo "Building binaries..."
	go build -o bash_tool bash_tool.go provider.go provider_anthropic.go provider_openai.go provider_factory.go
	go build -o chat chat.go provider.go provider_anthropic.go provider_openai.go provider_factory.go
	go build -o code_search_tool code_search_tool.go provider.go provider_anthropic.go provider_openai.go provider_factory.go
	go build -o edit_tool edit_tool.go provider.go provider_anthropic.go provider_openai.go provider_factory.go
	go build -o list_files list_files.go provider.go provider_anthropic.go provider_openai.go provider_factory.go
	go build -o read read.go provider.go provider_anthropic.go provider_openai.go provider_factory.go

# Format all Go files
fmt:
	@echo "Formatting Go files..."
	go fmt ./...

# Check (lint and vet) all Go files
check:
	@echo "Running go vet on individual files..."
	go vet bash_tool.go provider.go provider_anthropic.go provider_openai.go provider_factory.go
	go vet chat.go provider.go provider_anthropic.go provider_openai.go provider_factory.go
	go vet code_search_tool.go provider.go provider_anthropic.go provider_openai.go provider_factory.go
	go vet edit_tool.go provider.go provider_anthropic.go provider_openai.go provider_factory.go
	go vet list_files.go provider.go provider_anthropic.go provider_openai.go provider_factory.go
	go vet read.go provider.go provider_anthropic.go provider_openai.go provider_factory.go
	@echo "Running go mod tidy..."
	go mod tidy

# Clean built binaries
clean:
	@echo "Cleaning binaries..."
	rm -f $(BINARIES)

# Build everything and run checks
all: fmt check build

# Default target
.DEFAULT_GOAL := all
