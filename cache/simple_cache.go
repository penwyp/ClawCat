package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/penwyp/ClawCat/logging"
)

// SimpleSummaryCache provides a simplified cache for file summaries
// It loads all summaries into memory at startup and provides simple Get/Set operations
type SimpleSummaryCache struct {
	mu          sync.RWMutex
	summaries   map[string]*FileSummary // key: absolute file path
	persistPath string                  // path to persistent storage file
}


// NewSimpleSummaryCache creates a new simplified summary cache
func NewSimpleSummaryCache(persistPath string) *SimpleSummaryCache {
	cache := &SimpleSummaryCache{
		summaries:   make(map[string]*FileSummary),
		persistPath: persistPath,
	}
	
	// Load existing summaries from disk
	if err := cache.LoadFromDisk(); err != nil {
		logging.LogErrorf("Failed to load cache from disk: %v", err)
	}
	
	return cache
}

// Get retrieves a file summary from cache
func (c *SimpleSummaryCache) GetFileSummary(absolutePath string) (*FileSummary, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	summary, exists := c.summaries[absolutePath]
	if !exists {
		return nil, fmt.Errorf("file summary not found: %s", absolutePath)
	}
	
	return summary, nil
}

// Set stores a file summary in cache and immediately persists to disk
func (c *SimpleSummaryCache) SetFileSummary(summary *FileSummary) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Update memory cache
	c.summaries[summary.AbsolutePath] = summary
	
	// Immediately persist to disk
	return c.saveToDiskLocked()
}

// HasFileSummary checks if a file summary exists in cache
func (c *SimpleSummaryCache) HasFileSummary(absolutePath string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	_, exists := c.summaries[absolutePath]
	return exists
}

// InvalidateFileSummary removes a file summary from cache and persists the change
func (c *SimpleSummaryCache) InvalidateFileSummary(absolutePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.summaries, absolutePath)
	
	// Immediately persist to disk
	return c.saveToDiskLocked()
}

// IsFileChanged checks if a file has changed based on modTime and size
func (c *SimpleSummaryCache) IsFileChanged(filePath string, stat os.FileInfo) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	summary, exists := c.summaries[filePath]
	if !exists {
		return true // File not cached, needs processing
	}
	
	// Check modTime and size only (no TTL)
	return !summary.ModTime.Equal(stat.ModTime()) || summary.FileSize != stat.Size()
}

// LoadFromDisk loads all summaries from persistent storage
func (c *SimpleSummaryCache) LoadFromDisk() error {
	if c.persistPath == "" {
		return fmt.Errorf("persist path not set")
	}
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(c.persistPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	
	// Check if file exists
	if _, err := os.Stat(c.persistPath); os.IsNotExist(err) {
		// File doesn't exist, start with empty cache
		logging.LogDebugf("Cache file doesn't exist, starting with empty cache: %s", c.persistPath)
		return nil
	}
	
	// Read file
	data, err := os.ReadFile(c.persistPath)
	if err != nil {
		return fmt.Errorf("failed to read cache file: %w", err)
	}
	
	// Parse JSON
	var summaries map[string]*FileSummary
	if err := json.Unmarshal(data, &summaries); err != nil {
		return fmt.Errorf("failed to unmarshal cache data: %w", err)
	}
	
	// Load into memory
	c.mu.Lock()
	c.summaries = summaries
	c.mu.Unlock()
	
	logging.LogInfof("Loaded %d file summaries from cache", len(summaries))
	return nil
}

// saveToDiskLocked saves all summaries to persistent storage
// Note: This method assumes the caller holds the write lock
func (c *SimpleSummaryCache) saveToDiskLocked() error {
	if c.persistPath == "" {
		return fmt.Errorf("persist path not set")
	}
	
	// Marshal to JSON
	data, err := json.MarshalIndent(c.summaries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}
	
	// Write to temporary file first
	tempPath := c.persistPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp cache file: %w", err)
	}
	
	// Atomic rename
	if err := os.Rename(tempPath, c.persistPath); err != nil {
		return fmt.Errorf("failed to rename cache file: %w", err)
	}
	
	logging.LogDebugf("Saved %d file summaries to cache", len(c.summaries))
	return nil
}

// Clear removes all summaries from cache and persistent storage
func (c *SimpleSummaryCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Clear memory
	c.summaries = make(map[string]*FileSummary)
	
	// Remove persistent file
	if c.persistPath != "" {
		if err := os.Remove(c.persistPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove cache file: %w", err)
		}
	}
	
	logging.LogInfof("Cache cleared")
	return nil
}

// GetStats returns cache statistics
func (c *SimpleSummaryCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	totalEntries := 0
	totalCost := 0.0
	totalTokens := 0
	
	for _, summary := range c.summaries {
		totalEntries += summary.EntryCount
		totalCost += summary.TotalCost
		totalTokens += summary.TotalTokens
	}
	
	return map[string]interface{}{
		"cached_files":  len(c.summaries),
		"total_entries": totalEntries,
		"total_cost":    totalCost,
		"total_tokens":  totalTokens,
		"persist_path":  c.persistPath,
	}
}