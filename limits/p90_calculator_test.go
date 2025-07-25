package limits

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/penwyp/ClawCat/config"
)

func TestP90Calculator_Calculate(t *testing.T) {
	calc := NewP90Calculator()

	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{
			name:     "Empty slice",
			values:   []float64{},
			expected: 0,
		},
		{
			name:     "Single value",
			values:   []float64{10.0},
			expected: 10.0,
		},
		{
			name:     "Simple ascending",
			values:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			expected: 10.0, // 90th percentile of 10 values
		},
		{
			name:     "Unsorted values",
			values:   []float64{5, 1, 9, 3, 7, 2, 8, 4, 6, 10},
			expected: 10.0,
		},
		{
			name:     "With duplicates",
			values:   []float64{1, 1, 2, 2, 3, 3, 4, 4, 5, 5},
			expected: 5.0,
		},
		{
			name:     "Large dataset",
			values:   generateSequence(1, 100, 1),
			expected: 90.0, // 90th percentile of 1-100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.Calculate(tt.values)
			if math.Abs(result-tt.expected) > 0.001 {
				t.Errorf("Calculate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestP90Calculator_Percentile(t *testing.T) {
	calc := NewP90Calculator()
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		percentile float64
		expected   float64
	}{
		{0, 1.0},
		{25, 3.25},
		{50, 5.5},
		{75, 7.75},
		{90, 9.1},
		{95, 9.55},
		{100, 10.0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("P%.0f", tt.percentile), func(t *testing.T) {
			result := calc.percentile(values, tt.percentile)
			if math.Abs(result-tt.expected) > 0.001 {
				t.Errorf("percentile(%.0f) = %v, want %v", tt.percentile, result, tt.expected)
			}
		})
	}
}

func TestP90Calculator_CalculateWithOutlierRemoval(t *testing.T) {
	calc := NewP90Calculator()

	// Test with outliers
	values := append(generateSequence(1, 10, 1), 100, 200, 300) // Add extreme outliers

	regular := calc.Calculate(values)
	withoutOutliers := calc.CalculateWithOutlierRemoval(values)

	// Result without outliers should be lower than with outliers
	if withoutOutliers >= regular {
		t.Errorf("Expected outlier removal to reduce P90: regular=%v, filtered=%v", regular, withoutOutliers)
	}

	// Test with insufficient samples (should fall back to regular calculation)
	smallValues := []float64{1, 2, 3}
	result1 := calc.Calculate(smallValues)
	result2 := calc.CalculateWithOutlierRemoval(smallValues)

	if result1 != result2 {
		t.Errorf("With insufficient samples, results should be equal: %v vs %v", result1, result2)
	}
}

func TestP90Calculator_AnalyzeDistribution(t *testing.T) {
	calc := NewP90Calculator()
	values := generateSequence(1, 100, 1)

	dist := calc.AnalyzeDistribution(values)

	// Check basic statistics
	if dist.Min != 1.0 {
		t.Errorf("Expected min 1.0, got %v", dist.Min)
	}
	if dist.Max != 100.0 {
		t.Errorf("Expected max 100.0, got %v", dist.Max)
	}
	if math.Abs(dist.Mean-50.5) > 0.1 {
		t.Errorf("Expected mean ~50.5, got %v", dist.Mean)
	}
	if math.Abs(dist.Median-50.5) > 0.1 {
		t.Errorf("Expected median ~50.5, got %v", dist.Median)
	}

	// Check percentiles are in ascending order
	if !(dist.P25 < dist.Median && dist.Median < dist.P75 && dist.P75 < dist.P90 && dist.P90 < dist.P95 && dist.P95 < dist.P99) {
		t.Error("Percentiles should be in ascending order")
	}

	// Test empty distribution
	emptyDist := calc.AnalyzeDistribution([]float64{})
	if emptyDist.Min != 0 || emptyDist.Max != 0 || emptyDist.Mean != 0 {
		t.Error("Empty distribution should have all zeros")
	}
}

func TestP90Calculator_Mean(t *testing.T) {
	calc := NewP90Calculator()

	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{
			name:     "Empty",
			values:   []float64{},
			expected: 0,
		},
		{
			name:     "Single value",
			values:   []float64{5.0},
			expected: 5.0,
		},
		{
			name:     "Multiple values",
			values:   []float64{1, 2, 3, 4, 5},
			expected: 3.0,
		},
		{
			name:     "With decimals",
			values:   []float64{1.5, 2.5, 3.5},
			expected: 2.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.mean(tt.values)
			if math.Abs(result-tt.expected) > 0.001 {
				t.Errorf("mean() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestP90Calculator_StdDev(t *testing.T) {
	calc := NewP90Calculator()

	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{
			name:     "Empty",
			values:   []float64{},
			expected: 0,
		},
		{
			name:     "Single value",
			values:   []float64{5.0},
			expected: 0,
		},
		{
			name:     "Same values",
			values:   []float64{5, 5, 5, 5},
			expected: 0,
		},
		{
			name:     "Known standard deviation",
			values:   []float64{1, 2, 3, 4, 5},
			expected: math.Sqrt(2.5), // Standard deviation of 1,2,3,4,5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.stdDev(tt.values)
			if math.Abs(result-tt.expected) > 0.001 {
				t.Errorf("stdDev() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestP90Calculator_ValidateHistoricalData(t *testing.T) {
	calc := NewP90Calculator()

	tests := []struct {
		name         string
		values       []float64
		expectValid  bool
		expectIssues int
	}{
		{
			name:         "Insufficient data",
			values:       []float64{1, 2, 3},
			expectValid:  false,
			expectIssues: 1,
		},
		{
			name:         "Valid data",
			values:       generateSequence(1, 20, 1),
			expectValid:  true,
			expectIssues: 0,
		},
		{
			name:         "Too many zeros",
			values:       append(generateSequence(0, 0, 1), generateSequence(1, 5, 1)...),
			expectValid:  false,
			expectIssues: 1,
		},
		{
			name:         "Low variability",
			values:       []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10},
			expectValid:  false,
			expectIssues: 1,
		},
		{
			name:         "Extreme outliers",
			values:       append(generateSequence(1, 15, 1), 1000, 2000), // Extreme outliers
			expectValid:  false,
			expectIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, issues := calc.ValidateHistoricalData(tt.values)

			if valid != tt.expectValid {
				t.Errorf("ValidateHistoricalData() valid = %v, want %v", valid, tt.expectValid)
			}

			if len(issues) != tt.expectIssues {
				t.Errorf("ValidateHistoricalData() issues count = %v, want %v. Issues: %v",
					len(issues), tt.expectIssues, issues)
			}
		})
	}
}

func TestLimitManager_CalculateP90Limit(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan:            "custom",
			CustomCostLimit: 0, // Will be set by P90 calculation
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Test with insufficient data
	_, err = lm.CalculateP90Limit()
	if err == nil {
		t.Error("Expected error for insufficient historical data")
	}

	// Add historical data
	lm.mu.Lock()
	lm.history = []HistoricalUsage{
		{Cost: 10.0}, {Cost: 12.0}, {Cost: 8.0}, {Cost: 15.0}, {Cost: 11.0},
		{Cost: 9.0}, {Cost: 13.0}, {Cost: 14.0}, {Cost: 10.5}, {Cost: 12.5},
		{Cost: 11.5}, {Cost: 9.5}, {Cost: 13.5},
	}
	lm.mu.Unlock()

	limit, err := lm.CalculateP90Limit()
	if err != nil {
		t.Fatalf("Failed to calculate P90 limit: %v", err)
	}

	if limit <= 0 {
		t.Error("P90 limit should be positive")
	}

	// P90 limit should be higher than the 90th percentile of the data
	costs := make([]float64, len(lm.history))
	for i, h := range lm.history {
		costs[i] = h.Cost
	}
	p90 := lm.p90Calculator.Calculate(costs)
	expectedLimit := p90 * 1.1 // 10% buffer

	if math.Abs(limit-expectedLimit) > 0.001 {
		t.Errorf("Expected limit %v, got %v", expectedLimit, limit)
	}
}

func TestLimitManager_GetDistributionAnalysis(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Test with no historical data
	dist := lm.GetDistributionAnalysis()
	if dist.Min != 0 || dist.Max != 0 {
		t.Error("Empty distribution should have zeros")
	}

	// Add historical data
	lm.mu.Lock()
	lm.history = []HistoricalUsage{
		{Cost: 5.0}, {Cost: 10.0}, {Cost: 15.0}, {Cost: 20.0}, {Cost: 25.0},
	}
	lm.mu.Unlock()

	dist = lm.GetDistributionAnalysis()
	if dist.Min != 5.0 {
		t.Errorf("Expected min 5.0, got %v", dist.Min)
	}
	if dist.Max != 25.0 {
		t.Errorf("Expected max 25.0, got %v", dist.Max)
	}
	if dist.Mean != 15.0 {
		t.Errorf("Expected mean 15.0, got %v", dist.Mean)
	}
}

func TestLimitManager_GetRecommendedLimit(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Test with insufficient data
	_, _, err = lm.GetRecommendedLimit()
	if err == nil {
		t.Error("Expected error for insufficient historical data")
	}

	// Add historical data
	lm.mu.Lock()
	lm.history = []HistoricalUsage{
		{Cost: 10.0}, {Cost: 12.0}, {Cost: 8.0}, {Cost: 15.0}, {Cost: 11.0},
		{Cost: 9.0}, {Cost: 13.0}, {Cost: 14.0}, {Cost: 10.5}, {Cost: 12.5},
		{Cost: 11.5}, {Cost: 9.5}, {Cost: 13.5},
	}
	lm.mu.Unlock()

	limit, description, err := lm.GetRecommendedLimit()
	if err != nil {
		t.Fatalf("Failed to get recommended limit: %v", err)
	}

	if limit <= 0 {
		t.Error("Recommended limit should be positive")
	}

	if description == "" {
		t.Error("Description should not be empty")
	}

	// Description should contain one of the expected types
	validDescriptions := []string{"Conservative", "Balanced", "Liberal"}
	found := false
	for _, validDesc := range validDescriptions {
		if len(description) > 0 && description != "" && strings.Contains(description, validDesc) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Unexpected description: %s", description)
	}
}

// Helper function to generate sequences for testing
func generateSequence(start, end float64, step float64) []float64 {
	var result []float64
	for i := start; i <= end; i += step {
		result = append(result, i)
	}
	return result
}

// Benchmark tests
func BenchmarkP90Calculator_Calculate(b *testing.B) {
	calc := NewP90Calculator()
	values := generateSequence(1, 1000, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.Calculate(values)
	}
}

func BenchmarkP90Calculator_AnalyzeDistribution(b *testing.B) {
	calc := NewP90Calculator()
	values := generateSequence(1, 1000, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.AnalyzeDistribution(values)
	}
}
