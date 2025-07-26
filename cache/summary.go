package cache

import (
	"time"
)

// FileSummary represents a cached summary of a parsed usage file
type FileSummary struct {
	Path            string                          `json:"path"`
	AbsolutePath    string                          `json:"absolute_path"`
	ModTime         time.Time                       `json:"mod_time"`
	FileSize        int64                           `json:"file_size"`
	EntryCount      int                             `json:"entry_count"`
	TotalCost       float64                         `json:"total_cost"`
	TotalTokens     int                             `json:"total_tokens"`
	ModelStats      map[string]ModelStat            `json:"model_stats"`
	HourlyBuckets   map[string]*TemporalBucket      `json:"hourly_buckets"`   // Hour-level aggregations (key: "2006-01-02 15")
	DailyBuckets    map[string]*TemporalBucket      `json:"daily_buckets"`    // Day-level aggregations (key: "2006-01-02")
	ProcessedAt     time.Time                       `json:"processed_at"`
	Checksum        string                          `json:"checksum"`
	ProcessedHashes map[string]bool                 `json:"processed_hashes"` // For deduplication
}


// TemporalBucket represents aggregated usage data for a specific time period
type TemporalBucket struct {
	Period     string                     `json:"period"`      // The time period (e.g., "2006-01-02 15" for hour, "2006-01-02" for day)
	EntryCount int                        `json:"entry_count"`
	TotalCost  float64                    `json:"total_cost"`
	TotalTokens int                       `json:"total_tokens"`
	ModelStats map[string]*ModelStat      `json:"model_stats"` // Per-model statistics within this time bucket
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

// IsExpired checks if the summary is expired based on file modification time or size
func (fs *FileSummary) IsExpired(currentModTime time.Time, currentSize int64) bool {
	return !fs.ModTime.Equal(currentModTime) || fs.FileSize != currentSize
}


// MergeHashes merges processed hashes from summary into the target map
func (fs *FileSummary) MergeHashes(target map[string]bool) {
	for hash := range fs.ProcessedHashes {
		target[hash] = true
	}
}
