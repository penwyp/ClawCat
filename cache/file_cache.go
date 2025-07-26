package cache

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"strings"
	"time"

	"github.com/penwyp/claudecat/logging"
)

// FileBasedSummaryCache provides a file-based cache for file summaries with memory preloading
type FileBasedSummaryCache struct {
	baseDir  string
	memCache map[string]*FileSummary // Memory cache for fast access
	mu       sync.RWMutex
	stats    FileBasedCacheStats
}

// FileBasedCacheStats tracks cache statistics
type FileBasedCacheStats struct {
	Hits      int64
	Misses    int64
	Writes    int64
	Deletes   int64
	Errors    int64
	MemoryHits int64 // Hits from memory cache
}

// NewFileBasedSummaryCache creates a new file-based summary cache
func NewFileBasedSummaryCache(persistPath string) (*FileBasedSummaryCache, error) {
	// Create base directory if it doesn't exist
	summariesDir := filepath.Join(persistPath, "summaries")
	if err := os.MkdirAll(summariesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &FileBasedSummaryCache{
		baseDir:  summariesDir,
		memCache: make(map[string]*FileSummary),
	}

	// Preload existing summaries into memory
	if err := cache.preloadSummaries(); err != nil {
		logging.LogWarnf("Failed to preload summaries: %v", err)
		// Not a fatal error, continue with empty cache
	}

	logging.LogInfof("Initialized file-based cache at %s with %d preloaded summaries", summariesDir, len(cache.memCache))
	return cache, nil
}

// preloadSummaries loads all existing summaries into memory
func (c *FileBasedSummaryCache) preloadSummaries() error {
	startTime := time.Now()
	count := 0

	err := filepath.Walk(c.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		// Read and parse the summary file
		data, err := os.ReadFile(path)
		if err != nil {
			logging.LogDebugf("Failed to read cache file %s: %v", path, err)
			return nil // Skip this file
		}

		var summary FileSummary
		if err := json.Unmarshal(data, &summary); err != nil {
			logging.LogDebugf("Failed to unmarshal cache file %s: %v", path, err)
			return nil // Skip this file
		}

		// Add to memory cache
		c.memCache[summary.AbsolutePath] = &summary
		count++

		if count%100 == 0 {
			logging.LogDebugf("Preloaded %d summaries...", count)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk cache directory: %w", err)
	}

	logging.LogInfof("Preloaded %d summaries in %v", count, time.Since(startTime))
	return nil
}

// getCacheFilePath returns the cache file path for a given absolute path
func (c *FileBasedSummaryCache) getCacheFilePath(absolutePath string) string {
	// Calculate MD5 hash of the absolute path
	h := md5.New()
	io.WriteString(h, absolutePath)
	hash := fmt.Sprintf("%x", h.Sum(nil))

	// Use first 2 characters as subdirectory for better file system performance
	subDir := hash[:2]
	fileName := hash + ".json"

	return filepath.Join(c.baseDir, subDir, fileName)
}

// GetFileSummary retrieves a file summary from cache
func (c *FileBasedSummaryCache) GetFileSummary(absolutePath string) (*FileSummary, error) {
	c.mu.RLock()
	
	// Check memory cache first
	if summary, exists := c.memCache[absolutePath]; exists {
		c.stats.MemoryHits++
		c.stats.Hits++
		c.mu.RUnlock()
		return summary, nil
	}
	c.mu.RUnlock()

	// Not in memory, try to load from disk (shouldn't happen after preload)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if summary, exists := c.memCache[absolutePath]; exists {
		c.stats.MemoryHits++
		c.stats.Hits++
		return summary, nil
	}

	cacheFile := c.getCacheFilePath(absolutePath)
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			c.stats.Misses++
			return nil, fmt.Errorf("file summary not found: %s", absolutePath)
		}
		c.stats.Errors++
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var summary FileSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		c.stats.Errors++
		return nil, fmt.Errorf("failed to unmarshal summary: %w", err)
	}

	// Add to memory cache
	c.memCache[absolutePath] = &summary
	c.stats.Hits++

	return &summary, nil
}

// SetFileSummary stores a file summary in cache
func (c *FileBasedSummaryCache) SetFileSummary(summary *FileSummary) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update memory cache
	c.memCache[summary.AbsolutePath] = summary

	// Write to disk
	cacheFile := c.getCacheFilePath(summary.AbsolutePath)
	cacheDir := filepath.Dir(cacheFile)

	// Create subdirectory if needed
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		c.stats.Errors++
		return fmt.Errorf("failed to create cache subdirectory: %w", err)
	}

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		c.stats.Errors++
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	// Write to temporary file first
	tmpFile := cacheFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		c.stats.Errors++
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, cacheFile); err != nil {
		c.stats.Errors++
		os.Remove(tmpFile) // Clean up
		return fmt.Errorf("failed to rename cache file: %w", err)
	}

	c.stats.Writes++
	return nil
}

// HasFileSummary checks if a file summary exists in cache
func (c *FileBasedSummaryCache) HasFileSummary(absolutePath string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.memCache[absolutePath]
	return exists
}

// InvalidateFileSummary removes a file summary from cache
func (c *FileBasedSummaryCache) InvalidateFileSummary(absolutePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove from memory cache
	delete(c.memCache, absolutePath)

	// Remove from disk
	cacheFile := c.getCacheFilePath(absolutePath)
	if err := os.Remove(cacheFile); err != nil {
		if !os.IsNotExist(err) {
			c.stats.Errors++
			return fmt.Errorf("failed to delete cache file: %w", err)
		}
	}

	c.stats.Deletes++
	return nil
}

// IsFileChanged checks if a file has changed based on modTime and size
func (c *FileBasedSummaryCache) IsFileChanged(filePath string, stat os.FileInfo) bool {
	summary, err := c.GetFileSummary(filePath)
	if err != nil {
		return true // File not cached or error, needs processing
	}

	// Check modTime and size
	return !summary.ModTime.Equal(stat.ModTime()) || summary.FileSize != stat.Size()
}

// BatchSet performs multiple set operations
func (c *FileBasedSummaryCache) BatchSet(summaries []*FileSummary) error {
	for _, summary := range summaries {
		if err := c.SetFileSummary(summary); err != nil {
			return fmt.Errorf("failed to set summary for %s: %w", summary.AbsolutePath, err)
		}
	}
	return nil
}

// Clear removes all summaries from cache
func (c *FileBasedSummaryCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear memory cache
	c.memCache = make(map[string]*FileSummary)

	// Remove entire summaries directory
	if err := os.RemoveAll(c.baseDir); err != nil {
		return fmt.Errorf("failed to remove cache directory: %w", err)
	}

	// Recreate the directory
	if err := os.MkdirAll(c.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to recreate cache directory: %w", err)
	}

	logging.LogInfof("Cache cleared")
	return nil
}

// GetStats returns cache statistics
func (c *FileBasedSummaryCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Calculate total size and statistics
	var totalSize int64
	var fileCount int64
	var totalEntries int64
	var totalCost float64
	var totalTokens int64

	filepath.Walk(c.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".json") {
			fileCount++
			totalSize += info.Size()
		}
		return nil
	})

	// Get stats from memory cache
	for _, summary := range c.memCache {
		totalEntries += int64(summary.EntryCount)
		totalCost += summary.TotalCost
		totalTokens += int64(summary.TotalTokens)
	}

	hitRate := float64(0)
	if c.stats.Hits+c.stats.Misses > 0 {
		hitRate = float64(c.stats.Hits) / float64(c.stats.Hits+c.stats.Misses)
	}

	return map[string]interface{}{
		"cached_files":     len(c.memCache),
		"disk_files":       fileCount,
		"total_entries":    totalEntries,
		"total_cost":       totalCost,
		"total_tokens":     totalTokens,
		"cache_size_bytes": totalSize,
		"cache_size_mb":    float64(totalSize) / 1024 / 1024,
		"hits":             c.stats.Hits,
		"memory_hits":      c.stats.MemoryHits,
		"misses":           c.stats.Misses,
		"writes":           c.stats.Writes,
		"deletes":          c.stats.Deletes,
		"errors":           c.stats.Errors,
		"hit_rate":         hitRate,
		"persist_path":     c.baseDir,
	}
}

// Close is a no-op for file-based cache but satisfies the interface
func (c *FileBasedSummaryCache) Close() error {
	// Nothing to close for file-based cache
	return nil
}