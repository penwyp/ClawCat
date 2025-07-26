package fileio

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/penwyp/claudecat/cache"
	"github.com/penwyp/claudecat/logging"
	"github.com/penwyp/claudecat/models"
)

// ConcurrentLoader loads usage entries concurrently from multiple files
type ConcurrentLoader struct {
	workerCount int
	bufferSize  int
	logger      logging.LoggerInterface
}

// FileResult represents the result of processing a single file
type FileResult struct {
	FilePath    string
	Entries     []models.UsageEntry
	RawEntries  []map[string]interface{}
	FromCache   bool
	MissReason  string             // Reason for cache miss
	Summary     *cache.FileSummary // Summary to cache (if any)
	Error       error
	ProcessTime time.Duration
}

// LoadProgress tracks the progress of concurrent loading
type LoadProgress struct {
	TotalFiles     int32
	ProcessedFiles int32
	CacheHits      int32
	CacheMisses    int32
	Errors         int32
	TotalEntries   int32
}

// NewConcurrentLoader creates a new concurrent loader
func NewConcurrentLoader(workerCount int) *ConcurrentLoader {
	if workerCount <= 0 {
		workerCount = runtime.NumCPU()
	}

	return &ConcurrentLoader{
		workerCount: workerCount,
		bufferSize:  workerCount * 2,
		logger:      logging.GetLogger(),
	}
}

// LoadFiles processes multiple files concurrently
func (cl *ConcurrentLoader) LoadFiles(ctx context.Context, files []string, opts LoadUsageEntriesOptions, progressCallback func(*LoadProgress)) ([]FileResult, error) {
	if len(files) == 0 {
		return nil, nil
	}

	// Initialize progress tracking
	progress := &LoadProgress{
		TotalFiles: int32(len(files)),
	}

	// Create channels
	fileChan := make(chan string, cl.bufferSize)
	resultChan := make(chan FileResult, cl.bufferSize)

	// Calculate cutoff time if specified
	var cutoffTime *time.Time
	if opts.HoursBack != nil {
		cutoff := time.Now().UTC().Add(-time.Duration(*opts.HoursBack) * time.Hour)
		cutoffTime = &cutoff
	}

	// Start worker goroutines
	var wg sync.WaitGroup
	wg.Add(cl.workerCount)

	for i := 0; i < cl.workerCount; i++ {
		go func(workerID int) {
			defer wg.Done()
			cl.worker(ctx, workerID, fileChan, resultChan, opts, cutoffTime, progress, progressCallback)
		}(i)
	}

	// Start result collector
	results := make([]FileResult, 0, len(files))
	resultsDone := make(chan struct{})

	go func() {
		for result := range resultChan {
			results = append(results, result)
		}
		close(resultsDone)
	}()

	// Feed files to workers
	go func() {
		for _, file := range files {
			select {
			case fileChan <- file:
			case <-ctx.Done():
				break
			}
		}
		close(fileChan)
	}()

	// Wait for all workers to complete
	wg.Wait()
	close(resultChan)

	// Wait for results to be collected
	<-resultsDone

	// Final progress callback
	if progressCallback != nil {
		progressCallback(progress)
	}

	return results, nil
}

// worker processes files from the input channel
func (cl *ConcurrentLoader) worker(
	ctx context.Context,
	workerID int,
	fileChan <-chan string,
	resultChan chan<- FileResult,
	opts LoadUsageEntriesOptions,
	cutoffTime *time.Time,
	progress *LoadProgress,
	progressCallback func(*LoadProgress),
) {
	for {
		select {
		case filePath, ok := <-fileChan:
			if !ok {
				return
			}

			startTime := time.Now()

			// Process the file
			entries, rawEntries, fromCache, missReason, err, summary := processSingleFileWithCacheWithReason(filePath, opts, cutoffTime)

			// Create result
			result := FileResult{
				FilePath:    filePath,
				Entries:     entries,
				RawEntries:  rawEntries,
				FromCache:   fromCache,
				MissReason:  missReason,
				Summary:     summary,
				Error:       err,
				ProcessTime: time.Since(startTime),
			}

			// Update progress
			atomic.AddInt32(&progress.ProcessedFiles, 1)
			if fromCache {
				atomic.AddInt32(&progress.CacheHits, 1)
			} else {
				atomic.AddInt32(&progress.CacheMisses, 1)
			}
			if err != nil {
				atomic.AddInt32(&progress.Errors, 1)
			} else {
				atomic.AddInt32(&progress.TotalEntries, int32(len(entries)))
			}

			// Send progress update
			if progressCallback != nil && atomic.LoadInt32(&progress.ProcessedFiles)%10 == 0 {
				progressCallback(progress)
			}

			// Send result
			select {
			case resultChan <- result:
			case <-ctx.Done():
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

// LoadFilesWithProgress is a convenience method that provides a default progress printer
func (cl *ConcurrentLoader) LoadFilesWithProgress(ctx context.Context, files []string, opts LoadUsageEntriesOptions) ([]FileResult, error) {
	lastUpdate := time.Now()

	progressCallback := func(progress *LoadProgress) {
		if time.Since(lastUpdate) < 100*time.Millisecond {
			return // Throttle updates
		}
		lastUpdate = time.Now()

		processed := atomic.LoadInt32(&progress.ProcessedFiles)
		total := atomic.LoadInt32(&progress.TotalFiles)
		hits := atomic.LoadInt32(&progress.CacheHits)
		misses := atomic.LoadInt32(&progress.CacheMisses)

		hitRate := float64(0)
		if hits+misses > 0 {
			hitRate = float64(hits) / float64(hits+misses) * 100
		}

		cl.logger.Infof("Progress: %d/%d files (%.1f%%), Cache: %d hits, %d misses (%.1f%% hit rate)",
			processed, total, float64(processed)/float64(total)*100,
			hits, misses, hitRate)
	}

	return cl.LoadFiles(ctx, files, opts, progressCallback)
}

// MergeResults combines results from concurrent loading into a single sorted list
func MergeResults(results []FileResult) ([]models.UsageEntry, []map[string]interface{}, []error) {
	var allEntries []models.UsageEntry
	var allRawEntries []map[string]interface{}
	var errors []error

	// Calculate total capacity needed
	totalEntries := 0
	totalRaw := 0
	for _, result := range results {
		if result.Error != nil {
			errors = append(errors, fmt.Errorf("%s: %w", result.FilePath, result.Error))
			continue
		}
		totalEntries += len(result.Entries)
		totalRaw += len(result.RawEntries)
	}

	// Pre-allocate slices
	allEntries = make([]models.UsageEntry, 0, totalEntries)
	if totalRaw > 0 {
		allRawEntries = make([]map[string]interface{}, 0, totalRaw)
	}

	// Merge results
	for _, result := range results {
		if result.Error == nil {
			allEntries = append(allEntries, result.Entries...)
			if result.RawEntries != nil {
				allRawEntries = append(allRawEntries, result.RawEntries...)
			}
		}
	}

	return allEntries, allRawEntries, errors
}
