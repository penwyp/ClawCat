package cache

import (
	"fmt"
	"github.com/penwyp/claudecat/models"
	"path/filepath"
	"sync"
)

// Store provides a unified cache store with multiple backends
type Store struct {
	fileCache *FileCache
	diskCache *DiskCache // L2: Disk cache
	config    StoreConfig
	mu        sync.RWMutex
}

// StoreConfig configures the cache store behavior
type StoreConfig struct {
	MaxFileSize       int64  `json:"max_file_size"`
	MaxDiskSize       int64  `json:"max_disk_size"`  // L2 disk cache size
	DiskCacheDir      string `json:"disk_cache_dir"` // Disk cache directory
	CompressionLevel  int    `json:"compression_level"`
	EnableMetrics     bool   `json:"enable_metrics"`
	EnableCompression bool   `json:"enable_compression"`
}

// StoreStats provides overall cache store statistics
type StoreStats struct {
	FileCache FileCacheStats `json:"file_cache"`
	DiskCache DiskCacheStats `json:"disk_cache"`
}

// TotalStats provides aggregate statistics across all caches
type TotalStats struct {
	TotalHits      int64   `json:"total_hits"`
	TotalMisses    int64   `json:"total_misses"`
	TotalSize      int64   `json:"total_size"`
	TotalMemory    int64   `json:"total_memory"`
	OverallHitRate float64 `json:"overall_hit_rate"`
}

// StoreMemoryStats provides memory usage statistics for the store
type StoreMemoryStats struct {
	Used       int64   `json:"used"`
	Total      int64   `json:"total"`
	Percentage float64 `json:"percentage"`
}

// NewStore creates a new cache store with the given configuration
func NewStore(config StoreConfig) *Store {
	// Set defaults
	if config.MaxFileSize <= 0 {
		config.MaxFileSize = 50 * 1024 * 1024 // 50MB
	}
	if config.MaxDiskSize <= 0 {
		config.MaxDiskSize = 1024 * 1024 * 1024 // 1GB
	}
	if config.DiskCacheDir == "" {
		config.DiskCacheDir = "~/.cache/claudecat"
	}
	if config.CompressionLevel <= 0 {
		config.CompressionLevel = 6 // Default compression
	}

	// Create caches
	fileCache := NewFileCache(config.MaxFileSize)

	// Create disk cache if enabled
	var diskCache *DiskCache
	var err error
	diskCache, err = NewDiskCache(config.DiskCacheDir, config.MaxDiskSize)
	if err != nil {
		// Log error but continue without disk cache
		fmt.Printf("Warning: Failed to initialize disk cache: %v\n", err)
		diskCache = nil
	}

	store := &Store{
		fileCache: fileCache,
		diskCache: diskCache,
		config:    config,
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

// Clear removes all cached data
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.fileCache.Clear(); err != nil {
		return err
	}

	return nil
}

// Cleanup performs maintenance on all cache layers
func (s *Store) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Force garbage collection on memory caches
	// This is already handled by the memory manager and LRU policies

	return nil
}
