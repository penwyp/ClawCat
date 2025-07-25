package fileio

import (
	"github.com/penwyp/ClawCat/cache"
)

// StoreAdapter adapts cache.Store to implement CacheStore interface
type StoreAdapter struct {
	store *cache.Store
}

// NewStoreAdapter creates a new store adapter
func NewStoreAdapter(store *cache.Store) *StoreAdapter {
	return &StoreAdapter{store: store}
}

// GetFileSummary retrieves a cached file summary
func (sa *StoreAdapter) GetFileSummary(absolutePath string) (*FileSummary, error) {
	cacheSummary, err := sa.store.GetFileSummary(absolutePath)
	if err != nil {
		return nil, err
	}

	// Convert cache.FileSummary to fileio.FileSummary
	summary := &FileSummary{
		Path:         cacheSummary.Path,
		AbsolutePath: cacheSummary.AbsolutePath,
		ModTime:      cacheSummary.ModTime,
		FileSize:     cacheSummary.FileSize,
		EntryCount:   cacheSummary.EntryCount,
		TotalCost:    cacheSummary.TotalCost,
		TotalTokens:  cacheSummary.TotalTokens,
		DateRange: DateRange{
			Start: cacheSummary.DateRange.Start,
			End:   cacheSummary.DateRange.End,
		},
		ProcessedAt:     cacheSummary.ProcessedAt,
		Checksum:        cacheSummary.Checksum,
		ProcessedHashes: cacheSummary.ProcessedHashes,
		ModelStats:      make(map[string]ModelStat),
	}

	// Convert model stats
	for model, cacheStat := range cacheSummary.ModelStats {
		summary.ModelStats[model] = ModelStat{
			Model:               cacheStat.Model,
			EntryCount:          cacheStat.EntryCount,
			TotalCost:           cacheStat.TotalCost,
			InputTokens:         cacheStat.InputTokens,
			OutputTokens:        cacheStat.OutputTokens,
			CacheCreationTokens: cacheStat.CacheCreationTokens,
			CacheReadTokens:     cacheStat.CacheReadTokens,
		}
	}

	return summary, nil
}

// SetFileSummary stores a file summary in cache
func (sa *StoreAdapter) SetFileSummary(summary *FileSummary) error {
	// Convert fileio.FileSummary to cache.FileSummary
	cacheSummary := &cache.FileSummary{
		Path:         summary.Path,
		AbsolutePath: summary.AbsolutePath,
		ModTime:      summary.ModTime,
		FileSize:     summary.FileSize,
		EntryCount:   summary.EntryCount,
		TotalCost:    summary.TotalCost,
		TotalTokens:  summary.TotalTokens,
		DateRange: cache.DateRange{
			Start: summary.DateRange.Start,
			End:   summary.DateRange.End,
		},
		ProcessedAt:     summary.ProcessedAt,
		Checksum:        summary.Checksum,
		ProcessedHashes: summary.ProcessedHashes,
		ModelStats:      make(map[string]cache.ModelStat),
	}

	// Convert model stats
	for model, stat := range summary.ModelStats {
		cacheSummary.ModelStats[model] = cache.ModelStat{
			Model:               stat.Model,
			EntryCount:          stat.EntryCount,
			TotalCost:           stat.TotalCost,
			InputTokens:         stat.InputTokens,
			OutputTokens:        stat.OutputTokens,
			CacheCreationTokens: stat.CacheCreationTokens,
			CacheReadTokens:     stat.CacheReadTokens,
		}
	}

	return sa.store.SetFileSummary(cacheSummary)
}

// HasFileSummary checks if a file summary exists in cache
func (sa *StoreAdapter) HasFileSummary(absolutePath string) bool {
	return sa.store.HasFileSummary(absolutePath)
}

// InvalidateFileSummary removes a file summary from cache
func (sa *StoreAdapter) InvalidateFileSummary(absolutePath string) error {
	return sa.store.InvalidateFileSummary(absolutePath)
}
