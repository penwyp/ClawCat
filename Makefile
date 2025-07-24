.PHONY: all build test lint bench deps install clean run fmt fmt-check build-all release race ci help

# Variables
BINARY_NAME := clawcat
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt
GOLINT := golangci-lint

# Build flags
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Default target
all: clean lint test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) .

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	@echo "Running linter..."
	$(GOLINT) run --timeout=5m

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem -run=^# ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(LDFLAGS) .

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@rm -rf dist/

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./bin/$(BINARY_NAME)

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Check formatting
fmt-check:
	@echo "Checking code formatting..."
	@test -z "$$($(GOFMT) -s -l . | tee /dev/stderr)" || (echo "Please run 'make fmt' to format code" && exit 1)

# Build for all platforms
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p bin
	
	@echo "Building for macOS (amd64)..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64
	
	@echo "Building for macOS (arm64)..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64
	
	@echo "Building for Linux (amd64)..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64
	
	@echo "Building for Linux (arm64)..."
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64
	
	@echo "Building for Windows (amd64)..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe

# Create release archives
release: build-all
	@echo "Creating release archives..."
	@mkdir -p dist
	@cp bin/$(BINARY_NAME)-darwin-amd64 dist/
	@cp bin/$(BINARY_NAME)-darwin-arm64 dist/
	@cp bin/$(BINARY_NAME)-linux-amd64 dist/
	@cp bin/$(BINARY_NAME)-linux-arm64 dist/
	@cp bin/$(BINARY_NAME)-windows-amd64.exe dist/
	@cd dist && tar -czf $(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	@cd dist && tar -czf $(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	@cd dist && tar -czf $(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	@cd dist && tar -czf $(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	@cd dist && zip $(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe

# Run with race detector
race:
	@echo "Running with race detector..."
	@mkdir -p bin
	$(GOBUILD) -race $(LDFLAGS) -o bin/$(BINARY_NAME)-race .
	./bin/$(BINARY_NAME)-race

# CI/CD tasks
ci: deps lint fmt-check test

# Show help
help:
	@echo "Available targets:"
	@echo "  all          - Clean, lint, test, and build"
	@echo "  build        - Build the binary"
	@echo "  test         - Run tests with coverage"
	@echo "  lint         - Run linter"
	@echo "  bench        - Run benchmarks"
	@echo "  deps         - Install dependencies"
	@echo "  install      - Install the binary"
	@echo "  clean        - Clean build artifacts"
	@echo "  run          - Build and run the application"
	@echo "  fmt          - Format code"
	@echo "  fmt-check    - Check code formatting"
	@echo "  build-all    - Build for all platforms"
	@echo "  release      - Create release archives"
	@echo "  race         - Run with race detector"
	@echo "  ci           - Run CI tasks"
	@echo "  help         - Show this help message"

# Default help
.DEFAULT_GOAL := help