package cache

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector collects and reports cache performance metrics
type MetricsCollector struct {
	mu sync.RWMutex
	
	// Real-time metrics (atomic for lock-free updates)
	totalRequests    int64
	cacheHits        int64
	cacheMisses      int64
	cacheWrites      int64
	cacheEvictions   int64
	
	// Latency tracking
	readLatencies    *LatencyTracker
	writeLatencies   *LatencyTracker
	
	// Time-series data
	minuteMetrics    []TimeSeriesMetric
	hourlyMetrics    []TimeSeriesMetric
	
	// Configuration
	retentionMinutes int
	retentionHours   int
	
	// Start time
	startTime        time.Time
}

// TimeSeriesMetric represents metrics for a time period
type TimeSeriesMetric struct {
	Timestamp       time.Time
	Requests        int64
	Hits            int64
	Misses          int64
	Writes          int64
	Evictions       int64
	HitRate         float64
	AvgReadLatency  time.Duration
	AvgWriteLatency time.Duration
	P95ReadLatency  time.Duration
	P95WriteLatency time.Duration
}

// LatencyTracker tracks latency percentiles
type LatencyTracker struct {
	mu       sync.Mutex
	samples  []time.Duration
	maxSize  int
}

// CacheMetrics represents current cache metrics
type CacheMetrics struct {
	// Counts
	TotalRequests  int64   `json:"total_requests"`
	CacheHits      int64   `json:"cache_hits"`
	CacheMisses    int64   `json:"cache_misses"`
	CacheWrites    int64   `json:"cache_writes"`
	CacheEvictions int64   `json:"cache_evictions"`
	
	// Rates
	HitRate        float64 `json:"hit_rate"`
	MissRate       float64 `json:"miss_rate"`
	RequestsPerSec float64 `json:"requests_per_sec"`
	
	// Latencies
	AvgReadLatency  time.Duration `json:"avg_read_latency"`
	AvgWriteLatency time.Duration `json:"avg_write_latency"`
	P50ReadLatency  time.Duration `json:"p50_read_latency"`
	P95ReadLatency  time.Duration `json:"p95_read_latency"`
	P99ReadLatency  time.Duration `json:"p99_read_latency"`
	P50WriteLatency time.Duration `json:"p50_write_latency"`
	P95WriteLatency time.Duration `json:"p95_write_latency"`
	P99WriteLatency time.Duration `json:"p99_write_latency"`
	
	// Memory usage
	MemoryUsed      int64   `json:"memory_used_bytes"`
	MemoryCapacity  int64   `json:"memory_capacity_bytes"`
	MemoryUtilization float64 `json:"memory_utilization"`
	
	// Time info
	Uptime          time.Duration `json:"uptime"`
	LastResetTime   time.Time     `json:"last_reset_time"`
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		readLatencies:    NewLatencyTracker(10000),  // Keep last 10k samples
		writeLatencies:   NewLatencyTracker(10000),
		minuteMetrics:    make([]TimeSeriesMetric, 0, 60),    // Last 60 minutes
		hourlyMetrics:    make([]TimeSeriesMetric, 0, 24),    // Last 24 hours
		retentionMinutes: 60,
		retentionHours:   24,
		startTime:        time.Now(),
	}
}

// RecordCacheHit records a cache hit with latency
func (mc *MetricsCollector) RecordCacheHit(latency time.Duration) {
	atomic.AddInt64(&mc.totalRequests, 1)
	atomic.AddInt64(&mc.cacheHits, 1)
	mc.readLatencies.Record(latency)
}

// RecordCacheMiss records a cache miss with latency
func (mc *MetricsCollector) RecordCacheMiss(latency time.Duration) {
	atomic.AddInt64(&mc.totalRequests, 1)
	atomic.AddInt64(&mc.cacheMisses, 1)
	mc.readLatencies.Record(latency)
}

// RecordCacheWrite records a cache write with latency
func (mc *MetricsCollector) RecordCacheWrite(latency time.Duration) {
	atomic.AddInt64(&mc.cacheWrites, 1)
	mc.writeLatencies.Record(latency)
}

// RecordEviction records a cache eviction
func (mc *MetricsCollector) RecordEviction(count int) {
	atomic.AddInt64(&mc.cacheEvictions, int64(count))
}

// GetMetrics returns current cache metrics
func (mc *MetricsCollector) GetMetrics() CacheMetrics {
	hits := atomic.LoadInt64(&mc.cacheHits)
	misses := atomic.LoadInt64(&mc.cacheMisses)
	requests := atomic.LoadInt64(&mc.totalRequests)
	writes := atomic.LoadInt64(&mc.cacheWrites)
	evictions := atomic.LoadInt64(&mc.cacheEvictions)
	
	uptime := time.Since(mc.startTime)
	
	metrics := CacheMetrics{
		TotalRequests:  requests,
		CacheHits:      hits,
		CacheMisses:    misses,
		CacheWrites:    writes,
		CacheEvictions: evictions,
		Uptime:         uptime,
		LastResetTime:  mc.startTime,
	}
	
	// Calculate rates
	if requests > 0 {
		metrics.HitRate = float64(hits) / float64(requests)
		metrics.MissRate = float64(misses) / float64(requests)
	}
	
	if uptime.Seconds() > 0 {
		metrics.RequestsPerSec = float64(requests) / uptime.Seconds()
	}
	
	// Get latency percentiles
	metrics.AvgReadLatency = mc.readLatencies.Average()
	metrics.AvgWriteLatency = mc.writeLatencies.Average()
	metrics.P50ReadLatency = mc.readLatencies.Percentile(0.50)
	metrics.P95ReadLatency = mc.readLatencies.Percentile(0.95)
	metrics.P99ReadLatency = mc.readLatencies.Percentile(0.99)
	metrics.P50WriteLatency = mc.writeLatencies.Percentile(0.50)
	metrics.P95WriteLatency = mc.writeLatencies.Percentile(0.95)
	metrics.P99WriteLatency = mc.writeLatencies.Percentile(0.99)
	
	return metrics
}

// CollectTimeSeries collects metrics for time series storage
func (mc *MetricsCollector) CollectTimeSeries() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	now := time.Now()
	metrics := mc.GetMetrics()
	
	// Create time series point
	point := TimeSeriesMetric{
		Timestamp:       now,
		Requests:        metrics.TotalRequests,
		Hits:            metrics.CacheHits,
		Misses:          metrics.CacheMisses,
		Writes:          metrics.CacheWrites,
		Evictions:       metrics.CacheEvictions,
		HitRate:         metrics.HitRate,
		AvgReadLatency:  metrics.AvgReadLatency,
		AvgWriteLatency: metrics.AvgWriteLatency,
		P95ReadLatency:  metrics.P95ReadLatency,
		P95WriteLatency: metrics.P95WriteLatency,
	}
	
	// Add to minute metrics
	mc.minuteMetrics = append(mc.minuteMetrics, point)
	
	// Trim old minute metrics
	cutoff := now.Add(-time.Duration(mc.retentionMinutes) * time.Minute)
	for len(mc.minuteMetrics) > 0 && mc.minuteMetrics[0].Timestamp.Before(cutoff) {
		mc.minuteMetrics = mc.minuteMetrics[1:]
	}
	
	// Aggregate to hourly if on the hour
	if now.Minute() == 0 {
		mc.aggregateHourly(now)
	}
}

// aggregateHourly aggregates minute metrics into hourly metrics
func (mc *MetricsCollector) aggregateHourly(now time.Time) {
	// Implementation would aggregate the last 60 minutes
	// For brevity, using the current point as hourly
	if len(mc.minuteMetrics) > 0 {
		hourlyPoint := mc.minuteMetrics[len(mc.minuteMetrics)-1]
		mc.hourlyMetrics = append(mc.hourlyMetrics, hourlyPoint)
		
		// Trim old hourly metrics
		cutoff := now.Add(-time.Duration(mc.retentionHours) * time.Hour)
		for len(mc.hourlyMetrics) > 0 && mc.hourlyMetrics[0].Timestamp.Before(cutoff) {
			mc.hourlyMetrics = mc.hourlyMetrics[1:]
		}
	}
}

// GetTimeSeries returns time series data for a given duration
func (mc *MetricsCollector) GetTimeSeries(duration time.Duration) []TimeSeriesMetric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	cutoff := time.Now().Add(-duration)
	
	// Return minute metrics for short durations
	if duration <= time.Hour {
		var result []TimeSeriesMetric
		for _, m := range mc.minuteMetrics {
			if m.Timestamp.After(cutoff) {
				result = append(result, m)
			}
		}
		return result
	}
	
	// Return hourly metrics for longer durations
	var result []TimeSeriesMetric
	for _, m := range mc.hourlyMetrics {
		if m.Timestamp.After(cutoff) {
			result = append(result, m)
		}
	}
	return result
}

// Reset resets all metrics
func (mc *MetricsCollector) Reset() {
	atomic.StoreInt64(&mc.totalRequests, 0)
	atomic.StoreInt64(&mc.cacheHits, 0)
	atomic.StoreInt64(&mc.cacheMisses, 0)
	atomic.StoreInt64(&mc.cacheWrites, 0)
	atomic.StoreInt64(&mc.cacheEvictions, 0)
	
	mc.readLatencies.Reset()
	mc.writeLatencies.Reset()
	
	mc.mu.Lock()
	mc.minuteMetrics = mc.minuteMetrics[:0]
	mc.hourlyMetrics = mc.hourlyMetrics[:0]
	mc.startTime = time.Now()
	mc.mu.Unlock()
}

// LatencyTracker implementation

func NewLatencyTracker(maxSize int) *LatencyTracker {
	return &LatencyTracker{
		samples: make([]time.Duration, 0, maxSize),
		maxSize: maxSize,
	}
}

func (lt *LatencyTracker) Record(latency time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	
	lt.samples = append(lt.samples, latency)
	
	// Keep only the most recent samples
	if len(lt.samples) > lt.maxSize {
		lt.samples = lt.samples[len(lt.samples)-lt.maxSize:]
	}
}

func (lt *LatencyTracker) Average() time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	
	if len(lt.samples) == 0 {
		return 0
	}
	
	var sum time.Duration
	for _, s := range lt.samples {
		sum += s
	}
	
	return sum / time.Duration(len(lt.samples))
}

func (lt *LatencyTracker) Percentile(p float64) time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	
	if len(lt.samples) == 0 {
		return 0
	}
	
	// Create a sorted copy
	sorted := make([]time.Duration, len(lt.samples))
	copy(sorted, lt.samples)
	
	// Simple bubble sort for small datasets
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	index := int(float64(len(sorted)-1) * p)
	return sorted[index]
}

func (lt *LatencyTracker) Reset() {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.samples = lt.samples[:0]
}

// FormatMetrics formats metrics for display
func FormatMetrics(m CacheMetrics) string {
	return fmt.Sprintf(`Cache Performance Metrics:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Requests:     %d total (%.2f/sec)
Hit Rate:     %.1f%% (%d hits, %d misses)
Writes:       %d (%.0f evictions)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Read Latency:  avg: %v, p50: %v, p95: %v, p99: %v
Write Latency: avg: %v, p50: %v, p95: %v, p99: %v
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Memory:       %.2f MB / %.2f MB (%.1f%% used)
Uptime:       %v
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`,
		m.TotalRequests, m.RequestsPerSec,
		m.HitRate*100, m.CacheHits, m.CacheMisses,
		m.CacheWrites, float64(m.CacheEvictions),
		m.AvgReadLatency, m.P50ReadLatency, m.P95ReadLatency, m.P99ReadLatency,
		m.AvgWriteLatency, m.P50WriteLatency, m.P95WriteLatency, m.P99WriteLatency,
		float64(m.MemoryUsed)/1024/1024, float64(m.MemoryCapacity)/1024/1024, m.MemoryUtilization*100,
		m.Uptime,
	)
}