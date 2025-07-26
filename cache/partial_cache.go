package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/models"
)

// PartialFileCache provides caching for partial file contents based on time ranges
type PartialFileCache struct {
	cache      *LRUCache
	mu         sync.RWMutex
	serializer Serializer
}

// TimeRangeEntry represents cached entries for a specific time range
type TimeRangeEntry struct {
	Path       string              `json:"path"`
	StartTime  time.Time           `json:"start_time"`
	EndTime    time.Time           `json:"end_time"`
	Entries    []models.UsageEntry `json:"entries"`
	TotalCount int                 `json:"total_count"`
	CachedAt   time.Time           `json:"cached_at"`
}

// NewPartialFileCache creates a new partial file cache
func NewPartialFileCache(maxSize int64) *PartialFileCache {
	cache := NewLRUCache(maxSize)
	cache.SetPriority(3) // Higher priority than regular file cache

	return &PartialFileCache{
		cache:      cache,
		serializer: NewSonicSerializer(),
	}
}

// GetEntriesInRange retrieves cached entries for a specific time range
func (p *PartialFileCache) GetEntriesInRange(path string, startTime, endTime time.Time) ([]models.UsageEntry, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Generate cache key based on path and time range
	key := p.generateRangeKey(path, startTime, endTime)

	value, exists := p.cache.Get(key)
	if !exists {
		return nil, false
	}

	rangeEntry, ok := value.(*TimeRangeEntry)
	if !ok {
		// Invalid cache entry
		_ = p.cache.Delete(key)
		return nil, false
	}

	// Check if the cached range covers our requested range
	if rangeEntry.StartTime.After(startTime) || rangeEntry.EndTime.Before(endTime) {
		// Cached range doesn't fully cover requested range
		return nil, false
	}

	// Filter entries to match exact requested range
	var filteredEntries []models.UsageEntry
	for _, entry := range rangeEntry.Entries {
		if !entry.Timestamp.Before(startTime) && !entry.Timestamp.After(endTime) {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	return filteredEntries, true
}

// SetEntriesInRange caches entries for a specific time range
func (p *PartialFileCache) SetEntriesInRange(path string, entries []models.UsageEntry, startTime, endTime time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(entries) == 0 {
		return nil
	}

	rangeEntry := &TimeRangeEntry{
		Path:       path,
		StartTime:  startTime,
		EndTime:    endTime,
		Entries:    entries,
		TotalCount: len(entries),
		CachedAt:   time.Now(),
	}

	key := p.generateRangeKey(path, startTime, endTime)
	size := p.estimateSize(rangeEntry)

	return p.cache.SetWithSize(key, rangeEntry, size)
}

// GetOverlappingRanges finds cached ranges that overlap with the requested range
func (p *PartialFileCache) GetOverlappingRanges(path string, startTime, endTime time.Time) ([]*TimeRangeEntry, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var overlapping []*TimeRangeEntry

	// This is a simplified implementation
	// In production, we'd maintain an index of ranges for efficient lookup
	// For now, we'll check a few common range patterns

	// Check daily ranges
	for t := startTime.Truncate(24 * time.Hour); !t.After(endTime); t = t.Add(24 * time.Hour) {
		dayEnd := t.Add(24 * time.Hour)
		key := p.generateRangeKey(path, t, dayEnd)
		
		if value, exists := p.cache.Get(key); exists {
			if rangeEntry, ok := value.(*TimeRangeEntry); ok {
				overlapping = append(overlapping, rangeEntry)
			}
		}
	}

	// Check hourly ranges for recent data
	recentStart := time.Now().Add(-24 * time.Hour)
	if startTime.After(recentStart) {
		for t := startTime.Truncate(time.Hour); !t.After(endTime); t = t.Add(time.Hour) {
			hourEnd := t.Add(time.Hour)
			key := p.generateRangeKey(path, t, hourEnd)
			
			if value, exists := p.cache.Get(key); exists {
				if rangeEntry, ok := value.(*TimeRangeEntry); ok {
					overlapping = append(overlapping, rangeEntry)
				}
			}
		}
	}

	return overlapping, nil
}

// MergeEntries combines entries from multiple ranges and new data
func (p *PartialFileCache) MergeEntries(cachedRanges []*TimeRangeEntry, newEntries []models.UsageEntry, startTime, endTime time.Time) []models.UsageEntry {
	// Use a map to deduplicate entries
	entryMap := make(map[string]models.UsageEntry)

	// Add cached entries
	for _, rangeEntry := range cachedRanges {
		for _, entry := range rangeEntry.Entries {
			if !entry.Timestamp.Before(startTime) && !entry.Timestamp.After(endTime) {
				key := fmt.Sprintf("%s-%s-%d", entry.Timestamp.Format(time.RFC3339), entry.Model, entry.TotalTokens)
				entryMap[key] = entry
			}
		}
	}

	// Add new entries (will override cached if duplicate)
	for _, entry := range newEntries {
		if !entry.Timestamp.Before(startTime) && !entry.Timestamp.After(endTime) {
			key := fmt.Sprintf("%s-%s-%d", entry.Timestamp.Format(time.RFC3339), entry.Model, entry.TotalTokens)
			entryMap[key] = entry
		}
	}

	// Convert map back to slice
	result := make([]models.UsageEntry, 0, len(entryMap))
	for _, entry := range entryMap {
		result = append(result, entry)
	}

	return result
}

// InvalidateRange removes cached data for a specific time range
func (p *PartialFileCache) InvalidateRange(path string, startTime, endTime time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := p.generateRangeKey(path, startTime, endTime)
	return p.cache.Delete(key)
}

// InvalidateFile removes all cached ranges for a file
func (p *PartialFileCache) InvalidateFile(path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// In a production implementation, we'd maintain an index
	// For now, this is a no-op as we can't efficiently find all ranges
	// The ranges will naturally expire based on LRU policy
	return nil
}

// generateRangeKey creates a cache key for a time range
func (p *PartialFileCache) generateRangeKey(path string, startTime, endTime time.Time) string {
	// Use hourly granularity for recent data, daily for older data
	duration := endTime.Sub(startTime)
	
	if duration <= time.Hour {
		// Hourly range
		return fmt.Sprintf("partial:%s:%s:hour", path, startTime.Truncate(time.Hour).Format("2006-01-02T15"))
	} else if duration <= 24*time.Hour {
		// Daily range
		return fmt.Sprintf("partial:%s:%s:day", path, startTime.Truncate(24*time.Hour).Format("2006-01-02"))
	} else {
		// Custom range
		return fmt.Sprintf("partial:%s:%s:%s", path, startTime.Format("2006-01-02T15"), endTime.Format("2006-01-02T15"))
	}
}

// estimateSize calculates the approximate memory size of a range entry
func (p *PartialFileCache) estimateSize(entry *TimeRangeEntry) int64 {
	size := int64(len(entry.Path))
	size += 8 * 3 // Three time fields
	size += int64(len(entry.Entries)) * 200 // Rough estimate per entry
	size += 100 // Overhead
	return size
}

// Stats returns partial cache statistics
func (p *PartialFileCache) Stats() CacheStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cache.Stats()
}