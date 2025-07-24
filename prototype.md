# Claude Code Usage Monitor - Core Functionality Prototype

This document describes the core functionality and calculations for the Claude Code Usage Monitor, designed to facilitate rewriting the project in Golang.

## Overview

The Claude Code Usage Monitor tracks token usage, costs, and provides real-time analytics for Claude AI usage. It operates on a 5-hour session window model with multiple concurrent sessions support.

## Core Data Structures

### 1. Usage Entry
```go
type UsageEntry struct {
    Timestamp          time.Time
    Model              string
    InputTokens        int
    OutputTokens       int
    CacheCreationTokens int
    CacheReadTokens    int
    TotalTokens        int    // Calculated field
    CostUSD            float64 // Calculated field
}
```

### 2. Session Block
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
    InputTokens        int
    OutputTokens       int
    CacheCreationTokens int
    CacheReadTokens    int
    TotalTokens        int
    Cost               float64
}
```

### 3. Model Pricing
```go
type ModelPricing struct {
    Input         float64 // Per million tokens
    Output        float64 // Per million tokens
    CacheCreation float64 // Per million tokens
    CacheRead     float64 // Per million tokens
}

var ModelPrices = map[string]ModelPricing{
    "claude-3-opus-20240229": {
        Input:         15.00,
        Output:        75.00,
        CacheCreation: 18.75,
        CacheRead:     1.50,
    },
    "claude-3-5-sonnet-20241022": {
        Input:         3.00,
        Output:        15.00,
        CacheCreation: 3.75,
        CacheRead:     0.30,
    },
    "claude-3-5-haiku-20241022": {
        Input:         0.25,
        Output:        1.25,
        CacheCreation: 0.30,
        CacheRead:     0.03,
    },
}
```

### 4. Subscription Plans
```go
type Plan struct {
    Name         string
    TokenLimit   int
    CostLimit    float64
}

var Plans = map[string]Plan{
    "pro": {
        Name:       "Pro",
        TokenLimit: 19000,   // Approximate
        CostLimit:  18.00,
    },
    "max5": {
        Name:       "Max5",
        TokenLimit: 88000,   // Approximate
        CostLimit:  35.00,
    },
    "max20": {
        Name:       "Max20",
        TokenLimit: 220000,  // Approximate
        CostLimit:  140.00,
    },
}
```

## Core Calculations

### 1. Cost Calculation
```go
func CalculateCost(entry UsageEntry, pricing ModelPricing) float64 {
    return (float64(entry.InputTokens) / 1_000_000.0 * pricing.Input) +
           (float64(entry.OutputTokens) / 1_000_000.0 * pricing.Output) +
           (float64(entry.CacheCreationTokens) / 1_000_000.0 * pricing.CacheCreation) +
           (float64(entry.CacheReadTokens) / 1_000_000.0 * pricing.CacheRead)
}
```

### 2. Token Usage Tracking
```go
func CalculateTotalTokens(entry UsageEntry) int {
    return entry.InputTokens + entry.OutputTokens + 
           entry.CacheCreationTokens + entry.CacheReadTokens
}
```

### 3. Time to Reset Calculation
```go
func CalculateTimeToReset(sessionStart time.Time) time.Duration {
    // Sessions are 5-hour windows
    resetTime := sessionStart.Add(5 * time.Hour)
    return time.Until(resetTime)
}

func GetMinutesToReset(sessionStart time.Time) float64 {
    duration := CalculateTimeToReset(sessionStart)
    return duration.Minutes()
}
```

### 4. Model Distribution Analysis
```go
type ModelDistribution struct {
    Model      string
    Tokens     int
    Percentage float64
}

func CalculateModelDistribution(entries []UsageEntry) []ModelDistribution {
    modelTokens := make(map[string]int)
    totalTokens := 0
    
    for _, entry := range entries {
        tokens := entry.InputTokens + entry.OutputTokens
        modelTokens[entry.Model] += tokens
        totalTokens += tokens
    }
    
    var distribution []ModelDistribution
    for model, tokens := range modelTokens {
        percentage := float64(tokens) / float64(totalTokens) * 100.0
        distribution = append(distribution, ModelDistribution{
            Model:      model,
            Tokens:     tokens,
            Percentage: percentage,
        })
    }
    
    return distribution
}
```

### 5. Burn Rate Calculations
```go
type BurnRate struct {
    TokensPerMinute float64
    CostPerHour     float64
    HourlyBurnRate  float64 // Tokens in last hour
}

func CalculateBurnRate(entries []UsageEntry, duration time.Duration) BurnRate {
    totalTokens := 0
    totalCost := 0.0
    
    for _, entry := range entries {
        totalTokens += entry.TotalTokens
        totalCost += entry.CostUSD
    }
    
    durationMinutes := duration.Minutes()
    
    return BurnRate{
        TokensPerMinute: float64(totalTokens) / durationMinutes,
        CostPerHour:     (totalCost / durationMinutes) * 60.0,
        HourlyBurnRate:  CalculateTokensInLastHour(entries),
    }
}

func CalculateTokensInLastHour(entries []UsageEntry) float64 {
    oneHourAgo := time.Now().Add(-1 * time.Hour)
    tokensInHour := 0
    
    for _, entry := range entries {
        if entry.Timestamp.After(oneHourAgo) {
            tokensInHour += entry.TotalTokens
        }
    }
    
    return float64(tokensInHour) / 60.0 // Per minute
}
```

### 6. Cost Rate Calculations
```go
type CostRate struct {
    CostPerMinute           float64
    MinutesToCostDepletion  float64
    CostRemaining           float64
}

func CalculateCostRate(sessionCost float64, elapsedMinutes float64, costLimit float64) CostRate {
    costPerMinute := sessionCost / elapsedMinutes
    costRemaining := costLimit - sessionCost
    
    var minutesToDepletion float64
    if costPerMinute > 0 && costRemaining > 0 {
        minutesToDepletion = costRemaining / costPerMinute
    }
    
    return CostRate{
        CostPerMinute:          costPerMinute,
        MinutesToCostDepletion: minutesToDepletion,
        CostRemaining:          costRemaining,
    }
}
```

### 7. Predictions
```go
type Prediction struct {
    ProjectedTotalTokens int
    ProjectedTotalCost   float64
    PredictedEndTime     time.Time
}

func CalculatePredictions(current SessionStats, burnRate BurnRate, costRate CostRate) Prediction {
    sessionEnd := current.StartTime.Add(5 * time.Hour)
    remainingMinutes := time.Until(sessionEnd).Minutes()
    
    // Token projection
    projectedAdditionalTokens := int(burnRate.TokensPerMinute * remainingMinutes)
    projectedTotalTokens := current.TotalTokens + projectedAdditionalTokens
    
    // Cost projection
    projectedAdditionalCost := burnRate.CostPerHour * (remainingMinutes / 60.0)
    projectedTotalCost := current.TotalCost + projectedAdditionalCost
    
    // Predicted end time (when resources run out)
    var predictedEndTime time.Time
    if costRate.CostPerMinute > 0 && costRate.CostRemaining > 0 {
        predictedEndTime = time.Now().Add(time.Duration(costRate.MinutesToCostDepletion) * time.Minute)
    } else {
        predictedEndTime = sessionEnd
    }
    
    return Prediction{
        ProjectedTotalTokens: projectedTotalTokens,
        ProjectedTotalCost:   projectedTotalCost,
        PredictedEndTime:     predictedEndTime,
    }
}
```

### 8. P90 Limit Detection
```go
func DetectP90Limit(sessions []SessionData) int {
    // Common token limits to check against
    commonLimits := []int{19000, 88000, 220000} // pro, max5, max20
    
    var limitHitTokens []int
    for _, session := range sessions {
        for _, limit := range commonLimits {
            if float64(session.TotalTokens) >= float64(limit)*0.95 {
                limitHitTokens = append(limitHitTokens, session.TotalTokens)
                break
            }
        }
    }
    
    if len(limitHitTokens) == 0 {
        return 0
    }
    
    // Calculate 90th percentile
    sort.Ints(limitHitTokens)
    index := int(float64(len(limitHitTokens)) * 0.9)
    return limitHitTokens[index]
}
```

### 9. Session Block Aggregation
```go
func AggregateIntoBlocks(entries []UsageEntry) []SessionBlock {
    var blocks []SessionBlock
    var currentBlock *SessionBlock
    
    for _, entry := range entries {
        if currentBlock == nil {
            currentBlock = &SessionBlock{
                StartTime:  RoundToHour(entry.Timestamp),
                EndTime:    RoundToHour(entry.Timestamp).Add(5 * time.Hour),
                ModelStats: make(map[string]ModelStat),
            }
        }
        
        // Check for gap (>= 5 hours of inactivity)
        if entry.Timestamp.Sub(currentBlock.EndTime) >= 5*time.Hour {
            blocks = append(blocks, *currentBlock)
            
            // Create gap block
            gapBlock := SessionBlock{
                StartTime: currentBlock.EndTime,
                EndTime:   entry.Timestamp,
                IsGap:     true,
            }
            blocks = append(blocks, gapBlock)
            
            // Start new block
            currentBlock = &SessionBlock{
                StartTime:  RoundToHour(entry.Timestamp),
                EndTime:    RoundToHour(entry.Timestamp).Add(5 * time.Hour),
                ModelStats: make(map[string]ModelStat),
            }
        }
        
        // Add entry to current block
        modelStat := currentBlock.ModelStats[entry.Model]
        modelStat.InputTokens += entry.InputTokens
        modelStat.OutputTokens += entry.OutputTokens
        modelStat.CacheCreationTokens += entry.CacheCreationTokens
        modelStat.CacheReadTokens += entry.CacheReadTokens
        modelStat.TotalTokens += entry.TotalTokens
        modelStat.Cost += entry.CostUSD
        currentBlock.ModelStats[entry.Model] = modelStat
        
        currentBlock.TotalTokens += entry.TotalTokens
        currentBlock.TotalCost += entry.CostUSD
    }
    
    if currentBlock != nil {
        blocks = append(blocks, *currentBlock)
    }
    
    return blocks
}

func RoundToHour(t time.Time) time.Time {
    return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}
```

### 10. Daily/Monthly Aggregation
```go
type PeriodStats struct {
    Period              string // "2024-01-15" or "2024-01"
    InputTokens         int
    OutputTokens        int
    CacheCreationTokens int
    CacheReadTokens     int
    TotalTokens         int
    TotalCost           float64
    EntryCount          int
}

func AggregateByDay(entries []UsageEntry) map[string]PeriodStats {
    dailyStats := make(map[string]PeriodStats)
    
    for _, entry := range entries {
        day := entry.Timestamp.Format("2006-01-02")
        stats := dailyStats[day]
        
        stats.Period = day
        stats.InputTokens += entry.InputTokens
        stats.OutputTokens += entry.OutputTokens
        stats.CacheCreationTokens += entry.CacheCreationTokens
        stats.CacheReadTokens += entry.CacheReadTokens
        stats.TotalTokens += entry.TotalTokens
        stats.TotalCost += entry.CostUSD
        stats.EntryCount++
        
        dailyStats[day] = stats
    }
    
    return dailyStats
}

func AggregateByMonth(entries []UsageEntry) map[string]PeriodStats {
    monthlyStats := make(map[string]PeriodStats)
    
    for _, entry := range entries {
        month := entry.Timestamp.Format("2006-01")
        stats := monthlyStats[month]
        
        stats.Period = month
        stats.InputTokens += entry.InputTokens
        stats.OutputTokens += entry.OutputTokens
        stats.CacheCreationTokens += entry.CacheCreationTokens
        stats.CacheReadTokens += entry.CacheReadTokens
        stats.TotalTokens += entry.TotalTokens
        stats.TotalCost += entry.CostUSD
        stats.EntryCount++
        
        monthlyStats[month] = stats
    }
    
    return monthlyStats
}
```

## Data Flow

### 1. Data Discovery
```go
func DiscoverDataPaths() []string {
    var paths []string
    
    // Check standard locations
    locations := []string{
        filepath.Join(os.Getenv("HOME"), ".claude", "projects"),
        filepath.Join(os.Getenv("HOME"), ".config", "claude", "projects"),
    }
    
    for _, location := range locations {
        if _, err := os.Stat(location); err == nil {
            paths = append(paths, location)
        }
    }
    
    return paths
}
```

### 2. JSONL File Reading
```go
type RawMessage struct {
    Type      string    `json:"type"`
    Timestamp time.Time `json:"timestamp"`
    Model     string    `json:"model"`
    Usage     struct {
        InputTokens        int `json:"input_tokens"`
        OutputTokens       int `json:"output_tokens"`
        CacheCreationTokens int `json:"cache_creation_tokens"`
        CacheReadTokens    int `json:"cache_read_tokens"`
    } `json:"usage"`
}

func ReadConversationFile(filepath string) ([]UsageEntry, error) {
    file, err := os.Open(filepath)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    var entries []UsageEntry
    scanner := bufio.NewScanner(file)
    
    for scanner.Scan() {
        var msg RawMessage
        if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
            continue
        }
        
        if msg.Type == "message" && msg.Usage.InputTokens > 0 {
            entry := UsageEntry{
                Timestamp:           msg.Timestamp,
                Model:               msg.Model,
                InputTokens:         msg.Usage.InputTokens,
                OutputTokens:        msg.Usage.OutputTokens,
                CacheCreationTokens: msg.Usage.CacheCreationTokens,
                CacheReadTokens:     msg.Usage.CacheReadTokens,
            }
            
            // Calculate derived fields
            entry.TotalTokens = CalculateTotalTokens(entry)
            if pricing, ok := ModelPrices[entry.Model]; ok {
                entry.CostUSD = CalculateCost(entry, pricing)
            }
            
            entries = append(entries, entry)
        }
    }
    
    return entries, scanner.Err()
}
```

## Session Management

### Active Session Detection
```go
func FindActiveSessions(entries []UsageEntry) []SessionInfo {
    now := time.Now()
    var sessions []SessionInfo
    
    // Group entries by session start time (rounded to hour)
    sessionMap := make(map[time.Time][]UsageEntry)
    
    for _, entry := range entries {
        sessionStart := RoundToHour(entry.Timestamp)
        sessionEnd := sessionStart.Add(5 * time.Hour)
        
        // Check if session is still active
        if now.Before(sessionEnd) && now.After(sessionStart) {
            sessionMap[sessionStart] = append(sessionMap[sessionStart], entry)
        }
    }
    
    // Convert to session info
    for start, sessionEntries := range sessionMap {
        info := SessionInfo{
            StartTime: start,
            EndTime:   start.Add(5 * time.Hour),
            Entries:   sessionEntries,
        }
        sessions = append(sessions, info)
    }
    
    return sessions
}
```

### Multiple Concurrent Sessions
```go
func CalculateCombinedBurnRate(sessions []SessionInfo) float64 {
    totalTokensPerMinute := 0.0
    
    for _, session := range sessions {
        elapsed := time.Since(session.StartTime).Minutes()
        if elapsed > 0 {
            tokensInSession := 0
            for _, entry := range session.Entries {
                tokensInSession += entry.TotalTokens
            }
            totalTokensPerMinute += float64(tokensInSession) / elapsed
        }
    }
    
    return totalTokensPerMinute
}
```

## Performance Considerations

1. **File Watching**: Use filesystem notifications (fsnotify) for real-time updates
2. **Caching**: Implement LRU cache for parsed JSONL data
3. **Streaming**: Process JSONL files line-by-line to minimize memory usage
4. **Concurrency**: Use goroutines for parallel file processing
5. **Batching**: Aggregate updates before UI refresh to reduce overhead

## Error Handling

1. **File Access**: Handle missing files, permission errors gracefully
2. **Data Parsing**: Skip malformed JSONL lines, log errors
3. **Calculations**: Check for division by zero, handle edge cases
4. **Time Zones**: Always use UTC internally, convert for display

## Configuration

```go
type Config struct {
    Plan              string
    CustomTokenLimit  int
    CustomCostLimit   float64
    RefreshRate       float64 // Hz
    DataRefreshRate   int     // Seconds
    Theme             string
    Timezone          string
    Debug             bool
}
```

This prototype provides all the core functionality needed to reimplement the Claude Code Usage Monitor in Golang, maintaining feature parity with the Python version while leveraging Go's performance advantages.