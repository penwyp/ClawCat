package calculations

import (
	"fmt"
	"testing"
	"time"

	"github.com/penwyp/claudecat/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCostCalculator(t *testing.T) {
	calc := NewCostCalculator()

	require.NotNil(t, calc)
	assert.NotNil(t, calc.pricing)
	assert.NotNil(t, calc.rates)
	assert.Equal(t, 1.0, calc.rates["USD"])
}

func TestNewCostCalculatorWithRates(t *testing.T) {
	rates := map[string]float64{
		"USD": 1.0,
		"EUR": 0.85,
		"GBP": 0.73,
	}

	calc := NewCostCalculatorWithRates(rates)

	require.NotNil(t, calc)
	assert.Equal(t, rates, calc.rates)
}

func TestCostCalculator_Calculate(t *testing.T) {
	calc := NewCostCalculator()

	tests := []struct {
		name     string
		entry    models.UsageEntry
		expected CostResult
		hasError bool
	}{
		{
			name: "Sonnet model basic calculation",
			entry: models.UsageEntry{
				Model:        models.ModelSonnet,
				InputTokens:  1000000, // 1M tokens
				OutputTokens: 500000,  // 500K tokens
				TotalTokens:  1500000,
			},
			expected: CostResult{
				Model:        models.ModelSonnet,
				InputTokens:  1000000,
				OutputTokens: 500000,
				TotalTokens:  1500000,
				InputCost:    3.0,  // $3 for 1M input tokens
				OutputCost:   7.5,  // $15 for 1M tokens, so $7.5 for 500K
				TotalCost:    10.5, // $3 + $7.5
			},
		},
		{
			name: "Opus model with cache tokens",
			entry: models.UsageEntry{
				Model:               models.ModelOpus,
				InputTokens:         500000,
				OutputTokens:        200000,
				CacheCreationTokens: 100000,
				CacheReadTokens:     50000,
				TotalTokens:         850000,
			},
			expected: CostResult{
				Model:               models.ModelOpus,
				InputTokens:         500000,
				OutputTokens:        200000,
				CacheCreationTokens: 100000,
				CacheReadTokens:     50000,
				TotalTokens:         850000,
				InputCost:           7.5,    // $15 per 1M * 0.5M = $7.5
				OutputCost:          15.0,   // $75 per 1M * 0.2M = $15
				CacheCreationCost:   1.875,  // $18.75 per 1M * 0.1M = $1.875
				CacheReadCost:       0.094,  // $1.875 per 1M * 0.05M = $0.09375, rounded
				TotalCost:           24.469, // Sum of all costs, rounded
			},
		},
		{
			name: "Haiku model zero tokens",
			entry: models.UsageEntry{
				Model:        models.ModelHaiku,
				InputTokens:  0,
				OutputTokens: 0,
				TotalTokens:  0,
			},
			expected: CostResult{
				Model:        models.ModelHaiku,
				InputTokens:  0,
				OutputTokens: 0,
				TotalTokens:  0,
				InputCost:    0,
				OutputCost:   0,
				TotalCost:    0,
			},
		},
		{
			name: "Empty model name should return error",
			entry: models.UsageEntry{
				Model:        "",
				InputTokens:  1000,
				OutputTokens: 500,
			},
			hasError: true,
		},
		{
			name: "Unknown model uses default pricing",
			entry: models.UsageEntry{
				Model:        "unknown-model",
				InputTokens:  1000000,
				OutputTokens: 1000000,
				TotalTokens:  2000000,
			},
			expected: CostResult{
				Model:        "unknown-model",
				InputTokens:  1000000,
				OutputTokens: 1000000,
				TotalTokens:  2000000,
				InputCost:    3.0,  // Uses Sonnet pricing (default)
				OutputCost:   15.0, // Uses Sonnet pricing (default)
				TotalCost:    18.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calc.Calculate(tt.entry)

			if tt.hasError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.Model, result.Model)
			assert.Equal(t, tt.expected.InputTokens, result.InputTokens)
			assert.Equal(t, tt.expected.OutputTokens, result.OutputTokens)
			assert.Equal(t, tt.expected.CacheCreationTokens, result.CacheCreationTokens)
			assert.Equal(t, tt.expected.CacheReadTokens, result.CacheReadTokens)
			assert.Equal(t, tt.expected.TotalTokens, result.TotalTokens)

			// Use InDelta for floating point comparisons
			assert.InDelta(t, tt.expected.InputCost, result.InputCost, 0.001)
			assert.InDelta(t, tt.expected.OutputCost, result.OutputCost, 0.001)
			assert.InDelta(t, tt.expected.CacheCreationCost, result.CacheCreationCost, 0.001)
			assert.InDelta(t, tt.expected.CacheReadCost, result.CacheReadCost, 0.001)
			assert.InDelta(t, tt.expected.TotalCost, result.TotalCost, 0.001)
		})
	}
}

func TestCostCalculator_CalculateBatch(t *testing.T) {
	calc := NewCostCalculator()

	entries := []models.UsageEntry{
		{
			Model:        models.ModelSonnet,
			InputTokens:  500000,
			OutputTokens: 250000,
			TotalTokens:  750000,
		},
		{
			Model:        models.ModelSonnet,
			InputTokens:  300000,
			OutputTokens: 200000,
			TotalTokens:  500000,
		},
		{
			Model:        models.ModelHaiku,
			InputTokens:  1000000,
			OutputTokens: 500000,
			TotalTokens:  1500000,
		},
	}

	result, err := calc.CalculateBatch(entries)
	require.NoError(t, err)

	// Check overall totals
	assert.Equal(t, 3, result.EntryCount)
	assert.Equal(t, 2750000, result.TotalTokens)     // 750K + 500K + 1500K
	assert.InDelta(t, 11.95, result.TotalCost, 0.01) // Should be sum of all costs

	// Check model breakdown
	assert.Len(t, result.ModelResults, 2) // Sonnet and Haiku

	sonnetStats, exists := result.ModelResults[models.ModelSonnet]
	assert.True(t, exists)
	assert.Equal(t, 800000, sonnetStats.InputTokens)     // 500K + 300K
	assert.Equal(t, 450000, sonnetStats.OutputTokens)    // 250K + 200K
	assert.Equal(t, 1250000, sonnetStats.TotalTokens)    // 750K + 500K
	assert.InDelta(t, 9.15, sonnetStats.TotalCost, 0.01) // Cost for both Sonnet entries

	haikuStats, exists := result.ModelResults[models.ModelHaiku]
	assert.True(t, exists)
	assert.Equal(t, 1000000, haikuStats.InputTokens)
	assert.Equal(t, 500000, haikuStats.OutputTokens)
	assert.Equal(t, 1500000, haikuStats.TotalTokens)
	assert.InDelta(t, 2.8, haikuStats.TotalCost, 0.01) // $0.8 + $2.0

	// Check details
	assert.Len(t, result.Details, 3)
}

func TestCostCalculator_CalculateBatch_EmptySlice(t *testing.T) {
	calc := NewCostCalculator()

	_, err := calc.CalculateBatch([]models.UsageEntry{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "entries slice cannot be empty")
}

func TestCostCalculator_CalculateWithCurrency(t *testing.T) {
	rates := map[string]float64{
		"USD": 1.0,
		"EUR": 0.85,
		"GBP": 0.73,
	}
	calc := NewCostCalculatorWithRates(rates)

	entry := models.UsageEntry{
		Model:        models.ModelSonnet,
		InputTokens:  1000000,
		OutputTokens: 1000000,
		TotalTokens:  2000000,
	}

	// Test USD (base currency)
	resultUSD, err := calc.CalculateWithCurrency(entry, "USD")
	require.NoError(t, err)
	assert.InDelta(t, 18.0, resultUSD.TotalCost, 0.001) // $3 + $15

	// Test EUR conversion
	resultEUR, err := calc.CalculateWithCurrency(entry, "EUR")
	require.NoError(t, err)
	assert.InDelta(t, 15.3, resultEUR.TotalCost, 0.001) // $18 * 0.85

	// Test GBP conversion
	resultGBP, err := calc.CalculateWithCurrency(entry, "GBP")
	require.NoError(t, err)
	assert.InDelta(t, 13.14, resultGBP.TotalCost, 0.001) // $18 * 0.73

	// Test unsupported currency
	_, err = calc.CalculateWithCurrency(entry, "JPY")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported currency: JPY")
}

func TestCostCalculator_GetCostForTokens(t *testing.T) {
	calc := NewCostCalculator()

	cost, err := calc.GetCostForTokens(models.ModelSonnet, 1000000, 500000, 100000, 50000)
	require.NoError(t, err)

	// Expected: $3 (input) + $7.5 (output) + $0.375 (cache creation) + $0.015 (cache read)
	assert.InDelta(t, 10.89, cost, 0.01)
}

func TestCostCalculator_UpdatePricing(t *testing.T) {
	calc := NewCostCalculator()

	newPricing := models.ModelPricing{
		Input:         5.0,
		Output:        25.0,
		CacheCreation: 6.25,
		CacheRead:     0.5,
	}

	calc.UpdatePricing("custom-model", newPricing)

	retrieved, exists := calc.GetPricingForModel("custom-model")
	assert.True(t, exists)
	assert.Equal(t, newPricing, retrieved)
}

func TestCostCalculator_UpdateCurrencyRate(t *testing.T) {
	calc := NewCostCalculator()

	err := calc.UpdateCurrencyRate("EUR", 0.85)
	assert.NoError(t, err)

	// Test invalid rate
	err = calc.UpdateCurrencyRate("EUR", -0.5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "currency rate must be positive")

	err = calc.UpdateCurrencyRate("EUR", 0)
	assert.Error(t, err)
}

func TestCostCalculator_GetSupportedCurrencies(t *testing.T) {
	rates := map[string]float64{
		"USD": 1.0,
		"EUR": 0.85,
		"GBP": 0.73,
	}
	calc := NewCostCalculatorWithRates(rates)

	currencies := calc.GetSupportedCurrencies()
	assert.Len(t, currencies, 3)
	assert.Contains(t, currencies, "USD")
	assert.Contains(t, currencies, "EUR")
	assert.Contains(t, currencies, "GBP")
}

func TestCostCalculator_EstimateCostFromRate(t *testing.T) {
	calc := NewCostCalculator()

	estimated := calc.EstimateCostFromRate(10.0, 2.5) // $10/hour for 2.5 hours
	assert.InDelta(t, 25.0, estimated, 0.001)
}

func TestCostCalculator_CompareCosts(t *testing.T) {
	calc := NewCostCalculator()

	comparison, err := calc.CompareCosts(1000000, 500000, 0, 0, models.ModelSonnet, models.ModelHaiku)
	require.NoError(t, err)

	assert.InDelta(t, 10.5, comparison[models.ModelSonnet], 0.01) // $3 + $7.5
	assert.InDelta(t, 2.8, comparison[models.ModelHaiku], 0.01)   // $0.8 + $2.0
	assert.InDelta(t, 7.7, comparison["difference"], 0.01)        // |10.5 - 2.8|
	assert.InDelta(t, 7.7, comparison["savings"], 0.01)           // max - min
}

func TestCostCalculator_RoundCost(t *testing.T) {
	calc := NewCostCalculator()

	tests := []struct {
		input    float64
		expected float64
	}{
		{1.1234567, 1.123457},
		{1.1234564, 1.123456},
		{1.1234565, 1.123457}, // Banker's rounding: round to nearest
		{1.1234575, 1.123458}, // Banker's rounding: round to nearest
		{0.0, 0.0},
		{23.456789, 23.456789},
	}

	for _, tt := range tests {
		result := calc.roundCost(tt.input)
		assert.InDelta(t, tt.expected, result, 0.0000001, "Input: %f", tt.input)
	}
}

func TestCostCalculator_CalculateTokenCost(t *testing.T) {
	calc := NewCostCalculator()

	// Test various token amounts
	tests := []struct {
		tokens         int
		ratePerMillion float64
		expected       float64
	}{
		{1000000, 3.0, 3.0},   // 1M tokens at $3/M = $3
		{500000, 3.0, 1.5},    // 500K tokens at $3/M = $1.5
		{100, 3.0, 0.0003},    // 100 tokens at $3/M = $0.0003
		{0, 3.0, 0.0},         // 0 tokens = $0
		{2000000, 15.0, 30.0}, // 2M tokens at $15/M = $30
	}

	for _, tt := range tests {
		result := calc.calculateTokenCost(tt.tokens, tt.ratePerMillion)
		assert.InDelta(t, tt.expected, result, 0.0001,
			"Tokens: %d, Rate: %f", tt.tokens, tt.ratePerMillion)
	}
}

func TestCostCalculator_PrecisionAndRounding(t *testing.T) {
	calc := NewCostCalculator()

	// Test with very small token amounts that might cause precision issues
	entry := models.UsageEntry{
		Model:        models.ModelSonnet,
		InputTokens:  1, // 1 token
		OutputTokens: 1, // 1 token
		TotalTokens:  2,
	}

	result, err := calc.Calculate(entry)
	require.NoError(t, err)

	// 1 token of input at $3/M = $0.000003
	// 1 token of output at $15/M = $0.000015
	// Total should be $0.000018
	assert.InDelta(t, 0.000018, result.TotalCost, 0.0000001)

	// Ensure the result is properly rounded (check that it's not zero and formatted correctly)
	costStr := fmt.Sprintf("%.6f", result.TotalCost)
	assert.Contains(t, costStr, "0.000018")
}

// Helper function needed for the test above

func BenchmarkCostCalculator_Calculate(b *testing.B) {
	calc := NewCostCalculator()
	entry := models.UsageEntry{
		Model:        models.ModelSonnet,
		InputTokens:  1000000,
		OutputTokens: 500000,
		TotalTokens:  1500000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = calc.Calculate(entry)
	}
}

func BenchmarkCostCalculator_CalculateBatch(b *testing.B) {
	calc := NewCostCalculator()

	entries := make([]models.UsageEntry, 1000)
	for i := 0; i < 1000; i++ {
		entries[i] = models.UsageEntry{
			Model:        models.ModelSonnet,
			InputTokens:  1000,
			OutputTokens: 500,
			TotalTokens:  1500,
			Timestamp:    time.Now().Add(time.Duration(i) * time.Minute),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = calc.CalculateBatch(entries)
	}
}
