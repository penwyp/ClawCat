package cache

import (
	"time"
)

// FileSummary represents a cached summary of a parsed usage file
type FileSummary struct {
	Path            string               `json:"path"`
	AbsolutePath    string               `json:"absolute_path"`
	ModTime         time.Time            `json:"mod_time"`
	FileSize        int64                `json:"file_size"`
	EntryCount      int                  `json:"entry_count"`
	TotalCost       float64              `json:"total_cost"`
	TotalTokens     int                  `json:"total_tokens"`
	DateRange       DateRange            `json:"date_range"`
	ModelStats      map[string]ModelStat `json:"model_stats"`
	ProcessedAt     time.Time            `json:"processed_at"`
	Checksum        string               `json:"checksum"`
	ProcessedHashes map[string]bool      `json:"processed_hashes"` // For deduplication
}

// DateRange represents the time range of entries in a file
type DateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ModelStat contains statistics for a specific model
type ModelStat struct {
	Model               string  `json:"model"`
	EntryCount          int     `json:"entry_count"`
	TotalCost           float64 `json:"total_cost"`
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
}

// IsExpired checks if the summary is expired based on file modification time
func (fs *FileSummary) IsExpired(currentModTime time.Time) bool {
	return !fs.ModTime.Equal(currentModTime)
}

// ShouldUseCache determines if cache should be used based on time since last modification
func (fs *FileSummary) ShouldUseCache(currentModTime time.Time, cacheThreshold time.Duration) bool {
	// If file has been modified, don't use cache
	if fs.IsExpired(currentModTime) {
		return false
	}

	// If file hasn't been modified for longer than threshold, use cache
	timeSinceModification := time.Since(currentModTime)
	return timeSinceModification >= cacheThreshold
}

// MergeHashes merges processed hashes from summary into the target map
func (fs *FileSummary) MergeHashes(target map[string]bool) {
	for hash := range fs.ProcessedHashes {
		target[hash] = true
	}
}
