package cache

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLRUCache(t *testing.T) {
	cache := NewLRUCache(1024)
	
	assert.Equal(t, int64(1024), cache.capacity)
	assert.Equal(t, int64(0), cache.size)
	assert.Equal(t, 0, len(cache.items))
	assert.NotNil(t, cache.head)
	assert.NotNil(t, cache.tail)
	assert.Equal(t, cache.tail, cache.head.next)
	assert.Equal(t, cache.head, cache.tail.prev)
}

func TestLRUCache_SetAndGet(t *testing.T) {
	cache := NewLRUCache(1024)
	
	// Test setting and getting a value
	err := cache.Set("key1", "value1")
	require.NoError(t, err)
	
	value, exists := cache.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value)
	assert.Equal(t, 1, cache.Size())
	
	// Test getting non-existent key
	_, exists = cache.Get("nonexistent")
	assert.False(t, exists)
	
	// Test stats
	stats := cache.Stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, 0.5, stats.HitRate)
}

func TestLRUCache_Update(t *testing.T) {
	cache := NewLRUCache(1024)
	
	// Set initial value
	err := cache.Set("key1", "value1")
	require.NoError(t, err)
	
	// Update value
	err = cache.Set("key1", "value2")
	require.NoError(t, err)
	
	value, exists := cache.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value2", value)
	assert.Equal(t, 1, cache.Size()) // Size should not change
}

func TestLRUCache_Delete(t *testing.T) {
	cache := NewLRUCache(1024)
	
	// Set and delete
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	assert.Equal(t, 2, cache.Size())
	
	err := cache.Delete("key1")
	require.NoError(t, err)
	assert.Equal(t, 1, cache.Size())
	
	_, exists := cache.Get("key1")
	assert.False(t, exists)
	
	_, exists = cache.Get("key2")
	assert.True(t, exists)
	
	// Delete non-existent key should not error
	err = cache.Delete("nonexistent")
	require.NoError(t, err)
}

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(1024)
	
	// Add some items
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")
	assert.Equal(t, 3, cache.Size())
	
	// Clear cache
	err := cache.Clear()
	require.NoError(t, err)
	assert.Equal(t, 0, cache.Size())
	
	// Verify items are gone
	_, exists := cache.Get("key1")
	assert.False(t, exists)
}

func TestLRUCache_LRUEviction(t *testing.T) {
	cache := NewLRUCache(100) // Small capacity
	
	// Fill cache to capacity
	_ = cache.SetWithSize("key1", "value1", 30)
	_ = cache.SetWithSize("key2", "value2", 30)
	_ = cache.SetWithSize("key3", "value3", 30)
	assert.Equal(t, 3, cache.Size())
	
	// Access key1 to make it most recently used
	cache.Get("key1")
	
	// Add another item that should evict key2 (least recently used)
	cache.SetWithSize("key4", "value4", 20)
	
	// key2 should be evicted
	_, exists := cache.Get("key2")
	assert.False(t, exists)
	
	// key1, key3, key4 should still exist
	_, exists = cache.Get("key1")
	assert.True(t, exists)
	_, exists = cache.Get("key3")
	assert.True(t, exists)
	_, exists = cache.Get("key4")
	assert.True(t, exists)
}

func TestLRUCache_CapacityExceeded(t *testing.T) {
	cache := NewLRUCache(50)
	
	// Try to add item larger than capacity
	err := cache.SetWithSize("large", "value", 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds cache capacity")
}

func TestLRUCache_EvictionCallback(t *testing.T) {
	cache := NewLRUCache(100)
	
	evictedKeys := make([]string, 0)
	cache.SetEvictionCallback(func(key string, value interface{}) {
		evictedKeys = append(evictedKeys, key)
	})
	
	// Fill cache to capacity
	cache.SetWithSize("key1", "value1", 40)
	cache.SetWithSize("key2", "value2", 40)
	cache.SetWithSize("key3", "value3", 40) // Should evict key1
	
	assert.Equal(t, []string{"key1"}, evictedKeys)
}

func TestLRUCache_Resize(t *testing.T) {
	cache := NewLRUCache(100)
	
	// Add items
	cache.SetWithSize("key1", "value1", 30)
	cache.SetWithSize("key2", "value2", 30)
	cache.SetWithSize("key3", "value3", 30)
	assert.Equal(t, 3, cache.Size())
	
	// Resize to smaller capacity
	err := cache.Resize(50)
	require.NoError(t, err)
	
	// Some items should be evicted
	assert.Less(t, cache.Size(), 3)
	assert.LessOrEqual(t, cache.MemoryUsage(), int64(50))
}

func TestLRUCache_ConcurrentAccess(t *testing.T) {
	cache := NewLRUCache(10000)
	numGoroutines := 10
	numOperations := 100
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Concurrent writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				cache.Set(key, value)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify some items exist (exact count may vary due to evictions)
	assert.Greater(t, cache.Size(), 0)
}

func TestLRUCache_EstimateSize(t *testing.T) {
	cache := NewLRUCache(1024)
	
	tests := []struct {
		name     string
		value    interface{}
		expected int64
	}{
		{"string", "hello", 5},
		{"bytes", []byte{1, 2, 3}, 3},
		{"int", 42, 8},
		{"float64", 3.14, 8},
		{"bool", true, 1},
		{"struct", struct{ name string }{name: "test"}, 64}, // default estimate
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := cache.estimateSize(tt.value)
			assert.Equal(t, tt.expected, size)
		})
	}
}

func TestLRUCache_Priority(t *testing.T) {
	cache := NewLRUCache(1024)
	
	assert.Equal(t, 1, cache.Priority())
	
	cache.SetPriority(5)
	assert.Equal(t, 5, cache.Priority())
}

func TestLRUCache_EvictOldest(t *testing.T) {
	cache := NewLRUCache(1024)
	
	// Test eviction on empty cache - should not error, just do nothing
	err := cache.EvictOldest(1)
	assert.NoError(t, err)
	
	// Add items
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")
	assert.Equal(t, 3, cache.Size())
	
	// Evict oldest
	err = cache.EvictOldest(2)
	require.NoError(t, err)
	assert.Equal(t, 1, cache.Size())
	
	// The most recently added should remain
	_, exists := cache.Get("key3")
	assert.True(t, exists)
}

func TestLRUCache_MoveToFront(t *testing.T) {
	cache := NewLRUCache(1024)
	
	// Add items in order
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")
	
	// Access key1 to move it to front
	cache.Get("key1")
	
	// Verify the order by checking the linked list
	// key1 should now be at the front (most recent)
	assert.Equal(t, "key1", cache.head.next.key)
}

// Benchmark LRU operations
func BenchmarkLRUCache_SetWithEviction(b *testing.B) {
	cache := NewLRUCache(1000) // Small cache to force evictions
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, "benchmark value")
	}
}

func BenchmarkLRUCache_GetHit(b *testing.B) {
	cache := NewLRUCache(10000)
	
	// Pre-populate
	for i := 0; i < 1000; i++ {
		cache.Set(fmt.Sprintf("key%d", i), "value")
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(fmt.Sprintf("key%d", i%1000))
	}
}

func BenchmarkLRUCache_GetMiss(b *testing.B) {
	cache := NewLRUCache(10000)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(fmt.Sprintf("nonexistent%d", i))
	}
}