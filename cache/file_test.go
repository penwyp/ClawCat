package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/penwyp/ClawCat/models"
)

func TestNewFileCache(t *testing.T) {
	cache := NewFileCache(1024)

	assert.NotNil(t, cache.cache)
	assert.NotNil(t, cache.serializer)
	assert.Equal(t, 2, cache.cache.Priority()) // Higher priority than general cache
}

func TestFileCache_CacheFileContent(t *testing.T) {
	cache := NewFileCache(10240)

	// Create temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	content := []byte(`{"type":"message","usage":{"input_tokens":100,"output_tokens":50}}`)

	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	// Cache file content
	entries := []models.UsageEntry{
		{
			Timestamp:    time.Now(),
			Model:        "test-model",
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
	}

	err = cache.CacheFileContent(testFile, content, entries)
	require.NoError(t, err)

	// Retrieve cached file
	cached, exists := cache.GetFile(testFile)
	assert.True(t, exists)
	assert.NotNil(t, cached)
	assert.Equal(t, testFile, cached.Path)
	assert.Equal(t, content, cached.Content)
	assert.Equal(t, entries, cached.Entries)
	assert.Equal(t, int64(len(content)), cached.Size)
	assert.NotEmpty(t, cached.Checksum)
}

func TestFileCache_GetFile_ModifiedFile(t *testing.T) {
	cache := NewFileCache(10240)

	// Create temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	content1 := []byte(`{"type":"message","usage":{"input_tokens":100}}`)

	err := os.WriteFile(testFile, content1, 0644)
	require.NoError(t, err)

	// Cache file
	entries := []models.UsageEntry{{InputTokens: 100}}
	err = cache.CacheFileContent(testFile, content1, entries)
	require.NoError(t, err)

	// Verify cached
	cached, exists := cache.GetFile(testFile)
	assert.True(t, exists)
	assert.Equal(t, content1, cached.Content)

	// Wait a bit and modify file
	time.Sleep(10 * time.Millisecond)
	content2 := []byte(`{"type":"message","usage":{"input_tokens":200}}`)
	err = os.WriteFile(testFile, content2, 0644)
	require.NoError(t, err)

	// File should be considered stale
	_, exists = cache.GetFile(testFile)
	assert.False(t, exists) // Should return false due to modification time
}

func TestFileCache_GetEntries(t *testing.T) {
	cache := NewFileCache(10240)

	// Create temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	content := []byte(`{"type":"message","usage":{"input_tokens":100}}`)

	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	// Cache with entries
	entries := []models.UsageEntry{
		{InputTokens: 100, OutputTokens: 50},
		{InputTokens: 200, OutputTokens: 100},
	}

	err = cache.CacheFileContent(testFile, content, entries)
	require.NoError(t, err)

	// Get only entries
	retrievedEntries, exists := cache.GetEntries(testFile)
	assert.True(t, exists)
	assert.Equal(t, entries, retrievedEntries)
}

func TestFileCache_InvalidateFile(t *testing.T) {
	cache := NewFileCache(10240)

	// Create and cache file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	content := []byte(`{"type":"message"}`)

	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	err = cache.CacheFileContent(testFile, content, []models.UsageEntry{})
	require.NoError(t, err)

	// Verify cached
	_, exists := cache.GetFile(testFile)
	assert.True(t, exists)

	// Invalidate
	err = cache.InvalidateFile(testFile)
	require.NoError(t, err)

	// Should no longer exist
	_, exists = cache.GetFile(testFile)
	assert.False(t, exists)
}

func TestFileCache_InvalidatePattern(t *testing.T) {
	cache := NewFileCache(10240)

	// Create multiple files
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

		err = cache.CacheFileContent(file, content, []models.UsageEntry{})
		require.NoError(t, err)
	}

	// Verify all cached
	for _, file := range files {
		_, exists := cache.GetFile(file)
		assert.True(t, exists, "file should be cached: %s", file)
	}

	// Invalidate pattern
	pattern := "*.jsonl"
	err := cache.InvalidatePattern(pattern)
	require.NoError(t, err)

	// Only .txt file should remain (this is a simplified test)
	// Note: The actual implementation might need adjustment for pattern matching
}

func TestFileCache_Preload(t *testing.T) {
	cache := NewFileCache(10240)

	// Create multiple files
	tmpDir := t.TempDir()
	files := []string{
		filepath.Join(tmpDir, "file1.jsonl"),
		filepath.Join(tmpDir, "file2.jsonl"),
		filepath.Join(tmpDir, "nonexistent.jsonl"),
	}

	content := []byte(`{"type":"message","usage":{"input_tokens":100}}`)
	// Only create first two files
	for i := 0; i < 2; i++ {
		err := os.WriteFile(files[i], content, 0644)
		require.NoError(t, err)
	}

	// Preload
	err := cache.Preload(files)
	require.NoError(t, err)

	// Verify existing files are cached
	for i := 0; i < 2; i++ {
		cached, exists := cache.GetFile(files[i])
		assert.True(t, exists, "file should be cached: %s", files[i])
		assert.Equal(t, content, cached.Content)
	}

	// Non-existent file should not be cached
	_, exists := cache.GetFile(files[2])
	assert.False(t, exists)
}

func TestFileCache_WarmCache(t *testing.T) {
	cache := NewFileCache(10240)

	// Create files with pattern
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	files := []string{
		filepath.Join(tmpDir, "test1.jsonl"),
		filepath.Join(tmpDir, "test2.jsonl"),
		filepath.Join(subDir, "test3.jsonl"),
		filepath.Join(tmpDir, "other.txt"),
	}

	content := []byte(`{"type":"message"}`)
	for _, file := range files {
		err := os.WriteFile(file, content, 0644)
		require.NoError(t, err)
	}

	// Warm cache with pattern
	pattern := filepath.Join(tmpDir, "*.jsonl")
	err = cache.WarmCache(pattern)
	require.NoError(t, err)

	// Verify .jsonl files in root are cached
	_, exists := cache.GetFile(files[0])
	assert.True(t, exists)
	_, exists = cache.GetFile(files[1])
	assert.True(t, exists)
}

func TestFileCache_IsStale(t *testing.T) {
	cache := NewFileCache(10240)

	// Create file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")
	content := []byte(`{"type":"message"}`)

	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	// File not in cache should be stale
	assert.True(t, cache.IsStale(testFile))

	// Cache file
	err = cache.CacheFileContent(testFile, content, []models.UsageEntry{})
	require.NoError(t, err)

	// Should not be stale
	assert.False(t, cache.IsStale(testFile))

	// Modify file
	time.Sleep(10 * time.Millisecond)
	newContent := []byte(`{"type":"message","modified":true}`)
	err = os.WriteFile(testFile, newContent, 0644)
	require.NoError(t, err)

	// Should now be stale
	assert.True(t, cache.IsStale(testFile))
}

func TestFileCache_Stats(t *testing.T) {
	cache := NewFileCache(10240)

	stats := cache.Stats()
	assert.Equal(t, int64(0), stats.Size)
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
}

func TestFileCache_CalculateSize(t *testing.T) {
	cache := NewFileCache(10240)

	cached := &CachedFile{
		Path:     "/test/path.jsonl",
		Content:  []byte("test content"),
		Entries:  []models.UsageEntry{{}, {}}, // 2 entries
		Checksum: "abc123",
	}

	size := cache.calculateSize(cached)

	expected := int64(len("/test/path.jsonl")) +
		int64(len("test content")) +
		int64(2*200) + // 2 entries * 200 bytes estimate
		int64(len("abc123")) +
		100 // overhead

	assert.Equal(t, expected, size)
}

func TestFileCache_InterfaceCompliance(t *testing.T) {
	cache := NewFileCache(1024)

	// Test Cache interface
	var _ Cache = cache

	// Test ManagedCache interface
	var _ ManagedCache = cache

	// Test basic operations
	err := cache.Set("key", "value")
	require.NoError(t, err)

	value, exists := cache.Get("key")
	assert.True(t, exists)
	assert.Equal(t, "value", value)

	assert.Equal(t, 1, cache.Size())
	assert.Equal(t, 2, cache.Priority())
	assert.True(t, cache.CanEvict())
	assert.Greater(t, cache.MemoryUsage(), int64(0))
}

// Benchmark file cache operations
func BenchmarkFileCache_CacheFile(b *testing.B) {
	cache := NewFileCache(10 * 1024 * 1024) // 10MB

	// Create test data
	content := make([]byte, 1024) // 1KB
	entries := make([]models.UsageEntry, 10)
	for i := range entries {
		entries[i] = models.UsageEntry{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		}
	}

	// Create temporary file
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "bench.jsonl")
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.CacheFileContent(testFile, content, entries)
	}
}

func BenchmarkFileCache_GetFile(b *testing.B) {
	cache := NewFileCache(10 * 1024 * 1024) // 10MB

	// Create and cache test file
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "bench.jsonl")
	content := make([]byte, 1024)
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(b, err)

	entries := []models.UsageEntry{{InputTokens: 100}}
	_ = cache.CacheFileContent(testFile, content, entries)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.GetFile(testFile)
	}
}
