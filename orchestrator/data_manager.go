package orchestrator

import (
	"fmt"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/cache"
	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/fileio"
	"github.com/penwyp/ClawCat/logging"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/sessions"
)

// DataManager manages data fetching and caching for monitoring
type DataManager struct {
	hoursBack int
	dataPath  string

	// Cache management
	cache          *AnalysisResult
	cacheTimestamp time.Time
	mu             sync.RWMutex

	// Error tracking
	lastError           error
	lastSuccessfulFetch time.Time

	// Summary cache store
	cacheStore         *cache.SimpleSummaryCache
	summaryCacheConfig config.SummaryCacheConfig
}

// NewDataManager creates a new data manager with cache and fetch settings
func NewDataManager(hoursBack int, dataPath string) *DataManager {
	return &DataManager{
		hoursBack: hoursBack,
		dataPath:  dataPath,
	}
}

// SetCacheStore sets the cache store for file summaries
func (dm *DataManager) SetCacheStore(cacheStore *cache.SimpleSummaryCache, config config.SummaryCacheConfig) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.cacheStore = cacheStore
	dm.summaryCacheConfig = config
}

// GetData gets monitoring data with caching and error handling
func (dm *DataManager) GetData(forceRefresh bool) (*AnalysisResult, error) {
	dm.mu.RLock()
	// Check cache validity
	if !forceRefresh {
		result := dm.cache
		dm.mu.RUnlock()
		return result, nil
	}
	dm.mu.RUnlock()

	// Fetch fresh data with retries
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		logging.LogDebugf("Fetching fresh usage data (attempt %d/%d)", attempt+1, maxRetries)

		data, err := dm.analyzeUsage()
		if err != nil {
			dm.mu.Lock()
			dm.lastError = err
			dm.mu.Unlock()

			if attempt < maxRetries-1 {
				// Exponential backoff
				backoff := time.Duration(100*(1<<attempt)) * time.Millisecond
				time.Sleep(backoff)
				continue
			}

			// All retries failed, check if we have cached data to fall back on
			dm.mu.RLock()
			if dm.cache != nil {
				logging.LogWarn("Using cached data due to fetch error")
				result := dm.cache
				dm.mu.RUnlock()
				return result, nil
			}
			dm.mu.RUnlock()

			return nil, fmt.Errorf("failed to get usage data after %d attempts: %w", maxRetries, err)
		}

		// Success - update cache
		dm.mu.Lock()
		dm.cache = data
		dm.cacheTimestamp = time.Now()
		dm.lastSuccessfulFetch = time.Now()
		dm.lastError = nil
		dm.mu.Unlock()

		return data, nil
	}

	return nil, fmt.Errorf("unexpected error in data fetching loop")
}

// InvalidateCache invalidates the cache
func (dm *DataManager) InvalidateCache() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.cache = nil
	dm.cacheTimestamp = time.Time{}
}

// GetCacheAge returns the age of cached data in seconds
func (dm *DataManager) GetCacheAge() float64 {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.cacheTimestamp.IsZero() {
		return -1 // No cache
	}

	return time.Since(dm.cacheTimestamp).Seconds()
}

// GetLastError returns the last error encountered
func (dm *DataManager) GetLastError() error {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.lastError
}

// GetLastSuccessfulFetchTime returns the timestamp of the last successful fetch
func (dm *DataManager) GetLastSuccessfulFetchTime() time.Time {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.lastSuccessfulFetch
}

// analyzeUsage performs the equivalent of Claude Monitor's analyze_usage function
func (dm *DataManager) analyzeUsage() (*AnalysisResult, error) {
	_ = time.Now() // startTime was unused

	// Load usage entries with cache support
	opts := fileio.LoadUsageEntriesOptions{
		DataPath:           dm.dataPath,
		HoursBack:          &dm.hoursBack,
		Mode:               models.CostModeAuto,
		IncludeRaw:         true,
		EnableSummaryCache: dm.cacheStore != nil && dm.summaryCacheConfig.Enabled,
		IsWatchMode:        true, // DataManager is used in TUI mode with periodic updates
	}

	// Set cache store if available
	if dm.cacheStore != nil {
		opts.CacheStore = dm.cacheStore
	}

	result, err := fileio.LoadUsageEntries(opts)
	if err != nil {
		logging.LogErrorf("Error loading usage entries from %s: %v", dm.dataPath, err)
		return nil, fmt.Errorf("failed to load usage entries: %w", err)
	}

	logging.LogInfof("Loaded %d usage entries from %s", len(result.Entries), dm.dataPath)
	if len(result.Entries) == 0 {
		logging.LogWarnf("No usage entries found in %s", dm.dataPath)
		return nil, fmt.Errorf("no usage entries found")
	}

	loadTime := result.Metadata.LoadDuration
	logging.LogInfof("Data loaded in %.3fs", loadTime.Seconds())

	// Transform entries to blocks using SessionAnalyzer
	transformStart := time.Now()
	analyzer := sessions.NewSessionAnalyzer(5) // 5-hour sessions
	blocks := analyzer.TransformToBlocks(result.Entries)
	transformTime := time.Since(transformStart)
	logging.LogInfof("Created %d blocks in %.3fs", len(blocks), transformTime.Seconds())

	// Detect limits if we have raw entries
	var limitsDetected int
	if result.RawEntries != nil {
		// Convert raw entries to the format expected by analyzer
		rawEntries := make([]map[string]interface{}, len(result.RawEntries))
		for i, entry := range result.RawEntries {
			rawEntries[i] = entry
		}

		limitDetections := analyzer.DetectLimits(rawEntries)
		limitsDetected = len(limitDetections)

		// Add limit messages to appropriate blocks
		for i := range blocks {
			var blockLimits []models.LimitMessage
			for _, limit := range limitDetections {
				if dm.isLimitInBlockTimerange(limit, blocks[i]) {
					blockLimits = append(blockLimits, limit)
				}
			}
			if len(blockLimits) > 0 {
				blocks[i].LimitMessages = blockLimits
			}
		}
	}

	// Create metadata
	metadata := AnalysisMetadata{
		GeneratedAt:          time.Now(),
		HoursAnalyzed:        fmt.Sprintf("%d", dm.hoursBack),
		EntriesProcessed:     len(result.Entries),
		BlocksCreated:        len(blocks),
		LimitsDetected:       limitsDetected,
		LoadTimeSeconds:      loadTime.Seconds(),
		TransformTimeSeconds: transformTime.Seconds(),
		CacheUsed:            false,
		QuickStart:           false,
	}

	analysisResult := &AnalysisResult{
		Blocks:   blocks,
		Metadata: metadata,
	}

	logging.LogInfof("Analysis completed, returning %d blocks", len(blocks))
	return analysisResult, nil
}

// isLimitInBlockTimerange checks if a limit detection falls within a block's time range
func (dm *DataManager) isLimitInBlockTimerange(limit models.LimitMessage, block models.SessionBlock) bool {
	return (limit.Timestamp.After(block.StartTime) || limit.Timestamp.Equal(block.StartTime)) &&
		(limit.Timestamp.Before(block.EndTime) || limit.Timestamp.Equal(block.EndTime))
}
