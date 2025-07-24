package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/penwyp/ClawCat/models"
)

func TestNewStore(t *testing.T) {
	config := StoreConfig{
		MaxFileSize:   1024,
		MaxMemory:     2048,
		FileCacheTTL:  5 * time.Minute,
		CalcCacheTTL:  1 * time.Minute,
	}
	
	store := NewStore(config)
	
	assert.NotNil(t, store.fileCache)
	assert.NotNil(t, store.lruCache)
	assert.NotNil(t, store.memManager)
	// The store applies defaults, so we need to check the expected values
	expectedConfig := config
	expectedConfig.CompressionLevel = 6 // Default value applied
	assert.Equal(t, expectedConfig, store.config)
}

func TestNewStore_Defaults(t *testing.T) {
	// Test with empty config to verify defaults
	config := StoreConfig{}
	store := NewStore(config)
	
	assert.Equal(t, int64(50*1024*1024), store.config.MaxFileSize)
	assert.Equal(t, int64(100*1024*1024), store.config.MaxMemory)
	assert.Equal(t, 5*time.Minute, store.config.FileCacheTTL)
	assert.Equal(t, 1*time.Minute, store.config.CalcCacheTTL)
	assert.Equal(t, 6, store.config.CompressionLevel)
}

func TestStore_CacheAndGetFile(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	// Create test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	content := []byte(`{"type":"message","usage":{"input_tokens":100}}`)
	entries := []models.UsageEntry{
		{
			Timestamp:    time.Now(),
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
	}
	
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)
	
	// Cache file
	err = store.CacheFile(testFile, content, entries)
	require.NoError(t, err)
	
	// Retrieve file
	cached, err := store.GetFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, testFile, cached.Path)
	assert.Equal(t, content, cached.Content)
	assert.Equal(t, entries, cached.Entries)
}

func TestStore_GetFile_NotFound(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	_, err := store.GetFile("/nonexistent/file.jsonl")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in cache")
}

func TestStore_GetEntries(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	// Create and cache file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	content := []byte(`{"type":"message"}`)
	entries := []models.UsageEntry{
		{InputTokens: 100, OutputTokens: 50},
		{InputTokens: 200, OutputTokens: 100},
	}
	
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)
	
	err = store.CacheFile(testFile, content, entries)
	require.NoError(t, err)
	
	// Get entries
	retrievedEntries, err := store.GetEntries(testFile)
	require.NoError(t, err)
	assert.Equal(t, entries, retrievedEntries)
}

func TestStore_Calculations(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	// Test setting and getting calculation
	key := "test_calculation"
	value := map[string]interface{}{
		"result": 42.5,
		"time":   time.Now(),
	}
	
	err := store.SetCalculation(key, value)
	require.NoError(t, err)
	
	retrieved, err := store.GetCalculation(key)
	require.NoError(t, err)
	assert.Equal(t, value, retrieved)
}

func TestStore_GetCalculation_NotFound(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	_, err := store.GetCalculation("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in cache")
}

func TestStore_Preload(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	// Create test files
	tmpDir := t.TempDir()
	files := []string{
		filepath.Join(tmpDir, "file1.jsonl"),
		filepath.Join(tmpDir, "file2.jsonl"),
	}
	
	content := []byte(`{"type":"message"}`)
	for _, file := range files {
		err := os.WriteFile(file, content, 0644)
		require.NoError(t, err)
	}
	
	// Preload files
	err := store.Preload(files)
	require.NoError(t, err)
	
	// Verify files are cached
	for _, file := range files {
		cached, err := store.GetFile(file)
		require.NoError(t, err)
		assert.Equal(t, file, cached.Path)
	}
}

func TestStore_PreloadPattern(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	// Create test files
	tmpDir := t.TempDir()
	files := []string{
		filepath.Join(tmpDir, "test1.jsonl"),
		filepath.Join(tmpDir, "test2.jsonl"),
		filepath.Join(tmpDir, "other.txt"),
	}
	
	content := []byte(`{"type":"message"}`)
	for _, file := range files {
		err := os.WriteFile(file, content, 0644)
		require.NoError(t, err)
	}
	
	// Preload pattern
	pattern := filepath.Join(tmpDir, "*.jsonl")
	err := store.PreloadPattern(pattern)
	require.NoError(t, err)
	
	// Verify .jsonl files are cached
	for i := 0; i < 2; i++ {
		_, err := store.GetFile(files[i])
		assert.NoError(t, err)
	}
}

func TestStore_InvalidateFile(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	// Create and cache file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	content := []byte(`{"type":"message"}`)
	
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)
	
	err = store.CacheFile(testFile, content, []models.UsageEntry{})
	require.NoError(t, err)
	
	// Verify cached
	_, err = store.GetFile(testFile)
	require.NoError(t, err)
	
	// Invalidate
	err = store.InvalidateFile(testFile)
	require.NoError(t, err)
	
	// Should be gone
	_, err = store.GetFile(testFile)
	assert.Error(t, err)
}

func TestStore_InvalidateCalculations(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	// Set calculations
	store.SetCalculation("calc1", "result1")
	store.SetCalculation("calc2", "result2")
	
	// Verify cached
	_, err := store.GetCalculation("calc1")
	require.NoError(t, err)
	
	// Invalidate all
	err = store.InvalidateCalculations()
	require.NoError(t, err)
	
	// Should be gone
	_, err = store.GetCalculation("calc1")
	assert.Error(t, err)
	_, err = store.GetCalculation("calc2")
	assert.Error(t, err)
}

func TestStore_Clear(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	// Add data to both caches
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	content := []byte(`{"type":"message"}`)
	
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)
	
	err = store.CacheFile(testFile, content, []models.UsageEntry{})
	require.NoError(t, err)
	
	err = store.SetCalculation("test", "value")
	require.NoError(t, err)
	
	// Clear all
	err = store.Clear()
	require.NoError(t, err)
	
	// Verify everything is gone
	_, err = store.GetFile(testFile)
	assert.Error(t, err)
	
	_, err = store.GetCalculation("test")
	assert.Error(t, err)
}

func TestStore_Stats(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	stats := store.Stats()
	
	assert.NotNil(t, stats.FileCache)
	assert.NotNil(t, stats.LRUCache)
	assert.NotNil(t, stats.Memory)
	assert.NotNil(t, stats.Total)
	
	// Initially should be zero
	assert.Equal(t, int64(0), stats.Total.TotalHits)
	assert.Equal(t, int64(0), stats.Total.TotalMisses)
	assert.Equal(t, float64(0), stats.Total.OverallHitRate)
}

func TestStore_UpdateConfig(t *testing.T) {
	config := StoreConfig{
		MaxMemory:   1024,
		MaxFileSize: 512,
	}
	store := NewStore(config)
	
	// Update config
	newConfig := StoreConfig{
		MaxMemory:   2048,
		MaxFileSize: 1024,
	}
	
	err := store.UpdateConfig(newConfig)
	require.NoError(t, err)
	
	assert.Equal(t, newConfig, store.Config())
}

func TestStore_Optimize(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	err := store.Optimize()
	require.NoError(t, err)
}

func TestStore_IsHealthy(t *testing.T) {
	config := StoreConfig{MaxMemory: 1000}
	store := NewStore(config)
	
	// Initially should be healthy
	assert.True(t, store.IsHealthy())
	
	// Add data close to capacity
	// This is a simplified test - in reality we'd need to add enough data
	// to bring memory usage over 90%
}

func TestStore_WarmCache(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	// Create test files
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	content := []byte(`{"type":"message"}`)
	
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)
	
	// Warm cache with patterns
	patterns := []string{
		filepath.Join(tmpDir, "*.jsonl"),
		"/nonexistent/*.jsonl", // Should not error
	}
	
	err = store.WarmCache(patterns)
	require.NoError(t, err)
	
	// Verify file is cached
	_, err = store.GetFile(testFile)
	require.NoError(t, err)
}

func TestStore_ConcurrentAccess(t *testing.T) {
	config := StoreConfig{MaxMemory: 10240}
	store := NewStore(config)
	
	// Test concurrent file caching and retrieval
	tmpDir := t.TempDir()
	content := []byte(`{"type":"message"}`)
	
	// Create multiple files
	files := make([]string, 10)
	for i := 0; i < 10; i++ {
		files[i] = filepath.Join(tmpDir, fmt.Sprintf("file%d.jsonl", i))
		err := os.WriteFile(files[i], content, 0644)
		require.NoError(t, err)
	}
	
	// Concurrent operations
	done := make(chan bool, 20)
	
	// Cache files concurrently
	for i := 0; i < 10; i++ {
		go func(file string) {
			defer func() { done <- true }()
			store.CacheFile(file, content, []models.UsageEntry{})
		}(files[i])
	}
	
	// Read files concurrently
	for i := 0; i < 10; i++ {
		go func(file string) {
			defer func() { done <- true }()
			store.GetFile(file)
		}(files[i])
	}
	
	// Wait for all operations
	for i := 0; i < 20; i++ {
		<-done
	}
}

// Benchmark store operations
func BenchmarkStore_CacheFile(b *testing.B) {
	config := StoreConfig{MaxMemory: 10 * 1024 * 1024}
	store := NewStore(config)
	
	tmpDir := b.TempDir()
	content := make([]byte, 1024) // 1KB
	entries := []models.UsageEntry{{InputTokens: 100}}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		file := filepath.Join(tmpDir, fmt.Sprintf("bench%d.jsonl", i))
		os.WriteFile(file, content, 0644)
		store.CacheFile(file, content, entries)
	}
}

func BenchmarkStore_GetFile(b *testing.B) {
	config := StoreConfig{MaxMemory: 10 * 1024 * 1024}
	store := NewStore(config)
	
	// Pre-cache files
	tmpDir := b.TempDir()
	content := make([]byte, 1024)
	entries := []models.UsageEntry{{InputTokens: 100}}
	
	files := make([]string, 100)
	for i := 0; i < 100; i++ {
		files[i] = filepath.Join(tmpDir, fmt.Sprintf("bench%d.jsonl", i))
		os.WriteFile(files[i], content, 0644)
		store.CacheFile(files[i], content, entries)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.GetFile(files[i%100])
	}
}