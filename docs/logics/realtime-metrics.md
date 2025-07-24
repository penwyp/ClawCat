# 实时指标计算功能开发计划

## 1. 功能概述

实时指标计算是 ClawCat 的核心功能之一，负责在用户使用 Claude 期间实时计算和更新各种使用指标。这些指标包括 token 使用量、成本、燃烧率以及预测值，为用户提供实时的资源使用洞察。

### 1.1 核心指标

- **当前会话统计**: token 使用量、成本累计
- **速率计算**: tokens/分钟、成本/小时
- **预测分析**: 预计总使用量、资源耗尽时间
- **进度跟踪**: 会话时间进度、资源使用进度

## 2. 技术设计

### 2.1 数据结构设计

```go
// RealtimeMetrics 实时指标数据结构
type RealtimeMetrics struct {
    // 基础统计
    SessionStart        time.Time
    SessionEnd          time.Time
    CurrentTokens       int
    CurrentCost         float64
    SessionProgress     float64  // 0-100%
    TimeRemaining       time.Duration
    
    // 速率计算
    TokensPerMinute     float64
    TokensPerHour       float64
    CostPerMinute       float64
    CostPerHour         float64
    BurnRate            float64  // 最近一小时的燃烧率
    
    // 预测值
    ProjectedTokens     int
    ProjectedCost       float64
    PredictedEndTime    time.Time
    ConfidenceLevel     float64  // 预测置信度
    
    // 模型分布
    ModelDistribution   map[string]ModelMetrics
}

type ModelMetrics struct {
    TokenCount      int
    Cost            float64
    Percentage      float64
    LastUsed        time.Time
}
```

### 2.2 计算引擎设计

```go
// MetricsCalculator 指标计算引擎
type MetricsCalculator struct {
    entries         []models.UsageEntry
    sessionStart    time.Time
    windowSize      time.Duration  // 默认5小时
    updateInterval  time.Duration  // 默认10秒
    
    // 缓存
    lastCalculated  time.Time
    cachedMetrics   *RealtimeMetrics
    
    // 配置
    config          *config.Config
    logger          *Logger
}

// 主要方法
func (mc *MetricsCalculator) Calculate() *RealtimeMetrics
func (mc *MetricsCalculator) UpdateWithNewEntry(entry models.UsageEntry)
func (mc *MetricsCalculator) GetProjections() ProjectionResult
func (mc *MetricsCalculator) GetBurnRate(duration time.Duration) float64
```

## 3. 实现步骤

### 3.1 创建指标计算模块

**文件**: `calculations/realtime.go`

```go
package calculations

import (
    "math"
    "time"
    "github.com/penwyp/ClawCat/models"
)

// NewMetricsCalculator 创建新的指标计算器
func NewMetricsCalculator(sessionStart time.Time, config *config.Config) *MetricsCalculator {
    return &MetricsCalculator{
        sessionStart:   sessionStart,
        windowSize:     5 * time.Hour,
        updateInterval: 10 * time.Second,
        entries:        make([]models.UsageEntry, 0),
        config:         config,
    }
}

// Calculate 计算当前指标
func (mc *MetricsCalculator) Calculate() *RealtimeMetrics {
    now := time.Now()
    
    // 检查缓存
    if mc.cachedMetrics != nil && now.Sub(mc.lastCalculated) < mc.updateInterval {
        return mc.cachedMetrics
    }
    
    metrics := &RealtimeMetrics{
        SessionStart:      mc.sessionStart,
        SessionEnd:        mc.sessionStart.Add(mc.windowSize),
        ModelDistribution: make(map[string]ModelMetrics),
    }
    
    // 基础统计
    mc.calculateBasicStats(metrics)
    
    // 速率计算
    mc.calculateRates(metrics, now)
    
    // 预测分析
    mc.calculateProjections(metrics, now)
    
    // 模型分布
    mc.calculateModelDistribution(metrics)
    
    // 更新缓存
    mc.cachedMetrics = metrics
    mc.lastCalculated = now
    
    return metrics
}

// calculateBasicStats 计算基础统计
func (mc *MetricsCalculator) calculateBasicStats(metrics *RealtimeMetrics) {
    for _, entry := range mc.entries {
        metrics.CurrentTokens += entry.TotalTokens
        metrics.CurrentCost += entry.CostUSD
    }
    
    // 计算进度
    elapsed := time.Since(mc.sessionStart)
    metrics.SessionProgress = math.Min(100, (elapsed.Seconds()/mc.windowSize.Seconds())*100)
    metrics.TimeRemaining = mc.windowSize - elapsed
    if metrics.TimeRemaining < 0 {
        metrics.TimeRemaining = 0
    }
}

// calculateRates 计算使用速率
func (mc *MetricsCalculator) calculateRates(metrics *RealtimeMetrics, now time.Time) {
    // 获取不同时间窗口的数据
    lastMinute := mc.getEntriesInWindow(now.Add(-1*time.Minute), now)
    lastHour := mc.getEntriesInWindow(now.Add(-1*time.Hour), now)
    
    // 计算每分钟速率
    if len(lastMinute) > 0 {
        tokensLastMin := sumTokens(lastMinute)
        metrics.TokensPerMinute = float64(tokensLastMin)
        metrics.CostPerMinute = sumCost(lastMinute)
    }
    
    // 计算每小时速率
    if len(lastHour) > 0 {
        tokensLastHour := sumTokens(lastHour)
        metrics.TokensPerHour = float64(tokensLastHour)
        metrics.CostPerHour = sumCost(lastHour)
        
        // 燃烧率：最近一小时的平均每分钟消耗
        metrics.BurnRate = float64(tokensLastHour) / 60.0
    }
}

// calculateProjections 计算预测值
func (mc *MetricsCalculator) calculateProjections(metrics *RealtimeMetrics, now time.Time) {
    if metrics.TokensPerMinute == 0 {
        // 没有足够数据进行预测
        metrics.ProjectedTokens = metrics.CurrentTokens
        metrics.ProjectedCost = metrics.CurrentCost
        metrics.ConfidenceLevel = 0
        return
    }
    
    // 基于当前速率预测
    remainingMinutes := metrics.TimeRemaining.Minutes()
    if remainingMinutes > 0 {
        additionalTokens := int(metrics.TokensPerMinute * remainingMinutes)
        additionalCost := metrics.CostPerMinute * remainingMinutes
        
        metrics.ProjectedTokens = metrics.CurrentTokens + additionalTokens
        metrics.ProjectedCost = metrics.CurrentCost + additionalCost
    }
    
    // 计算资源耗尽时间
    if limit := mc.getPlanLimit(); limit > 0 && metrics.CostPerMinute > 0 {
        minutesToLimit := (limit - metrics.CurrentCost) / metrics.CostPerMinute
        if minutesToLimit > 0 {
            metrics.PredictedEndTime = now.Add(time.Duration(minutesToLimit) * time.Minute)
        }
    }
    
    // 计算置信度（基于数据点数量）
    dataPoints := len(mc.entries)
    metrics.ConfidenceLevel = math.Min(100, float64(dataPoints)/10.0*100)
}

// getEntriesInWindow 获取时间窗口内的条目
func (mc *MetricsCalculator) getEntriesInWindow(start, end time.Time) []models.UsageEntry {
    var result []models.UsageEntry
    for _, entry := range mc.entries {
        if entry.Timestamp.After(start) && entry.Timestamp.Before(end) {
            result = append(result, entry)
        }
    }
    return result
}

// getPlanLimit 获取订阅计划限额
func (mc *MetricsCalculator) getPlanLimit() float64 {
    switch mc.config.Subscription.Plan {
    case "pro":
        return 18.00
    case "max5":
        return 35.00
    case "max20":
        return 140.00
    case "custom":
        return mc.calculateP90Limit()
    default:
        return 0
    }
}
```

### 3.2 集成到会话管理器

**文件**: `sessions/realtime_integration.go`

```go
package sessions

import (
    "github.com/penwyp/ClawCat/calculations"
)

// SessionWithMetrics 带实时指标的会话
type SessionWithMetrics struct {
    *Session
    calculator *calculations.MetricsCalculator
    metrics    *calculations.RealtimeMetrics
}

// UpdateMetrics 更新会话指标
func (s *SessionWithMetrics) UpdateMetrics() {
    s.metrics = s.calculator.Calculate()
}

// AddEntry 添加新条目并更新指标
func (s *SessionWithMetrics) AddEntry(entry models.UsageEntry) {
    s.Session.AddEntry(entry)
    s.calculator.UpdateWithNewEntry(entry)
    s.UpdateMetrics()
}

// GetCurrentMetrics 获取当前指标
func (s *SessionWithMetrics) GetCurrentMetrics() *calculations.RealtimeMetrics {
    if s.metrics == nil {
        s.UpdateMetrics()
    }
    return s.metrics
}
```

### 3.3 UI 集成

**文件**: `ui/components/metrics_display.go`

```go
package ui

import (
    "fmt"
    "github.com/charmbracelet/lipgloss"
    "github.com/penwyp/ClawCat/calculations"
)

// MetricsDisplay 实时指标显示组件
type MetricsDisplay struct {
    metrics *calculations.RealtimeMetrics
    styles  Styles
    width   int
}

// Render 渲染指标显示
func (md *MetricsDisplay) Render() string {
    if md.metrics == nil {
        return "No metrics available"
    }
    
    // 渲染各个指标卡片
    cards := []string{
        md.renderTokenCard(),
        md.renderCostCard(),
        md.renderBurnRateCard(),
        md.renderProjectionCard(),
    }
    
    return lipgloss.JoinVertical(lipgloss.Left, cards...)
}

// renderTokenCard 渲染 Token 使用卡片
func (md *MetricsDisplay) renderTokenCard() string {
    current := formatNumber(md.metrics.CurrentTokens)
    projected := formatNumber(md.metrics.ProjectedTokens)
    rate := fmt.Sprintf("%.1f/min", md.metrics.TokensPerMinute)
    
    content := fmt.Sprintf(
        "Tokens: %s → %s\nRate: %s",
        current, projected, rate,
    )
    
    return md.styles.MetricCard.Render(content)
}

// renderBurnRateCard 渲染燃烧率卡片
func (md *MetricsDisplay) renderBurnRateCard() string {
    burnRate := fmt.Sprintf("%.1f tok/min", md.metrics.BurnRate)
    costRate := fmt.Sprintf("$%.2f/hr", md.metrics.CostPerHour)
    
    // 根据燃烧率设置颜色
    style := md.styles.Normal
    if md.metrics.BurnRate > 100 {
        style = md.styles.Warning
    } else if md.metrics.BurnRate > 200 {
        style = md.styles.Error
    }
    
    content := fmt.Sprintf(
        "Burn Rate: %s\nCost Rate: %s",
        burnRate, costRate,
    )
    
    return style.Render(content)
}
```

## 4. 测试计划

### 4.1 单元测试

```go
// calculations/realtime_test.go

func TestMetricsCalculator_Calculate(t *testing.T) {
    // 测试基础计算
    t.Run("basic calculations", func(t *testing.T) {
        calc := NewMetricsCalculator(time.Now(), testConfig)
        
        // 添加测试数据
        entries := generateTestEntries(10)
        for _, entry := range entries {
            calc.UpdateWithNewEntry(entry)
        }
        
        metrics := calc.Calculate()
        
        assert.Greater(t, metrics.CurrentTokens, 0)
        assert.Greater(t, metrics.CurrentCost, 0.0)
        assert.Greater(t, metrics.SessionProgress, 0.0)
    })
    
    // 测试速率计算
    t.Run("rate calculations", func(t *testing.T) {
        calc := NewMetricsCalculator(time.Now().Add(-30*time.Minute), testConfig)
        
        // 模拟持续使用
        for i := 0; i < 30; i++ {
            entry := models.UsageEntry{
                Timestamp:   time.Now().Add(-time.Duration(30-i) * time.Minute),
                TotalTokens: 100,
                CostUSD:     0.01,
            }
            calc.UpdateWithNewEntry(entry)
        }
        
        metrics := calc.Calculate()
        
        assert.InDelta(t, 100.0, metrics.TokensPerMinute, 10.0)
        assert.Greater(t, metrics.BurnRate, 0.0)
    })
    
    // 测试预测
    t.Run("projections", func(t *testing.T) {
        calc := NewMetricsCalculator(time.Now().Add(-1*time.Hour), testConfig)
        
        // 添加稳定的使用模式
        for i := 0; i < 60; i++ {
            entry := models.UsageEntry{
                Timestamp:   time.Now().Add(-time.Duration(60-i) * time.Minute),
                TotalTokens: 50,
                CostUSD:     0.005,
            }
            calc.UpdateWithNewEntry(entry)
        }
        
        metrics := calc.Calculate()
        
        assert.Greater(t, metrics.ProjectedTokens, metrics.CurrentTokens)
        assert.Greater(t, metrics.ProjectedCost, metrics.CurrentCost)
        assert.Greater(t, metrics.ConfidenceLevel, 50.0)
    })
}

// 测试辅助函数
func generateTestEntries(count int) []models.UsageEntry {
    entries := make([]models.UsageEntry, count)
    for i := 0; i < count; i++ {
        entries[i] = models.UsageEntry{
            Timestamp:    time.Now().Add(-time.Duration(count-i) * time.Minute),
            Model:        "claude-3-opus",
            InputTokens:  100 + rand.Intn(900),
            OutputTokens: 200 + rand.Intn(1800),
            TotalTokens:  300 + rand.Intn(2700),
            CostUSD:      0.01 + rand.Float64()*0.09,
        }
    }
    return entries
}
```

### 4.2 集成测试

```go
// integration/realtime_metrics_test.go

func TestRealtimeMetricsIntegration(t *testing.T) {
    // 创建测试环境
    app := setupTestApp(t)
    
    // 模拟实时数据流
    t.Run("realtime data stream", func(t *testing.T) {
        // 启动数据生成器
        stopCh := make(chan struct{})
        go generateRealtimeData(app, stopCh)
        
        // 等待数据积累
        time.Sleep(30 * time.Second)
        
        // 获取指标
        metrics := app.GetCurrentMetrics()
        
        // 验证指标更新
        assert.Greater(t, metrics.CurrentTokens, 0)
        assert.Greater(t, metrics.TokensPerMinute, 0.0)
        assert.NotZero(t, metrics.ProjectedTokens)
        
        close(stopCh)
    })
    
    // 测试限额警告
    t.Run("limit warnings", func(t *testing.T) {
        app.Config.Subscription.Plan = "pro"
        
        // 添加接近限额的数据
        for i := 0; i < 100; i++ {
            entry := models.UsageEntry{
                Timestamp: time.Now(),
                CostUSD:   0.17, // 接近 $18 限额
            }
            app.AddEntry(entry)
        }
        
        metrics := app.GetCurrentMetrics()
        
        assert.NotZero(t, metrics.PredictedEndTime)
        assert.Less(t, metrics.TimeRemaining, 1*time.Hour)
    })
}
```

## 5. 性能优化

### 5.1 缓存策略

- 实现 10 秒缓存机制，避免频繁重计算
- 使用增量计算，只处理新增数据
- 维护滑动窗口，自动清理过期数据

### 5.2 并发处理

```go
// 使用读写锁保护数据
type MetricsCalculator struct {
    mu sync.RWMutex
    // ... 其他字段
}

func (mc *MetricsCalculator) Calculate() *RealtimeMetrics {
    mc.mu.RLock()
    defer mc.mu.RUnlock()
    // ... 计算逻辑
}

func (mc *MetricsCalculator) UpdateWithNewEntry(entry models.UsageEntry) {
    mc.mu.Lock()
    defer mc.mu.Unlock()
    // ... 更新逻辑
}
```

### 5.3 内存优化

- 限制历史数据保留量（最多保留 5 小时数据）
- 使用环形缓冲区存储最近的条目
- 定期清理过期的模型分布数据

## 6. 监控和调试

### 6.1 日志记录

```go
// 添加详细的调试日志
func (mc *MetricsCalculator) Calculate() *RealtimeMetrics {
    mc.logger.Debug("Starting metrics calculation", 
        "entries", len(mc.entries),
        "session_start", mc.sessionStart,
    )
    
    // ... 计算逻辑
    
    mc.logger.Debug("Metrics calculated",
        "current_tokens", metrics.CurrentTokens,
        "burn_rate", metrics.BurnRate,
        "projected_tokens", metrics.ProjectedTokens,
    )
}
```

### 6.2 性能指标

```go
// 添加性能监控
type MetricsStats struct {
    CalculationCount    int64
    AvgCalculationTime  time.Duration
    CacheHitRate        float64
    MemoryUsage         int64
}
```

## 7. 错误处理

### 7.1 异常情况处理

- 处理空数据集
- 处理时间戳异常
- 处理数值溢出
- 优雅降级到基础指标

### 7.2 错误恢复

```go
func (mc *MetricsCalculator) Calculate() (metrics *RealtimeMetrics, err error) {
    defer func() {
        if r := recover(); r != nil {
            mc.logger.Error("Panic in metrics calculation", "error", r)
            metrics = mc.getDefaultMetrics()
            err = fmt.Errorf("calculation failed: %v", r)
        }
    }()
    
    // ... 正常计算逻辑
}
```

## 8. 文档和示例

### 8.1 API 文档

```go
// Package calculations provides real-time metrics calculation for Claude usage.
//
// Example usage:
//
//     calc := calculations.NewMetricsCalculator(time.Now(), config)
//     
//     // Add entries as they come in
//     calc.UpdateWithNewEntry(entry)
//     
//     // Get current metrics
//     metrics := calc.Calculate()
//     
//     fmt.Printf("Current tokens: %d\n", metrics.CurrentTokens)
//     fmt.Printf("Burn rate: %.2f tokens/min\n", metrics.BurnRate)
//     fmt.Printf("Projected cost: $%.2f\n", metrics.ProjectedCost)
```

### 8.2 配置示例

```yaml
# config.yaml
metrics:
  update_interval: 10s
  cache_duration: 10s
  history_limit: 5h
  calculation:
    enable_projections: true
    confidence_threshold: 0.7
    rate_window: 1h
```

## 9. 部署清单

- [ ] 实现 `calculations/realtime.go`
- [ ] 实现 `sessions/realtime_integration.go`
- [ ] 实现 `ui/components/metrics_display.go`
- [ ] 编写单元测试
- [ ] 编写集成测试
- [ ] 性能测试和优化
- [ ] 更新配置文件
- [ ] 更新 UI 集成
- [ ] 编写用户文档
- [ ] 代码审查和合并

## 10. 未来增强

- 机器学习预测模型
- 多会话对比分析
- 自定义指标定义
- 导出指标历史
- Webhook 通知集成