package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheStats_UpdateHitRate(t *testing.T) {
	tests := []struct {
		name     string
		hits     int64
		misses   int64
		expected float64
	}{
		{"No requests", 0, 0, 0.0},
		{"All hits", 100, 0, 1.0},
		{"All misses", 0, 100, 0.0},
		{"Mixed", 75, 25, 0.75},
		{"Half and half", 50, 50, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := CacheStats{
				Hits:   tt.hits,
				Misses: tt.misses,
			}
			
			stats.UpdateHitRate()
			assert.Equal(t, tt.expected, stats.HitRate)
		})
	}
}

func TestEntry_IsExpired(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name     string
		entry    Entry
		expected bool
	}{
		{
			name: "No TTL",
			entry: Entry{
				CreateTime: now.Add(-1 * time.Hour),
				TTL:        0,
			},
			expected: false,
		},
		{
			name: "Not expired",
			entry: Entry{
				CreateTime: now.Add(-30 * time.Second),
				TTL:        1 * time.Minute,
			},
			expected: false,
		},
		{
			name: "Expired",
			entry: Entry{
				CreateTime: now.Add(-2 * time.Minute),
				TTL:        1 * time.Minute,
			},
			expected: true,
		},
		{
			name: "Just expired",
			entry: Entry{
				CreateTime: now.Add(-1*time.Minute - 1*time.Second),
				TTL:        1 * time.Minute,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.entry.IsExpired())
		})
	}
}

func TestCacheInterface(t *testing.T) {
	// Test that LRUCache implements Cache interface
	cache := NewLRUCache(1024)
	
	var _ Cache = cache
	
	// Test basic operations
	assert.Equal(t, 0, cache.Size())
	
	// Test set and get
	err := cache.Set("key1", "value1")
	require.NoError(t, err)
	
	value, exists := cache.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value)
	
	// Test delete
	err = cache.Delete("key1")
	require.NoError(t, err)
	
	_, exists = cache.Get("key1")
	assert.False(t, exists)
	
	// Test clear
	err = cache.Set("key2", "value2")
	require.NoError(t, err)
	err = cache.Set("key3", "value3")
	require.NoError(t, err)
	assert.Equal(t, 2, cache.Size())
	
	err = cache.Clear()
	require.NoError(t, err)
	assert.Equal(t, 0, cache.Size())
}

func TestManagedCacheInterface(t *testing.T) {
	// Test that LRUCache implements ManagedCache interface
	cache := NewLRUCache(1024)
	
	var _ ManagedCache = cache
	
	// Test managed cache operations
	assert.Equal(t, 1, cache.Priority())
	
	cache.SetPriority(5)
	assert.Equal(t, 5, cache.Priority())
	
	assert.False(t, cache.CanEvict()) // Empty cache
	
	err := cache.Set("key1", "value1")
	require.NoError(t, err)
	assert.True(t, cache.CanEvict())
	
	err = cache.EvictOldest(1)
	require.NoError(t, err)
	assert.Equal(t, 0, cache.Size())
	
	assert.Equal(t, int64(0), cache.MemoryUsage())
}

// Benchmark cache operations
func BenchmarkCacheSet(b *testing.B) {
	cache := NewLRUCache(1024 * 1024) // 1MB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		_ = cache.Set(key, "some test value")
	}
}

func BenchmarkCacheGet(b *testing.B) {
	cache := NewLRUCache(1024 * 1024) // 1MB
	
	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		_ = cache.Set(key, "some test value")
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%1000)
		cache.Get(key)
	}
}

func BenchmarkCacheSetGet(b *testing.B) {
	cache := NewLRUCache(1024 * 1024) // 1MB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		_ = cache.Set(key, "test value")
		cache.Get(key)
	}
}