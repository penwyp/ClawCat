package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/penwyp/claudecat/logging"
)

// BadgerSummaryCache provides a BadgerDB-based cache for file summaries
// It offers better concurrent performance compared to SimpleSummaryCache
type BadgerSummaryCache struct {
	db          *badger.DB
	persistPath string
	mu          sync.RWMutex // For stats tracking
	stats       BadgerCacheStats
}

// BadgerCacheStats tracks cache statistics
type BadgerCacheStats struct {
	Hits    int64
	Misses  int64
	Writes  int64
	Deletes int64
	Errors  int64
}

// NewBadgerSummaryCache creates a new BadgerDB-based summary cache
func NewBadgerSummaryCache(persistPath string) (*BadgerSummaryCache, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(persistPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Open BadgerDB
	opts := badger.DefaultOptions(persistPath)
	opts.Logger = nil       // Disable BadgerDB's internal logging
	opts.SyncWrites = false // Better performance, acceptable for cache
	opts.NumVersionsToKeep = 1
	opts.NumMemtables = 2
	opts.NumLevelZeroTables = 2
	opts.NumLevelZeroTablesStall = 4
	opts.ValueLogFileSize = 64 << 20 // 64MB
	opts.MaxLevels = 7
	opts.ValueThreshold = 256
	opts.NumCompactors = 2

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
	}

	cache := &BadgerSummaryCache{
		db:          db,
		persistPath: persistPath,
	}

	// Start garbage collection routine
	go cache.runGCRoutine()

	logging.LogInfof("Initialized BadgerDB cache at %s", persistPath)
	return cache, nil
}

// GetFileSummary retrieves a file summary from cache
func (c *BadgerSummaryCache) GetFileSummary(absolutePath string) (*FileSummary, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var summary *FileSummary
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(absolutePath))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				c.stats.Misses++
				return fmt.Errorf("file summary not found: %s", absolutePath)
			}
			c.stats.Errors++
			return err
		}

		return item.Value(func(val []byte) error {
			summary = &FileSummary{}
			if err := json.Unmarshal(val, summary); err != nil {
				c.stats.Errors++
				return fmt.Errorf("failed to unmarshal summary: %w", err)
			}
			c.stats.Hits++
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return summary, nil
}

// SetFileSummary stores a file summary in cache
func (c *BadgerSummaryCache) SetFileSummary(summary *FileSummary) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(summary)
	if err != nil {
		c.stats.Errors++
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	err = c.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(summary.AbsolutePath), data)
	})

	if err != nil {
		c.stats.Errors++
		return fmt.Errorf("failed to store summary: %w", err)
	}

	c.stats.Writes++
	return nil
}

// HasFileSummary checks if a file summary exists in cache
func (c *BadgerSummaryCache) HasFileSummary(absolutePath string) bool {
	err := c.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(absolutePath))
		return err
	})
	return err == nil
}

// InvalidateFileSummary removes a file summary from cache
func (c *BadgerSummaryCache) InvalidateFileSummary(absolutePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(absolutePath))
	})

	if err != nil {
		c.stats.Errors++
		return fmt.Errorf("failed to delete summary: %w", err)
	}

	c.stats.Deletes++
	return nil
}

// IsFileChanged checks if a file has changed based on modTime and size
func (c *BadgerSummaryCache) IsFileChanged(filePath string, stat os.FileInfo) bool {
	summary, err := c.GetFileSummary(filePath)
	if err != nil {
		return true // File not cached or error, needs processing
	}

	// Check modTime and size
	return !summary.ModTime.Equal(stat.ModTime()) || summary.FileSize != stat.Size()
}

// BatchSet performs multiple set operations in a single transaction
func (c *BadgerSummaryCache) BatchSet(summaries []*FileSummary) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	txn := c.db.NewTransaction(true)
	defer txn.Discard()

	for _, summary := range summaries {
		data, err := json.Marshal(summary)
		if err != nil {
			c.stats.Errors++
			return fmt.Errorf("failed to marshal summary: %w", err)
		}

		if err := txn.Set([]byte(summary.AbsolutePath), data); err != nil {
			c.stats.Errors++
			return fmt.Errorf("failed to store summary in batch: %w", err)
		}
	}

	if err := txn.Commit(); err != nil {
		c.stats.Errors++
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	c.stats.Writes += int64(len(summaries))
	return nil
}

// Clear removes all summaries from cache
func (c *BadgerSummaryCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close and reopen the database to clear it
	if err := c.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	// Remove the database directory
	if err := os.RemoveAll(c.persistPath); err != nil {
		return fmt.Errorf("failed to remove database directory: %w", err)
	}

	// Recreate and reopen
	opts := badger.DefaultOptions(c.persistPath)
	opts.Logger = nil
	opts.SyncWrites = false

	db, err := badger.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to reopen database: %w", err)
	}

	c.db = db
	logging.LogInfof("Cache cleared")
	return nil
}

// GetStats returns cache statistics
func (c *BadgerSummaryCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Get database size
	lsm, vlog := c.db.Size()

	// Count entries
	var entryCount int64
	var totalEntries int64
	var totalCost float64
	var totalTokens int64

	c.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			entryCount++
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var summary FileSummary
				if err := json.Unmarshal(val, &summary); err == nil {
					totalEntries += int64(summary.EntryCount)
					totalCost += summary.TotalCost
					totalTokens += int64(summary.TotalTokens)
				}
				return nil
			})
			if err != nil {
				logging.LogWarnf("Failed to read cache entry: %v", err)
			}
		}
		return nil
	})

	return map[string]interface{}{
		"cached_files":  entryCount,
		"total_entries": totalEntries,
		"total_cost":    totalCost,
		"total_tokens":  totalTokens,
		"db_size_lsm":   lsm,
		"db_size_vlog":  vlog,
		"hits":          c.stats.Hits,
		"misses":        c.stats.Misses,
		"writes":        c.stats.Writes,
		"deletes":       c.stats.Deletes,
		"errors":        c.stats.Errors,
		"hit_rate":      float64(c.stats.Hits) / float64(c.stats.Hits+c.stats.Misses),
		"persist_path":  c.persistPath,
	}
}

// Close closes the BadgerDB database
func (c *BadgerSummaryCache) Close() error {
	return c.db.Close()
}

// runGCRoutine runs garbage collection periodically
func (c *BadgerSummaryCache) runGCRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		err := c.db.RunValueLogGC(0.5)
		if err != nil && err != badger.ErrNoRewrite {
			logging.LogDebugf("BadgerDB GC error: %v", err)
		}
	}
}
