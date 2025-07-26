package cache

import (
	"crypto/md5"
	"fmt"
	"sort"
	"time"

	"github.com/penwyp/ClawCat/models"
)

// ModelStats represents usage statistics for a single model
type ModelStats struct {
	EntryCount          int     `json:"entry_count"`          // Number of usage entries
	InputTokens         int     `json:"input_tokens"`         // Total input tokens
	OutputTokens        int     `json:"output_tokens"`        // Total output tokens
	CacheCreationTokens int     `json:"cache_creation_tokens"` // Cache creation tokens
	CacheReadTokens     int     `json:"cache_read_tokens"`    // Cache read tokens
	TotalTokens         int     `json:"total_tokens"`         // Sum of all token types
	TotalCost           float64 `json:"total_cost"`           // Total cost in USD
	Sessions            []string `json:"sessions"`            // Unique session IDs for this model
}

// HourlyAggregation represents usage data aggregated by hour (all models)
type HourlyAggregation struct {
	Timestamp   time.Time                    `json:"timestamp"`    // Hour timestamp (e.g., 2024-01-15T14:00:00Z)
	Models      map[string]*ModelStats       `json:"models"`       // Model name -> statistics
	TotalStats  *ModelStats                  `json:"total_stats"`  // Aggregated stats across all models
	Sessions    []string                     `json:"sessions"`     // Unique session IDs for this hour
	FirstEntry  time.Time                    `json:"first_entry"`  // Timestamp of first entry in this hour
	LastEntry   time.Time                    `json:"last_entry"`   // Timestamp of last entry in this hour
	UpdatedAt   time.Time                    `json:"updated_at"`   // When this aggregation was last updated
}

// DailyAggregation represents usage data aggregated by day (all models)
type DailyAggregation struct {
	Date            time.Time                    `json:"date"`             // Date (e.g., 2024-01-15T00:00:00Z)
	Models          map[string]*ModelStats       `json:"models"`           // Model name -> statistics
	TotalStats      *ModelStats                  `json:"total_stats"`      // Aggregated stats across all models
	Sessions        []string                     `json:"sessions"`         // Unique session IDs for the day
	HourlyBreakdown map[int]*HourlyAggregation   `json:"hourly_breakdown"` // Hour (0-23) -> HourlyAggregation
	FirstEntry      time.Time                    `json:"first_entry"`      // Timestamp of first entry in this day
	LastEntry       time.Time                    `json:"last_entry"`       // Timestamp of last entry in this day
	UpdatedAt       time.Time                    `json:"updated_at"`       // When this aggregation was last updated
}

// ModelSummary provides overall statistics for a model across all time
type ModelSummary struct {
	Model               string    `json:"model"`                // Model name
	TotalEntries        int       `json:"total_entries"`        // Total entries ever
	TotalInputTokens    int       `json:"total_input_tokens"`   // Total input tokens
	TotalOutputTokens   int       `json:"total_output_tokens"`  // Total output tokens
	TotalCacheTokens    int       `json:"total_cache_tokens"`   // Total cache tokens (creation + read)
	TotalTokens         int       `json:"total_tokens"`         // Sum of all token types
	TotalCost           float64   `json:"total_cost"`           // Total cost in USD
	UniqueSessions      int       `json:"unique_sessions"`      // Number of unique sessions
	FirstUsage          time.Time `json:"first_usage"`          // First time this model was used
	LastUsage           time.Time `json:"last_usage"`           // Last time this model was used
	DaysActive          int       `json:"days_active"`          // Number of days with usage
	AverageCostPerDay   float64   `json:"average_cost_per_day"` // Average cost per active day
	AverageTokensPerDay int       `json:"average_tokens_per_day"` // Average tokens per active day
	UpdatedAt           time.Time `json:"updated_at"`           // When this summary was last updated
}

// AggregationKey generates cache keys for different aggregation types
type AggregationKey struct{}

// HourlyKey generates a cache key for hourly aggregations
func (AggregationKey) HourlyKey(timestamp time.Time) string {
	// Format: hourly:2024-01-15-14
	return fmt.Sprintf("hourly:%s", 
		timestamp.UTC().Format("2006-01-02-15"))
}

// DailyKey generates a cache key for daily aggregations
func (AggregationKey) DailyKey(date time.Time) string {
	// Format: daily:2024-01-15
	return fmt.Sprintf("daily:%s", 
		date.UTC().Format("2006-01-02"))
}

// ModelSummaryKey generates a cache key for model summaries
func (AggregationKey) ModelSummaryKey(model string) string {
	// Format: model_summary:claude-3-sonnet
	return fmt.Sprintf("model_summary:%s", model)
}

// ModelsListKey generates a cache key for the list of all models
func (AggregationKey) ModelsListKey() string {
	return "models:list"
}

// FileMetadataKey generates a cache key for file metadata
func (AggregationKey) FileMetadataKey(filePath string) string {
	// Use MD5 hash of file path to avoid issues with special characters
	hash := fmt.Sprintf("%x", md5.Sum([]byte(filePath)))
	return fmt.Sprintf("file_meta:%s", hash)
}

// NewModelStats creates model statistics from usage entries
func NewModelStats(entries []models.UsageEntry) *ModelStats {
	if len(entries) == 0 {
		return &ModelStats{}
	}

	stats := &ModelStats{
		EntryCount: len(entries),
	}

	// Track unique sessions
	sessionSet := make(map[string]bool)

	// Aggregate all entries
	for _, entry := range entries {
		stats.InputTokens += entry.InputTokens
		stats.OutputTokens += entry.OutputTokens
		stats.CacheCreationTokens += entry.CacheCreationTokens
		stats.CacheReadTokens += entry.CacheReadTokens
		stats.TotalCost += entry.Cost

		// Track sessions
		if entry.SessionID != "" {
			sessionSet[entry.SessionID] = true
		}
	}

	// Calculate total tokens
	stats.TotalTokens = stats.InputTokens + stats.OutputTokens + stats.CacheCreationTokens + stats.CacheReadTokens

	// Convert session set to slice
	stats.Sessions = make([]string, 0, len(sessionSet))
	for session := range sessionSet {
		stats.Sessions = append(stats.Sessions, session)
	}
	sort.Strings(stats.Sessions)

	return stats
}

// NewHourlyAggregation creates a new hourly aggregation from usage entries
func NewHourlyAggregation(timestamp time.Time, entries []models.UsageEntry) *HourlyAggregation {
	if len(entries) == 0 {
		return nil
	}

	agg := &HourlyAggregation{
		Timestamp: timestamp.UTC().Truncate(time.Hour),
		Models:    make(map[string]*ModelStats),
		UpdatedAt: time.Now().UTC(),
	}

	// Track all sessions across all models
	allSessionSet := make(map[string]bool)
	
	// Initialize with first entry
	agg.FirstEntry = entries[0].Timestamp
	agg.LastEntry = entries[0].Timestamp

	// Group entries by model
	modelEntries := make(map[string][]models.UsageEntry)
	for _, entry := range entries {
		modelEntries[entry.Model] = append(modelEntries[entry.Model], entry)
		
		// Track all sessions
		if entry.SessionID != "" {
			allSessionSet[entry.SessionID] = true
		}
		
		// Update time range
		if entry.Timestamp.Before(agg.FirstEntry) {
			agg.FirstEntry = entry.Timestamp
		}
		if entry.Timestamp.After(agg.LastEntry) {
			agg.LastEntry = entry.Timestamp
		}
	}

	// Create stats for each model
	totalStats := &ModelStats{}
	for model, modelEntryList := range modelEntries {
		modelStats := NewModelStats(modelEntryList)
		agg.Models[model] = modelStats
		
		// Aggregate to total stats
		totalStats.EntryCount += modelStats.EntryCount
		totalStats.InputTokens += modelStats.InputTokens
		totalStats.OutputTokens += modelStats.OutputTokens
		totalStats.CacheCreationTokens += modelStats.CacheCreationTokens
		totalStats.CacheReadTokens += modelStats.CacheReadTokens
		totalStats.TotalTokens += modelStats.TotalTokens
		totalStats.TotalCost += modelStats.TotalCost
	}

	agg.TotalStats = totalStats

	// Convert session set to slice
	agg.Sessions = make([]string, 0, len(allSessionSet))
	for session := range allSessionSet {
		agg.Sessions = append(agg.Sessions, session)
	}
	sort.Strings(agg.Sessions)

	return agg
}

// NewDailyAggregation creates a new daily aggregation from hourly aggregations
func NewDailyAggregation(date time.Time, hourlyAggs []*HourlyAggregation) *DailyAggregation {
	if len(hourlyAggs) == 0 {
		return nil
	}

	agg := &DailyAggregation{
		Date:            date.UTC().Truncate(24 * time.Hour),
		Models:          make(map[string]*ModelStats),
		HourlyBreakdown: make(map[int]*HourlyAggregation),
		UpdatedAt:       time.Now().UTC(),
	}

	// Track unique sessions across all hours
	sessionSet := make(map[string]bool)
	
	// Initialize with first hourly aggregation
	agg.FirstEntry = hourlyAggs[0].FirstEntry
	agg.LastEntry = hourlyAggs[0].LastEntry

	// Aggregate model stats across all models
	modelStatsMap := make(map[string]*ModelStats)

	// Aggregate all hourly data
	for _, hourlyAgg := range hourlyAggs {
		// Store hourly breakdown
		hour := hourlyAgg.Timestamp.Hour()
		agg.HourlyBreakdown[hour] = hourlyAgg

		// Collect unique sessions
		for _, session := range hourlyAgg.Sessions {
			sessionSet[session] = true
		}

		// Aggregate model stats
		for model, stats := range hourlyAgg.Models {
			if modelStatsMap[model] == nil {
				modelStatsMap[model] = &ModelStats{}
			}
			
			ms := modelStatsMap[model]
			ms.EntryCount += stats.EntryCount
			ms.InputTokens += stats.InputTokens
			ms.OutputTokens += stats.OutputTokens
			ms.CacheCreationTokens += stats.CacheCreationTokens
			ms.CacheReadTokens += stats.CacheReadTokens
			ms.TotalTokens += stats.TotalTokens
			ms.TotalCost += stats.TotalCost
			
			// Merge sessions (could be optimized with set operations)
			sessionSet := make(map[string]bool)
			for _, s := range ms.Sessions {
				sessionSet[s] = true
			}
			for _, s := range stats.Sessions {
				sessionSet[s] = true
			}
			ms.Sessions = make([]string, 0, len(sessionSet))
			for s := range sessionSet {
				ms.Sessions = append(ms.Sessions, s)
			}
			sort.Strings(ms.Sessions)
		}

		// Update time range
		if hourlyAgg.FirstEntry.Before(agg.FirstEntry) {
			agg.FirstEntry = hourlyAgg.FirstEntry
		}
		if hourlyAgg.LastEntry.After(agg.LastEntry) {
			agg.LastEntry = hourlyAgg.LastEntry
		}
	}

	agg.Models = modelStatsMap

	// Calculate total stats
	totalStats := &ModelStats{}
	for _, modelStats := range modelStatsMap {
		totalStats.EntryCount += modelStats.EntryCount
		totalStats.InputTokens += modelStats.InputTokens
		totalStats.OutputTokens += modelStats.OutputTokens
		totalStats.CacheCreationTokens += modelStats.CacheCreationTokens
		totalStats.CacheReadTokens += modelStats.CacheReadTokens
		totalStats.TotalTokens += modelStats.TotalTokens
		totalStats.TotalCost += modelStats.TotalCost
	}
	agg.TotalStats = totalStats

	// Convert session set to slice
	agg.Sessions = make([]string, 0, len(sessionSet))
	for session := range sessionSet {
		agg.Sessions = append(agg.Sessions, session)
	}
	sort.Strings(agg.Sessions)

	return agg
}

// UpdateModelSummary updates a model summary with new daily aggregation data
func UpdateModelSummary(summary *ModelSummary, model string, dailyAgg *DailyAggregation) *ModelSummary {
	if summary == nil {
		summary = &ModelSummary{
			Model:      model,
			FirstUsage: dailyAgg.FirstEntry,
			LastUsage:  dailyAgg.LastEntry,
		}
	}

	// Get model stats from daily aggregation
	modelStats := dailyAgg.Models[model]
	if modelStats == nil {
		return summary // No data for this model in this day
	}

	// Update totals
	summary.TotalEntries += modelStats.EntryCount
	summary.TotalInputTokens += modelStats.InputTokens
	summary.TotalOutputTokens += modelStats.OutputTokens
	summary.TotalCacheTokens += modelStats.CacheCreationTokens + modelStats.CacheReadTokens
	summary.TotalTokens += modelStats.TotalTokens
	summary.TotalCost += modelStats.TotalCost

	// Update time range
	if dailyAgg.FirstEntry.Before(summary.FirstUsage) {
		summary.FirstUsage = dailyAgg.FirstEntry
	}
	if dailyAgg.LastEntry.After(summary.LastUsage) {
		summary.LastUsage = dailyAgg.LastEntry
	}

	// Count unique sessions across all time (this is an approximation)
	// For exact count, we'd need to track all sessions globally
	if len(modelStats.Sessions) > 0 {
		summary.UniqueSessions += len(modelStats.Sessions)
	}

	// Calculate days active (approximate based on time range)
	daysDiff := int(summary.LastUsage.Sub(summary.FirstUsage).Hours() / 24)
	if daysDiff < 1 {
		daysDiff = 1
	}
	summary.DaysActive = daysDiff

	// Calculate averages
	if summary.DaysActive > 0 {
		summary.AverageCostPerDay = summary.TotalCost / float64(summary.DaysActive)
		summary.AverageTokensPerDay = summary.TotalTokens / summary.DaysActive
	}

	summary.UpdatedAt = time.Now().UTC()
	return summary
}

// MergeHourlyAggregations merges multiple hourly aggregations for the same hour
func MergeHourlyAggregations(aggs []*HourlyAggregation) *HourlyAggregation {
	if len(aggs) == 0 {
		return nil
	}
	if len(aggs) == 1 {
		return aggs[0]
	}

	// Start with the first aggregation
	merged := &HourlyAggregation{
		Timestamp:  aggs[0].Timestamp,
		Models:     make(map[string]*ModelStats),
		FirstEntry: aggs[0].FirstEntry,
		LastEntry:  aggs[0].LastEntry,
		UpdatedAt:  time.Now().UTC(),
	}

	// Track unique sessions
	sessionSet := make(map[string]bool)

	// Merge all aggregations
	for _, agg := range aggs {
		// Collect unique sessions
		for _, session := range agg.Sessions {
			sessionSet[session] = true
		}

		// Merge model stats
		for model, stats := range agg.Models {
			if merged.Models[model] == nil {
				merged.Models[model] = &ModelStats{}
			}
			
			ms := merged.Models[model]
			ms.EntryCount += stats.EntryCount
			ms.InputTokens += stats.InputTokens
			ms.OutputTokens += stats.OutputTokens
			ms.CacheCreationTokens += stats.CacheCreationTokens
			ms.CacheReadTokens += stats.CacheReadTokens
			ms.TotalTokens += stats.TotalTokens
			ms.TotalCost += stats.TotalCost
			
			// Merge sessions
			modelSessionSet := make(map[string]bool)
			for _, s := range ms.Sessions {
				modelSessionSet[s] = true
			}
			for _, s := range stats.Sessions {
				modelSessionSet[s] = true
			}
			ms.Sessions = make([]string, 0, len(modelSessionSet))
			for s := range modelSessionSet {
				ms.Sessions = append(ms.Sessions, s)
			}
			sort.Strings(ms.Sessions)
		}

		// Update time range
		if agg.FirstEntry.Before(merged.FirstEntry) {
			merged.FirstEntry = agg.FirstEntry
		}
		if agg.LastEntry.After(merged.LastEntry) {
			merged.LastEntry = agg.LastEntry
		}
	}

	// Calculate total stats
	totalStats := &ModelStats{}
	for _, modelStats := range merged.Models {
		totalStats.EntryCount += modelStats.EntryCount
		totalStats.InputTokens += modelStats.InputTokens
		totalStats.OutputTokens += modelStats.OutputTokens
		totalStats.CacheCreationTokens += modelStats.CacheCreationTokens
		totalStats.CacheReadTokens += modelStats.CacheReadTokens
		totalStats.TotalTokens += modelStats.TotalTokens
		totalStats.TotalCost += modelStats.TotalCost
	}
	merged.TotalStats = totalStats

	// Convert session set to slice
	merged.Sessions = make([]string, 0, len(sessionSet))
	for session := range sessionSet {
		merged.Sessions = append(merged.Sessions, session)
	}
	sort.Strings(merged.Sessions)

	return merged
}

// GetHourlyTimestamp returns the hour timestamp for a given time
func GetHourlyTimestamp(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour)
}

// GetDailyTimestamp returns the daily timestamp for a given time
func GetDailyTimestamp(t time.Time) time.Time {
	return t.UTC().Truncate(24 * time.Hour)
}

// GroupEntriesByHour groups usage entries by hour (all models together)
func GroupEntriesByHour(entries []models.UsageEntry) map[string][]models.UsageEntry {
	groups := make(map[string][]models.UsageEntry)

	for _, entry := range entries {
		hourKey := GetHourlyTimestamp(entry.Timestamp).Format("2006-01-02-15")
		groups[hourKey] = append(groups[hourKey], entry)
	}

	return groups
}