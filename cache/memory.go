package cache

import (
	"fmt"
	"sort"
	"sync"
)

// MemoryManager manages memory allocation across multiple caches
type MemoryManager struct {
	maxMemory    int64
	currentUsage int64
	caches       []ManagedCache
	mu           sync.RWMutex
}

// MemoryStats provides memory usage statistics
type MemoryStats struct {
	CurrentUsage int64 `json:"current_usage"`
	MaxMemory    int64 `json:"max_memory"`
	CacheCount   int   `json:"cache_count"`
}

// NewMemoryManager creates a new memory manager with the specified limit
func NewMemoryManager(maxMemory int64) *MemoryManager {
	return &MemoryManager{
		maxMemory:    maxMemory,
		currentUsage: 0,
		caches:       make([]ManagedCache, 0),
	}
}

// Register adds a cache to be managed
func (m *MemoryManager) Register(cache ManagedCache) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if cache is already registered
	for _, c := range m.caches {
		if c == cache {
			return fmt.Errorf("cache already registered")
		}
	}

	m.caches = append(m.caches, cache)
	m.updateUsage()

	return nil
}

// Unregister removes a cache from management
func (m *MemoryManager) Unregister(cache ManagedCache) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, c := range m.caches {
		if c == cache {
			// Remove from slice
			m.caches = append(m.caches[:i], m.caches[i+1:]...)
			m.updateUsage()
			return nil
		}
	}

	return fmt.Errorf("cache not found")
}

// AllocateMemory attempts to allocate the specified amount of memory
func (m *MemoryManager) AllocateMemory(size int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentUsage+size <= m.maxMemory {
		return nil // Allocation fits within current limits
	}

	// Need to free memory
	needed := (m.currentUsage + size) - m.maxMemory
	freed, err := m.freeMemory(needed)
	if err != nil {
		return fmt.Errorf("failed to free memory: %w", err)
	}

	if freed < needed {
		return fmt.Errorf("insufficient memory: needed %d, freed %d", needed, freed)
	}

	return nil
}

// ReleaseMemory releases the specified amount of memory
func (m *MemoryManager) ReleaseMemory(size int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updateUsage()
}

// Rebalance redistributes memory among caches based on priority
func (m *MemoryManager) Rebalance() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updateUsage()

	if m.currentUsage <= m.maxMemory {
		return nil // No rebalancing needed
	}

	// Need to evict some data
	excess := m.currentUsage - m.maxMemory
	return m.evictExcess(excess)
}

// SetMaxMemory updates the maximum memory limit
func (m *MemoryManager) SetMaxMemory(maxMemory int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldMax := m.maxMemory
	m.maxMemory = maxMemory

	// If reducing memory, need to evict
	if maxMemory < oldMax && m.currentUsage > maxMemory {
		excess := m.currentUsage - maxMemory
		return m.evictExcess(excess)
	}

	return nil
}

// Stats returns current memory statistics
func (m *MemoryManager) Stats() MemoryStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.updateUsage()

	return MemoryStats{
		CurrentUsage: m.currentUsage,
		MaxMemory:    m.maxMemory,
		CacheCount:   len(m.caches),
	}
}

// GetCaches returns a copy of the managed caches slice
func (m *MemoryManager) GetCaches() []ManagedCache {
	m.mu.RLock()
	defer m.mu.RUnlock()

	caches := make([]ManagedCache, len(m.caches))
	copy(caches, m.caches)
	return caches
}

// Private methods

// updateUsage recalculates current memory usage across all caches
func (m *MemoryManager) updateUsage() {
	total := int64(0)
	for _, cache := range m.caches {
		total += cache.MemoryUsage()
	}
	m.currentUsage = total
}

// freeMemory attempts to free the specified amount of memory
func (m *MemoryManager) freeMemory(needed int64) (int64, error) {
	// Sort caches by priority (lower priority first)
	sortedCaches := make([]ManagedCache, len(m.caches))
	copy(sortedCaches, m.caches)

	sort.Slice(sortedCaches, func(i, j int) bool {
		return sortedCaches[i].Priority() < sortedCaches[j].Priority()
	})

	freed := int64(0)

	for _, cache := range sortedCaches {
		if freed >= needed {
			break
		}

		if !cache.CanEvict() {
			continue
		}

		// Calculate how much to evict from this cache
		remaining := needed - freed
		currentUsage := cache.MemoryUsage()

		// Try to evict up to 50% of cache or what's needed, whichever is less
		maxEvict := currentUsage / 2
		toEvict := remaining
		if toEvict > maxEvict {
			toEvict = maxEvict
		}

		// Estimate items to evict (rough heuristic)
		avgItemSize := currentUsage / int64(cache.Size())
		if avgItemSize == 0 {
			avgItemSize = 1024 // Default estimate
		}
		itemsToEvict := int(toEvict / avgItemSize)
		if itemsToEvict == 0 {
			itemsToEvict = 1
		}

		beforeUsage := cache.MemoryUsage()
		if err := cache.EvictOldest(itemsToEvict); err != nil {
			continue
		}
		afterUsage := cache.MemoryUsage()

		freed += beforeUsage - afterUsage
	}

	m.updateUsage()
	return freed, nil
}

// evictExcess evicts data to bring usage below maximum
func (m *MemoryManager) evictExcess(excess int64) error {
	freed, err := m.freeMemory(excess)
	if err != nil {
		return err
	}

	if freed < excess {
		return fmt.Errorf("could not free enough memory: needed %d, freed %d", excess, freed)
	}

	return nil
}

// GetMemoryUsage returns current memory usage for debugging
func (m *MemoryManager) GetMemoryUsage() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.updateUsage()
	return m.currentUsage
}

// GetMaxMemory returns the maximum memory limit
func (m *MemoryManager) GetMaxMemory() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxMemory
}

// IsOverLimit checks if current usage exceeds the limit
func (m *MemoryManager) IsOverLimit() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.updateUsage()
	return m.currentUsage > m.maxMemory
}

// GetUsagePercentage returns memory usage as a percentage
func (m *MemoryManager) GetUsagePercentage() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.updateUsage()
	if m.maxMemory == 0 {
		return 0.0
	}

	return float64(m.currentUsage) / float64(m.maxMemory) * 100.0
}
