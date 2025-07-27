package calculations

import (
	"time"
)

// StatsAggregator provides flexible aggregation for various time periods
type StatsAggregator struct {
	timezone   *time.Location
	calculator *CostCalculator
}

// PeriodStats represents aggregated statistics for a time period
type PeriodStats struct {
	Period              string                `json:"period"`
	StartTime           time.Time             `json:"start_time"`
	EndTime             time.Time             `json:"end_time"`
	InputTokens         int                   `json:"input_tokens"`
	OutputTokens        int                   `json:"output_tokens"`
	CacheCreationTokens int                   `json:"cache_creation_tokens"`
	CacheReadTokens     int                   `json:"cache_read_tokens"`
	TotalTokens         int                   `json:"total_tokens"`
	TotalCost           float64               `json:"total_cost"`
	EntryCount          int                   `json:"entry_count"`
	ModelBreakdown      map[string]ModelStats `json:"model_breakdown"`
	AverageCostPerToken float64               `json:"average_cost_per_token"`
	AverageCostPerEntry float64               `json:"average_cost_per_entry"`
}

// ModelStats represents statistics for a specific model
type ModelStats struct {
	Model               string  `json:"model"`
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	TotalTokens         int     `json:"total_tokens"`
	TotalCost           float64 `json:"total_cost"`
	EntryCount          int     `json:"entry_count"`
	Percentage          float64 `json:"percentage"`
	AverageCostPerToken float64 `json:"average_cost_per_token"`
}

// OverallStats represents overall statistics across all periods
type OverallStats struct {
	TotalTokens        int                   `json:"total_tokens"`
	TotalCost          float64               `json:"total_cost"`
	TotalEntries       int                   `json:"total_entries"`
	UniqueDays         int                   `json:"unique_days"`
	AverageDailyCost   float64               `json:"average_daily_cost"`
	AverageDailyTokens int                   `json:"average_daily_tokens"`
	ModelDistribution  map[string]ModelStats `json:"model_distribution"`
	TimeRange          TimeRange             `json:"time_range"`
	PeakUsageHour      int                   `json:"peak_usage_hour"`
	CacheUtilization   CacheStats            `json:"cache_utilization"`
}

// TimeRange represents a time range
type TimeRange struct {
	Start    time.Time     `json:"start"`
	End      time.Time     `json:"end"`
	Duration time.Duration `json:"duration"`
}

// CacheStats represents cache utilization statistics
type CacheStats struct {
	TotalCacheTokens    int     `json:"total_cache_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	CacheHitRate        float64 `json:"cache_hit_rate"`
	CacheSavingsPercent float64 `json:"cache_savings_percent"`
	CacheSavingsCost    float64 `json:"cache_savings_cost"`
}
