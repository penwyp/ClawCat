package models

import (
	"fmt"
	"time"
)

// ValidationError represents a validation error with field and message
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// PricingError represents an error related to pricing calculations
type PricingError struct {
	Model   string
	Message string
}

func (e PricingError) Error() string {
	return fmt.Sprintf("pricing error for model '%s': %s", e.Model, e.Message)
}

// Validate validates a UsageEntry
func (u *UsageEntry) Validate() error {
	if u.Timestamp.IsZero() {
		return ValidationError{Field: "Timestamp", Message: "timestamp cannot be zero"}
	}
	
	if u.Timestamp.After(time.Now()) {
		return ValidationError{Field: "Timestamp", Message: "timestamp cannot be in the future"}
	}
	
	if u.Model == "" {
		return ValidationError{Field: "Model", Message: "model cannot be empty"}
	}
	
	if u.InputTokens < 0 {
		return ValidationError{Field: "InputTokens", Message: "input tokens cannot be negative"}
	}
	
	if u.OutputTokens < 0 {
		return ValidationError{Field: "OutputTokens", Message: "output tokens cannot be negative"}
	}
	
	if u.CacheCreationTokens < 0 {
		return ValidationError{Field: "CacheCreationTokens", Message: "cache creation tokens cannot be negative"}
	}
	
	if u.CacheReadTokens < 0 {
		return ValidationError{Field: "CacheReadTokens", Message: "cache read tokens cannot be negative"}
	}
	
	// Validate that at least some tokens were used
	totalTokens := u.InputTokens + u.OutputTokens + u.CacheCreationTokens + u.CacheReadTokens
	if totalTokens == 0 {
		return ValidationError{Field: "Tokens", Message: "at least one token type must be greater than zero"}
	}
	
	// If TotalTokens is set, validate it matches calculated value
	if u.TotalTokens > 0 && u.TotalTokens != totalTokens {
		return ValidationError{Field: "TotalTokens", Message: "total tokens does not match sum of individual token types"}
	}
	
	if u.CostUSD < 0 {
		return ValidationError{Field: "CostUSD", Message: "cost cannot be negative"}
	}
	
	return nil
}

// Validate validates a SessionBlock
func (s *SessionBlock) Validate() error {
	if s.StartTime.IsZero() {
		return ValidationError{Field: "StartTime", Message: "start time cannot be zero"}
	}
	
	if s.EndTime.IsZero() {
		return ValidationError{Field: "EndTime", Message: "end time cannot be zero"}
	}
	
	if s.EndTime.Before(s.StartTime) {
		return ValidationError{Field: "EndTime", Message: "end time cannot be before start time"}
	}
	
	// Validate session duration (should not exceed MaxGapDuration unless it's a gap)
	duration := s.EndTime.Sub(s.StartTime)
	if !s.IsGap && duration > SessionDuration {
		return ValidationError{Field: "Duration", Message: "session duration exceeds maximum allowed"}
	}
	
	if s.TotalCost < 0 {
		return ValidationError{Field: "TotalCost", Message: "total cost cannot be negative"}
	}
	
	if s.TotalTokens < 0 {
		return ValidationError{Field: "TotalTokens", Message: "total tokens cannot be negative"}
	}
	
	// Validate ModelStats if present
	if len(s.ModelStats) > 0 {
		calculatedCost := 0.0
		calculatedTokens := 0
		
		for model, stat := range s.ModelStats {
			if model == "" {
				return ValidationError{Field: "ModelStats", Message: "model name cannot be empty"}
			}
			
			if err := validateModelStat(stat); err != nil {
				return fmt.Errorf("invalid stats for model %s: %w", model, err)
			}
			
			calculatedCost += stat.Cost
			calculatedTokens += stat.TotalTokens
		}
		
		// Allow small floating point differences for cost
		costDiff := s.TotalCost - calculatedCost
		if costDiff < -0.001 || costDiff > 0.001 {
			return ValidationError{Field: "TotalCost", Message: "total cost does not match sum of model costs"}
		}
		
		if s.TotalTokens != calculatedTokens {
			return ValidationError{Field: "TotalTokens", Message: "total tokens does not match sum of model tokens"}
		}
	}
	
	return nil
}

// Validate validates a ModelPricing
func (m *ModelPricing) Validate() error {
	if m.Input < 0 {
		return ValidationError{Field: "Input", Message: "input price cannot be negative"}
	}
	
	if m.Output < 0 {
		return ValidationError{Field: "Output", Message: "output price cannot be negative"}
	}
	
	if m.CacheCreation < 0 {
		return ValidationError{Field: "CacheCreation", Message: "cache creation price cannot be negative"}
	}
	
	if m.CacheRead < 0 {
		return ValidationError{Field: "CacheRead", Message: "cache read price cannot be negative"}
	}
	
	// Validate that output is typically more expensive than input
	if m.Output < m.Input {
		return ValidationError{Field: "Output", Message: "output price is typically higher than input price"}
	}
	
	return nil
}

// validateModelStat validates a ModelStat
func validateModelStat(stat ModelStat) error {
	if stat.InputTokens < 0 {
		return ValidationError{Field: "InputTokens", Message: "input tokens cannot be negative"}
	}
	
	if stat.OutputTokens < 0 {
		return ValidationError{Field: "OutputTokens", Message: "output tokens cannot be negative"}
	}
	
	if stat.CacheCreationTokens < 0 {
		return ValidationError{Field: "CacheCreationTokens", Message: "cache creation tokens cannot be negative"}
	}
	
	if stat.CacheReadTokens < 0 {
		return ValidationError{Field: "CacheReadTokens", Message: "cache read tokens cannot be negative"}
	}
	
	if stat.Cost < 0 {
		return ValidationError{Field: "Cost", Message: "cost cannot be negative"}
	}
	
	calculatedTotal := stat.InputTokens + stat.OutputTokens + stat.CacheCreationTokens + stat.CacheReadTokens
	if stat.TotalTokens != calculatedTotal {
		return ValidationError{Field: "TotalTokens", Message: "total tokens does not match sum of individual token types"}
	}
	
	return nil
}