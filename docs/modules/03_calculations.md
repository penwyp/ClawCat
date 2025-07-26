# Module: calculations

## Overview
The calculations package provides all mathematical and statistical computations for token usage, costs, burn rates, predictions, and analytics. It's designed for high performance and accuracy with comprehensive error handling.

## Package Structure
```
calculations/
├── cost.go         # Cost calculation logic
├── stats.go        # Statistical aggregations
├── burnrate.go     # Burn rate calculations
├── predictions.go  # Usage predictions
├── analytics.go    # Advanced analytics
├── utils.go        # Utility functions
└── *_test.go       # Comprehensive tests
```

## Core Components

### Cost Calculator
Precise cost calculations with multi-currency support.

```go
type CostCalculator struct {
    pricing map[string]models.ModelPricing
    rates   map[string]float64 // Currency conversion rates
}

func NewCostCalculator() *CostCalculator
func (c *CostCalculator) Calculate(entry models.UsageEntry) (float64, error)
func (c *CostCalculator) CalculateBatch(entries []models.UsageEntry) (float64, error)
func (c *CostCalculator) CalculateWithCurrency(entry models.UsageEntry, currency string) (float64, error)
```

### Statistics Aggregator
Flexible aggregation for various time periods.

```go
type StatsAggregator struct {
    timezone *time.Location
}

type PeriodStats struct {
    Period              string
    StartTime           time.Time
    EndTime             time.Time
    InputTokens         int
    OutputTokens        int
    CacheCreationTokens int
    CacheReadTokens     int
    TotalTokens         int
    TotalCost           float64
    EntryCount          int
    ModelBreakdown      map[string]ModelStats
}

func NewStatsAggregator(tz *time.Location) *StatsAggregator
func (s *StatsAggregator) AggregateByHour(entries []models.UsageEntry) []PeriodStats
func (s *StatsAggregator) AggregateByDay(entries []models.UsageEntry) []PeriodStats
func (s *StatsAggregator) AggregateByWeek(entries []models.UsageEntry) []PeriodStats
func (s *StatsAggregator) AggregateByMonth(entries []models.UsageEntry) []PeriodStats
func (s *StatsAggregator) AggregateCustom(entries []models.UsageEntry, duration time.Duration) []PeriodStats
```

### Burn Rate Calculator
Real-time and historical burn rate analysis.

```go
type BurnRateCalculator struct {
    windowSize time.Duration
}

type BurnRate struct {
    TokensPerMinute     float64
    TokensPerHour       float64
    CostPerMinute       float64
    CostPerHour         float64
    TrendDirection      TrendType
    TrendPercentage     float64
    MovingAverage       float64
    StandardDeviation   float64
}

type TrendType int
const (
    TrendUp TrendType = iota
    TrendDown
    TrendStable
)

func NewBurnRateCalculator(window time.Duration) *BurnRateCalculator
func (b *BurnRateCalculator) Calculate(entries []models.UsageEntry) BurnRate
func (b *BurnRateCalculator) CalculateInstant(entries []models.UsageEntry, at time.Time) BurnRate
func (b *BurnRateCalculator) CalculateHistory(entries []models.UsageEntry, intervals int) []BurnRate
```

### Predictor
Advanced prediction algorithms for usage and costs.

```go
type Predictor struct {
    algorithm PredictionAlgorithm
}

type PredictionAlgorithm int
const (
    AlgorithmLinear PredictionAlgorithm = iota
    AlgorithmExponential
    AlgorithmARIMA
)

type Prediction struct {
    TimeHorizon          time.Duration
    ProjectedTokens      int
    ProjectedCost        float64
    ConfidenceInterval   ConfidenceInterval
    PredictedEndTime     time.Time
    ResourceDepletion    *time.Time
    Accuracy             float64
}

type ConfidenceInterval struct {
    Lower float64
    Upper float64
    Level float64 // e.g., 0.95 for 95%
}

func NewPredictor(algorithm PredictionAlgorithm) *Predictor
func (p *Predictor) Predict(historical []models.UsageEntry, horizon time.Duration) Prediction
func (p *Predictor) PredictSession(session models.SessionBlock, burnRate BurnRate) Prediction
```

### Analytics Engine
Advanced analytics and insights.

```go
type AnalyticsEngine struct {
    calculator *CostCalculator
    aggregator *StatsAggregator
}

type ModelDistribution struct {
    Model           string
    TokenCount      int
    TokenPercentage float64
    CostTotal       float64
    CostPercentage  float64
    UsagePattern    UsagePattern
}

type UsagePattern struct {
    PeakHours       []int
    AverageSession  time.Duration
    TokensPerQuery  float64
    CacheHitRate    float64
}

type LimitAnalysis struct {
    CurrentUsage     float64
    LimitPercentage  float64
    ProjectedLimit   time.Time
    SafeOperatingTime time.Duration
    RecommendedAction string
}

func NewAnalyticsEngine() *AnalyticsEngine
func (a *AnalyticsEngine) AnalyzeModelDistribution(entries []models.UsageEntry) []ModelDistribution
func (a *AnalyticsEngine) AnalyzeUsagePatterns(entries []models.UsageEntry) UsagePattern
func (a *AnalyticsEngine) AnalyzeLimits(entries []models.UsageEntry, plan models.Plan) LimitAnalysis
func (a *AnalyticsEngine) DetectAnomalies(entries []models.UsageEntry) []Anomaly
```

## Utility Functions

```go
// Time calculations
func MinutesUntilReset(sessionStart time.Time) float64
func CalculateElapsedMinutes(start, end time.Time) float64
func RoundToInterval(t time.Time, interval time.Duration) time.Time

// Statistical helpers
func CalculatePercentile(values []float64, percentile float64) float64
func CalculateStandardDeviation(values []float64) float64
func CalculateMovingAverage(values []float64, window int) []float64

// Rate calculations
func CalculateRate(value float64, duration time.Duration, unit time.Duration) float64
func ExtrapolateValue(current float64, rate float64, duration time.Duration) float64
```

## Performance Optimizations

1. **Batch Processing**: Process multiple entries in single pass
2. **Caching**: Cache frequently used calculations
3. **Parallel Computation**: Use goroutines for independent calculations
4. **Memory Pooling**: Reuse slices and maps
5. **SIMD Operations**: Utilize SIMD for vector calculations where possible

## Usage Example

```go
package main

import (
    "github.com/penwyp/claudecat/calculations"
    "github.com/penwyp/claudecat/models"
    "time"
)

func main() {
    entries := loadEntries() // Load from fileio
    
    // Calculate costs
    calc := calculations.NewCostCalculator()
    totalCost, _ := calc.CalculateBatch(entries)
    
    // Calculate burn rate
    burnCalc := calculations.NewBurnRateCalculator(time.Hour)
    burnRate := burnCalc.Calculate(entries)
    
    // Make predictions
    predictor := calculations.NewPredictor(calculations.AlgorithmLinear)
    prediction := predictor.Predict(entries, 5*time.Hour)
    
    // Analyze usage
    engine := calculations.NewAnalyticsEngine()
    distribution := engine.AnalyzeModelDistribution(entries)
    limits := engine.AnalyzeLimits(entries, models.Plans["pro"])
}
```

## Testing Requirements

1. **Unit Tests**:
   - All calculation functions with various inputs
   - Edge cases (empty data, single entry, large datasets)
   - Precision tests for financial calculations
   - Time zone handling

2. **Property-Based Tests**:
   - Calculation invariants
   - Statistical properties
   - Prediction bounds

3. **Benchmarks**:
   - Large dataset processing
   - Real-time calculation performance
   - Memory allocation tracking

## Accuracy Requirements

1. **Cost Calculations**: 
   - Precision: 6 decimal places
   - Rounding: Banker's rounding
   - Currency: ISO 4217 compliance

2. **Statistical Calculations**:
   - Floating point: IEEE 754 compliance
   - Aggregations: Exact for integers
   - Percentiles: Linear interpolation

3. **Predictions**:
   - Confidence intervals: Statistical validity
   - Time calculations: Nanosecond precision
   - Trend detection: Configurable sensitivity