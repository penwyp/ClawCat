# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

claudecat is a Go-powered real-time usage tracker for Claude Code that monitors token usage, costs, and session statistics. It processes JSONL files from Claude projects to provide analytics via a terminal UI (TUI).

## Common Development Commands

### Build and Run
```bash
# Build the binary
make build

# Run with hot reload during development
make run

# Build for all platforms
make build-all
```

### Testing
```bash
# Run all tests with coverage
make test

# Run tests with race detector
make race

# Run benchmarks
make bench

# Run a specific test
go test -v -run TestName ./path/to/package
```

### Code Quality
```bash
# Run linter (golangci-lint)
make lint

# Format code
make fmt

# Check formatting without changes
make fmt-check

# Full CI pipeline (deps, lint, fmt-check, test)
make ci
```

## Architecture Overview

### Data Flow Pipeline
```
JSONL Files → FileIO (Discovery/Reading) → Models (Parsing) → Sessions (Detection) → 
Calculations (Metrics) → Cache (Storage) → UI (Display)
```

### Key Components

1. **FileIO Module** (`fileio/`): Discovers and monitors Claude conversation.jsonl files
   - Supports file watching for real-time updates
   - Handles concurrent file loading

2. **Models** (`models/`): Core data structures and pricing calculations
   - Token usage tracking with cache-aware pricing
   - Model-specific cost calculations

3. **Sessions** (`sessions/`): 5-hour window session detection and management
   - Supports overlapping sessions
   - Real-time session tracking

4. **Calculations** (`calculations/`): Statistical computations
   - Burn rate, projections, aggregations
   - Real-time metrics updates

5. **UI** (`ui/`): Bubble Tea-based TUI with dashboard views
   - Progress bars, statistics tables, model distribution
   - Responsive terminal rendering

6. **Cache** (`cache/`): Multi-layer caching system
   - Memory, disk, and BadgerDB backends
   - LRU eviction and sharding support

## Important Business Rules

- **Session Duration**: Strict 5-hour windows, no extensions
- **Cost Limits**: Based on subscription plans (Pro: $18, Team: $35, Max5: $35, Max20: $140)
- **Token Pricing**: Model-specific with cache creation/read differentiation
- **Update Frequency**: File changes detected within 10 seconds, UI refresh at 0.75Hz

## Performance Considerations

- Uses sonic for JSON parsing (faster than encoding/json)
- Streaming JSONL processing to avoid loading entire files
- Concurrent file processing with goroutines
- Sharded cache for better concurrency
- Incremental updates for real-time monitoring

## Debugging

Enable debug mode for verbose logging:
```bash
# Run with debug flag
./bin/claudecat --debug

# Or set in config
CLAWCAT_DEBUG_ENABLED=true ./bin/claudecat
```

## Configuration

claudecat looks for configuration in:
1. `~/.claudecat.yaml`
2. Environment variables (prefix: `CLAWCAT_`)
3. Command-line flags

Priority: CLI flags > Environment > Config file > Defaults