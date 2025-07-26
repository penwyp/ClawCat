package calculations

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/penwyp/claudecat/config"
	"github.com/penwyp/claudecat/models"
)

// EnhancedRealtimeMetrics provides comprehensive real-time metrics aligned with Claude Monitor
type EnhancedRealtimeMetrics struct {
	// Session information
	SessionStart    time.Time     `json:"session_start"`
	SessionEnd      time.Time     `json:"session_end"`
	SessionProgress float64       `json:"session_progress"` // 0-100%
	TimeRemaining   time.Duration `json:"time_remaining"`
	IsActive        bool          `json:"is_active"`

	// Current usage
	CurrentTokens int     `json:"current_tokens"`
	CurrentCost   float64 `json:"current_cost"`

	// Burn rate metrics (aligned with Claude Monitor's BurnRate)
	BurnRate *models.BurnRate `json:"burn_rate,omitempty"`

	// Usage projections (aligned with Claude Monitor's UsageProjection)
	Projection *models.UsageProjection `json:"projection,omitempty"`

	// Model distribution (enhanced to match Claude Monitor format)
	ModelDistribution map[string]EnhancedModelMetrics `json:"model_distribution"`

	// Time-based rates
	TokensPerMinute float64 `json:"tokens_per_minute"`
	TokensPerHour   float64 `json:"tokens_per_hour"`
	CostPerMinute   float64 `json:"cost_per_minute"`
	CostPerHour     float64 `json:"cost_per_hour"`

	// Confidence and health metrics
	ConfidenceLevel float64 `json:"confidence_level"` // 0-100%
	HealthStatus    string  `json:"health_status"`    // "healthy", "warning", "critical"

	// Metadata
	LastUpdated     time.Time `json:"last_updated"`
	DataPoints      int       `json:"data_points"`
	CalculationTime float64   `json:"calculation_time_ms"`
}

// EnhancedModelMetrics provides detailed model usage statistics
type EnhancedModelMetrics struct {
	TokenCounts models.TokenCounts `json:"token_counts"`
	TotalTokens int                `json:"total_tokens"`
	Cost        float64            `json:"cost"`
	Percentage  float64            `json:"percentage"`
	LastUsed    time.Time          `json:"last_used"`
	EntryCount  int                `json:"entry_count"`
	BurnRate    *models.BurnRate   `json:"burn_rate,omitempty"`
}

// EnhancedMetricsCalculator provides real-time metrics calculation aligned with Claude Monitor
type EnhancedMetricsCalculator struct {
	mu            sync.RWMutex
	burnRateCalc  *BurnRateCalculator
	config        *config.Config
	sessionBlocks []models.SessionBlock

	// Cache management
	cacheEnabled   bool
	lastCalculated time.Time
	cachedMetrics  *EnhancedRealtimeMetrics

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Configuration
	updateInterval   time.Duration
	confidenceWindow time.Duration
}

// NewEnhancedMetricsCalculator creates a new enhanced metrics calculator
func NewEnhancedMetricsCalculator(cfg *config.Config) *EnhancedMetricsCalculator {
	ctx, cancel := context.WithCancel(context.Background())

	return &EnhancedMetricsCalculator{
		burnRateCalc:     NewBurnRateCalculator(),
		config:           cfg,
		sessionBlocks:    make([]models.SessionBlock, 0),
		cacheEnabled:     true,
		updateInterval:   10 * time.Second,
		confidenceWindow: 1 * time.Hour,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// UpdateSessionBlocks updates the session blocks for metrics calculation
func (emc *EnhancedMetricsCalculator) UpdateSessionBlocks(blocks []models.SessionBlock) {
	emc.mu.Lock()
	defer emc.mu.Unlock()

	emc.sessionBlocks = make([]models.SessionBlock, len(blocks))
	copy(emc.sessionBlocks, blocks)

	// Invalidate cache
	emc.cachedMetrics = nil
}

// Calculate computes comprehensive real-time metrics
func (emc *EnhancedMetricsCalculator) Calculate() *EnhancedRealtimeMetrics {
	emc.mu.Lock()
	defer emc.mu.Unlock()

	now := time.Now()
	calculationStart := now

	// Check cache
	if emc.cacheEnabled && emc.cachedMetrics != nil {
		return emc.cachedMetrics
	}

	// Find active session
	var activeBlock *models.SessionBlock
	for i := range emc.sessionBlocks {
		if emc.sessionBlocks[i].IsActive && !emc.sessionBlocks[i].IsGap {
			activeBlock = &emc.sessionBlocks[i]
			break
		}
	}

	metrics := &EnhancedRealtimeMetrics{
		LastUpdated:       now,
		ModelDistribution: make(map[string]EnhancedModelMetrics),
		DataPoints:        len(emc.sessionBlocks),
	}

	if activeBlock != nil {
		emc.calculateActiveSessionMetrics(metrics, activeBlock, now)
	} else {
		emc.calculateInactiveMetrics(metrics, now)
	}

	// Calculate model distribution
	emc.calculateModelDistribution(metrics, activeBlock)

	// Calculate confidence level
	emc.calculateConfidenceLevel(metrics)

	// Determine health status
	emc.determineHealthStatus(metrics)

	// Calculate burn rates using BurnRateCalculator
	if activeBlock != nil {
		metrics.BurnRate = emc.burnRateCalc.CalculateBurnRate(*activeBlock)
		metrics.Projection = emc.burnRateCalc.ProjectBlockUsage(*activeBlock)
	}

	// Calculate processing time
	metrics.CalculationTime = float64(time.Since(calculationStart).Nanoseconds()) / 1e6

	// Update cache
	emc.cachedMetrics = metrics
	emc.lastCalculated = now

	return metrics
}

// calculateActiveSessionMetrics calculates metrics for an active session
func (emc *EnhancedMetricsCalculator) calculateActiveSessionMetrics(
	metrics *EnhancedRealtimeMetrics,
	activeBlock *models.SessionBlock,
	now time.Time,
) {
	metrics.SessionStart = activeBlock.StartTime
	metrics.SessionEnd = activeBlock.EndTime
	metrics.IsActive = true
	metrics.CurrentTokens = activeBlock.TokenCounts.TotalTokens()
	metrics.CurrentCost = activeBlock.CostUSD

	// Calculate session progress
	elapsed := now.Sub(activeBlock.StartTime)
	totalDuration := activeBlock.EndTime.Sub(activeBlock.StartTime)
	if totalDuration > 0 {
		metrics.SessionProgress = math.Min(100, (elapsed.Seconds()/totalDuration.Seconds())*100)
	}

	// Calculate time remaining
	metrics.TimeRemaining = activeBlock.EndTime.Sub(now)
	if metrics.TimeRemaining < 0 {
		metrics.TimeRemaining = 0
	}

	// Calculate rates
	sessionDuration := activeBlock.DurationMinutes()
	if sessionDuration > 0 {
		metrics.TokensPerMinute = float64(metrics.CurrentTokens) / sessionDuration
		metrics.CostPerMinute = metrics.CurrentCost / sessionDuration
		metrics.TokensPerHour = metrics.TokensPerMinute * 60
		metrics.CostPerHour = metrics.CostPerMinute * 60
	}
}

// calculateInactiveMetrics calculates metrics when no active session exists
func (emc *EnhancedMetricsCalculator) calculateInactiveMetrics(
	metrics *EnhancedRealtimeMetrics,
	now time.Time,
) {
	metrics.IsActive = false
	metrics.SessionProgress = 0
	metrics.TimeRemaining = 0
	metrics.CurrentTokens = 0
	metrics.CurrentCost = 0
	metrics.TokensPerMinute = 0
	metrics.TokensPerHour = 0
	metrics.CostPerMinute = 0
	metrics.CostPerHour = 0

	// Find the most recent session
	var mostRecentBlock *models.SessionBlock
	for i := range emc.sessionBlocks {
		if !emc.sessionBlocks[i].IsGap {
			if mostRecentBlock == nil ||
				emc.sessionBlocks[i].StartTime.After(mostRecentBlock.StartTime) {
				mostRecentBlock = &emc.sessionBlocks[i]
			}
		}
	}

	if mostRecentBlock != nil {
		metrics.SessionStart = mostRecentBlock.StartTime
		metrics.SessionEnd = mostRecentBlock.EndTime
	}
}

// calculateModelDistribution calculates per-model usage statistics
func (emc *EnhancedMetricsCalculator) calculateModelDistribution(
	metrics *EnhancedRealtimeMetrics,
	activeBlock *models.SessionBlock,
) {
	if activeBlock == nil {
		return
	}

	totalTokens := activeBlock.TokenCounts.TotalTokens()

	for model, stats := range activeBlock.PerModelStats {
		modelMetrics := EnhancedModelMetrics{
			TokenCounts: models.TokenCounts{
				InputTokens:         getIntFromMap(stats, "input_tokens"),
				OutputTokens:        getIntFromMap(stats, "output_tokens"),
				CacheCreationTokens: getIntFromMap(stats, "cache_creation_tokens"),
				CacheReadTokens:     getIntFromMap(stats, "cache_read_tokens"),
			},
			Cost:       getFloatFromMap(stats, "cost_usd"),
			EntryCount: getIntFromMap(stats, "entries_count"),
		}

		modelMetrics.TotalTokens = modelMetrics.TokenCounts.TotalTokens()

		// Calculate percentage
		if totalTokens > 0 {
			modelMetrics.Percentage = float64(modelMetrics.TotalTokens) / float64(totalTokens) * 100
		}

		// Find last usage time
		modelMetrics.LastUsed = emc.findLastUsageTime(model, *activeBlock)

		metrics.ModelDistribution[model] = modelMetrics
	}
}

// calculateConfidenceLevel calculates confidence in the metrics
func (emc *EnhancedMetricsCalculator) calculateConfidenceLevel(metrics *EnhancedRealtimeMetrics) {
	dataPoints := float64(metrics.DataPoints)
	if dataPoints == 0 {
		metrics.ConfidenceLevel = 0
		return
	}

	// Base confidence on data points (more data = higher confidence)
	baseConfidence := math.Min(100, dataPoints/10.0*100)

	// Adjust based on time range
	var timeConfidence float64 = 100
	if metrics.IsActive {
		elapsed := time.Since(metrics.SessionStart).Minutes()
		timeConfidence = math.Min(100, elapsed/60.0*100) // 1 hour for 100% confidence
	}

	// Combined confidence (weighted average)
	metrics.ConfidenceLevel = math.Min(100, (baseConfidence*0.6 + timeConfidence*0.4))
}

// determineHealthStatus determines the health status of the metrics
func (emc *EnhancedMetricsCalculator) determineHealthStatus(metrics *EnhancedRealtimeMetrics) {
	// Default to healthy
	metrics.HealthStatus = "healthy"

	// Check for warning conditions
	if metrics.ConfidenceLevel < 50 {
		metrics.HealthStatus = "warning"
	}

	// Check for critical conditions
	if !metrics.IsActive && metrics.DataPoints == 0 {
		metrics.HealthStatus = "critical"
	}

	// Check cost burn rate
	if metrics.BurnRate != nil && metrics.BurnRate.CostPerHour > 50 { // Arbitrary threshold
		if metrics.HealthStatus == "healthy" {
			metrics.HealthStatus = "warning"
		}
	}
}

// findLastUsageTime finds the last usage time for a specific model
func (emc *EnhancedMetricsCalculator) findLastUsageTime(model string, block models.SessionBlock) time.Time {
	var lastUsed time.Time

	for _, entry := range block.Entries {
		if models.NormalizeModelName(entry.Model) == model &&
			entry.Timestamp.After(lastUsed) {
			lastUsed = entry.Timestamp
		}
	}

	return lastUsed
}

// GetCurrentBurnRate returns the current system-wide burn rate
func (emc *EnhancedMetricsCalculator) GetCurrentBurnRate() models.BurnRate {
	emc.mu.RLock()
	defer emc.mu.RUnlock()

	return emc.burnRateCalc.CalculateGlobalBurnRate(emc.sessionBlocks)
}

// GetProjectedUsage returns projected usage for active sessions
func (emc *EnhancedMetricsCalculator) GetProjectedUsage() []*models.UsageProjection {
	emc.mu.RLock()
	defer emc.mu.RUnlock()

	var projections []*models.UsageProjection

	for _, block := range emc.sessionBlocks {
		if block.IsActive && !block.IsGap {
			if projection := emc.burnRateCalc.ProjectBlockUsage(block); projection != nil {
				projections = append(projections, projection)
			}
		}
	}

	return projections
}

// InvalidateCache forces recalculation on next call
func (emc *EnhancedMetricsCalculator) InvalidateCache() {
	emc.mu.Lock()
	defer emc.mu.Unlock()
	emc.cachedMetrics = nil
}

// Close shuts down the metrics calculator
func (emc *EnhancedMetricsCalculator) Close() {
	emc.cancel()
}

// Helper functions
func getIntFromMap(m map[string]any, key string) int {
	if val, ok := m[key]; ok {
		if intVal, ok := val.(int); ok {
			return intVal
		}
	}
	return 0
}

func getFloatFromMap(m map[string]any, key string) float64 {
	if val, ok := m[key]; ok {
		if floatVal, ok := val.(float64); ok {
			return floatVal
		}
	}
	return 0.0
}
