package cache

import (
	"path/filepath"
	"os"
)

// NewSimpleCacheStore creates a new SimpleSummaryCache that implements the CacheStore interface
func NewSimpleCacheStore(cacheDir string) *SimpleSummaryCache {
	// Expand home directory if needed
	if cacheDir[:2] == "~/" {
		homeDir, _ := os.UserHomeDir()
		cacheDir = filepath.Join(homeDir, cacheDir[2:])
	}
	
	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		// Fallback to temp directory if we can't create the desired one
		cacheDir = os.TempDir()
	}
	
	persistPath := filepath.Join(cacheDir, "file_summaries.json")
	return NewSimpleSummaryCache(persistPath)
}

