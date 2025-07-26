package cache

import (
	"fmt"
	"hash/fnv"
	"sync"
)

const (
	// DefaultShardCount is the default number of cache shards
	DefaultShardCount = 32
)

// ShardedCache implements a sharded cache to reduce lock contention
type ShardedCache struct {
	shards    []*cacheShard
	shardMask uint32
	config    ShardedCacheConfig
}

// ShardedCacheConfig configures the sharded cache
type ShardedCacheConfig struct {
	ShardCount int   // Number of shards (must be power of 2)
	MaxSize    int64 // Total max size across all shards
}

// cacheShard represents a single shard in the cache
type cacheShard struct {
	mu      sync.RWMutex
	cache   *LRUCache
	shardID int
	maxSize int64
}

// NewShardedCache creates a new sharded cache
func NewShardedCache(config ShardedCacheConfig) *ShardedCache {
	// Ensure shard count is power of 2
	shardCount := config.ShardCount
	if shardCount <= 0 {
		shardCount = DefaultShardCount
	}

	// Round up to nearest power of 2
	shardCount = nearestPowerOfTwo(shardCount)

	// Calculate size per shard
	sizePerShard := config.MaxSize / int64(shardCount)
	if sizePerShard < 1024*1024 { // Minimum 1MB per shard
		sizePerShard = 1024 * 1024
	}

	sc := &ShardedCache{
		shards:    make([]*cacheShard, shardCount),
		shardMask: uint32(shardCount - 1),
		config:    config,
	}

	// Initialize shards
	for i := 0; i < shardCount; i++ {
		sc.shards[i] = &cacheShard{
			cache:   NewLRUCache(sizePerShard),
			shardID: i,
			maxSize: sizePerShard,
		}
		// Set a higher priority for sharded caches
		sc.shards[i].cache.SetPriority(4)
	}

	return sc
}

// Get retrieves a value from the appropriate shard
func (sc *ShardedCache) Get(key string) (interface{}, bool) {
	shard := sc.getShard(key)

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	return shard.cache.Get(key)
}

// Set stores a value in the appropriate shard
func (sc *ShardedCache) Set(key string, value interface{}) error {
	shard := sc.getShard(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	return shard.cache.Set(key, value)
}

// SetWithSize stores a value with a specific size in the appropriate shard
func (sc *ShardedCache) SetWithSize(key string, value interface{}, size int64) error {
	shard := sc.getShard(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	return shard.cache.SetWithSize(key, value, size)
}

// SetWithOptions stores a value with options in the appropriate shard
func (sc *ShardedCache) SetWithOptions(key string, value interface{}, size int64, persistent bool) error {
	shard := sc.getShard(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	return shard.cache.SetWithOptions(key, value, size, persistent)
}

// Delete removes a value from the appropriate shard
func (sc *ShardedCache) Delete(key string) error {
	shard := sc.getShard(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	return shard.cache.Delete(key)
}

// Clear removes all values from all shards
func (sc *ShardedCache) Clear() error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(sc.shards))

	// Clear all shards in parallel
	for _, shard := range sc.shards {
		wg.Add(1)
		go func(s *cacheShard) {
			defer wg.Done()

			s.mu.Lock()
			defer s.mu.Unlock()

			if err := s.cache.Clear(); err != nil {
				errChan <- fmt.Errorf("shard %d: %w", s.shardID, err)
			}
		}(shard)
	}

	wg.Wait()
	close(errChan)

	// Collect any errors
	for err := range errChan {
		return err // Return first error
	}

	return nil
}

// Size returns the total number of items across all shards
func (sc *ShardedCache) Size() int {
	total := 0

	// Use read locks to count items
	for _, shard := range sc.shards {
		shard.mu.RLock()
		total += shard.cache.Size()
		shard.mu.RUnlock()
	}

	return total
}

// Stats returns aggregated statistics from all shards
func (sc *ShardedCache) Stats() CacheStats {
	stats := CacheStats{}

	// Aggregate stats from all shards
	for _, shard := range sc.shards {
		shard.mu.RLock()
		shardStats := shard.cache.Stats()
		shard.mu.RUnlock()

		stats.Hits += shardStats.Hits
		stats.Misses += shardStats.Misses
		stats.Evictions += shardStats.Evictions
		stats.Size += shardStats.Size
	}

	// Calculate hit rate
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}

	return stats
}

// getShard determines which shard should handle a given key
func (sc *ShardedCache) getShard(key string) *cacheShard {
	hash := fnv32(key)
	shardIndex := hash & sc.shardMask
	return sc.shards[shardIndex]
}

// ShardStats returns individual statistics for each shard
func (sc *ShardedCache) ShardStats() []CacheStats {
	stats := make([]CacheStats, len(sc.shards))

	for i, shard := range sc.shards {
		shard.mu.RLock()
		stats[i] = shard.cache.Stats()
		shard.mu.RUnlock()
	}

	return stats
}

// Rebalance attempts to rebalance load across shards
func (sc *ShardedCache) Rebalance() error {
	// In a sharded cache, rebalancing would require rehashing all keys
	// For now, this is a no-op, but could be implemented if needed
	return nil
}

// MemoryUsage returns the total memory usage across all shards
func (sc *ShardedCache) MemoryUsage() int64 {
	var total int64

	for _, shard := range sc.shards {
		shard.mu.RLock()
		total += shard.cache.MemoryUsage()
		shard.mu.RUnlock()
	}

	return total
}

// fnv32 computes a 32-bit FNV-1a hash
func fnv32(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

// nearestPowerOfTwo returns the nearest power of two greater than or equal to n
func nearestPowerOfTwo(n int) int {
	if n <= 1 {
		return 1
	}

	// Check if already a power of two
	if n&(n-1) == 0 {
		return n
	}

	// Find the next power of two
	power := 1
	for power < n {
		power <<= 1
	}

	return power
}

// GetShardForKey returns the shard index for a given key (useful for debugging)
func (sc *ShardedCache) GetShardForKey(key string) int {
	hash := fnv32(key)
	return int(hash & sc.shardMask)
}

// IsBalanced checks if the cache is reasonably balanced across shards
func (sc *ShardedCache) IsBalanced() bool {
	stats := sc.ShardStats()
	if len(stats) == 0 {
		return true
	}

	// Calculate average size per shard
	var totalSize int64
	for _, stat := range stats {
		totalSize += stat.Size
	}
	avgSize := float64(totalSize) / float64(len(stats))

	// Check if any shard deviates more than 50% from average
	for _, stat := range stats {
		deviation := float64(stat.Size) - avgSize
		if deviation < 0 {
			deviation = -deviation
		}

		if avgSize > 0 && deviation/avgSize > 0.5 {
			return false
		}
	}

	return true
}
