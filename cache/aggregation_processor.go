package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/models"
)

// AggregationProcessor handles the processing and caching of usage data aggregations
type AggregationProcessor struct {
	cache   *BadgerCache
	keyGen  AggregationKey
	mu      sync.RWMutex
	metrics ProcessorMetrics
}

// ProcessorMetrics tracks aggregation processor performance
type ProcessorMetrics struct {
	FilesProcessed     int64 `json:"files_processed"`
	EntriesProcessed   int64 `json:"entries_processed"`
	HourlyAggregations int64 `json:"hourly_aggregations"`
	DailyAggregations  int64 `json:"daily_aggregations"`
	ProcessingErrors   int64 `json:"processing_errors"`
	ProcessingTimeMs   int64 `json:"processing_time_ms"`
}

// NewAggregationProcessor creates a new aggregation processor
func NewAggregationProcessor(cache *BadgerCache) *AggregationProcessor {
	return &AggregationProcessor{
		cache:  cache,
		keyGen: AggregationKey{},
	}
}

// ProcessFile processes usage entries from a file and updates aggregations
func (ap *AggregationProcessor) ProcessFile(filePath string, entries []models.UsageEntry) error {
	start := time.Now()
	ap.mu.Lock()
	defer ap.mu.Unlock()

	defer func() {
		ap.metrics.FilesProcessed++
		ap.metrics.EntriesProcessed += int64(len(entries))
		ap.metrics.ProcessingTimeMs += time.Since(start).Milliseconds()
	}()

	if len(entries) == 0 {
		return nil
	}

	// Group entries by hour
	hourlyGroups := GroupEntriesByHour(entries)

	// Process each hour's data
	for hourKey, hourEntries := range hourlyGroups {
		if err := ap.processHourlyEntries(hourKey, hourEntries); err != nil {
			ap.metrics.ProcessingErrors++
			return fmt.Errorf("failed to process hourly entries for %s: %w", hourKey, err)
		}
		ap.metrics.HourlyAggregations++
	}

	// Update daily aggregations based on hourly data
	if err := ap.updateDailyAggregations(entries); err != nil {
		ap.metrics.ProcessingErrors++
		return fmt.Errorf("failed to update daily aggregations: %w", err)
	}

	// Update model summaries
	if err := ap.updateModelSummaries(entries); err != nil {
		ap.metrics.ProcessingErrors++
		return fmt.Errorf("failed to update model summaries: %w", err)
	}

	// Update models list
	if err := ap.updateModelsList(entries); err != nil {
		ap.metrics.ProcessingErrors++
		return fmt.Errorf("failed to update models list: %w", err)
	}

	return nil
}

// processHourlyEntries processes entries for a specific hour
func (ap *AggregationProcessor) processHourlyEntries(hourKey string, entries []models.UsageEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Parse timestamp from hour key
	timestamp, err := time.Parse("2006-01-02-15", hourKey)
	if err != nil {
		return fmt.Errorf("failed to parse hour key %s: %w", hourKey, err)
	}

	// Create new hourly aggregation
	newAgg := NewHourlyAggregation(timestamp, entries)
	if newAgg == nil {
		return nil
	}

	// Generate cache key
	cacheKey := ap.keyGen.HourlyKey(timestamp)

	// Check if aggregation already exists
	if existingData, exists := ap.cache.Get(cacheKey); exists {
		if existingAgg, ok := existingData.(*HourlyAggregation); ok {
			// Merge with existing aggregation
			merged := MergeHourlyAggregations([]*HourlyAggregation{existingAgg, newAgg})
			newAgg = merged
		}
	}

	// Store updated aggregation
	return ap.cache.Set(cacheKey, newAgg)
}

// updateDailyAggregations updates daily aggregations based on new entries
func (ap *AggregationProcessor) updateDailyAggregations(entries []models.UsageEntry) error {
	// Group entries by date
	dateGroups := make(map[string][]models.UsageEntry)
	for _, entry := range entries {
		dateKey := GetDailyTimestamp(entry.Timestamp).Format("2006-01-02")
		dateGroups[dateKey] = append(dateGroups[dateKey], entry)
	}

	// Process each date
	for dateKey, dateEntries := range dateGroups {
		if err := ap.processDailyEntries(dateKey, dateEntries); err != nil {
			return fmt.Errorf("failed to process daily entries for %s: %w", dateKey, err)
		}
		ap.metrics.DailyAggregations++
	}

	return nil
}

// processDailyEntries processes entries for a specific date
func (ap *AggregationProcessor) processDailyEntries(dateKey string, entries []models.UsageEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Parse date from key
	date, err := time.Parse("2006-01-02", dateKey)
	if err != nil {
		return fmt.Errorf("failed to parse date key %s: %w", dateKey, err)
	}

	// Generate cache key
	cacheKey := ap.keyGen.DailyKey(date)

	// Get all hourly aggregations for this date
	hourlyAggs := make([]*HourlyAggregation, 0, 24)
	for hour := 0; hour < 24; hour++ {
		hourTimestamp := date.Add(time.Duration(hour) * time.Hour)
		hourKey := ap.keyGen.HourlyKey(hourTimestamp)
		
		if hourlyData, exists := ap.cache.Get(hourKey); exists {
			if hourlyAgg, ok := hourlyData.(*HourlyAggregation); ok {
				hourlyAggs = append(hourlyAggs, hourlyAgg)
			}
		}
	}

	// Create daily aggregation from hourly aggregations
	dailyAgg := NewDailyAggregation(date, hourlyAggs)
	if dailyAgg == nil {
		return nil
	}

	// Store daily aggregation
	return ap.cache.Set(cacheKey, dailyAgg)
}

// updateModelSummaries updates model summaries based on new entries
func (ap *AggregationProcessor) updateModelSummaries(entries []models.UsageEntry) error {
	// Get unique models from entries
	modelSet := make(map[string]bool)
	for _, entry := range entries {
		modelSet[entry.Model] = true
	}

	// Group entries by date to update summaries incrementally
	dateGroups := make(map[string][]models.UsageEntry)
	for _, entry := range entries {
		dateKey := GetDailyTimestamp(entry.Timestamp).Format("2006-01-02")
		dateGroups[dateKey] = append(dateGroups[dateKey], entry)
	}

	// Update each model's summary
	for model := range modelSet {
		summaryKey := ap.keyGen.ModelSummaryKey(model)
		
		// Get existing summary
		var existingSummary *ModelSummary
		if summaryData, exists := ap.cache.Get(summaryKey); exists {
			if summary, ok := summaryData.(*ModelSummary); ok {
				existingSummary = summary
			}
		}

		// Update summary with daily aggregations
		for dateKey := range dateGroups {
			date, _ := time.Parse("2006-01-02", dateKey)
			dailyKey := ap.keyGen.DailyKey(date)
			
			if dailyData, exists := ap.cache.Get(dailyKey); exists {
				if dailyAgg, ok := dailyData.(*DailyAggregation); ok {
					existingSummary = UpdateModelSummary(existingSummary, model, dailyAgg)
				}
			}
		}

		// Store updated summary
		if existingSummary != nil {
			if err := ap.cache.Set(summaryKey, existingSummary); err != nil {
				return fmt.Errorf("failed to store model summary for %s: %w", model, err)
			}
		}
	}

	return nil
}

// updateModelsList updates the list of all models
func (ap *AggregationProcessor) updateModelsList(entries []models.UsageEntry) error {
	// Get current models list
	modelsKey := ap.keyGen.ModelsListKey()
	var existingModels []string
	
	if modelsData, exists := ap.cache.Get(modelsKey); exists {
		if models, ok := modelsData.([]string); ok {
			existingModels = models
		}
	}

	// Create set of existing models
	modelSet := make(map[string]bool)
	for _, model := range existingModels {
		modelSet[model] = true
	}

	// Add new models
	updated := false
	for _, entry := range entries {
		if !modelSet[entry.Model] {
			modelSet[entry.Model] = true
			updated = true
		}
	}

	// Update list if new models were found
	if updated {
		newModelsList := make([]string, 0, len(modelSet))
		for model := range modelSet {
			newModelsList = append(newModelsList, model)
		}
		
		return ap.cache.Set(modelsKey, newModelsList)
	}

	return nil
}

// GetHourlyAggregation retrieves hourly aggregation for a specific timestamp
func (ap *AggregationProcessor) GetHourlyAggregation(timestamp time.Time) (*HourlyAggregation, error) {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	key := ap.keyGen.HourlyKey(timestamp)
	if data, exists := ap.cache.Get(key); exists {
		if agg, ok := data.(*HourlyAggregation); ok {
			return agg, nil
		}
	}

	return nil, fmt.Errorf("hourly aggregation not found for %s", timestamp.Format("2006-01-02-15"))
}

// GetDailyAggregation retrieves daily aggregation for a specific date
func (ap *AggregationProcessor) GetDailyAggregation(date time.Time) (*DailyAggregation, error) {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	key := ap.keyGen.DailyKey(date)
	if data, exists := ap.cache.Get(key); exists {
		if agg, ok := data.(*DailyAggregation); ok {
			return agg, nil
		}
	}

	return nil, fmt.Errorf("daily aggregation not found for %s", date.Format("2006-01-02"))
}

// GetModelSummary retrieves model summary for a specific model
func (ap *AggregationProcessor) GetModelSummary(model string) (*ModelSummary, error) {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	key := ap.keyGen.ModelSummaryKey(model)
	if data, exists := ap.cache.Get(key); exists {
		if summary, ok := data.(*ModelSummary); ok {
			return summary, nil
		}
	}

	return nil, fmt.Errorf("model summary not found for %s", model)
}

// GetModelsList retrieves the list of all models
func (ap *AggregationProcessor) GetModelsList() ([]string, error) {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	key := ap.keyGen.ModelsListKey()
	if data, exists := ap.cache.Get(key); exists {
		if models, ok := data.([]string); ok {
			return models, nil
		}
	}

	return []string{}, nil // Return empty list if not found
}

// GetHourlyRange retrieves hourly aggregations for a time range
func (ap *AggregationProcessor) GetHourlyRange(start, end time.Time) ([]*HourlyAggregation, error) {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	var aggregations []*HourlyAggregation
	
	// Iterate through each hour in the range
	current := GetHourlyTimestamp(start)
	endHour := GetHourlyTimestamp(end)
	
	for current.Before(endHour) || current.Equal(endHour) {
		if agg, err := ap.GetHourlyAggregation(current); err == nil {
			aggregations = append(aggregations, agg)
		}
		current = current.Add(time.Hour)
	}

	return aggregations, nil
}

// GetDailyRange retrieves daily aggregations for a date range
func (ap *AggregationProcessor) GetDailyRange(start, end time.Time) ([]*DailyAggregation, error) {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	var aggregations []*DailyAggregation
	
	// Iterate through each day in the range
	current := GetDailyTimestamp(start)
	endDay := GetDailyTimestamp(end)
	
	for current.Before(endDay) || current.Equal(endDay) {
		if agg, err := ap.GetDailyAggregation(current); err == nil {
			aggregations = append(aggregations, agg)
		}
		current = current.Add(24 * time.Hour)
	}

	return aggregations, nil
}

// GetMetrics returns processor metrics
func (ap *AggregationProcessor) GetMetrics() ProcessorMetrics {
	ap.mu.RLock()
	defer ap.mu.RUnlock()
	return ap.metrics
}

// Clear removes all aggregation data from cache
func (ap *AggregationProcessor) Clear() error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	return ap.cache.Clear()
}

// Close closes the aggregation processor
func (ap *AggregationProcessor) Close() error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	return ap.cache.Close()
}