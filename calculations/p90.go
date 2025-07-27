package calculations

import (
	"sort"
	"sync"
	"time"

	"github.com/penwyp/claudecat/models"
)

// P90Config contains configuration for P90 calculation
type P90Config struct {
	CommonLimits    []int
	LimitThreshold  float64
	DefaultMinLimit int
	CacheTTLSeconds int
}

// DefaultP90Config returns the default P90 configuration
func DefaultP90Config() P90Config {
	return P90Config{
		CommonLimits:    []int{1000000, 2000000, 8000000}, // Pro: 1M, Max5: 2M, Max20: 8M
		LimitThreshold:  0.95,                             // 95% threshold for limit detection
		DefaultMinLimit: 1000000,                          // Default to Pro limit
		CacheTTLSeconds: 3600,                             // 1 hour cache
	}
}

// P90Calculator calculates P90 token limits from historical session data
type P90Calculator struct {
	config  P90Config
	cache   *p90Cache
	cacheMu sync.RWMutex
}

// p90Cache stores cached P90 calculations
type p90Cache struct {
	value      int
	expireTime time.Time
}

// NewP90Calculator creates a new P90 calculator with default config
func NewP90Calculator() *P90Calculator {
	return NewP90CalculatorWithConfig(DefaultP90Config())
}

// NewP90CalculatorWithConfig creates a new P90 calculator with custom config
func NewP90CalculatorWithConfig(config P90Config) *P90Calculator {
	return &P90Calculator{
		config: config,
		cache:  nil,
	}
}

// CalculateP90Limit calculates the P90 token limit from session blocks
func (p *P90Calculator) CalculateP90Limit(blocks []models.SessionBlock, useCache bool) int {
	if len(blocks) == 0 {
		return p.config.DefaultMinLimit
	}

	// Check cache if enabled
	if useCache {
		p.cacheMu.RLock()
		if p.cache != nil && time.Now().Before(p.cache.expireTime) {
			cachedValue := p.cache.value
			p.cacheMu.RUnlock()
			return cachedValue
		}
		p.cacheMu.RUnlock()
	}

	// Calculate P90
	p90Value := p.calculateP90FromBlocks(blocks)

	// Update cache
	if useCache {
		p.cacheMu.Lock()
		p.cache = &p90Cache{
			value:      p90Value,
			expireTime: time.Now().Add(time.Duration(p.config.CacheTTLSeconds) * time.Second),
		}
		p.cacheMu.Unlock()
	}

	return p90Value
}

// calculateP90FromBlocks performs the actual P90 calculation
func (p *P90Calculator) calculateP90FromBlocks(blocks []models.SessionBlock) int {
	// First try to get sessions that hit limits
	limitSessions := p.extractLimitSessions(blocks)

	// If no limit sessions, use all completed sessions
	if len(limitSessions) == 0 {
		limitSessions = p.extractAllCompletedSessions(blocks)
	}

	// If still no sessions, return default
	if len(limitSessions) == 0 {
		return p.config.DefaultMinLimit
	}

	// Sort sessions by token count
	sort.Ints(limitSessions)

	// Calculate P90 (90th percentile)
	p90Index := int(float64(len(limitSessions)) * 0.9)
	if p90Index >= len(limitSessions) {
		p90Index = len(limitSessions) - 1
	}

	p90Value := limitSessions[p90Index]

	// Ensure minimum value
	if p90Value < p.config.DefaultMinLimit {
		p90Value = p.config.DefaultMinLimit
	}

	return p90Value
}

// extractLimitSessions extracts sessions that hit token limits
func (p *P90Calculator) extractLimitSessions(blocks []models.SessionBlock) []int {
	var sessions []int

	for _, block := range blocks {
		// Skip gaps and active sessions
		if block.IsGap || block.IsActive {
			continue
		}

		totalTokens := block.TotalTokens
		if totalTokens == 0 {
			totalTokens = block.TokenCounts.TotalTokens()
		}

		// Check if session hit a limit
		if p.didHitLimit(totalTokens) {
			sessions = append(sessions, totalTokens)
		}
	}

	return sessions
}

// extractAllCompletedSessions extracts all completed sessions with tokens
func (p *P90Calculator) extractAllCompletedSessions(blocks []models.SessionBlock) []int {
	var sessions []int

	for _, block := range blocks {
		// Skip gaps and active sessions
		if block.IsGap || block.IsActive {
			continue
		}

		totalTokens := block.TotalTokens
		if totalTokens == 0 {
			totalTokens = block.TokenCounts.TotalTokens()
		}

		if totalTokens > 0 {
			sessions = append(sessions, totalTokens)
		}
	}

	return sessions
}

// didHitLimit checks if a token count indicates hitting a limit
func (p *P90Calculator) didHitLimit(tokens int) bool {
	for _, limit := range p.config.CommonLimits {
		threshold := float64(limit) * p.config.LimitThreshold
		if float64(tokens) >= threshold {
			return true
		}
	}
	return false
}

// GetCostP90 calculates P90 cost limit from session blocks
func (p *P90Calculator) GetCostP90(blocks []models.SessionBlock) float64 {
	var costs []float64

	for _, block := range blocks {
		// Skip gaps and active sessions
		if block.IsGap || block.IsActive {
			continue
		}

		if block.CostUSD > 0 {
			costs = append(costs, block.CostUSD)
		}
	}

	if len(costs) == 0 {
		return 100.0 // Default cost limit
	}

	// Sort costs
	sort.Float64s(costs)

	// Calculate P90
	p90Index := int(float64(len(costs)) * 0.9)
	if p90Index >= len(costs) {
		p90Index = len(costs) - 1
	}

	return costs[p90Index]
}

// GetMessagesP90 calculates P90 messages limit from session blocks
func (p *P90Calculator) GetMessagesP90(blocks []models.SessionBlock) int {
	var messages []int

	for _, block := range blocks {
		// Skip gaps and active sessions
		if block.IsGap || block.IsActive {
			continue
		}

		if block.SentMessagesCount > 0 {
			messages = append(messages, block.SentMessagesCount)
		}
	}

	if len(messages) == 0 {
		return 150 // Default messages limit
	}

	// Sort messages
	sort.Ints(messages)

	// Calculate P90
	p90Index := int(float64(len(messages)) * 0.9)
	if p90Index >= len(messages) {
		p90Index = len(messages) - 1
	}

	return messages[p90Index]
}
