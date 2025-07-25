package models

import (
	"crypto/md5"
	"fmt"
	"strings"
	"time"
)

// CostMode represents different modes for cost calculation
type CostMode int

const (
	CostModeAuto CostMode = iota
	CostModeCached
	CostModeCalculated
)

// UsageEntry represents a single token usage event from Claude API
type UsageEntry struct {
	Timestamp           time.Time `json:"timestamp"`
	Model               string    `json:"model"`
	InputTokens         int       `json:"input_tokens"`
	OutputTokens        int       `json:"output_tokens"`
	CacheCreationTokens int       `json:"cache_creation_tokens"`
	CacheReadTokens     int       `json:"cache_read_tokens"`
	TotalTokens         int       `json:"total_tokens"` // Calculated field
	CostUSD             float64   `json:"cost_usd"`     // Calculated field
	MessageID           string    `json:"message_id"`
	RequestID           string    `json:"request_id"`
	SessionID           string    `json:"session_id"`   // Claude Code session ID
}

// TokenCounts aggregates token counts with computed totals
type TokenCounts struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_tokens"`
	CacheReadTokens     int `json:"cache_read_tokens"`
}

// TotalTokens returns the sum of all token types
func (tc *TokenCounts) TotalTokens() int {
	return tc.InputTokens + tc.OutputTokens + tc.CacheCreationTokens + tc.CacheReadTokens
}

// BurnRate represents token consumption rate metrics
type BurnRate struct {
	TokensPerMinute float64 `json:"tokens_per_minute"`
	CostPerHour     float64 `json:"cost_per_hour"`
}

// UsageProjection contains projection calculations for active blocks
type UsageProjection struct {
	ProjectedTotalTokens int     `json:"projected_total_tokens"`
	ProjectedTotalCost   float64 `json:"projected_total_cost"`
	RemainingMinutes     float64 `json:"remaining_minutes"`
}

// LimitMessage represents a limit detection message
type LimitMessage struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
}

// SessionBlock represents a 5-hour session window with aggregated statistics
type SessionBlock struct {
	ID               string                     `json:"id"`
	StartTime        time.Time                  `json:"start_time"`
	EndTime          time.Time                  `json:"end_time"`
	Entries          []UsageEntry               `json:"entries"`
	TokenCounts      TokenCounts                `json:"token_counts"`
	IsActive         bool                       `json:"is_active"`
	IsGap            bool                       `json:"is_gap"`
	BurnRate         *BurnRate                  `json:"burn_rate,omitempty"`
	ActualEndTime    *time.Time                 `json:"actual_end_time,omitempty"`
	PerModelStats    map[string]map[string]any  `json:"per_model_stats"`
	Models           []string                   `json:"models"`
	SentMessagesCount int                       `json:"sent_messages_count"`
	CostUSD          float64                    `json:"cost_usd"`
	LimitMessages    []LimitMessage             `json:"limit_messages"`
	ProjectionData   *UsageProjection           `json:"projection_data,omitempty"`
	BurnRateSnapshot *BurnRate                  `json:"burn_rate_snapshot,omitempty"`
	
	// Legacy fields for backward compatibility
	TotalCost   float64              `json:"total_cost"`
	TotalTokens int                  `json:"total_tokens"`
	ModelStats  map[string]ModelStat `json:"model_stats"`
}

// ModelStat contains aggregated statistics for a specific model
type ModelStat struct {
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	TotalTokens         int     `json:"total_tokens"`
	Cost                float64 `json:"cost"`
}

// CalculateTotalTokens calculates the total tokens for a usage entry
func (u *UsageEntry) CalculateTotalTokens() int {
	return u.InputTokens + u.OutputTokens + u.CacheCreationTokens + u.CacheReadTokens
}

// CalculateCost calculates the cost for a usage entry based on model pricing
func (u *UsageEntry) CalculateCost(pricing ModelPricing) float64 {
	inputCost := float64(u.InputTokens) / 1_000_000 * pricing.Input
	outputCost := float64(u.OutputTokens) / 1_000_000 * pricing.Output
	cacheCreationCost := float64(u.CacheCreationTokens) / 1_000_000 * pricing.CacheCreation
	cacheReadCost := float64(u.CacheReadTokens) / 1_000_000 * pricing.CacheRead

	return inputCost + outputCost + cacheCreationCost + cacheReadCost
}


// NormalizeModel normalizes the model name for the entry
func (u *UsageEntry) NormalizeModel() {
	u.Model = NormalizeModelName(u.Model)
}

// AddEntry adds a usage entry to the session block
func (s *SessionBlock) AddEntry(entry UsageEntry) {
	if s.ModelStats == nil {
		s.ModelStats = make(map[string]ModelStat)
	}

	stat := s.ModelStats[entry.Model]
	stat.InputTokens += entry.InputTokens
	stat.OutputTokens += entry.OutputTokens
	stat.CacheCreationTokens += entry.CacheCreationTokens
	stat.CacheReadTokens += entry.CacheReadTokens
	stat.TotalTokens += entry.TotalTokens
	stat.Cost += entry.CostUSD

	s.ModelStats[entry.Model] = stat
	s.CalculateTotals()
}

// CalculateTotals recalculates the total cost and tokens for the session block
func (s *SessionBlock) CalculateTotals() {
	s.TotalCost = 0
	s.TotalTokens = 0

	for _, stat := range s.ModelStats {
		s.TotalCost += stat.Cost
		s.TotalTokens += stat.TotalTokens
	}
	
	// Update new fields for consistency
	s.CostUSD = s.TotalCost
	s.TokenCounts.InputTokens = 0
	s.TokenCounts.OutputTokens = 0
	s.TokenCounts.CacheCreationTokens = 0
	s.TokenCounts.CacheReadTokens = 0
	
	for _, stat := range s.ModelStats {
		s.TokenCounts.InputTokens += stat.InputTokens
		s.TokenCounts.OutputTokens += stat.OutputTokens
		s.TokenCounts.CacheCreationTokens += stat.CacheCreationTokens
		s.TokenCounts.CacheReadTokens += stat.CacheReadTokens
	}
}

// GetTotalTokens returns the total tokens from TokenCounts
func (s *SessionBlock) GetTotalTokens() int {
	return s.TokenCounts.TotalTokens()
}

// GetTotalCost returns the total cost (alias for CostUSD)
func (s *SessionBlock) GetTotalCost() float64 {
	return s.CostUSD
}

// DurationMinutes returns the duration in minutes
func (s *SessionBlock) DurationMinutes() float64 {
	var endTime time.Time
	if s.ActualEndTime != nil {
		endTime = *s.ActualEndTime
	} else {
		endTime = s.EndTime
	}
	
	duration := endTime.Sub(s.StartTime).Minutes()
	if duration < 1.0 {
		return 1.0
	}
	return duration
}

// GenerateID generates a unique ID for the session block
func (s *SessionBlock) GenerateID() string {
	idStr := fmt.Sprintf("%d-%d", s.StartTime.Unix(), s.EndTime.Unix())
	hash := md5.Sum([]byte(idStr))
	s.ID = fmt.Sprintf("%x", hash)[:12]
	return s.ID
}

// NormalizeModelName normalizes model names for consistent usage across the application
func NormalizeModelName(model string) string {
	if model == "" {
		return ""
	}

	modelLower := strings.ToLower(model)

	// Handle Claude 4 models
	if strings.Contains(modelLower, "claude-opus-4-") ||
		strings.Contains(modelLower, "claude-sonnet-4-") ||
		strings.Contains(modelLower, "claude-haiku-4-") ||
		strings.Contains(modelLower, "sonnet-4-") ||
		strings.Contains(modelLower, "opus-4-") ||
		strings.Contains(modelLower, "haiku-4-") {
		return modelLower
	}

	// Handle specific model types
	if strings.Contains(modelLower, "opus") {
		if strings.Contains(modelLower, "4-") {
			return modelLower
		}
		return "claude-3-opus"
	}
	if strings.Contains(modelLower, "sonnet") {
		if strings.Contains(modelLower, "4-") {
			return modelLower
		}
		if strings.Contains(modelLower, "3.5") || strings.Contains(modelLower, "3-5") {
			return "claude-3-5-sonnet"
		}
		return "claude-3-sonnet"
	}
	if strings.Contains(modelLower, "haiku") {
		if strings.Contains(modelLower, "3.5") || strings.Contains(modelLower, "3-5") {
			return "claude-3-5-haiku"
		}
		return "claude-3-haiku"
	}

	return model
}

// AnalysisResult represents the result of data analysis operations
type AnalysisResult struct {
	Timestamp           time.Time `json:"timestamp"`
	Model               string    `json:"model"`
	SessionID           string    `json:"session_id"`
	InputTokens         int       `json:"input_tokens"`
	OutputTokens        int       `json:"output_tokens"`
	CacheCreationTokens int       `json:"cache_creation_tokens"`
	CacheReadTokens     int       `json:"cache_read_tokens"`
	TotalTokens         int       `json:"total_tokens"`
	CostUSD             float64   `json:"cost_usd"`
	Count               int       `json:"count"` // For grouped results
	GroupKey            string    `json:"group_key,omitempty"` // For grouped results
}

// SummaryStats represents summary statistics for analysis results
type SummaryStats struct {
	StartTime           time.Time          `json:"start_time"`
	EndTime             time.Time          `json:"end_time"`
	TotalEntries        int                `json:"total_entries"`
	TotalTokens         int                `json:"total_tokens"`
	TotalCost           float64            `json:"total_cost"`
	InputTokens         int                `json:"input_tokens"`
	OutputTokens        int                `json:"output_tokens"`
	CacheCreationTokens int                `json:"cache_creation_tokens"`
	CacheReadTokens     int                `json:"cache_read_tokens"`
	MaxCost             float64            `json:"max_cost"`
	MaxTokens           int                `json:"max_tokens"`
	AvgCost             float64            `json:"avg_cost"`
	AvgTokens           float64            `json:"avg_tokens"`
	ModelCounts         map[string]int     `json:"model_counts"`
}
