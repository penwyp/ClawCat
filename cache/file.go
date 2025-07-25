package cache

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/penwyp/ClawCat/models"
)

// FileCache provides specialized caching for file contents and parsed entries
type FileCache struct {
	cache      *LRUCache
	serializer Serializer
	stats      FileCacheStats
}

// FileCacheStats extends CacheStats with file-specific metrics
type FileCacheStats struct {
	CacheStats
	BytesSaved    int64   `json:"bytes_saved"`
	ParseTime     int64   `json:"parse_time_ns"`
	CompressRatio float64 `json:"compress_ratio"`
}

// CachedFile represents a cached file with its parsed entries
type CachedFile struct {
	Path       string              `json:"path"`
	Content    []byte              `json:"content,omitempty"`
	Entries    []models.UsageEntry `json:"entries"`
	ModTime    time.Time           `json:"mod_time"`
	Size       int64               `json:"size"`
	Checksum   string              `json:"checksum"`
	Compressed bool                `json:"compressed"`
	ParsedAt   time.Time           `json:"parsed_at"`
}

// Serializer defines the interface for serializing cache values
type Serializer interface {
	Serialize(v interface{}) ([]byte, error)
	Deserialize(data []byte, v interface{}) error
}

// NewFileCache creates a new file cache with the specified maximum size
func NewFileCache(maxSize int64) *FileCache {
	cache := NewLRUCache(maxSize)
	cache.SetPriority(2) // Higher priority than general cache

	return &FileCache{
		cache:      cache,
		serializer: NewSonicSerializer(),
		stats: FileCacheStats{
			CacheStats: cache.Stats(),
		},
	}
}

// GetFile retrieves a cached file by path
func (f *FileCache) GetFile(path string) (*CachedFile, bool) {
	// Check if file exists and get mod time
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}

	// Get from cache
	value, exists := f.cache.Get(path)
	if !exists {
		return nil, false
	}

	cached, ok := value.(*CachedFile)
	if !ok {
		// Invalid cache entry, remove it
		_ = f.cache.Delete(path)
		return nil, false
	}

	// Check if file has been modified
	if !cached.ModTime.Equal(info.ModTime()) {
		_ = f.cache.Delete(path)
		return nil, false
	}

	return cached, true
}

// SetFile stores a file in the cache
func (f *FileCache) SetFile(path string, cached *CachedFile) error {
	cached.Path = path
	cached.ParsedAt = time.Now()

	// Calculate size for cache management
	size := f.calculateSize(cached)

	return f.cache.SetWithSize(path, cached, size)
}

// GetEntries retrieves only the parsed entries for a file
func (f *FileCache) GetEntries(path string) ([]models.UsageEntry, bool) {
	cached, exists := f.GetFile(path)
	if !exists {
		return nil, false
	}

	return cached.Entries, true
}

// InvalidateFile removes a file from the cache
func (f *FileCache) InvalidateFile(path string) error {
	return f.cache.Delete(path)
}

// InvalidatePattern removes all files matching a pattern from the cache
func (f *FileCache) InvalidatePattern(pattern string) error {
	// Get all cache keys and match pattern
	keysToDelete := make([]string, 0)

	// This is a simplified approach - in a real implementation,
	// we'd need to iterate through all keys
	f.cache.mu.RLock()
	for key := range f.cache.items {
		matched, err := filepath.Match(pattern, key)
		if err != nil {
			continue
		}
		if matched {
			keysToDelete = append(keysToDelete, key)
		}
	}
	f.cache.mu.RUnlock()

	// Delete matched keys
	for _, key := range keysToDelete {
		if err := f.cache.Delete(key); err != nil {
			return err
		}
	}

	return nil
}

// CacheFileContent caches file content and parses it into entries
func (f *FileCache) CacheFileContent(path string, content []byte, entries []models.UsageEntry) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	// Calculate checksum
	checksum := fmt.Sprintf("%x", md5.Sum(content))

	cached := &CachedFile{
		Path:     path,
		Content:  content,
		Entries:  entries,
		ModTime:  info.ModTime(),
		Size:     info.Size(),
		Checksum: checksum,
	}

	return f.SetFile(path, cached)
}

// FileCacheStats returns file cache statistics
func (f *FileCache) FileCacheStats() FileCacheStats {
	baseStats := f.cache.Stats()
	f.stats.CacheStats = baseStats
	return f.stats
}

// Preload attempts to cache multiple files concurrently
func (f *FileCache) Preload(paths []string) error {
	// Simple sequential preload - could be made concurrent
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			continue // Skip non-existent files
		}

		// Check if already cached
		if _, exists := f.GetFile(path); exists {
			continue
		}

		// Read and cache file
		content, err := os.ReadFile(path)
		if err != nil {
			continue // Skip unreadable files
		}

		// For preloading, we'll cache just the content without parsing
		// The parsing will happen on first access
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		checksum := fmt.Sprintf("%x", md5.Sum(content))
		cached := &CachedFile{
			Path:     path,
			Content:  content,
			Entries:  nil, // Will be parsed on demand
			ModTime:  info.ModTime(),
			Size:     info.Size(),
			Checksum: checksum,
		}

		_ = f.SetFile(path, cached)
	}

	return nil
}

// WarmCache loads files matching a pattern into cache
func (f *FileCache) WarmCache(pattern string) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to expand pattern %s: %w", pattern, err)
	}

	return f.Preload(matches)
}

// Implements Cache interface

func (f *FileCache) Get(key string) (interface{}, bool) {
	return f.cache.Get(key)
}

func (f *FileCache) Set(key string, value interface{}) error {
	return f.cache.Set(key, value)
}

func (f *FileCache) Delete(key string) error {
	return f.cache.Delete(key)
}

func (f *FileCache) Clear() error {
	return f.cache.Clear()
}

func (f *FileCache) Size() int {
	return f.cache.Size()
}

func (f *FileCache) Stats() CacheStats {
	return f.cache.Stats()
}

// Implements ManagedCache interface

func (f *FileCache) Priority() int {
	return f.cache.Priority()
}

func (f *FileCache) CanEvict() bool {
	return f.cache.CanEvict()
}

func (f *FileCache) EvictOldest(count int) error {
	return f.cache.EvictOldest(count)
}

func (f *FileCache) MemoryUsage() int64 {
	return f.cache.MemoryUsage()
}

// Private methods

func (f *FileCache) calculateSize(cached *CachedFile) int64 {
	size := int64(len(cached.Path))
	size += int64(len(cached.Content))
	size += int64(len(cached.Entries)) * 200 // Rough estimate per entry
	size += int64(len(cached.Checksum))
	size += 100 // Overhead for struct fields

	return size
}

// IsStale checks if a cached file is stale compared to the filesystem
func (f *FileCache) IsStale(path string) bool {
	cached, exists := f.GetFile(path)
	if !exists {
		return true
	}

	info, err := os.Stat(path)
	if err != nil {
		return true
	}

	return !cached.ModTime.Equal(info.ModTime())
}
