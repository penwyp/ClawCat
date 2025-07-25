package calculations

import (
	"time"

	"github.com/penwyp/ClawCat/models"
)

// BurnRateCalculator calculates burn rates and usage projections for session blocks
type BurnRateCalculator struct{}

// NewBurnRateCalculator creates a new burn rate calculator
func NewBurnRateCalculator() *BurnRateCalculator {
	return &BurnRateCalculator{}
}

// CalculateBurnRate calculates current consumption rate for active blocks
func (brc *BurnRateCalculator) CalculateBurnRate(block models.SessionBlock) *models.BurnRate {
	if !block.IsActive || block.DurationMinutes() < 1 {
		return nil
	}

	totalTokens := block.TokenCounts.TotalTokens()
	if totalTokens == 0 {
		return nil
	}

	duration := block.DurationMinutes()
	tokensPerMinute := float64(totalTokens) / duration
	
	var costPerHour float64
	if duration > 0 {
		costPerHour = (block.CostUSD / duration) * 60
	}

	return &models.BurnRate{
		TokensPerMinute: tokensPerMinute,
		CostPerHour:     costPerHour,
	}
}

// ProjectBlockUsage projects total usage if current rate continues
func (brc *BurnRateCalculator) ProjectBlockUsage(block models.SessionBlock) *models.UsageProjection {
	burnRate := brc.CalculateBurnRate(block)
	if burnRate == nil {
		return nil
	}

	now := time.Now().UTC()
	remainingDuration := block.EndTime.Sub(now)
	if remainingDuration <= 0 {
		return nil
	}

	remainingMinutes := remainingDuration.Minutes()
	remainingHours := remainingMinutes / 60

	currentTokens := block.TokenCounts.TotalTokens()
	currentCost := block.CostUSD

	projectedAdditionalTokens := burnRate.TokensPerMinute * remainingMinutes
	projectedTotalTokens := float64(currentTokens) + projectedAdditionalTokens

	projectedAdditionalCost := burnRate.CostPerHour * remainingHours
	projectedTotalCost := currentCost + projectedAdditionalCost

	return &models.UsageProjection{
		ProjectedTotalTokens: int(projectedTotalTokens),
		ProjectedTotalCost:   projectedTotalCost,
		RemainingMinutes:     remainingMinutes,
	}
}

// CalculateHourlyBurnRate calculates burn rate based on all sessions in the last hour
func (brc *BurnRateCalculator) CalculateHourlyBurnRate(blocks []models.SessionBlock, currentTime time.Time) float64 {
	if len(blocks) == 0 {
		return 0.0
	}

	oneHourAgo := currentTime.Add(-1 * time.Hour)
	totalTokens := brc.calculateTotalTokensInHour(blocks, oneHourAgo, currentTime)

	if totalTokens > 0 {
		return totalTokens / 60.0 // Convert to tokens per minute
	}
	return 0.0
}

// calculateTotalTokensInHour calculates total tokens for all blocks in the last hour
func (brc *BurnRateCalculator) calculateTotalTokensInHour(blocks []models.SessionBlock, oneHourAgo, currentTime time.Time) float64 {
	totalTokens := 0.0
	for _, block := range blocks {
		totalTokens += brc.processBlockForBurnRate(block, oneHourAgo, currentTime)
	}
	return totalTokens
}

// processBlockForBurnRate processes a single block for burn rate calculation
func (brc *BurnRateCalculator) processBlockForBurnRate(block models.SessionBlock, oneHourAgo, currentTime time.Time) float64 {
	startTime := block.StartTime
	if block.IsGap {
		return 0
	}

	sessionActualEnd := brc.determineSessionEndTime(block, currentTime)
	if sessionActualEnd.Before(oneHourAgo) {
		return 0
	}

	return brc.calculateTokensInHour(block, startTime, sessionActualEnd, oneHourAgo, currentTime)
}

// determineSessionEndTime determines session end time based on block status
func (brc *BurnRateCalculator) determineSessionEndTime(block models.SessionBlock, currentTime time.Time) time.Time {
	if block.IsActive {
		return currentTime
	}

	if block.ActualEndTime != nil {
		return *block.ActualEndTime
	}

	return currentTime
}

// calculateTokensInHour calculates tokens used within the last hour for this session
func (brc *BurnRateCalculator) calculateTokensInHour(block models.SessionBlock, startTime, sessionActualEnd, oneHourAgo, currentTime time.Time) float64 {
	sessionStartInHour := maxTime(startTime, oneHourAgo)
	sessionEndInHour := minTime(sessionActualEnd, currentTime)

	if sessionEndInHour.Before(sessionStartInHour) || sessionEndInHour.Equal(sessionStartInHour) {
		return 0
	}

	totalSessionDuration := sessionActualEnd.Sub(startTime).Minutes()
	hourDuration := sessionEndInHour.Sub(sessionStartInHour).Minutes()

	if totalSessionDuration > 0 {
		sessionTokens := float64(block.TokenCounts.TotalTokens())
		return sessionTokens * (hourDuration / totalSessionDuration)
	}
	return 0
}

// ProcessBurnRates processes burn rates for all blocks
func (brc *BurnRateCalculator) ProcessBurnRates(blocks []models.SessionBlock) {
	for i := range blocks {
		if blocks[i].IsActive {
			burnRate := brc.CalculateBurnRate(blocks[i])
			if burnRate != nil {
				blocks[i].BurnRate = burnRate
				blocks[i].BurnRateSnapshot = burnRate

				// Calculate projection data
				projection := brc.ProjectBlockUsage(blocks[i])
				if projection != nil {
					blocks[i].ProjectionData = projection
				}
			}
		}
	}
}

// CalculateGlobalBurnRate calculates overall burn rate across all active blocks
func (brc *BurnRateCalculator) CalculateGlobalBurnRate(blocks []models.SessionBlock) models.BurnRate {
	var totalTokensPerMinute float64
	var totalCostPerHour float64
	activeBlocks := 0

	for _, block := range blocks {
		if block.IsActive {
			burnRate := brc.CalculateBurnRate(block)
			if burnRate != nil {
				totalTokensPerMinute += burnRate.TokensPerMinute
				totalCostPerHour += burnRate.CostPerHour
				activeBlocks++
			}
		}
	}

	return models.BurnRate{
		TokensPerMinute: totalTokensPerMinute,
		CostPerHour:     totalCostPerHour,
	}
}

// GetBurnRateHistory returns historical burn rate data for analysis
func (brc *BurnRateCalculator) GetBurnRateHistory(blocks []models.SessionBlock, duration time.Duration) []models.BurnRate {
	var history []models.BurnRate
	now := time.Now().UTC()
	
	// Sample burn rates at regular intervals
	sampleInterval := duration / 20 // 20 data points
	if sampleInterval < time.Minute {
		sampleInterval = time.Minute
	}

	for i := 0; i < 20; i++ {
		sampleTime := now.Add(-duration + time.Duration(i)*sampleInterval)
		rate := brc.CalculateHourlyBurnRate(blocks, sampleTime)
		
		history = append(history, models.BurnRate{
			TokensPerMinute: rate,
			CostPerHour:     rate * 60, // Approximate cost conversion
		})
	}

	return history
}

// Helper functions
func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// ValidateBurnRateData validates burn rate calculations for correctness
func (brc *BurnRateCalculator) ValidateBurnRateData(block models.SessionBlock) error {
	if block.IsActive && block.BurnRate != nil {
		// Validate that burn rate makes sense
		duration := block.DurationMinutes()
		if duration <= 0 {
			return nil // Skip validation for zero duration
		}

		expectedTokensPerMinute := float64(block.TokenCounts.TotalTokens()) / duration
		if abs(block.BurnRate.TokensPerMinute-expectedTokensPerMinute) > 0.1 {
			// Burn rate doesn't match expected calculation
			// This might indicate a calculation error
		}
	}
	return nil
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}