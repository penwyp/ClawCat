package models

import (
	"time"
)

// UsageEntry represents a single token usage event from Claude API
type UsageEntry struct {
	Timestamp           time.Time `json:"timestamp"`
	Model               string    `json:"model"`
	InputTokens         int       `json:"input_tokens"`
	OutputTokens        int       `json:"output_tokens"`
	CacheCreationTokens int       `json:"cache_creation_tokens"`
	CacheReadTokens     int       `json:"cache_read_tokens"`
	TotalTokens         int       `json:"total_tokens"`     // Calculated field
	CostUSD             float64   `json:"cost_usd"`         // Calculated field
}

// SessionBlock represents a 5-hour session window with aggregated statistics
type SessionBlock struct {
	StartTime    time.Time            `json:"start_time"`
	EndTime      time.Time            `json:"end_time"`
	IsGap        bool                 `json:"is_gap"`
	TotalCost    float64              `json:"total_cost"`
	TotalTokens  int                  `json:"total_tokens"`
	ModelStats   map[string]ModelStat `json:"model_stats"`
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
}

