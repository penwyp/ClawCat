.PHONY: all build test lint bench install clean run help

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
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run tests with short flag
test-short:
	@echo "Running short tests..."
	$(GOTEST) -v -short ./...

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

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(LDFLAGS) .

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@rm -rf dist/

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Check formatting
fmt-check:
	@echo "Checking code formatting..."
	@test -z "$$($(GOFMT) -s -l . | tee /dev/stderr)" || (echo "Please run 'make fmt' to format code" && exit 1)

# Generate mocks
mocks:
	@echo "Generating mocks..."
	$(GOCMD) generate ./...

# Run security scan
security:
	@echo "Running security scan..."
	gosec -fmt=junit-xml -out=gosec-report.xml ./... || true

# Build for all platforms
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p dist
	
	@echo "Building for macOS (amd64)..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64
	
	@echo "Building for macOS (arm64)..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64
	
	@echo "Building for Linux (amd64)..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64
	
	@echo "Building for Linux (arm64)..."
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64
	
	@echo "Building for Windows (amd64)..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe

# Create release archives
release: build-all
	@echo "Creating release archives..."
	@cd dist && tar -czf $(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	@cd dist && tar -czf $(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	@cd dist && tar -czf $(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	@cd dist && tar -czf $(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	@cd dist && zip $(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe

# Run with race detector
race:
	@echo "Running with race detector..."
	$(GOBUILD) -race $(LDFLAGS) -o $(BINARY_NAME)-race .
	./$(BINARY_NAME)-race

# Profile CPU
profile-cpu:
	@echo "Running CPU profile..."
	$(GOTEST) -cpuprofile=cpu.prof -bench=. ./...
	$(GOCMD) tool pprof cpu.prof

# Profile memory
profile-mem:
	@echo "Running memory profile..."
	$(GOTEST) -memprofile=mem.prof -bench=. ./...
	$(GOCMD) tool pprof mem.prof

# Check for outdated dependencies
deps-check:
	@echo "Checking for outdated dependencies..."
	$(GOCMD) list -u -m all

# Development setup
dev-setup:
	@echo "Setting up development environment..."
	$(GOCMD) install github.com/golang/mock/mockgen@latest
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOCMD) install github.com/securego/gosec/v2/cmd/gosec@latest
	$(GOCMD) install github.com/axw/gocov/gocov@latest
	$(GOCMD) install github.com/AlekSi/gocov-xml@latest

# CI/CD tasks
ci: deps lint fmt-check test security

# Show help
help:
	@echo "Available targets:"
	@echo "  all          - Clean, lint, test, and build"
	@echo "  build        - Build the binary"
	@echo "  test         - Run tests with coverage"
	@echo "  test-short   - Run short tests"
	@echo "  lint         - Run linter"
	@echo "  bench        - Run benchmarks"
	@echo "  deps         - Install dependencies"
	@echo "  deps-update  - Update dependencies"
	@echo "  install      - Install the binary"
	@echo "  clean        - Clean build artifacts"
	@echo "  run          - Build and run the application"
	@echo "  fmt          - Format code"
	@echo "  fmt-check    - Check code formatting"
	@echo "  mocks        - Generate mocks"
	@echo "  security     - Run security scan"
	@echo "  build-all    - Build for all platforms"
	@echo "  release      - Create release archives"
	@echo "  race         - Run with race detector"
	@echo "  profile-cpu  - Run CPU profiling"
	@echo "  profile-mem  - Run memory profiling"
	@echo "  deps-check   - Check for outdated dependencies"
	@echo "  dev-setup    - Set up development environment"
	@echo "  ci           - Run CI tasks"
	@echo "  help         - Show this help message"

# Default help
.DEFAULT_GOAL := help