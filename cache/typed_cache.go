package cache

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/dgraph-io/badger/v3"
)

// TypedCache provides type-safe cache operations using gob encoding
type TypedCache struct {
	db     *badger.DB
	config BadgerConfig
}

// NewTypedCache creates a new typed cache
func NewTypedCache(config BadgerConfig) (*TypedCache, error) {
	badgerCache, err := NewBadgerCache(config)
	if err != nil {
		return nil, err
	}

	return &TypedCache{
		db:     badgerCache.db,
		config: config,
	}, nil
}

// SetHourlyAggregation stores an hourly aggregation
func (tc *TypedCache) SetHourlyAggregation(key string, agg *HourlyAggregation) error {
	return tc.setTyped(key, agg)
}

// GetHourlyAggregation retrieves an hourly aggregation
func (tc *TypedCache) GetHourlyAggregation(key string) (*HourlyAggregation, bool) {
	var result HourlyAggregation
	if ok := tc.getTyped(key, &result); ok {
		return &result, true
	}
	return nil, false
}

// SetDailyAggregation stores a daily aggregation
func (tc *TypedCache) SetDailyAggregation(key string, agg *DailyAggregation) error {
	return tc.setTyped(key, agg)
}

// GetDailyAggregation retrieves a daily aggregation
func (tc *TypedCache) GetDailyAggregation(key string) (*DailyAggregation, bool) {
	var result DailyAggregation
	if ok := tc.getTyped(key, &result); ok {
		return &result, true
	}
	return nil, false
}

// SetModelSummary stores a model summary
func (tc *TypedCache) SetModelSummary(key string, summary *ModelSummary) error {
	return tc.setTyped(key, summary)
}

// GetModelSummary retrieves a model summary
func (tc *TypedCache) GetModelSummary(key string) (*ModelSummary, bool) {
	var result ModelSummary
	if ok := tc.getTyped(key, &result); ok {
		return &result, true
	}
	return nil, false
}

// SetModels stores a list of models
func (tc *TypedCache) SetModels(key string, models []string) error {
	return tc.setTyped(key, models)
}

// GetModels retrieves a list of models
func (tc *TypedCache) GetModels(key string) ([]string, bool) {
	var result []string
	if ok := tc.getTyped(key, &result); ok {
		return result, true
	}
	return nil, false
}

// setTyped stores a value using gob encoding
func (tc *TypedCache) setTyped(key string, value interface{}) error {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(value)
	if err != nil {
		return fmt.Errorf("failed to encode value: %w", err)
	}

	return tc.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), buf.Bytes())
		if tc.config.DefaultTTL > 0 {
			entry = entry.WithTTL(tc.config.DefaultTTL)
		}
		return txn.SetEntry(entry)
	})
}

// getTyped retrieves a value using gob decoding
func (tc *TypedCache) getTyped(key string, result interface{}) bool {
	err := tc.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			buf := bytes.NewBuffer(val)
			decoder := gob.NewDecoder(buf)
			return decoder.Decode(result)
		})
	})

	return err == nil
}

// Delete removes a key from the cache
func (tc *TypedCache) Delete(key string) error {
	return tc.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// Clear removes all entries from the cache
func (tc *TypedCache) Clear() error {
	return tc.db.DropAll()
}

// Close closes the cache
func (tc *TypedCache) Close() error {
	return tc.db.Close()
}

// GetStats returns cache statistics
func (tc *TypedCache) GetStats() BadgerStats {
	lsm, vlog := tc.db.Size()
	
	return BadgerStats{
		LSMSize:   lsm,
		VLogSize:  vlog,
		TotalSize: lsm + vlog,
		NumKeys:   tc.countKeys(),
		Config:    tc.config,
	}
}

// countKeys counts the number of keys in the database
func (tc *TypedCache) countKeys() int64 {
	var count int64
	
	tc.db.View(func(txn *badger.Txn) error {
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