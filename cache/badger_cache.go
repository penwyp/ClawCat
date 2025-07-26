package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v3"
)

// BadgerCache provides a BadgerDB-based cache implementation
type BadgerCache struct {
	db     *badger.DB
	config BadgerConfig
	mu     sync.RWMutex
	closed bool
}

// BadgerConfig configures the BadgerDB cache
type BadgerConfig struct {
	DBPath           string        `json:"db_path"`
	MaxMemoryUsage   int64         `json:"max_memory_usage"`   // Memory usage limit in bytes
	ValueThreshold   int64         `json:"value_threshold"`    // Values larger than this are stored separately
	CompactionLevel  int           `json:"compaction_level"`   // Compression level (0-3)
	GCDiscardRatio   float64       `json:"gc_discard_ratio"`   // GC discard ratio (0.5 recommended)
	GCInterval       time.Duration `json:"gc_interval"`        // Garbage collection interval
	DefaultTTL       time.Duration `json:"default_ttl"`        // Default TTL for entries
	EnableEncryption bool          `json:"enable_encryption"`  // Enable encryption at rest
	LogLevel         string        `json:"log_level"`          // Log level: DEBUG, INFO, WARNING, ERROR
}

// NewBadgerCache creates a new BadgerDB cache
func NewBadgerCache(config BadgerConfig) (*BadgerCache, error) {
	// Set defaults
	if config.DBPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.DBPath = filepath.Join(homeDir, ".cache", "clawcat", "badger")
	}
	if config.MaxMemoryUsage <= 0 {
		config.MaxMemoryUsage = 256 * 1024 * 1024 // 256MB default
	}
	if config.ValueThreshold <= 0 {
		config.ValueThreshold = 1024 // 1KB default
	}
	if config.CompactionLevel <= 0 {
		config.CompactionLevel = 1 // Default compression
	}
	if config.GCDiscardRatio <= 0 {
		config.GCDiscardRatio = 0.5
	}
	if config.GCInterval <= 0 {
		config.GCInterval = 5 * time.Minute
	}
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = 24 * time.Hour
	}
	if config.LogLevel == "" {
		config.LogLevel = "WARNING"
	}

	// Ensure directory exists
	if err := os.MkdirAll(config.DBPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Configure BadgerDB options
	opts := badger.DefaultOptions(config.DBPath)
	opts = opts.WithValueThreshold(config.ValueThreshold)
	opts = opts.WithCompression(getCompressionOptions(config.CompactionLevel))
	opts = opts.WithMemTableSize(config.MaxMemoryUsage / 4) // Use 1/4 of memory for memtable
	opts = opts.WithValueLogFileSize(64 * 1024 * 1024)     // 64MB value log files
	opts = opts.WithNumMemtables(3)                        // 3 memtables for better write performance
	opts = opts.WithNumLevelZeroTables(5)                  // Level 0 SST tables
	opts = opts.WithNumLevelZeroTablesStall(10)            // Stall writes threshold
	
	// Set log level
	switch config.LogLevel {
	case "DEBUG":
		opts = opts.WithLogger(&badgerLogger{level: badger.DEBUG})
	case "INFO":
		opts = opts.WithLogger(&badgerLogger{level: badger.INFO})
	case "WARNING":
		opts = opts.WithLogger(&badgerLogger{level: badger.WARNING})
	case "ERROR":
		opts = opts.WithLogger(&badgerLogger{level: badger.ERROR})
	default:
		opts = opts.WithLogger(&badgerLogger{level: badger.WARNING})
	}

	// Open database
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
	}

	cache := &BadgerCache{
		db:     db,
		config: config,
	}

	// Start background garbage collection
	cache.startGC()

	return cache, nil
}

// Get retrieves a value from the cache
func (bc *BadgerCache) Get(key string) (interface{}, bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.closed {
		return nil, false
	}

	var result interface{}
	err := bc.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &result)
		})
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, false
		}
		fmt.Printf("BadgerCache.Get error for key %s: %v\n", key, err)
		return nil, false
	}

	return result, true
}

// Set stores a value in the cache with default TTL
func (bc *BadgerCache) Set(key string, value interface{}) error {
	return bc.SetWithTTL(key, value, bc.config.DefaultTTL)
}

// SetWithTTL stores a value in the cache with specified TTL
func (bc *BadgerCache) SetWithTTL(key string, value interface{}, ttl time.Duration) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.closed {
		return fmt.Errorf("cache is closed")
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return bc.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), data)
		if ttl > 0 {
			entry = entry.WithTTL(ttl)
		}
		return txn.SetEntry(entry)
	})
}

// Delete removes a key from the cache
func (bc *BadgerCache) Delete(key string) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.closed {
		return fmt.Errorf("cache is closed")
	}

	return bc.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// Clear removes all entries from the cache
func (bc *BadgerCache) Clear() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if bc.closed {
		return fmt.Errorf("cache is closed")
	}

	return bc.db.DropAll()
}

// GetByPrefix retrieves all keys with the given prefix
func (bc *BadgerCache) GetByPrefix(prefix string) (map[string]interface{}, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.closed {
		return nil, fmt.Errorf("cache is closed")
	}

	results := make(map[string]interface{})
	
	err := bc.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100 // Prefetch for better performance
		it := txn.NewIterator(opts)
		defer it.Close()

		prefixBytes := []byte(prefix)
		for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
			item := it.Item()
			key := string(item.Key())
			
			var value interface{}
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &value)
			})
			if err != nil {
				fmt.Printf("Failed to unmarshal value for key %s: %v\n", key, err)
				continue
			}
			
			results[key] = value
		}
		return nil
	})

	return results, err
}

// GetStats returns cache statistics
func (bc *BadgerCache) GetStats() BadgerStats {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.closed {
		return BadgerStats{}
	}

	lsm, vlog := bc.db.Size()
	
	return BadgerStats{
		LSMSize:       lsm,
		VLogSize:      vlog,
		TotalSize:     lsm + vlog,
		NumKeys:       bc.countKeys(),
		Config:        bc.config,
	}
}

// BadgerStats provides BadgerDB statistics
type BadgerStats struct {
	LSMSize   int64        `json:"lsm_size"`   // LSM tree size in bytes
	VLogSize  int64        `json:"vlog_size"`  // Value log size in bytes  
	TotalSize int64        `json:"total_size"` // Total database size
	NumKeys   int64        `json:"num_keys"`   // Number of keys
	Config    BadgerConfig `json:"config"`     // Configuration
}

// Close closes the BadgerDB cache
func (bc *BadgerCache) Close() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if bc.closed {
		return nil
	}

	bc.closed = true
	return bc.db.Close()
}

// Backup creates a backup of the database
func (bc *BadgerCache) Backup(backupPath string) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.closed {
		return fmt.Errorf("cache is closed")
	}

	backupFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer backupFile.Close()

	_, err = bc.db.Backup(backupFile, 0)
	return err
}

// Restore restores the database from a backup
func (bc *BadgerCache) Restore(backupPath string) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if bc.closed {
		return fmt.Errorf("cache is closed")
	}

	backupFile, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer backupFile.Close()

	return bc.db.Load(backupFile, 256)
}

// RunGC runs garbage collection manually
func (bc *BadgerCache) RunGC() error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.closed {
		return fmt.Errorf("cache is closed")
	}

	return bc.db.RunValueLogGC(bc.config.GCDiscardRatio)
}

// startGC starts background garbage collection
func (bc *BadgerCache) startGC() {
	go func() {
		ticker := time.NewTicker(bc.config.GCInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if bc.closed {
					return
				}
				// Run GC if there's enough garbage
				err := bc.RunGC()
				if err != nil && err != badger.ErrNoRewrite {
					fmt.Printf("BadgerCache GC error: %v\n", err)
				}
			}
		}
	}()
}

// countKeys counts the number of keys in the database
func (bc *BadgerCache) countKeys() int64 {
	var count int64
	
	bc.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // Only count keys, don't fetch values
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}
		return nil
	})
	
	return count
}

// getCompressionOptions returns compression options based on level
func getCompressionOptions(level int) badger.CompressionType {
	switch level {
	case 0:
		return badger.None
	case 1:
		return badger.Snappy
	case 2:
		return badger.ZSTD
	default:
		return badger.Snappy
	}
}

// badgerLogger implements badger.Logger interface
type badgerLogger struct {
	level badger.LogLevel
}

func (l *badgerLogger) Errorf(format string, args ...interface{}) {
	if l.level <= badger.ERROR {
		fmt.Printf("[BADGER ERROR] "+format+"\n", args...)
	}
}

func (l *badgerLogger) Warningf(format string, args ...interface{}) {
	if l.level <= badger.WARNING {
		fmt.Printf("[BADGER WARNING] "+format+"\n", args...)
	}
}

func (l *badgerLogger) Infof(format string, args ...interface{}) {
	if l.level <= badger.INFO {
		fmt.Printf("[BADGER INFO] "+format+"\n", args...)
	}
}

func (l *badgerLogger) Debugf(format string, args ...interface{}) {
	if l.level <= badger.DEBUG {
		fmt.Printf("[BADGER DEBUG] "+format+"\n", args...)
	}
}