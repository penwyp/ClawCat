# Module: models

## Overview
The models package defines all core data structures used throughout the application. It provides type-safe representations of Claude API usage data, session information, pricing models, and subscription plans.

## Package Structure
```
models/
├── types.go          # Core data structures
├── pricing.go        # Pricing and plan definitions
├── validation.go     # Data validation functions
├── constants.go      # Application constants
└── types_test.go     # Unit tests
```

## Core Types

### UsageEntry
Represents a single token usage event from Claude API.

```go
type UsageEntry struct {
    Timestamp           time.Time
    Model               string
    InputTokens         int
    OutputTokens        int
    CacheCreationTokens int
    CacheReadTokens     int
    TotalTokens         int     // Calculated field
    CostUSD             float64 // Calculated field
}
```

### SessionBlock
Represents a 5-hour session window with aggregated statistics.

```go
type SessionBlock struct {
    StartTime    time.Time
    EndTime      time.Time
    IsGap        bool
    TotalCost    float64
    TotalTokens  int
    ModelStats   map[string]ModelStat
}

type ModelStat struct {
    InputTokens         int
    OutputTokens        int
    CacheCreationTokens int
    CacheReadTokens     int
    TotalTokens         int
    Cost                float64
}
```

### ModelPricing
Defines token pricing for different Claude models.

```go
type ModelPricing struct {
    Input         float64 // Per million tokens
    Output        float64 // Per million tokens
    CacheCreation float64 // Per million tokens
    CacheRead     float64 // Per million tokens
}
```

### Plan
Subscription plan with token and cost limits.

```go
type Plan struct {
    Name       string
    TokenLimit int
    CostLimit  float64
}
```

## Key Methods

### Validation
```go
func (u *UsageEntry) Validate() error
func (s *SessionBlock) Validate() error
func (m *ModelPricing) Validate() error
```

### Calculations
```go
func (u *UsageEntry) CalculateTotalTokens() int
func (u *UsageEntry) CalculateCost(pricing ModelPricing) float64
func (s *SessionBlock) AddEntry(entry UsageEntry)
func (s *SessionBlock) CalculateTotals()
```

### Serialization
```go
func (u *UsageEntry) MarshalJSON() ([]byte, error)
func (u *UsageEntry) UnmarshalJSON(data []byte) error
```

## Constants

```go
const (
    SessionDuration = 5 * time.Hour
    MaxGapDuration  = 5 * time.Hour
    
    ModelOpus   = "claude-3-opus-20240229"
    ModelSonnet = "claude-3-5-sonnet-20241022"
    ModelHaiku  = "claude-3-5-haiku-20241022"
    
    PlanPro   = "pro"
    PlanMax5  = "max5"
    PlanMax20 = "max20"
)
```

## Error Types

```go
type ValidationError struct {
    Field   string
    Message string
}

type PricingError struct {
    Model   string
    Message string
}
```

## Testing Requirements

1. **Unit Tests**:
   - Validation logic for all types
   - Token calculation accuracy
   - Cost calculation precision
   - JSON marshaling/unmarshaling
   - Edge cases (zero values, negative tokens)

2. **Benchmarks**:
   - JSON parsing performance with Sonic
   - Map operations for ModelStats
   - Calculation performance

3. **Test Data**:
   - Valid and invalid usage entries
   - Various model combinations
   - Edge case scenarios

## Usage Example

```go
package main

import (
    "github.com/penwyp/claudecat/models"
    "time"
)

func main() {
    // Create a usage entry
    entry := models.UsageEntry{
        Timestamp:    time.Now(),
        Model:        models.ModelSonnet,
        InputTokens:  1000,
        OutputTokens: 500,
    }
    
    // Calculate totals
    entry.TotalTokens = entry.CalculateTotalTokens()
    entry.CostUSD = entry.CalculateCost(models.GetPricing(entry.Model))
    
    // Validate
    if err := entry.Validate(); err != nil {
        log.Fatal(err)
    }
}
```

## Performance Considerations

1. Use value types where possible to avoid allocations
2. Pre-allocate maps for ModelStats
3. Cache pricing lookups
4. Use sync.Pool for frequently created objects
5. Optimize JSON parsing with Sonic

## Future Enhancements

1. Support for new model types
2. Custom pricing configurations
3. Historical pricing data
4. Currency conversion support
5. Usage pattern detection