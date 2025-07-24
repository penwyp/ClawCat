package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageEntry_Validate(t *testing.T) {
	tests := []struct {
		name    string
		entry   UsageEntry
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid entry",
			entry: UsageEntry{
				Timestamp:    time.Now().Add(-time.Hour),
				Model:        ModelSonnet,
				InputTokens:  100,
				OutputTokens: 50,
				TotalTokens:  150,
				CostUSD:      0.01,
			},
			wantErr: false,
		},
		{
			name: "zero timestamp",
			entry: UsageEntry{
				Model:       ModelSonnet,
				InputTokens: 100,
			},
			wantErr: true,
			errMsg:  "timestamp cannot be zero",
		},
		{
			name: "future timestamp",
			entry: UsageEntry{
				Timestamp:   time.Now().Add(time.Hour),
				Model:       ModelSonnet,
				InputTokens: 100,
			},
			wantErr: true,
			errMsg:  "timestamp cannot be in the future",
		},
		{
			name: "empty model",
			entry: UsageEntry{
				Timestamp:   time.Now(),
				InputTokens: 100,
			},
			wantErr: true,
			errMsg:  "model cannot be empty",
		},
		{
			name: "negative input tokens",
			entry: UsageEntry{
				Timestamp:   time.Now(),
				Model:       ModelSonnet,
				InputTokens: -100,
			},
			wantErr: true,
			errMsg:  "input tokens cannot be negative",
		},
		{
			name: "negative output tokens",
			entry: UsageEntry{
				Timestamp:    time.Now(),
				Model:        ModelSonnet,
				InputTokens:  100,
				OutputTokens:  -50,
			},
			wantErr: true,
			errMsg:  "output tokens cannot be negative",
		},
		{
			name: "negative cache creation tokens",
			entry: UsageEntry{
				Timestamp:           time.Now(),
				Model:               ModelSonnet,
				InputTokens:         100,
				CacheCreationTokens: -10,
			},
			wantErr: true,
			errMsg:  "cache creation tokens cannot be negative",
		},
		{
			name: "negative cache read tokens",
			entry: UsageEntry{
				Timestamp:       time.Now(),
				Model:           ModelSonnet,
				InputTokens:     100,
				CacheReadTokens: -5,
			},
			wantErr: true,
			errMsg:  "cache read tokens cannot be negative",
		},
		{
			name: "zero total tokens",
			entry: UsageEntry{
				Timestamp: time.Now(),
				Model:     ModelSonnet,
			},
			wantErr: true,
			errMsg:  "at least one token type must be greater than zero",
		},
		{
			name: "mismatched total tokens",
			entry: UsageEntry{
				Timestamp:   time.Now(),
				Model:       ModelSonnet,
				InputTokens: 100,
				TotalTokens: 200, // Should be 100
			},
			wantErr: true,
			errMsg:  "total tokens does not match sum of individual token types",
		},
		{
			name: "negative cost",
			entry: UsageEntry{
				Timestamp:   time.Now(),
				Model:       ModelSonnet,
				InputTokens: 100,
				CostUSD:     -0.01,
			},
			wantErr: true,
			errMsg:  "cost cannot be negative",
		},
		{
			name: "valid entry with cache tokens",
			entry: UsageEntry{
				Timestamp:           time.Now(),
				Model:               ModelOpus,
				InputTokens:         100,
				OutputTokens:        50,
				CacheCreationTokens: 20,
				CacheReadTokens:     10,
				TotalTokens:         180,
				CostUSD:             0.05,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSessionBlock_Validate(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name    string
		session SessionBlock
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid session",
			session: SessionBlock{
				StartTime:   now,
				EndTime:     now.Add(3 * time.Hour),
				TotalCost:   0.10,
				TotalTokens: 1000,
			},
			wantErr: false,
		},
		{
			name: "zero start time",
			session: SessionBlock{
				EndTime: now,
			},
			wantErr: true,
			errMsg:  "start time cannot be zero",
		},
		{
			name: "zero end time",
			session: SessionBlock{
				StartTime: now,
			},
			wantErr: true,
			errMsg:  "end time cannot be zero",
		},
		{
			name: "end before start",
			session: SessionBlock{
				StartTime: now,
				EndTime:   now.Add(-time.Hour),
			},
			wantErr: true,
			errMsg:  "end time cannot be before start time",
		},
		{
			name: "session too long",
			session: SessionBlock{
				StartTime: now,
				EndTime:   now.Add(6 * time.Hour),
				IsGap:     false,
			},
			wantErr: true,
			errMsg:  "session duration exceeds maximum allowed",
		},
		{
			name: "valid gap session",
			session: SessionBlock{
				StartTime: now,
				EndTime:   now.Add(10 * time.Hour),
				IsGap:     true,
			},
			wantErr: false,
		},
		{
			name: "negative total cost",
			session: SessionBlock{
				StartTime: now,
				EndTime:   now.Add(time.Hour),
				TotalCost: -0.01,
			},
			wantErr: true,
			errMsg:  "total cost cannot be negative",
		},
		{
			name: "negative total tokens",
			session: SessionBlock{
				StartTime:   now,
				EndTime:     now.Add(time.Hour),
				TotalTokens: -100,
			},
			wantErr: true,
			errMsg:  "total tokens cannot be negative",
		},
		{
			name: "valid session with model stats",
			session: SessionBlock{
				StartTime:   now,
				EndTime:     now.Add(2 * time.Hour),
				TotalCost:   0.05,
				TotalTokens: 500,
				ModelStats: map[string]ModelStat{
					ModelSonnet: {
						InputTokens:  300,
						OutputTokens:  200,
						TotalTokens:  500,
						Cost:         0.05,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "mismatched total cost",
			session: SessionBlock{
				StartTime:   now,
				EndTime:     now.Add(time.Hour),
				TotalCost:   0.10, // Should be 0.05
				TotalTokens: 500,
				ModelStats: map[string]ModelStat{
					ModelSonnet: {
						InputTokens: 300,
						OutputTokens: 200,
						TotalTokens: 500,
						Cost:        0.05,
					},
				},
			},
			wantErr: true,
			errMsg:  "total cost does not match sum of model costs",
		},
		{
			name: "mismatched total tokens",
			session: SessionBlock{
				StartTime:   now,
				EndTime:     now.Add(time.Hour),
				TotalCost:   0.05,
				TotalTokens: 1000, // Should be 500
				ModelStats: map[string]ModelStat{
					ModelSonnet: {
						InputTokens: 300,
						OutputTokens: 200,
						TotalTokens: 500,
						Cost:        0.05,
					},
				},
			},
			wantErr: true,
			errMsg:  "total tokens does not match sum of model tokens",
		},
		{
			name: "empty model name in stats",
			session: SessionBlock{
				StartTime: now,
				EndTime:   now.Add(time.Hour),
				ModelStats: map[string]ModelStat{
					"": {
						TotalTokens: 100,
					},
				},
			},
			wantErr: true,
			errMsg:  "model name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestModelPricing_Validate(t *testing.T) {
	tests := []struct {
		name    string
		pricing ModelPricing
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid pricing",
			pricing: ModelPricing{
				Input:         3.00,
				Output:        15.00,
				CacheCreation: 3.75,
				CacheRead:     0.30,
			},
			wantErr: false,
		},
		{
			name: "negative input price",
			pricing: ModelPricing{
				Input:  -1.00,
				Output: 5.00,
			},
			wantErr: true,
			errMsg:  "input price cannot be negative",
		},
		{
			name: "negative output price",
			pricing: ModelPricing{
				Input:  1.00,
				Output: -5.00,
			},
			wantErr: true,
			errMsg:  "output price cannot be negative",
		},
		{
			name: "negative cache creation price",
			pricing: ModelPricing{
				Input:         1.00,
				Output:        5.00,
				CacheCreation: -0.50,
			},
			wantErr: true,
			errMsg:  "cache creation price cannot be negative",
		},
		{
			name: "negative cache read price",
			pricing: ModelPricing{
				Input:     1.00,
				Output:    5.00,
				CacheRead: -0.10,
			},
			wantErr: true,
			errMsg:  "cache read price cannot be negative",
		},
		{
			name: "output cheaper than input",
			pricing: ModelPricing{
				Input:  5.00,
				Output: 3.00,
			},
			wantErr: true,
			errMsg:  "output price is typically higher than input price",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pricing.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	err := ValidationError{
		Field:   "TestField",
		Message: "test message",
	}
	
	assert.Equal(t, "validation error on field 'TestField': test message", err.Error())
}

func TestPricingError(t *testing.T) {
	err := PricingError{
		Model:   "test-model",
		Message: "pricing not found",
	}
	
	assert.Equal(t, "pricing error for model 'test-model': pricing not found", err.Error())
}