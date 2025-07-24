# Module: fileio

## Overview
The fileio package handles all file system operations including JSONL parsing, directory discovery, and real-time file watching. It uses ByteDance Sonic for high-performance JSON parsing and provides streaming capabilities for large files.

## Package Structure
```
fileio/
├── reader.go      # JSONL file reading and parsing
├── watcher.go     # File system watching with fsnotify
├── discovery.go   # Data path discovery
├── parser.go      # JSON parsing with Sonic
├── cache.go       # File reading cache
└── *_test.go      # Unit and integration tests
```

## Core Components

### JSONL Reader
Streaming reader for conversation files with Sonic integration.

```go
type Reader struct {
    decoder     sonic.Decoder
    scanner     *bufio.Scanner
    file        *os.File
    cacheStore  *cache.Store
}

type RawMessage struct {
    Type      string    `json:"type"`
    Timestamp time.Time `json:"timestamp"`
    Model     string    `json:"model"`
    Usage     struct {
        InputTokens         int `json:"input_tokens"`
        OutputTokens        int `json:"output_tokens"`
        CacheCreationTokens int `json:"cache_creation_tokens"`
        CacheReadTokens     int `json:"cache_read_tokens"`
    } `json:"usage"`
}

func NewReader(filepath string) (*Reader, error)
func (r *Reader) ReadEntries() (<-chan models.UsageEntry, <-chan error)
func (r *Reader) ReadAll() ([]models.UsageEntry, error)
func (r *Reader) Close() error
```

### File Watcher
Real-time monitoring of conversation files.

```go
type Watcher struct {
    watcher  *fsnotify.Watcher
    paths    []string
    events   chan FileEvent
    errors   chan error
    stopCh   chan struct{}
}

type FileEvent struct {
    Path      string
    Type      EventType
    Timestamp time.Time
}

type EventType int
const (
    EventCreate EventType = iota
    EventModify
    EventDelete
)

func NewWatcher(paths []string) (*Watcher, error)
func (w *Watcher) Start() error
func (w *Watcher) Stop() error
func (w *Watcher) Events() <-chan FileEvent
func (w *Watcher) Errors() <-chan error
```

### Path Discovery
Automatic discovery of Claude data directories.

```go
type PathDiscovery struct {
    searchPaths []string
    filters     []FilterFunc
}

type FilterFunc func(path string) bool

type DiscoveredPath struct {
    Path         string
    ProjectCount int
    LastModified time.Time
    Size         int64
}

func NewPathDiscovery() *PathDiscovery
func (p *PathDiscovery) Discover() ([]DiscoveredPath, error)
func (p *PathDiscovery) AddSearchPath(path string)
func (p *PathDiscovery) AddFilter(filter FilterFunc)
```

### Parser
High-performance JSON parsing with Sonic.

```go
type Parser struct {
    api sonic.API
}

func NewParser() *Parser
func (p *Parser) ParseLine(line []byte) (*RawMessage, error)
func (p *Parser) ParseBatch(lines [][]byte) ([]*RawMessage, error)
```

## Key Functions

### File Operations
```go
func ReadConversationFile(filepath string) ([]models.UsageEntry, error)
func StreamConversationFile(filepath string) (<-chan models.UsageEntry, <-chan error)
func WatchDirectory(dir string, callback func(FileEvent)) error
func DiscoverDataPaths() ([]string, error)
```

### Parsing Helpers
```go
func ParseJSONLine(line []byte) (*RawMessage, error)
func FilterValidMessages(messages []*RawMessage) []*RawMessage
func ConvertToUsageEntry(msg *RawMessage) (models.UsageEntry, error)
```

## Error Handling

```go
type FileError struct {
    Path    string
    Op      string
    Err     error
}

type ParseError struct {
    Line    int
    Content string
    Err     error
}

func (e *FileError) Error() string
func (e *ParseError) Error() string
```

## Configuration

```go
type Config struct {
    BufferSize      int           // Scanner buffer size
    MaxFileSize     int64         // Max file size to process
    WatchInterval   time.Duration // File watch debounce
    CacheEnabled    bool          // Enable file caching
    CacheSize       int           // Cache size in MB
    ConcurrentReads int           // Parallel file processing
}

var DefaultConfig = Config{
    BufferSize:      64 * 1024,  // 64KB
    MaxFileSize:     100 * 1024 * 1024, // 100MB
    WatchInterval:   100 * time.Millisecond,
    CacheEnabled:    true,
    CacheSize:       50, // 50MB
    ConcurrentReads: 4,
}
```

## Performance Optimizations

1. **Streaming Processing**: Process files line-by-line to handle large files
2. **Concurrent Reading**: Process multiple files in parallel
3. **Caching**: LRU cache for frequently accessed files
4. **Sonic Integration**: 10x faster JSON parsing
5. **Batch Processing**: Parse multiple lines in batches

## Usage Example

```go
package main

import (
    "github.com/penwyp/ClawCat/fileio"
    "log"
)

func main() {
    // Discover Claude data paths
    discovery := fileio.NewPathDiscovery()
    paths, err := discovery.Discover()
    if err != nil {
        log.Fatal(err)
    }
    
    // Read conversation file
    reader, err := fileio.NewReader(paths[0].Path)
    if err != nil {
        log.Fatal(err)
    }
    defer reader.Close()
    
    // Stream entries
    entries, errors := reader.ReadEntries()
    for {
        select {
        case entry, ok := <-entries:
            if !ok {
                return
            }
            // Process entry
        case err := <-errors:
            log.Printf("Error: %v", err)
        }
    }
}
```

## Testing Strategy

1. **Unit Tests**:
   - JSON parsing with valid/invalid data
   - File reading with various sizes
   - Path discovery on different platforms
   - Error handling scenarios

2. **Integration Tests**:
   - Real JSONL file processing
   - File watching behavior
   - Concurrent file operations
   - Cache hit/miss scenarios

3. **Benchmarks**:
   - Sonic vs standard JSON performance
   - Streaming vs batch processing
   - Cache performance impact
   - Concurrent read scalability

## Platform Considerations

1. **macOS**: 
   - Primary paths: `~/.claude/projects`, `~/.config/claude/projects`
   - File watching with FSEvents

2. **Linux**:
   - Paths: `~/.claude/projects`, `~/.config/claude/projects`
   - File watching with inotify

3. **Windows**:
   - Paths: `%APPDATA%\claude\projects`, `%LOCALAPPDATA%\claude\projects`
   - File watching with ReadDirectoryChangesW