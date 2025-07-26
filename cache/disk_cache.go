package cache

import (
	"crypto/md5"
	"encoding/gob"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/logging"
)

func init() {
	// Register types that will be stored as interfaces in CacheItem.Value
	gob.Register(&FileSummary{})
	gob.Register(&DateRange{})
	gob.Register(&ModelStat{})
}

// DiskCache implements persistent cache storage on disk
type DiskCache struct {
	baseDir     string
	maxSize     int64
	ttl         time.Duration
	currentSize int64
	mu          sync.RWMutex
	stats       DiskCacheStats
}

// DiskCacheStats tracks disk cache statistics
type DiskCacheStats struct {
	Hits        int64
	Misses      int64
	Writes      int64
	Evictions   int64
	Errors      int64
	Size        int64
	FileCount   int
	HitRate     float64
	LastCleanup time.Time
}

// CacheItem represents a cached item on disk
type CacheItem struct {
	Key        string      `gob:"key"`
	Value      interface{} `gob:"value"`
	CreatedAt  time.Time   `gob:"created_at"`
	AccessedAt time.Time   `gob:"accessed_at"`
	Size       int64       `gob:"size"`
}

// NewDiskCache creates a new disk cache instance
func NewDiskCache(baseDir string, maxSize int64, ttl time.Duration) (*DiskCache, error) {
	// Expand home directory if needed
	if strings.HasPrefix(baseDir, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		baseDir = filepath.Join(homeDir, baseDir[2:])
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory %s: %w", baseDir, err)
	}

	dc := &DiskCache{
		baseDir: baseDir,
		maxSize: maxSize,
		ttl:     ttl,
		stats: DiskCacheStats{
			LastCleanup: time.Now(),
		},
	}

	// Calculate initial size
	if err := dc.calculateSize(); err != nil {
		logging.LogWarnf("Failed to calculate initial cache size: %v", err)
	}

	logging.LogInfof("Disk cache initialized: dir=%s, maxSize=%d, ttl=%v", baseDir, maxSize, ttl)
	return dc, nil
}

// Get retrieves a value from disk cache
func (dc *DiskCache) Get(key string) (interface{}, bool) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	filePath := dc.getFilePath(key)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		dc.stats.Misses++
		dc.updateHitRate()
		return nil, false
	}

	// Read and deserialize
	file, err := os.Open(filePath)
	if err != nil {
		dc.stats.Errors++
		logging.LogWarnf("Failed to open cache file %s: %v", filePath, err)
		return nil, false
	}
	defer file.Close()

	var item CacheItem
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&item); err != nil {
		dc.stats.Errors++
		logging.LogWarnf("Failed to decode cache file %s: %v", filePath, err)
		return nil, false
	}

	// Check if expired based on dynamic TTL
	itemTTL := dc.getTTLForValue(item.Value)
	if itemTTL > 0 && time.Since(item.CreatedAt) > itemTTL {
		// Remove expired file
		os.Remove(filePath)
		dc.stats.Misses++
		dc.updateHitRate()
		return nil, false
	}

	// Update access time
	item.AccessedAt = time.Now()
	if err := dc.writeItem(filePath, &item); err != nil {
		logging.LogWarnf("Failed to update access time for %s: %v", filePath, err)
	}

	dc.stats.Hits++
	dc.updateHitRate()
	return item.Value, true
}

// Set stores a value in disk cache
func (dc *DiskCache) Set(key string, value interface{}) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	item := &CacheItem{
		Key:        key,
		Value:      value,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		Size:       dc.estimateSize(value),
	}

	filePath := dc.getFilePath(key)

	// Check if we need to evict items
	if dc.currentSize+item.Size > dc.maxSize {
		if err := dc.evictItems(item.Size); err != nil {
			return fmt.Errorf("failed to evict items: %w", err)
		}
	}

	// Write to disk
	if err := dc.writeItem(filePath, item); err != nil {
		dc.stats.Errors++
		return fmt.Errorf("failed to write cache item: %w", err)
	}

	dc.currentSize += item.Size
	dc.stats.Writes++
	dc.stats.FileCount++
	dc.stats.Size = dc.currentSize

	logging.LogDebugf("Cached item to disk: key=%s, size=%d, file=%s", key, item.Size, filePath)
	return nil
}

// Delete removes an item from disk cache
func (dc *DiskCache) Delete(key string) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	filePath := dc.getFilePath(key)

	// Get file size before deletion
	if stat, err := os.Stat(filePath); err == nil {
		dc.currentSize -= stat.Size()
		dc.stats.FileCount--
		dc.stats.Size = dc.currentSize
	}

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		dc.stats.Errors++
		return fmt.Errorf("failed to remove cache file: %w", err)
	}

	return nil
}

// Clear removes all cached items
func (dc *DiskCache) Clear() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	err := filepath.WalkDir(dc.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".cache") {
			return os.Remove(path)
		}
		return nil
	})

	if err != nil {
		dc.stats.Errors++
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	dc.currentSize = 0
	dc.stats.FileCount = 0
	dc.stats.Size = 0

	logging.LogInfof("Disk cache cleared")
	return nil
}

// Cleanup removes expired items and enforces size limits
func (dc *DiskCache) Cleanup() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	logging.LogDebugf("Starting disk cache cleanup")

	var filesToRemove []string
	var totalSize int64

	err := filepath.WalkDir(dc.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".cache") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			logging.LogWarnf("Failed to get file info for %s: %v", path, err)
			return nil
		}

		totalSize += info.Size()

		// Check if file is expired based on age
		// Use shorter TTL for recent files, longer for old files
		fileAge := time.Since(info.ModTime())
		var ttl time.Duration
		if fileAge < 24*time.Hour {
			ttl = dc.ttl // Standard TTL for recent files
		} else if fileAge < 7*24*time.Hour {
			ttl = dc.ttl * 2 // Double TTL for week-old files
		} else {
			ttl = dc.ttl * 4 // Quadruple TTL for older files
		}
		
		if ttl > 0 && fileAge > ttl {
			filesToRemove = append(filesToRemove, path)
			return nil
		}

		// Check if we should remove based on LRU (access time)
		// This is a simplified approach - in a more sophisticated implementation,
		// we'd sort by access time and remove oldest first
		return nil
	})

	if err != nil {
		dc.stats.Errors++
		return fmt.Errorf("failed to walk cache directory: %w", err)
	}

	// Remove expired files
	for _, filePath := range filesToRemove {
		if err := os.Remove(filePath); err != nil {
			logging.LogWarnf("Failed to remove expired cache file %s: %v", filePath, err)
			dc.stats.Errors++
		} else {
			dc.stats.Evictions++
		}
	}

	// Update stats
	dc.currentSize = totalSize
	dc.stats.Size = totalSize
	dc.stats.LastCleanup = time.Now()

	logging.LogDebugf("Disk cache cleanup completed: removed=%d files, size=%d", len(filesToRemove), totalSize)
	return nil
}

// GetStats returns current cache statistics
func (dc *DiskCache) GetStats() DiskCacheStats {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.stats
}

// Private methods

func (dc *DiskCache) getFilePath(key string) string {
	// Use MD5 hash to create safe filename
	hash := md5.Sum([]byte(key))
	filename := fmt.Sprintf("%x.cache", hash)
	return filepath.Join(dc.baseDir, filename)
}

func (dc *DiskCache) writeItem(filePath string, item *CacheItem) error {
	// Create temp file first for atomic writes
	tempPath := filePath + ".tmp"

	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(item); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to encode item: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, filePath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

func (dc *DiskCache) evictItems(neededSpace int64) error {
	// Simple LRU eviction based on file modification time
	// In a more sophisticated implementation, we'd maintain an access log

	type fileInfo struct {
		path    string
		size    int64
		modTime time.Time
	}

	var files []fileInfo

	err := filepath.WalkDir(dc.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".cache") {
			info, err := d.Info()
			if err != nil {
				return err
			}

			files = append(files, fileInfo{
				path:    path,
				size:    info.Size(),
				modTime: info.ModTime(),
			})
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to list cache files: %w", err)
	}

	// Sort by modification time (oldest first)
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].modTime.After(files[j].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	// Remove oldest files until we have enough space
	var freedSpace int64
	for _, file := range files {
		if freedSpace >= neededSpace {
			break
		}

		if err := os.Remove(file.path); err != nil {
			logging.LogWarnf("Failed to evict cache file %s: %v", file.path, err)
			continue
		}

		freedSpace += file.size
		dc.currentSize -= file.size
		dc.stats.Evictions++
		dc.stats.FileCount--

		logging.LogDebugf("Evicted cache file: %s (size: %d)", file.path, file.size)
	}

	dc.stats.Size = dc.currentSize
	return nil
}

func (dc *DiskCache) calculateSize() error {
	var totalSize int64
	var fileCount int

	err := filepath.WalkDir(dc.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".cache") {
			info, err := d.Info()
			if err != nil {
				return err
			}
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})

	if err != nil {
		return err
	}

	dc.currentSize = totalSize
	dc.stats.Size = totalSize
	dc.stats.FileCount = fileCount
	return nil
}

func (dc *DiskCache) estimateSize(value interface{}) int64 {
	// Simple size estimation - in a real implementation,
	// we might use reflection or other methods for more accuracy
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case *FileSummary:
		return int64(len(v.Path) + len(v.AbsolutePath) + 8*10) // rough estimate
	default:
		return 1024 // default estimate
	}
}

func (dc *DiskCache) updateHitRate() {
	total := dc.stats.Hits + dc.stats.Misses
	if total > 0 {
		dc.stats.HitRate = float64(dc.stats.Hits) / float64(total)
	}
}

// getTTLForValue returns age-based TTL for a value
func (dc *DiskCache) getTTLForValue(value interface{}) time.Duration {
	// Check if value is a FileSummary
	if summary, ok := value.(*FileSummary); ok {
		// Calculate age based on data range
		dataAge := time.Since(summary.DateRange.End)
		
		// Historical data (>30 days): Keep for months
		if dataAge > 30*24*time.Hour {
			return 90 * 24 * time.Hour // 90 days
		}
		
		// Weekly data (7-30 days): Keep for weeks
		if dataAge > 7*24*time.Hour {
			return 30 * 24 * time.Hour // 30 days
		}
		
		// Recent data (<7 days): Standard TTL
		return dc.ttl
	}
	
	// Check if value is DateRange
	if dr, ok := value.(*DateRange); ok {
		dataAge := time.Since(dr.End)
		
		if dataAge > 30*24*time.Hour {
			return 90 * 24 * time.Hour
		}
		if dataAge > 7*24*time.Hour {
			return 30 * 24 * time.Hour
		}
		return dc.ttl
	}
	
	// Default TTL for other types
	return dc.ttl
}
