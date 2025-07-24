# ClawCat Implementation Guide

## Overview
This guide provides step-by-step instructions for implementing ClawCat based on the detailed module specifications. Each module is designed to be implemented independently and integrated incrementally.

## Prerequisites

### Development Environment
- Go 1.21 or later
- Git for version control
- Make for build automation
- Your preferred Go IDE (VS Code, GoLand, etc.)

### Required Tools
```bash
# Install Go dependencies management tool
go install github.com/golang/mock/mockgen@latest

# Install linting tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install test coverage tool
go install github.com/axw/gocov/gocov@latest
go install github.com/AlekSi/gocov-xml@latest
```

## Implementation Order

### Phase 1: Core Foundation (Week 1)

#### Day 1-2: Project Setup and Models
1. **Initialize Go Module**
```bash
cd /Users/penwyp/go/src/github.com/penwyp/ClawCat
go mod init github.com/penwyp/ClawCat
```

2. **Add Core Dependencies**
```bash
go get github.com/charmbracelet/bubbletea@v0.27.1
go get github.com/charmbracelet/bubbles@v0.18.0
go get github.com/bytedance/sonic@latest
go get github.com/fsnotify/fsnotify@latest
go get github.com/stretchr/testify@latest
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
```

3. **Implement models package**
   - Start with `models/types.go`
   - Add validation in `models/validation.go`
   - Define constants in `models/constants.go`
   - Write comprehensive tests

#### Day 3-4: Basic File I/O
1. **Implement fileio/reader.go**
   - Basic JSONL parsing without Sonic first
   - Add streaming support
   - Integrate Sonic for performance

2. **Implement fileio/discovery.go**
   - Path discovery for different platforms
   - Basic file enumeration

#### Day 5: Core Calculations
1. **Implement calculations/cost.go**
   - Cost calculation functions
   - Model pricing lookups

2. **Implement calculations/stats.go**
   - Basic aggregation functions
   - Time-based grouping

### Phase 2: Data Processing (Week 2)

#### Day 6-7: Complete File I/O
1. **Implement fileio/watcher.go**
   - File system watching with fsnotify
   - Event debouncing

2. **Add caching to fileio**
   - Basic LRU cache
   - Integration with reader

#### Day 8-9: Session Management
1. **Implement sessions/manager.go**
   - Session lifecycle management
   - Concurrent session tracking

2. **Implement sessions/detector.go**
   - Session boundary detection
   - Gap detection logic

#### Day 10: Advanced Calculations
1. **Implement calculations/burnrate.go**
   - Real-time burn rate calculations
   - Historical analysis

2. **Implement calculations/predictions.go**
   - Linear prediction algorithm
   - Confidence intervals

### Phase 3: TUI Implementation (Week 3)

#### Day 11-12: Basic TUI Structure
1. **Implement ui/app.go**
   - Bubble Tea application setup
   - Basic model and update loop

2. **Implement ui/views/dashboard.go**
   - Main dashboard layout
   - Real-time updates

#### Day 13-14: Additional Views
1. **Implement ui/views/sessions.go**
   - Session list with table
   - Sorting and filtering

2. **Implement ui/views/analytics.go**
   - Basic charts
   - Time range selection

#### Day 15: UI Components
1. **Implement ui/components/**
   - Progress bars
   - Tables
   - Charts
   - Spinners

### Phase 4: Polish and Integration (Week 4)

#### Day 16-17: Configuration
1. **Implement config package**
   - Configuration loading
   - Environment variables
   - Validation

2. **Add configuration hot-reload**
   - File watching
   - Runtime updates

#### Day 18-19: Main Application
1. **Implement main.go and cmd/**
   - CLI structure with Cobra
   - Command implementations

2. **Implement internal/app.go**
   - Application orchestration
   - Graceful shutdown

#### Day 20: Testing and Optimization
1. **Integration tests**
   - End-to-end workflows
   - Performance benchmarks

2. **Performance optimization**
   - Profile and optimize
   - Memory usage reduction

## Module Implementation Details

### Models Module
```go
// Start with models/types.go
package models

import (
    "time"
    "github.com/bytedance/sonic"
)

type UsageEntry struct {
    Timestamp           time.Time
    Model               string
    InputTokens         int
    OutputTokens        int
    CacheCreationTokens int
    CacheReadTokens     int
    TotalTokens         int
    CostUSD             float64
}

// Add methods
func (u *UsageEntry) CalculateTotalTokens() int {
    return u.InputTokens + u.OutputTokens + 
           u.CacheCreationTokens + u.CacheReadTokens
}

// Add JSON marshaling with Sonic
func (u *UsageEntry) MarshalJSON() ([]byte, error) {
    return sonic.Marshal(u)
}
```

### File I/O Module
```go
// Start with fileio/reader.go
package fileio

import (
    "bufio"
    "os"
    "github.com/bytedance/sonic"
    "github.com/penwyp/ClawCat/models"
)

type Reader struct {
    file    *os.File
    scanner *bufio.Scanner
}

func NewReader(path string) (*Reader, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    
    return &Reader{
        file:    file,
        scanner: bufio.NewScanner(file),
    }, nil
}

func (r *Reader) ReadEntries() (<-chan models.UsageEntry, <-chan error) {
    entries := make(chan models.UsageEntry)
    errors := make(chan error, 1)
    
    go func() {
        defer close(entries)
        defer close(errors)
        defer r.file.Close()
        
        for r.scanner.Scan() {
            var msg RawMessage
            if err := sonic.Unmarshal(r.scanner.Bytes(), &msg); err != nil {
                continue // Skip invalid lines
            }
            
            if msg.Type == "message" && msg.Usage.InputTokens > 0 {
                entry := convertToUsageEntry(msg)
                entries <- entry
            }
        }
        
        if err := r.scanner.Err(); err != nil {
            errors <- err
        }
    }()
    
    return entries, errors
}
```

### TUI Module
```go
// Start with ui/app.go
package ui

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/penwyp/ClawCat/sessions"
)

type Model struct {
    sessions []*sessions.Session
    view     ViewType
    width    int
    height   int
}

func (m Model) Init() tea.Cmd {
    return tea.Batch(
        tickCmd(),
        tea.WindowSize(),
    )
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "tab":
            m.view = (m.view + 1) % ViewCount
        }
        
    case TickMsg:
        // Update data
        return m, tickCmd()
    }
    
    return m, nil
}

func (m Model) View() string {
    switch m.view {
    case ViewDashboard:
        return m.renderDashboard()
    case ViewSessions:
        return m.renderSessions()
    default:
        return ""
    }
}
```

## Testing Strategy

### Unit Test Example
```go
// models/types_test.go
package models

import (
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
)

func TestUsageEntry_CalculateTotalTokens(t *testing.T) {
    entry := UsageEntry{
        InputTokens:         100,
        OutputTokens:        50,
        CacheCreationTokens: 20,
        CacheReadTokens:     10,
    }
    
    total := entry.CalculateTotalTokens()
    assert.Equal(t, 180, total)
}

func TestUsageEntry_CalculateCost(t *testing.T) {
    entry := UsageEntry{
        Model:        ModelSonnet,
        InputTokens:  1000000, // 1M tokens
        OutputTokens: 500000,  // 500K tokens
    }
    
    pricing := GetPricing(ModelSonnet)
    cost := entry.CalculateCost(pricing)
    
    expectedCost := 3.0 + 7.5 // $3 + $7.50
    assert.InDelta(t, expectedCost, cost, 0.01)
}
```

### Integration Test Example
```go
// integration_test.go
package main

import (
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
)

func TestFullWorkflow(t *testing.T) {
    // Create test data
    testFile := createTestJSONL(t)
    defer os.Remove(testFile)
    
    // Initialize application
    cfg := config.TestConfig()
    app, err := internal.NewApplication(cfg)
    assert.NoError(t, err)
    
    // Process file
    entries, err := app.ProcessFile(testFile)
    assert.NoError(t, err)
    assert.NotEmpty(t, entries)
    
    // Verify calculations
    stats := app.CalculateStats(entries)
    assert.Greater(t, stats.TotalTokens, 0)
    assert.Greater(t, stats.TotalCost, 0.0)
}
```

## Performance Benchmarks

### Benchmark Example
```go
// fileio/reader_bench_test.go
package fileio

import (
    "testing"
)

func BenchmarkReader_Sonic(b *testing.B) {
    file := "testdata/large.jsonl"
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        reader, _ := NewReader(file)
        entries, _ := reader.ReadAll()
        _ = entries
    }
}

func BenchmarkReader_StandardJSON(b *testing.B) {
    // Compare with standard library
}
```

## Build and Deployment

### Makefile
```makefile
.PHONY: build test lint clean

VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

build:
	go build -ldflags "$(LDFLAGS)" -o clawcat .

test:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run

bench:
	go test -bench=. -benchmem ./...

install:
	go install -ldflags "$(LDFLAGS)" .

clean:
	rm -f clawcat coverage.out coverage.html

# Cross-compilation
build-all:
	GOOS=darwin GOARCH=amd64 go build -o dist/clawcat-darwin-amd64
	GOOS=darwin GOARCH=arm64 go build -o dist/clawcat-darwin-arm64
	GOOS=linux GOARCH=amd64 go build -o dist/clawcat-linux-amd64
	GOOS=windows GOARCH=amd64 go build -o dist/clawcat-windows-amd64.exe
```

## CI/CD Pipeline

### GitHub Actions
```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Install dependencies
      run: go mod download
    
    - name: Run tests
      run: make test
    
    - name: Run linter
      run: make lint
    
    - name: Run benchmarks
      run: make bench
    
    - name: Build
      run: make build
```

## Next Steps

1. **Start Implementation**
   - Begin with Phase 1: Core Foundation
   - Follow the implementation order strictly
   - Write tests as you go

2. **Regular Testing**
   - Run tests after each module
   - Benchmark critical paths
   - Profile for performance

3. **Documentation**
   - Update progress.md daily
   - Document any deviations from plan
   - Add inline code documentation

4. **Code Reviews**
   - Review each module completion
   - Check against specifications
   - Ensure test coverage > 80%

5. **Performance Validation**
   - Verify performance goals
   - Optimize bottlenecks
   - Validate memory usage

## Common Pitfalls to Avoid

1. **Over-engineering**: Start simple, optimize later
2. **Skipping tests**: Write tests for each function
3. **Ignoring errors**: Handle all errors properly
4. **Memory leaks**: Close resources, stop goroutines
5. **Race conditions**: Use proper synchronization

## Resources

- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Sonic JSON Documentation](https://github.com/bytedance/sonic)
- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)