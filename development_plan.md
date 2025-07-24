# Claude Code Usage Monitor - Go Development Plan

## Project Overview
A high-performance TUI application for monitoring Claude AI token usage and costs, built with Go and Bubble Tea framework. The application tracks usage across multiple concurrent sessions with a 5-hour window model.

## Technology Stack
- **Language**: Go 1.21+
- **TUI Framework**: 
  - github.com/charmbracelet/bubbletea v0.27.1
  - github.com/charmbracelet/bubbles v0.18.0
- **JSON Processing**: github.com/bytedance/sonic (for high-performance JSON parsing)
- **File Watching**: github.com/fsnotify/fsnotify
- **Testing**: Go standard testing + testify
- **Build**: Go modules

## Module Structure

### 1. models (models/types.go)
**Purpose**: Core data structures and type definitions
**Features**:
- Usage entry structure with token tracking
- Session block with aggregated statistics
- Model pricing configuration
- Subscription plan definitions
- Error types and constants

**Testing**: Unit tests for all calculations and validations

### 2. fileio (fileio/reader.go, fileio/watcher.go, fileio/discovery.go)
**Purpose**: File system operations and data discovery
**Features**:
- JSONL file parsing with streaming support
- Concurrent file processing
- File system watching for real-time updates
- Data path discovery across platforms
- Caching layer for performance

**Testing**: Integration tests with sample JSONL files

### 3. calculations (calculations/cost.go, calculations/stats.go, calculations/predictions.go)
**Purpose**: All calculation logic separated by concern
**Features**:
- Cost calculations per model and token type
- Burn rate calculations (tokens/minute, cost/hour)
- Statistical aggregations (daily, monthly)
- Prediction algorithms for session end
- P90 limit detection

**Testing**: Comprehensive unit tests with edge cases

### 4. sessions (sessions/manager.go, sessions/tracker.go)
**Purpose**: Session lifecycle management
**Features**:
- Active session detection
- Multiple concurrent session support
- Session block aggregation with gap detection
- Time-to-reset calculations
- Session state persistence

**Testing**: Unit tests with time mocking

### 5. ui (ui/app.go, ui/views/, ui/components/)
**Purpose**: TUI implementation using Bubble Tea
**Submodules**:
- ui/views/dashboard.go - Main dashboard view
- ui/views/sessions.go - Session list view
- ui/views/analytics.go - Analytics and charts view
- ui/components/progress.go - Progress bars
- ui/components/table.go - Data tables
- ui/components/chart.go - Simple ASCII charts

**Features**:
- Responsive layout
- Real-time updates
- Keyboard navigation
- Theme support

**Testing**: Component tests with mock data

### 6. config (config/config.go, config/loader.go)
**Purpose**: Configuration management
**Features**:
- YAML/JSON configuration loading
- Environment variable support
- Default values
- Runtime configuration updates
- Plan management

**Testing**: Unit tests for all config scenarios

### 7. cache (cache/lru.go, cache/store.go)
**Purpose**: Performance optimization through caching
**Features**:
- LRU cache for parsed JSONL data
- File metadata caching
- Session state caching
- Configurable cache sizes

**Testing**: Unit tests with cache hit/miss scenarios

### 8. main (main.go, cmd/)
**Purpose**: Application entry point and CLI
**Features**:
- Command-line argument parsing
- Application initialization
- Graceful shutdown
- Debug mode support

## Development Phases

### Phase 1: Core Foundation (Week 1)
1. Set up project structure and dependencies
2. Implement models package
3. Implement basic fileio for JSONL reading
4. Implement core calculations
5. Unit tests for models and calculations

### Phase 2: Data Processing (Week 2)
1. Complete fileio with watching and discovery
2. Implement cache layer
3. Implement session management
4. Integration tests for data flow
5. Performance benchmarks

### Phase 3: TUI Implementation (Week 3)
1. Basic Bubble Tea app structure
2. Dashboard view with real-time updates
3. Session list and analytics views
4. Keyboard navigation and commands
5. Theme support

### Phase 4: Polish and Optimization (Week 4)
1. Configuration management
2. Error handling improvements
3. Performance optimization with Sonic
4. Documentation
5. Release preparation

## Performance Goals
- < 100ms startup time
- < 10ms UI refresh rate
- < 50MB memory usage for 10k entries
- Support for 100+ concurrent sessions

## Testing Strategy
- Unit tests: 80%+ coverage
- Integration tests: Key workflows
- Benchmark tests: Critical paths
- Manual testing: TUI interactions

## Build and Release
- GitHub Actions CI/CD
- Cross-platform builds (Linux, macOS, Windows)
- Homebrew formula for macOS
- Snap package for Linux
- Signed binaries

## Directory Structure
```
ClawCat/
├── go.mod
├── go.sum
├── main.go
├── Makefile
├── README.md
├── development_plan.md
├── progress.md
├── models/
│   ├── types.go
│   └── types_test.go
├── fileio/
│   ├── reader.go
│   ├── watcher.go
│   ├── discovery.go
│   └── *_test.go
├── calculations/
│   ├── cost.go
│   ├── stats.go
│   ├── predictions.go
│   └── *_test.go
├── sessions/
│   ├── manager.go
│   ├── tracker.go
│   └── *_test.go
├── ui/
│   ├── app.go
│   ├── views/
│   └── components/
├── config/
│   ├── config.go
│   ├── loader.go
│   └── *_test.go
├── cache/
│   ├── lru.go
│   ├── store.go
│   └── *_test.go
├── cmd/
│   └── root.go
├── testdata/
│   └── sample.jsonl
└── docs/
    └── architecture.md
```

## Key Decisions
1. **Sonic over encoding/json**: 10x faster JSON parsing for large JSONL files
2. **Bubble Tea over raw termbox**: Better abstraction and component model
3. **Module separation**: Each module is independently testable and replaceable
4. **Streaming processing**: Handle arbitrarily large JSONL files
5. **Concurrent design**: Leverage Go's concurrency for file processing

## Risk Mitigation
1. **Performance**: Early benchmarking and profiling
2. **Cross-platform**: CI testing on all platforms
3. **Data accuracy**: Extensive unit tests for calculations
4. **UI responsiveness**: Separate data processing from UI updates
5. **Memory usage**: Streaming processing and configurable caches