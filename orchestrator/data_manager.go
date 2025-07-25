package orchestrator

import (
	"fmt"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/fileio"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/sessions"
)

// DataManager manages data fetching and caching for monitoring
type DataManager struct {
	cacheTTL               time.Duration
	hoursBack              int
	dataPath               string
	
	// Cache management
	cache                  *AnalysisResult
	cacheTimestamp         time.Time
	mu                     sync.RWMutex
	
	// Error tracking
	lastError              error
	lastSuccessfulFetch    time.Time
}

// NewDataManager creates a new data manager with cache and fetch settings
func NewDataManager(cacheTTL time.Duration, hoursBack int, dataPath string) *DataManager {
	return &DataManager{
		cacheTTL:  cacheTTL,
		hoursBack: hoursBack,
		dataPath:  dataPath,
	}
}

// GetData gets monitoring data with caching and error handling
func (dm *DataManager) GetData(forceRefresh bool) (*AnalysisResult, error) {
	dm.mu.RLock()
	// Check cache validity
	if !forceRefresh && dm.isCacheValid() {
		cacheAge := time.Since(dm.cacheTimestamp)
		fmt.Printf("Using cached data (age: %.1fs)\n", cacheAge.Seconds())
		result := dm.cache
		dm.mu.RUnlock()
		return result, nil
	}
	dm.mu.RUnlock()

	// Fetch fresh data with retries
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		fmt.Printf("Fetching fresh usage data (attempt %d/%d)\n", attempt+1, maxRetries)
		
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
				fmt.Println("Using cached data due to fetch error")
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

// isCacheValid checks if the cache is still valid (caller must hold read lock)
func (dm *DataManager) isCacheValid() bool {
	if dm.cache == nil || dm.cacheTimestamp.IsZero() {
		return false
	}
	
	cacheAge := time.Since(dm.cacheTimestamp)
	return cacheAge <= dm.cacheTTL
}

// analyzeUsage performs the equivalent of Claude Monitor's analyze_usage function
func (dm *DataManager) analyzeUsage() (*AnalysisResult, error) {
	_ = time.Now() // startTime was unused
	
	// Load usage entries
	opts := fileio.LoadUsageEntriesOptions{
		DataPath:   dm.dataPath,
		HoursBack:  &dm.hoursBack,
		Mode:       models.CostModeAuto,
		IncludeRaw: true,
	}
	
	result, err := fileio.LoadUsageEntries(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load usage entries: %w", err)
	}
	
	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("no usage entries found")
	}
	
	loadTime := result.Metadata.LoadDuration
	fmt.Printf("Data loaded in %.3fs\n", loadTime.Seconds())
	
	// Transform entries to blocks using SessionAnalyzer
	transformStart := time.Now()
	analyzer := sessions.NewSessionAnalyzer(5) // 5-hour sessions
	blocks := analyzer.TransformToBlocks(result.Entries)
	transformTime := time.Since(transformStart)
	fmt.Printf("Created %d blocks in %.3fs\n", len(blocks), transformTime.Seconds())
	
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
	
	fmt.Printf("Analysis completed, returning %d blocks\n", len(blocks))
	return analysisResult, nil
}

// isLimitInBlockTimerange checks if a limit detection falls within a block's time range
func (dm *DataManager) isLimitInBlockTimerange(limit models.LimitMessage, block models.SessionBlock) bool {
	return (limit.Timestamp.After(block.StartTime) || limit.Timestamp.Equal(block.StartTime)) &&
		(limit.Timestamp.Before(block.EndTime) || limit.Timestamp.Equal(block.EndTime))
}