package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/models"
)

// EnhancedStore provides a high-performance cache store with pre-aggregated data
type EnhancedStore struct {
	badgerCache *BadgerCache
	processor   *AggregationProcessor
	config      EnhancedStoreConfig
	mu          sync.RWMutex
	closed      bool
}

// EnhancedStoreConfig configures the enhanced cache store
type EnhancedStoreConfig struct {
	BadgerConfig   BadgerConfig  `json:"badger_config"`
	EnableMetrics  bool          `json:"enable_metrics"`
	AutoCleanup    bool          `json:"auto_cleanup"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
}

// EnhancedStoreStats provides comprehensive store statistics
type EnhancedStoreStats struct {
	BadgerStats       BadgerStats       `json:"badger_stats"`
	ProcessorMetrics  ProcessorMetrics  `json:"processor_metrics"`
	TotalModels       int               `json:"total_models"`
	HourlyAggregations int64            `json:"hourly_aggregations"`
	DailyAggregations int64            `json:"daily_aggregations"`
}

// NewEnhancedStore creates a new enhanced cache store
func NewEnhancedStore(config EnhancedStoreConfig) (*EnhancedStore, error) {
	// Set defaults
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 6 * time.Hour // Default cleanup every 6 hours
	}

	// Create BadgerDB cache
	badgerCache, err := NewBadgerCache(config.BadgerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create BadgerDB cache: %w", err)
	}

	// Create aggregation processor
	processor := NewAggregationProcessor(badgerCache)

	store := &EnhancedStore{
		badgerCache: badgerCache,
		processor:   processor,
		config:      config,
	}

	// Start automatic cleanup if enabled
	if config.AutoCleanup {
		store.startAutoCleanup()
	}

	return store, nil
}

// ProcessFile processes usage entries from a file and updates all aggregations
func (es *EnhancedStore) ProcessFile(filePath string, entries []models.UsageEntry) error {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return fmt.Errorf("store is closed")
	}

	return es.processor.ProcessFile(filePath, entries)
}

// GetHourlyAggregation retrieves hourly aggregation for a specific timestamp
func (es *EnhancedStore) GetHourlyAggregation(timestamp time.Time) (*HourlyAggregation, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return nil, fmt.Errorf("store is closed")
	}

	return es.processor.GetHourlyAggregation(timestamp)
}

// GetDailyAggregation retrieves daily aggregation for a specific date
func (es *EnhancedStore) GetDailyAggregation(date time.Time) (*DailyAggregation, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return nil, fmt.Errorf("store is closed")
	}

	return es.processor.GetDailyAggregation(date)
}

// GetModelSummary retrieves model summary for a specific model
func (es *EnhancedStore) GetModelSummary(model string) (*ModelSummary, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return nil, fmt.Errorf("store is closed")
	}

	return es.processor.GetModelSummary(model)
}

// GetModelsList retrieves the list of all models
func (es *EnhancedStore) GetModelsList() ([]string, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return nil, fmt.Errorf("store is closed")
	}

	return es.processor.GetModelsList()
}

// GetHourlyRange retrieves hourly aggregations for a time range
func (es *EnhancedStore) GetHourlyRange(start, end time.Time) ([]*HourlyAggregation, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return nil, fmt.Errorf("store is closed")
	}

	return es.processor.GetHourlyRange(start, end)
}

// GetDailyRange retrieves daily aggregations for a date range
func (es *EnhancedStore) GetDailyRange(start, end time.Time) ([]*DailyAggregation, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return nil, fmt.Errorf("store is closed")
	}

	return es.processor.GetDailyRange(start, end)
}

// GetModelUsageInRange retrieves usage for a specific model in a date range
func (es *EnhancedStore) GetModelUsageInRange(model string, start, end time.Time) (*ModelStats, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return nil, fmt.Errorf("store is closed")
	}

	// Get daily aggregations for the range
	dailyAggs, err := es.processor.GetDailyRange(start, end)
	if err != nil {
		return nil, err
	}

	// Aggregate model stats across all daily aggregations
	totalStats := &ModelStats{}
	sessionSet := make(map[string]bool)

	for _, dailyAgg := range dailyAggs {
		if modelStats, exists := dailyAgg.Models[model]; exists {
			totalStats.EntryCount += modelStats.EntryCount
			totalStats.InputTokens += modelStats.InputTokens
			totalStats.OutputTokens += modelStats.OutputTokens
			totalStats.CacheCreationTokens += modelStats.CacheCreationTokens
			totalStats.CacheReadTokens += modelStats.CacheReadTokens
			totalStats.TotalTokens += modelStats.TotalTokens
			totalStats.TotalCost += modelStats.TotalCost

			// Collect unique sessions
			for _, session := range modelStats.Sessions {
				sessionSet[session] = true
			}
		}
	}

	// Convert session set to slice
	totalStats.Sessions = make([]string, 0, len(sessionSet))
	for session := range sessionSet {
		totalStats.Sessions = append(totalStats.Sessions, session)
	}

	return totalStats, nil
}

// GetTotalUsageInRange retrieves total usage across all models in a date range
func (es *EnhancedStore) GetTotalUsageInRange(start, end time.Time) (*ModelStats, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return nil, fmt.Errorf("store is closed")
	}

	// Get daily aggregations for the range
	dailyAggs, err := es.processor.GetDailyRange(start, end)
	if err != nil {
		return nil, err
	}

	// Aggregate total stats across all daily aggregations
	totalStats := &ModelStats{}
	sessionSet := make(map[string]bool)

	for _, dailyAgg := range dailyAggs {
		if dailyAgg.TotalStats != nil {
			totalStats.EntryCount += dailyAgg.TotalStats.EntryCount
			totalStats.InputTokens += dailyAgg.TotalStats.InputTokens
			totalStats.OutputTokens += dailyAgg.TotalStats.OutputTokens
			totalStats.CacheCreationTokens += dailyAgg.TotalStats.CacheCreationTokens
			totalStats.CacheReadTokens += dailyAgg.TotalStats.CacheReadTokens
			totalStats.TotalTokens += dailyAgg.TotalStats.TotalTokens
			totalStats.TotalCost += dailyAgg.TotalStats.TotalCost
		}

		// Collect unique sessions
		for _, session := range dailyAgg.Sessions {
			sessionSet[session] = true
		}
	}

	// Convert session set to slice
	totalStats.Sessions = make([]string, 0, len(sessionSet))
	for session := range sessionSet {
		totalStats.Sessions = append(totalStats.Sessions, session)
	}

	return totalStats, nil
}

// GetModelComparison compares usage between multiple models in a date range
func (es *EnhancedStore) GetModelComparison(models []string, start, end time.Time) (map[string]*ModelStats, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return nil, fmt.Errorf("store is closed")
	}

	result := make(map[string]*ModelStats)

	for _, model := range models {
		stats, err := es.GetModelUsageInRange(model, start, end)
		if err != nil {
			// If model has no data, create empty stats
			stats = &ModelStats{}
		}
		result[model] = stats
	}

	return result, nil
}

// GetTopModels returns the top N models by total cost in a date range
func (es *EnhancedStore) GetTopModels(n int, start, end time.Time) ([]ModelRanking, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return nil, fmt.Errorf("store is closed")
	}

	// Get all models
	models, err := es.processor.GetModelsList()
	if err != nil {
		return nil, err
	}

	// Get usage for each model
	rankings := make([]ModelRanking, 0, len(models))
	for _, model := range models {
		stats, err := es.GetModelUsageInRange(model, start, end)
		if err != nil {
			continue
		}

		rankings = append(rankings, ModelRanking{
			Model:      model,
			TotalCost:  stats.TotalCost,
			TotalTokens: stats.TotalTokens,
			EntryCount: stats.EntryCount,
		})
	}

	// Sort by total cost (descending)
	for i := 0; i < len(rankings)-1; i++ {
		for j := i + 1; j < len(rankings); j++ {
			if rankings[i].TotalCost < rankings[j].TotalCost {
				rankings[i], rankings[j] = rankings[j], rankings[i]
			}
		}
	}

	// Return top N
	if n > len(rankings) {
		n = len(rankings)
	}
	return rankings[:n], nil
}

// ModelRanking represents a model's ranking by usage
type ModelRanking struct {
	Model       string  `json:"model"`
	TotalCost   float64 `json:"total_cost"`
	TotalTokens int     `json:"total_tokens"`
	EntryCount  int     `json:"entry_count"`
}

// Backup creates a backup of the entire cache
func (es *EnhancedStore) Backup(backupPath string) error {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return fmt.Errorf("store is closed")
	}

	return es.badgerCache.Backup(backupPath)
}

// Restore restores the cache from a backup
func (es *EnhancedStore) Restore(backupPath string) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.closed {
		return fmt.Errorf("store is closed")
	}

	return es.badgerCache.Restore(backupPath)
}

// Stats returns comprehensive store statistics
func (es *EnhancedStore) Stats() (*EnhancedStoreStats, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return nil, fmt.Errorf("store is closed")
	}

	badgerStats := es.badgerCache.GetStats()
	processorMetrics := es.processor.GetMetrics()

	// Get models count
	models, _ := es.processor.GetModelsList()
	modelsCount := len(models)

	return &EnhancedStoreStats{
		BadgerStats:        badgerStats,
		ProcessorMetrics:   processorMetrics,
		TotalModels:        modelsCount,
		HourlyAggregations: processorMetrics.HourlyAggregations,
		DailyAggregations:  processorMetrics.DailyAggregations,
	}, nil
}

// Clear removes all data from the cache
func (es *EnhancedStore) Clear() error {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.closed {
		return fmt.Errorf("store is closed")
	}

	return es.processor.Clear()
}

// Close closes the enhanced store
func (es *EnhancedStore) Close() error {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.closed {
		return nil
	}

	es.closed = true

	if err := es.processor.Close(); err != nil {
		return fmt.Errorf("failed to close processor: %w", err)
	}

	return nil
}

// RunGC runs garbage collection on the BadgerDB
func (es *EnhancedStore) RunGC() error {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return fmt.Errorf("store is closed")
	}

	return es.badgerCache.RunGC()
}

// startAutoCleanup starts automatic cache cleanup
func (es *EnhancedStore) startAutoCleanup() {
	go func() {
		ticker := time.NewTicker(es.config.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if es.closed {
					return
				}

				// Run garbage collection
				if err := es.RunGC(); err != nil {
					fmt.Printf("Auto cleanup GC error: %v\n", err)
				}

				// Additional cleanup tasks can be added here
				// For example: removing old aggregations, compacting data, etc.
			}
		}
	}()
}

// IsHealthy checks if the store is operating within normal parameters
func (es *EnhancedStore) IsHealthy() bool {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.closed {
		return false
	}

	stats := es.badgerCache.GetStats()
	
	// Consider healthy if database size is reasonable (< 10GB)
	return stats.TotalSize < 10*1024*1024*1024
}