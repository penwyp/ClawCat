package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/penwyp/claudecat/config"
	"github.com/penwyp/claudecat/fileio"
	"github.com/penwyp/claudecat/logging"
	"github.com/penwyp/claudecat/models"
	"github.com/penwyp/claudecat/sessions"
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
	cacheStore         fileio.CacheStore
	summaryCacheConfig config.SummaryCacheConfig

	// Initial load tracking
	initialLoadCompleted bool

	// Pricing and deduplication
	pricingProvider     models.PricingProvider
	enableDeduplication bool
}

// NewDataManager creates a new data manager with cache and fetch settings
func NewDataManager(hoursBack int, dataPath string) *DataManager {
	return &DataManager{
		hoursBack: hoursBack,
		dataPath:  dataPath,
	}
}

// SetCacheStore sets the cache store for file summaries
func (dm *DataManager) SetCacheStore(cacheStore fileio.CacheStore, config config.SummaryCacheConfig) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.cacheStore = cacheStore
	dm.summaryCacheConfig = config
}

// SetPricingProvider sets the pricing provider for cost calculations
func (dm *DataManager) SetPricingProvider(provider models.PricingProvider) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.pricingProvider = provider
}

// SetDeduplication sets whether to enable deduplication
func (dm *DataManager) SetDeduplication(enabled bool) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.enableDeduplication = enabled
}

// GetData gets monitoring data with caching and error handling
func (dm *DataManager) GetData(forceRefresh bool) (*AnalysisResult, error) {
	dm.mu.RLock()
	// Check if this is the first load
	isInitialLoad := !dm.initialLoadCompleted
	dm.mu.RUnlock()

	// For initial load, always fetch fresh data but allow cache writing
	if isInitialLoad {
		return dm.performInitialLoad()
	}

	dm.mu.RLock()
	// Check cache validity for subsequent loads
	if !forceRefresh {
		result := dm.cache
		dm.mu.RUnlock()
		return result, nil
	}
	dm.mu.RUnlock()

	// Fetch fresh data with retries (watch mode - no cache writing)
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		logging.LogDebugf("Fetching fresh usage data (attempt %d/%d)", attempt+1, maxRetries)

		data, err := dm.analyzeUsageWatchMode()
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

// performInitialLoad performs initial data loading with cache writing allowed
func (dm *DataManager) performInitialLoad() (*AnalysisResult, error) {
	logging.LogInfo("Performing initial data load with cache support")

	// First try to load from cache to check if we have cached data
	if dm.cacheStore != nil && dm.summaryCacheConfig.Enabled {
		logging.LogInfo("Checking for existing cached data...")

		// Load with cache first to check cache status
		optsCache := fileio.LoadUsageEntriesOptions{
			DataPath:            dm.dataPath,
			HoursBack:           &dm.hoursBack,
			Mode:                models.CostModeAuto,
			IncludeRaw:          true,
			EnableSummaryCache:  true,
			IsWatchMode:         true, // Use cache read mode first
			CacheStore:          dm.cacheStore,
			EnableDeduplication: dm.enableDeduplication,
			PricingProvider:     dm.pricingProvider,
		}

		resultCache, err := fileio.LoadUsageEntries(optsCache)
		if err == nil && len(resultCache.Entries) > 0 {
			// We have cached data, check if files have changed
			logging.LogInfof("Found %d cached entries, checking for file changes...", len(resultCache.Entries))

			hasChanges, err := dm.checkForFileChanges(&resultCache.Metadata)
			if err != nil {
				logging.LogWarnf("Error checking for file changes: %v, will reload data", err)
				hasChanges = true // Assume changes if we can't check
			}

			if !hasChanges {
				logging.LogInfo("No file changes detected, using cached data")
				data, err := dm.processUsageData(resultCache, "initial-cached")
				if err != nil {
					return nil, err
				}

				// Mark initial load as completed and update cache
				dm.mu.Lock()
				dm.initialLoadCompleted = true
				dm.cache = data
				dm.cacheTimestamp = time.Now()
				dm.lastSuccessfulFetch = time.Now()
				dm.lastError = nil
				dm.mu.Unlock()

				logging.LogInfo("Initial data load completed using cache")
				return data, nil
			} else {
				logging.LogInfo("File changes detected, reloading and updating cache...")
			}
		} else {
			logging.LogInfo("No cached data found or cache load failed, performing fresh load")
		}
	}

	// Load usage entries with cache support and allow cache writing for initial load
	opts := fileio.LoadUsageEntriesOptions{
		DataPath:            dm.dataPath,
		HoursBack:           &dm.hoursBack,
		Mode:                models.CostModeAuto,
		IncludeRaw:          true,
		EnableSummaryCache:  dm.cacheStore != nil && dm.summaryCacheConfig.Enabled,
		IsWatchMode:         false, // Initial load can write to cache
		EnableDeduplication: dm.enableDeduplication,
		PricingProvider:     dm.pricingProvider,
	}

	// Set cache store if available
	if dm.cacheStore != nil {
		opts.CacheStore = dm.cacheStore
	}

	result, err := fileio.LoadUsageEntries(opts)
	if err != nil {
		logging.LogErrorf("Error loading usage entries from %s during initial load: %v", dm.dataPath, err)
		return nil, fmt.Errorf("failed to load usage entries: %w", err)
	}

	data, err := dm.processUsageData(result, "initial")
	if err != nil {
		return nil, err
	}

	// Mark initial load as completed and update cache
	dm.mu.Lock()
	dm.initialLoadCompleted = true
	dm.cache = data
	dm.cacheTimestamp = time.Now()
	dm.lastSuccessfulFetch = time.Now()
	dm.lastError = nil
	dm.mu.Unlock()

	logging.LogInfo("Initial data load completed successfully")
	return data, nil
}

// analyzeUsageWatchMode performs analysis in watch mode (no cache writing)
func (dm *DataManager) analyzeUsageWatchMode() (*AnalysisResult, error) {
	// Load usage entries in watch mode - no cache writing
	opts := fileio.LoadUsageEntriesOptions{
		DataPath:            dm.dataPath,
		HoursBack:           &dm.hoursBack,
		Mode:                models.CostModeAuto,
		IncludeRaw:          true,
		EnableSummaryCache:  dm.cacheStore != nil && dm.summaryCacheConfig.Enabled,
		IsWatchMode:         true, // Watch mode - no cache writing
		EnableDeduplication: dm.enableDeduplication,
		PricingProvider:     dm.pricingProvider,
	}

	// Set cache store if available
	if dm.cacheStore != nil {
		opts.CacheStore = dm.cacheStore
	}

	result, err := fileio.LoadUsageEntries(opts)
	if err != nil {
		logging.LogErrorf("Error loading usage entries from %s in watch mode: %v", dm.dataPath, err)
		return nil, fmt.Errorf("failed to load usage entries: %w", err)
	}

	return dm.processUsageData(result, "watch")
}

// processUsageData processes loaded usage data into analysis result
func (dm *DataManager) processUsageData(result *fileio.LoadUsageEntriesResult, mode string) (*AnalysisResult, error) {
	logging.LogInfof("Loaded %d usage entries from %s (%s mode)", len(result.Entries), dm.dataPath, mode)
	if len(result.Entries) == 0 {
		logging.LogWarnf("No usage entries found in %s", dm.dataPath)
		return nil, fmt.Errorf("no usage entries found")
	}

	loadTime := result.Metadata.LoadDuration
	logging.LogInfof("Data loaded in %.3fs (%s mode)", loadTime.Seconds(), mode)

	// Transform entries to blocks using SessionAnalyzer
	transformStart := time.Now()
	analyzer := sessions.NewSessionAnalyzer(5) // 5-hour sessions
	blocks := analyzer.TransformToBlocks(result.Entries)
	transformTime := time.Since(transformStart)
	logging.LogInfof("Created %d blocks in %.3fs (%s mode)", len(blocks), transformTime.Seconds(), mode)

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

	logging.LogInfof("Analysis completed, returning %d blocks (%s mode)", len(blocks), mode)
	return analysisResult, nil
}

// checkForFileChanges checks if any files in the data path have changed since the cached metadata
func (dm *DataManager) checkForFileChanges(cachedMetadata *fileio.LoadMetadata) (bool, error) {
	logging.LogDebug("Checking for file changes since last cache...")

	// Walk through the data path to find all .jsonl files
	var hasChanges bool

	err := filepath.Walk(dm.dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logging.LogWarnf("Error accessing path %s: %v", path, err)
			return nil // Continue walking, don't fail entirely
		}

		// Skip directories and non-jsonl files
		if info.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}

		// Check if this file was processed in the cached metadata
		// For now, use a simple heuristic: if the file's modification time
		// is newer than the cache load time, consider it changed
		// Since LoadMetadata doesn't have CacheLoadTime, we'll use a different approach
		// We'll check if any file is newer than 1 minute ago (simple heuristic)
		oneMinuteAgo := time.Now().Add(-1 * time.Minute)

		if info.ModTime().After(oneMinuteAgo) {
			logging.LogDebugf("File %s modified recently (%s)",
				filepath.Base(path), info.ModTime())
			hasChanges = true
			return filepath.SkipDir // Exit early
		}

		return nil
	})

	if err != nil {
		return true, fmt.Errorf("error walking data path: %w", err)
	}

	if hasChanges {
		logging.LogDebug("File changes detected")
	} else {
		logging.LogDebug("No file changes detected")
	}

	return hasChanges, nil
}

// isLimitInBlockTimerange checks if a limit detection falls within a block's time range
func (dm *DataManager) isLimitInBlockTimerange(limit models.LimitMessage, block models.SessionBlock) bool {
	return (limit.Timestamp.After(block.StartTime) || limit.Timestamp.Equal(block.StartTime)) &&
		(limit.Timestamp.Before(block.EndTime) || limit.Timestamp.Equal(block.EndTime))
}
