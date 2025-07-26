package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/logging"
)

// LRUCache implements a Least Recently Used eviction policy cache
type LRUCache struct {
	capacity  int64
	size      int64
	items     map[string]*lruItem
	head      *lruItem
	tail      *lruItem
	mu        sync.RWMutex
	stats     CacheStats
	onEvicted EvictionCallback
	priority  int
}

// lruItem represents a node in the doubly-linked list
type lruItem struct {
	key        string
	value      interface{}
	size       int64
	accessTime time.Time
	createTime time.Time
	persistent bool // Never evict if true
	prev       *lruItem
	next       *lruItem
}

// NewLRUCache creates a new LRU cache with the specified capacity in bytes
func NewLRUCache(capacity int64) *LRUCache {
	c := &LRUCache{
		capacity: capacity,
		items:    make(map[string]*lruItem),
		priority: 1,
	}

	// Initialize sentinel nodes
	c.head = &lruItem{}
	c.tail = &lruItem{}
	c.head.next = c.tail
	c.tail.prev = c.head

	c.stats.MaxSize = capacity
	return c
}

// Get retrieves a value from the cache and marks it as recently used
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		c.stats.Misses++
		c.stats.UpdateHitRate()
		return nil, false
	}

	// Check if item has expired (but not persistent items)
	if !item.persistent {
		c.removeItem(item)
		c.stats.Misses++
		c.stats.UpdateHitRate()
		return nil, false
	}

	// Move to front (most recently used)
	c.moveToFront(item)
	item.accessTime = time.Now()

	c.stats.Hits++
	c.stats.UpdateHitRate()
	return item.value, true
}

// Set adds or updates a value in the cache
func (c *LRUCache) Set(key string, value interface{}) error {
	// Estimate size - simple heuristic for now
	size := c.estimateSize(value)
	return c.SetWithSize(key, value, size)
}

// SetWithSize adds or updates a value with explicit size
func (c *LRUCache) SetWithSize(key string, value interface{}, size int64) error {
	return c.SetWithOptions(key, value, size, false)
}

// SetWithOptions adds or updates a value with explicit options
func (c *LRUCache) SetWithOptions(key string, value interface{}, size int64, persistent bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Check if item already exists
	if item, exists := c.items[key]; exists {
		// Update existing item
		c.size = c.size - item.size + size
		item.value = value
		item.size = size
		item.accessTime = now
		item.persistent = persistent
		c.moveToFront(item)
		return nil
	}

	// Check capacity
	if size > c.capacity {
		return fmt.Errorf("item size %d exceeds cache capacity %d", size, c.capacity)
	}

	// Evict items if necessary
	for c.size+size > c.capacity && len(c.items) > 0 {
		if err := c.evictOldest(); err != nil {
			return fmt.Errorf("failed to evict item: %w", err)
		}
	}

	// Create new item
	item := &lruItem{
		key:        key,
		value:      value,
		size:       size,
		accessTime: now,
		createTime: now,
		persistent: persistent,
	}

	c.items[key] = item
	c.addToFront(item)
	c.size += size
	c.stats.Size = int64(len(c.items))

	return nil
}

// Delete removes an item from the cache
func (c *LRUCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return nil
	}

	c.removeItem(item)
	return nil
}

// Clear removes all items from the cache
func (c *LRUCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, item := range c.items {
		if c.onEvicted != nil {
			c.onEvicted(key, item.value)
		}
	}

	c.items = make(map[string]*lruItem)
	c.head.next = c.tail
	c.tail.prev = c.head
	c.size = 0
	c.stats.Size = 0

	return nil
}

// Size returns the number of items in the cache
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Stats returns cache statistics
func (c *LRUCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	stats.Size = int64(len(c.items))
	return stats
}

// Priority returns the cache priority for memory management
func (c *LRUCache) Priority() int {
	return c.priority
}

// SetPriority sets the cache priority
func (c *LRUCache) SetPriority(priority int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.priority = priority
}

// CanEvict returns true if the cache can evict items
func (c *LRUCache) CanEvict() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items) > 0
}

// EvictOldest removes the specified number of oldest items
func (c *LRUCache) EvictOldest(count int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < count && len(c.items) > 0; i++ {
		if err := c.evictOldest(); err != nil {
			// If we can't evict any more, just break instead of erroring
			break
		}
	}
	return nil
}

// MemoryUsage returns the current memory usage in bytes
func (c *LRUCache) MemoryUsage() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.size
}

// Resize changes the cache capacity
func (c *LRUCache) Resize(newCapacity int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.capacity = newCapacity
	c.stats.MaxSize = newCapacity

	// Evict items if over capacity
	for c.size > c.capacity && len(c.items) > 0 {
		if err := c.evictOldest(); err != nil {
			return err
		}
	}

	return nil
}

// SetEvictionCallback sets the callback function called when items are evicted
func (c *LRUCache) SetEvictionCallback(callback EvictionCallback) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEvicted = callback
}

// Private methods

func (c *LRUCache) moveToFront(item *lruItem) {
	c.removeFromList(item)
	c.addToFront(item)
}

func (c *LRUCache) addToFront(item *lruItem) {
	item.prev = c.head
	item.next = c.head.next
	c.head.next.prev = item
	c.head.next = item
}

func (c *LRUCache) removeFromList(item *lruItem) {
	item.prev.next = item.next
	item.next.prev = item.prev
}

func (c *LRUCache) removeItem(item *lruItem) {
	// Add debug logging for evicted items
	logging.LogDebugf("Evicting cache item: key=%s, size=%d, age=%v, persistent=%v",
		item.key, item.size, time.Since(item.createTime), item.persistent)

	delete(c.items, item.key)
	c.removeFromList(item)
	c.size -= item.size
	c.stats.Size = int64(len(c.items))

	if c.onEvicted != nil {
		c.onEvicted(item.key, item.value)
	}
}

func (c *LRUCache) evictOldest() error {
	if len(c.items) == 0 {
		return fmt.Errorf("cache is empty")
	}

	// Evict the oldest item regardless of persistent flag
	oldest := c.tail.prev
	if oldest != c.head {
		c.removeItem(oldest)
		c.stats.Evictions++
		return nil
	}

	return fmt.Errorf("no items to evict")
}

// estimateSize provides a rough size estimate for cache values
func (c *LRUCache) estimateSize(value interface{}) int64 {
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int, int32, int64, uint, uint32, uint64:
		return 8
	case float32, float64:
		return 8
	case bool:
		return 1
	default:
		// Default estimate for complex types
		return 64
	}
}
