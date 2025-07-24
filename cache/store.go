package cache

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/models"
)

// Store provides a unified cache store with multiple backends
type Store struct {
	fileCache  *FileCache
	lruCache   *LRUCache
	memManager *MemoryManager
	config     StoreConfig
	mu         sync.RWMutex
}

// StoreConfig configures the cache store behavior
type StoreConfig struct {
	MaxFileSize      int64         `json:"max_file_size"`
	MaxMemory        int64         `json:"max_memory"`
	FileCacheTTL     time.Duration `json:"file_cache_ttl"`
	CalcCacheTTL     time.Duration `json:"calc_cache_ttl"`
	CompressionLevel int           `json:"compression_level"`
	EnableMetrics    bool          `json:"enable_metrics"`
	EnableCompression bool         `json:"enable_compression"`
}

// StoreStats provides overall cache store statistics
type StoreStats struct {
	FileCache FileCacheStats   `json:"file_cache"`
	LRUCache  CacheStats       `json:"lru_cache"`
	Memory    StoreMemoryStats `json:"memory"`
	Total     TotalStats       `json:"total"`
}

// TotalStats provides aggregate statistics across all caches
type TotalStats struct {
	TotalHits     int64   `json:"total_hits"`
	TotalMisses   int64   `json:"total_misses"`
	TotalSize     int64   `json:"total_size"`
	TotalMemory   int64   `json:"total_memory"`
	OverallHitRate float64 `json:"overall_hit_rate"`
}

// StoreMemoryStats provides memory usage statistics for the store
type StoreMemoryStats struct {
	Used      int64   `json:"used"`
	Total     int64   `json:"total"`
	Percentage float64 `json:"percentage"`
}

// NewStore creates a new cache store with the given configuration
func NewStore(config StoreConfig) *Store {
	// Set defaults
	if config.MaxFileSize <= 0 {
		config.MaxFileSize = 50 * 1024 * 1024 // 50MB
	}
	if config.MaxMemory <= 0 {
		config.MaxMemory = 100 * 1024 * 1024 // 100MB
	}
	if config.FileCacheTTL <= 0 {
		config.FileCacheTTL = 5 * time.Minute
	}
	if config.CalcCacheTTL <= 0 {
		config.CalcCacheTTL = 1 * time.Minute
	}
	if config.CompressionLevel <= 0 {
		config.CompressionLevel = 6 // Default compression
	}

	// Create caches
	fileCache := NewFileCache(config.MaxFileSize)
	lruCache := NewLRUCache(config.MaxMemory / 2) // Allocate half memory to general cache
	
	// Create memory manager
	memManager := NewMemoryManager(config.MaxMemory)
	memManager.Register(lruCache)
	
	store := &Store{
		fileCache:  fileCache,
		lruCache:   lruCache,
		memManager: memManager,
		config:     config,
	}
	
	return store
}

// GetFile retrieves a file from cache, reading from disk if necessary
func (s *Store) GetFile(path string) (*CachedFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Try to get from file cache first
	if cached, exists := s.fileCache.GetFile(path); exists {
		return cached, nil
	}
	
	return nil, fmt.Errorf("file not found in cache: %s", path)
}

// CacheFile adds a file to the cache
func (s *Store) CacheFile(path string, content []byte, entries []models.UsageEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return s.fileCache.CacheFileContent(path, content, entries)
}

// GetEntries retrieves parsed entries for a file
func (s *Store) GetEntries(path string) ([]models.UsageEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	entries, exists := s.fileCache.GetEntries(path)
	if !exists {
		return nil, fmt.Errorf("entries not found in cache: %s", path)
	}
	
	return entries, nil
}

// GetCalculation retrieves a cached calculation result
func (s *Store) GetCalculation(key string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	value, exists := s.lruCache.Get(key)
	if !exists {
		return nil, fmt.Errorf("calculation not found in cache: %s", key)
	}
	
	return value, nil
}

// SetCalculation stores a calculation result in cache
func (s *Store) SetCalculation(key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return s.lruCache.Set(key, value)
}

// Preload loads multiple files into cache
func (s *Store) Preload(paths []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return s.fileCache.Preload(paths)
}

// PreloadPattern loads files matching a pattern into cache
func (s *Store) PreloadPattern(pattern string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to expand pattern %s: %w", pattern, err)
	}
	
	return s.fileCache.Preload(matches)
}

// InvalidateFile removes a file from cache
func (s *Store) InvalidateFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return s.fileCache.InvalidateFile(path)
}

// InvalidatePattern removes all files matching pattern from cache
func (s *Store) InvalidatePattern(pattern string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return s.fileCache.InvalidatePattern(pattern)
}

// InvalidateCalculations clears all cached calculations
func (s *Store) InvalidateCalculations() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return s.lruCache.Clear()
}

// Clear removes all cached data
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if err := s.fileCache.Clear(); err != nil {
		return err
	}
	
	if err := s.lruCache.Clear(); err != nil {
		return err
	}
	
	return nil
}

// Stats returns comprehensive cache statistics
func (s *Store) Stats() StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	fileCacheStats := s.fileCache.FileCacheStats()
	lruCacheStats := s.lruCache.Stats()
	memStats := s.memManager.Stats()
	
	// Calculate total stats
	totalHits := fileCacheStats.Hits + lruCacheStats.Hits
	totalMisses := fileCacheStats.Misses + lruCacheStats.Misses
	totalSize := fileCacheStats.Size + lruCacheStats.Size
	totalMemory := s.fileCache.MemoryUsage() + s.lruCache.MemoryUsage()
	
	var overallHitRate float64
	if totalHits+totalMisses > 0 {
		overallHitRate = float64(totalHits) / float64(totalHits+totalMisses)
	}
	
	return StoreStats{
		FileCache: fileCacheStats,
		LRUCache:  lruCacheStats,
		Memory: StoreMemoryStats{
			Used:       memStats.CurrentUsage,
			Total:      memStats.MaxMemory,
			Percentage: float64(memStats.CurrentUsage) / float64(memStats.MaxMemory) * 100,
		},
		Total: TotalStats{
			TotalHits:      totalHits,
			TotalMisses:    totalMisses,
			TotalSize:      totalSize,
			TotalMemory:    totalMemory,
			OverallHitRate: overallHitRate,
		},
	}
}

// Config returns the current store configuration
func (s *Store) Config() StoreConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig updates the store configuration
func (s *Store) UpdateConfig(config StoreConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Update cache sizes if changed
	if config.MaxFileSize != s.config.MaxFileSize {
		if err := s.fileCache.cache.Resize(config.MaxFileSize); err != nil {
			return fmt.Errorf("failed to resize file cache: %w", err)
		}
	}
	
	if config.MaxMemory != s.config.MaxMemory {
		if err := s.memManager.SetMaxMemory(config.MaxMemory); err != nil {
			return fmt.Errorf("failed to update memory limit: %w", err)
		}
	}
	
	s.config = config
	return nil
}

// Optimize performs cache optimization and cleanup
func (s *Store) Optimize() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Trigger memory rebalancing
	if err := s.memManager.Rebalance(); err != nil {
		return fmt.Errorf("failed to rebalance memory: %w", err)
	}
	
	return nil
}

// WarmCache warms the cache with commonly accessed files
func (s *Store) WarmCache(patterns []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for _, pattern := range patterns {
		if err := s.fileCache.WarmCache(pattern); err != nil {
			// Log error but continue with other patterns
			continue
		}
	}
	
	return nil
}

// IsHealthy checks if the cache store is operating within normal parameters
func (s *Store) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	stats := s.memManager.Stats()
	memoryPercentage := float64(stats.CurrentUsage) / float64(stats.MaxMemory) * 100
	
	// Consider healthy if memory usage is below 90%
	return memoryPercentage < 90.0
}