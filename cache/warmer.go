package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/penwyp/ClawCat/logging"
)

// CacheWarmer preheats cache with frequently accessed files
type CacheWarmer struct {
	store       *Store
	logger      logging.LoggerInterface
	workerCount int
	
	// Warmup state
	mu          sync.RWMutex
	isWarming   bool
	lastWarmup  time.Time
	warmupStats WarmupStats
}

// WarmupStats tracks cache warming statistics
type WarmupStats struct {
	FilesProcessed int32
	FilesWarmed    int32
	FilesFailed    int32
	BytesWarmed    int64
	Duration       time.Duration
	Errors         []error
}

// FilePattern represents a file pattern to warm
type FilePattern struct {
	Pattern  string
	Priority int // Higher priority patterns are warmed first
}

// WarmupConfig configures cache warming behavior
type WarmupConfig struct {
	Patterns        []FilePattern
	MaxFiles        int           // Maximum files to warm
	MaxAge          time.Duration // Only warm files modified within this duration
	WorkerCount     int           // Number of concurrent warmup workers
	TimeoutPerFile  time.Duration // Timeout for warming each file
}

// NewCacheWarmer creates a new cache warmer
func NewCacheWarmer(store *Store, workerCount int) *CacheWarmer {
	if workerCount <= 0 {
		workerCount = 4 // Default to 4 workers
	}
	
	return &CacheWarmer{
		store:       store,
		logger:      logging.GetLogger(),
		workerCount: workerCount,
	}
}

// WarmupAsync starts asynchronous cache warming
func (cw *CacheWarmer) WarmupAsync(ctx context.Context, config WarmupConfig) error {
	cw.mu.Lock()
	if cw.isWarming {
		cw.mu.Unlock()
		return fmt.Errorf("cache warming already in progress")
	}
	cw.isWarming = true
	cw.lastWarmup = time.Now()
	cw.warmupStats = WarmupStats{}
	cw.mu.Unlock()
	
	// Start warming in background
	go func() {
		defer func() {
			cw.mu.Lock()
			cw.isWarming = false
			cw.mu.Unlock()
		}()
		
		if err := cw.warmup(ctx, config); err != nil {
			cw.logger.Errorf("Cache warmup failed: %v", err)
		}
	}()
	
	return nil
}

// warmup performs the actual cache warming
func (cw *CacheWarmer) warmup(ctx context.Context, config WarmupConfig) error {
	startTime := time.Now()
	defer func() {
		cw.warmupStats.Duration = time.Since(startTime)
		cw.logger.Infof("Cache warmup completed: %d files warmed, %d failed, %.2f MB in %v",
			atomic.LoadInt32(&cw.warmupStats.FilesWarmed),
			atomic.LoadInt32(&cw.warmupStats.FilesFailed),
			float64(atomic.LoadInt64(&cw.warmupStats.BytesWarmed))/1024/1024,
			cw.warmupStats.Duration)
	}()
	
	// Collect files to warm
	files, err := cw.collectFiles(config)
	if err != nil {
		return fmt.Errorf("failed to collect files: %w", err)
	}
	
	if len(files) == 0 {
		cw.logger.Info("No files to warm in cache")
		return nil
	}
	
	cw.logger.Infof("Starting cache warmup for %d files", len(files))
	
	// Create work queue
	fileChan := make(chan warmupFile, len(files))
	for _, f := range files {
		fileChan <- f
	}
	close(fileChan)
	
	// Start workers
	var wg sync.WaitGroup
	workerCount := config.WorkerCount
	if workerCount <= 0 {
		workerCount = cw.workerCount
	}
	
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			cw.warmupWorker(ctx, workerID, fileChan, config.TimeoutPerFile)
		}(i)
	}
	
	// Wait for completion
	wg.Wait()
	
	return nil
}

// warmupFile represents a file to warm
type warmupFile struct {
	path     string
	info     os.FileInfo
	priority int
}

// collectFiles collects files matching the warmup patterns
func (cw *CacheWarmer) collectFiles(config WarmupConfig) ([]warmupFile, error) {
	fileMap := make(map[string]warmupFile)
	now := time.Now()
	
	// Process each pattern
	for _, pattern := range config.Patterns {
		matches, err := filepath.Glob(pattern.Pattern)
		if err != nil {
			cw.logger.Warnf("Invalid pattern %s: %v", pattern.Pattern, err)
			continue
		}
		
		for _, path := range matches {
			// Skip if already collected
			if _, exists := fileMap[path]; exists {
				continue
			}
			
			// Get file info
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				continue
			}
			
			// Check age if specified
			if config.MaxAge > 0 && now.Sub(info.ModTime()) > config.MaxAge {
				continue
			}
			
			fileMap[path] = warmupFile{
				path:     path,
				info:     info,
				priority: pattern.Priority,
			}
		}
	}
	
	// Convert to slice and sort by priority and modification time
	files := make([]warmupFile, 0, len(fileMap))
	for _, f := range fileMap {
		files = append(files, f)
	}
	
	sort.Slice(files, func(i, j int) bool {
		// Higher priority first
		if files[i].priority != files[j].priority {
			return files[i].priority > files[j].priority
		}
		// More recently modified first
		return files[i].info.ModTime().After(files[j].info.ModTime())
	})
	
	// Limit number of files if specified
	if config.MaxFiles > 0 && len(files) > config.MaxFiles {
		files = files[:config.MaxFiles]
	}
	
	return files, nil
}

// warmupWorker processes files from the work queue
func (cw *CacheWarmer) warmupWorker(ctx context.Context, workerID int, fileChan <-chan warmupFile, timeout time.Duration) {
	for {
		select {
		case file, ok := <-fileChan:
			if !ok {
				return
			}
			
			atomic.AddInt32(&cw.warmupStats.FilesProcessed, 1)
			
			// Create timeout context for this file
			fileCtx := ctx
			if timeout > 0 {
				var cancel context.CancelFunc
				fileCtx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}
			
			// Warm the file
			if err := cw.warmFile(fileCtx, file); err != nil {
				atomic.AddInt32(&cw.warmupStats.FilesFailed, 1)
				cw.warmupStats.Errors = append(cw.warmupStats.Errors, fmt.Errorf("%s: %w", file.path, err))
			} else {
				atomic.AddInt32(&cw.warmupStats.FilesWarmed, 1)
				atomic.AddInt64(&cw.warmupStats.BytesWarmed, file.info.Size())
			}
			
		case <-ctx.Done():
			return
		}
	}
}

// warmFile warms a single file into cache
func (cw *CacheWarmer) warmFile(ctx context.Context, file warmupFile) error {
	// Check if file is already in cache
	if cached, err := cw.store.GetFile(file.path); err == nil && cached != nil {
		// Check if cached version is still valid
		if !cached.ModTime.Before(file.info.ModTime()) {
			return nil // Already cached and up to date
		}
	}
	
	// For JSONL files, use specialized warming
	if filepath.Ext(file.path) == ".jsonl" {
		return cw.warmJSONLFile(ctx, file)
	}
	
	// For other files, just read and cache
	content, err := os.ReadFile(file.path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	// Cache the file content
	// Note: We're not parsing entries here, just caching raw content
	return cw.store.CacheFile(file.path, content, nil)
}

// warmJSONLFile warms a JSONL file with summary caching
func (cw *CacheWarmer) warmJSONLFile(ctx context.Context, file warmupFile) error {
	// Get absolute path for cache key
	absPath, err := filepath.Abs(file.path)
	if err != nil {
		absPath = file.path
	}
	
	// Check if we already have a valid summary
	if summary, err := cw.store.GetFileSummary(absPath); err == nil {
		// Check if summary is still valid
		if !summary.IsExpired(file.info.ModTime()) {
			return nil // Summary is still valid
		}
	}
	
	// Note: processSingleFileWithCache is not exported, so we'll use a simpler approach
	// Just ensure the file gets into the store's file cache
	content, err := os.ReadFile(file.path)
	if err != nil {
		return fmt.Errorf("failed to read JSONL file: %w", err)
	}
	
	// Cache the raw file content
	return cw.store.CacheFile(file.path, content, nil)
}

// IsWarming returns true if cache warming is in progress
func (cw *CacheWarmer) IsWarming() bool {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.isWarming
}

// GetStats returns the current warmup statistics
func (cw *CacheWarmer) GetStats() WarmupStats {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.warmupStats
}

// LastWarmupTime returns the time of the last warmup
func (cw *CacheWarmer) LastWarmupTime() time.Time {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.lastWarmup
}

// DefaultWarmupPatterns returns default patterns for warming based on usage
func DefaultWarmupPatterns(dataPath string) []FilePattern {
	return []FilePattern{
		// Warm today's files first
		{
			Pattern:  filepath.Join(dataPath, fmt.Sprintf("*%s*.jsonl", time.Now().Format("2006-01-02"))),
			Priority: 100,
		},
		// Then yesterday's files
		{
			Pattern:  filepath.Join(dataPath, fmt.Sprintf("*%s*.jsonl", time.Now().AddDate(0, 0, -1).Format("2006-01-02"))),
			Priority: 90,
		},
		// Last 7 days
		{
			Pattern:  filepath.Join(dataPath, "*.jsonl"),
			Priority: 50,
		},
	}
}