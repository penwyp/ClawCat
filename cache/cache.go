package cache

import (
	"time"
)

// Cache defines the common interface for all cache implementations
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}) error
	Delete(key string) error
	Clear() error
	Size() int
	Stats() CacheStats
}

// CacheStats provides metrics about cache performance
type CacheStats struct {
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	Evictions int64   `json:"evictions"`
	Size      int64   `json:"size"`
	MaxSize   int64   `json:"max_size"`
	HitRate   float64 `json:"hit_rate"`
}

// Entry represents a cache entry with metadata
type Entry struct {
	Key        string        `json:"key"`
	Value      interface{}   `json:"value"`
	Size       int64         `json:"size"`
	AccessTime time.Time     `json:"access_time"`
	CreateTime time.Time     `json:"create_time"`
	TTL        time.Duration `json:"ttl"`
}

// IsExpired checks if the entry has expired based on TTL
func (e *Entry) IsExpired() bool {
	if e.TTL == 0 {
		return false
	}
	return time.Since(e.CreateTime) > e.TTL
}

// UpdateCacheStats calculates the hit rate for cache stats
func (s *CacheStats) UpdateHitRate() {
	total := s.Hits + s.Misses
	if total > 0 {
		s.HitRate = float64(s.Hits) / float64(total)
	} else {
		s.HitRate = 0.0
	}
}

// ManagedCache extends Cache with memory management capabilities
type ManagedCache interface {
	Cache
	Priority() int
	CanEvict() bool
	EvictOldest(count int) error
	MemoryUsage() int64
}

// EvictionCallback is called when an entry is evicted from the cache
type EvictionCallback func(key string, value interface{})
