# Module: cache

## Overview
The cache package provides high-performance caching mechanisms to optimize file reading, JSON parsing, and calculation results. It implements multiple cache strategies including LRU, TTL-based eviction, and memory-bounded storage.

## Package Structure
```
cache/
├── cache.go        # Core cache interfaces
├── lru.go          # LRU cache implementation
├── store.go        # General cache store
├── file.go         # File content caching
├── memory.go       # Memory management
├── metrics.go      # Cache performance metrics
└── *_test.go       # Cache tests and benchmarks
```

## Core Components

### Cache Interface
Common interface for all cache implementations.

```go
type Cache interface {
    Get(key string) (interface{}, bool)
    Set(key string, value interface{}) error
    Delete(key string) error
    Clear() error
    Size() int
    Stats() CacheStats
}

type CacheStats struct {
    Hits       int64
    Misses     int64
    Evictions  int64
    Size       int64
    MaxSize    int64
    HitRate    float64
}

type Entry struct {
    Key        string
    Value      interface{}
    Size       int64
    AccessTime time.Time
    CreateTime time.Time
    TTL        time.Duration
}
```

### LRU Cache
Least Recently Used cache with size limits.

```go
type LRUCache struct {
    capacity   int64
    size       int64
    items      map[string]*lruItem
    head       *lruItem
    tail       *lruItem
    mu         sync.RWMutex
    stats      CacheStats
    onEvicted  func(key string, value interface{})
}

type lruItem struct {
    key   string
    value interface{}
    size  int64
    prev  *lruItem
    next  *lruItem
}

func NewLRUCache(capacity int64) *LRUCache
func (c *LRUCache) Get(key string) (interface{}, bool)
func (c *LRUCache) Set(key string, value interface{}, size int64) error
func (c *LRUCache) Delete(key string) error
func (c *LRUCache) Resize(newCapacity int64) error
```

### TTL Cache
Time-based expiration cache.

```go
type TTLCache struct {
    items      map[string]*ttlItem
    defaultTTL time.Duration
    cleanupInterval time.Duration
    mu         sync.RWMutex
    stopCh     chan struct{}
}

type ttlItem struct {
    value      interface{}
    expiration time.Time
}

func NewTTLCache(defaultTTL, cleanupInterval time.Duration) *TTLCache
func (c *TTLCache) Get(key string) (interface{}, bool)
func (c *TTLCache) SetWithTTL(key string, value interface{}, ttl time.Duration) error
func (c *TTLCache) Start() error
func (c *TTLCache) Stop() error
```

### File Cache
Specialized cache for file contents.

```go
type FileCache struct {
    cache      *LRUCache
    serializer Serializer
    stats      FileCacheStats
}

type FileCacheStats struct {
    CacheStats
    BytesSaved   int64
    ParseTime    time.Duration
    CompressRatio float64
}

type CachedFile struct {
    Path         string
    Content      []byte
    Entries      []models.UsageEntry
    ModTime      time.Time
    Size         int64
    Checksum     string
    Compressed   bool
}

func NewFileCache(maxSize int64) *FileCache
func (f *FileCache) GetFile(path string) (*CachedFile, bool)
func (f *FileCache) SetFile(path string, content *CachedFile) error
func (f *FileCache) InvalidateFile(path string) error
func (f *FileCache) GetEntries(path string) ([]models.UsageEntry, bool)
```

### Calculation Cache
Cache for expensive calculations.

```go
type CalculationCache struct {
    cache    *TTLCache
    keyGen   KeyGenerator
}

type KeyGenerator interface {
    Generate(params ...interface{}) string
}

type CachedCalculation struct {
    Result    interface{}
    Timestamp time.Time
    Duration  time.Duration
}

func NewCalculationCache(ttl time.Duration) *CalculationCache
func (c *CalculationCache) GetBurnRate(entries []models.UsageEntry) (*calculations.BurnRate, bool)
func (c *CalculationCache) SetBurnRate(entries []models.UsageEntry, rate *calculations.BurnRate) error
func (c *CalculationCache) GetPrediction(session *sessions.Session) (*calculations.Prediction, bool)
```

### Memory Manager
Manages cache memory usage.

```go
type MemoryManager struct {
    maxMemory   int64
    currentUsage int64
    caches      []ManagedCache
    mu          sync.RWMutex
}

type ManagedCache interface {
    Cache
    Priority() int
    CanEvict() bool
    EvictOldest(count int) error
}

func NewMemoryManager(maxMemory int64) *MemoryManager
func (m *MemoryManager) Register(cache ManagedCache) error
func (m *MemoryManager) AllocateMemory(size int64) error
func (m *MemoryManager) ReleaseMemory(size int64)
func (m *MemoryManager) Rebalance() error
```

### Cache Store
General purpose cache store with multiple backends.

```go
type Store struct {
    fileCache   *FileCache
    calcCache   *CalculationCache
    lruCache    *LRUCache
    memManager  *MemoryManager
    config      StoreConfig
}

type StoreConfig struct {
    MaxFileSize      int64
    MaxMemory        int64
    FileCacheTTL     time.Duration
    CalcCacheTTL     time.Duration
    CompressionLevel int
    EnableMetrics    bool
}

func NewStore(config StoreConfig) *Store
func (s *Store) GetFile(path string) (*CachedFile, error)
func (s *Store) GetCalculation(key string) (interface{}, error)
func (s *Store) Preload(paths []string) error
func (s *Store) Stats() StoreStats
```

## Serialization

```go
type Serializer interface {
    Serialize(v interface{}) ([]byte, error)
    Deserialize(data []byte, v interface{}) error
}

type SonicSerializer struct {
    api sonic.API
}

type CompressedSerializer struct {
    serializer Serializer
    level      int
}

func NewSonicSerializer() *SonicSerializer
func NewCompressedSerializer(s Serializer, level int) *CompressedSerializer
```

## Cache Warming

```go
type Warmer struct {
    store   *Store
    workers int
}

func NewWarmer(store *Store, workers int) *Warmer
func (w *Warmer) WarmFromPaths(paths []string) error
func (w *Warmer) WarmFromPattern(pattern string) error
func (w *Warmer) WarmAsync(paths []string) <-chan error
```

## Metrics Collection

```go
type Metrics struct {
    HitRate       *RateCounter
    MissRate      *RateCounter
    EvictionRate  *RateCounter
    BytesSaved    *Counter
    ResponseTime  *Histogram
}

func (m *Metrics) RecordHit(cacheType string)
func (m *Metrics) RecordMiss(cacheType string)
func (m *Metrics) RecordEviction(cacheType string, reason string)
func (m *Metrics) Export() MetricsSnapshot
```

## Usage Example

```go
package main

import (
    "github.com/penwyp/claudecat/cache"
    "github.com/penwyp/claudecat/models"
)

func main() {
    // Create cache store
    config := cache.StoreConfig{
        MaxFileSize:   10 * 1024 * 1024, // 10MB
        MaxMemory:     100 * 1024 * 1024, // 100MB
        FileCacheTTL:  5 * time.Minute,
        CalcCacheTTL:  1 * time.Minute,
    }
    
    store := cache.NewStore(config)
    
    // Cache file reading
    if cached, err := store.GetFile("/path/to/conversation.jsonl"); err == nil {
        entries := cached.Entries
        // Use cached entries
    }
    
    // Cache calculations
    key := "burnrate:session:123"
    if result, err := store.GetCalculation(key); err == nil {
        burnRate := result.(*calculations.BurnRate)
        // Use cached burn rate
    }
    
    // Warm cache
    warmer := cache.NewWarmer(store, 4)
    if err := warmer.WarmFromPattern("~/.claude/projects/**/*.jsonl"); err != nil {
        log.Printf("Cache warming failed: %v", err)
    }
    
    // Get stats
    stats := store.Stats()
    log.Printf("Cache hit rate: %.2f%%", stats.HitRate*100)
}
```

## Performance Optimization

1. **Lock-free reads**: Use atomic operations where possible
2. **Sharded caches**: Split large caches into shards
3. **Compression**: Compress large cached values
4. **Batch operations**: Support batch get/set
5. **Async eviction**: Evict in background

## Testing Strategy

1. **Unit Tests**:
   - Basic cache operations
   - Eviction policies
   - TTL expiration
   - Memory limits
   - Concurrent access

2. **Benchmarks**:
   - Cache hit/miss performance
   - Memory allocation
   - Serialization speed
   - Compression ratios
   - Concurrent load

3. **Stress Tests**:
   - High concurrency
   - Memory pressure
   - Large entries
   - Rapid eviction

## Configuration

```go
type CacheConfig struct {
    // File cache
    FileCache struct {
        Enabled   bool
        MaxSize   int64
        TTL       time.Duration
        Compress  bool
    }
    
    // Calculation cache
    CalcCache struct {
        Enabled   bool
        MaxItems  int
        TTL       time.Duration
    }
    
    // Memory limits
    Memory struct {
        MaxTotal  int64
        MaxEntry  int64
        GCPercent int
    }
    
    // Performance
    Performance struct {
        Shards    int
        Workers   int
        BatchSize int
    }
}
```