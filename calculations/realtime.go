package calculations

import (
	"math"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/models"
)

// RealtimeMetrics 实时指标数据结构
type RealtimeMetrics struct {
	// 基础统计
	SessionStart    time.Time     `json:"session_start"`
	SessionEnd      time.Time     `json:"session_end"`
	CurrentTokens   int           `json:"current_tokens"`
	CurrentCost     float64       `json:"current_cost"`
	SessionProgress float64       `json:"session_progress"` // 0-100%
	TimeRemaining   time.Duration `json:"time_remaining"`

	// 速率计算
	TokensPerMinute float64 `json:"tokens_per_minute"`
	TokensPerHour   float64 `json:"tokens_per_hour"`
	CostPerMinute   float64 `json:"cost_per_minute"`
	CostPerHour     float64 `json:"cost_per_hour"`
	BurnRate        float64 `json:"burn_rate"` // 最近一小时的燃烧率

	// 预测值
	ProjectedTokens  int       `json:"projected_tokens"`
	ProjectedCost    float64   `json:"projected_cost"`
	PredictedEndTime time.Time `json:"predicted_end_time"`
	ConfidenceLevel  float64   `json:"confidence_level"` // 预测置信度

	// 模型分布
	ModelDistribution map[string]ModelMetrics `json:"model_distribution"`
}

// ModelMetrics 模型使用指标
type ModelMetrics struct {
	TokenCount int       `json:"token_count"`
	Cost       float64   `json:"cost"`
	Percentage float64   `json:"percentage"`
	LastUsed   time.Time `json:"last_used"`
}

// MetricsCalculator 指标计算引擎
type MetricsCalculator struct {
	mu             sync.RWMutex
	entries        []models.UsageEntry
	sessionStart   time.Time
	windowSize     time.Duration // 默认5小时
	updateInterval time.Duration // 默认10秒

	// 缓存
	lastCalculated time.Time
	cachedMetrics  *RealtimeMetrics

	// 配置
	config *config.Config
}

// NewMetricsCalculator 创建新的指标计算器
func NewMetricsCalculator(sessionStart time.Time, cfg *config.Config) *MetricsCalculator {
	return &MetricsCalculator{
		sessionStart:   sessionStart,
		windowSize:     5 * time.Hour,
		updateInterval: 10 * time.Second,
		entries:        make([]models.UsageEntry, 0),
		config:         cfg,
	}
}

// Calculate 计算当前指标
func (mc *MetricsCalculator) Calculate() *RealtimeMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

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

// UpdateWithNewEntry 添加新的使用条目
func (mc *MetricsCalculator) UpdateWithNewEntry(entry models.UsageEntry) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 添加新条目
	mc.entries = append(mc.entries, entry)

	// 清理过期数据（保留最近5小时）
	cutoff := time.Now().Add(-mc.windowSize)
	validEntries := make([]models.UsageEntry, 0, len(mc.entries))
	for _, e := range mc.entries {
		if e.Timestamp.After(cutoff) {
			validEntries = append(validEntries, e)
		}
	}
	mc.entries = validEntries

	// 使缓存失效
	mc.cachedMetrics = nil
}

// calculateBasicStats 计算基础统计
func (mc *MetricsCalculator) calculateBasicStats(metrics *RealtimeMetrics) {
	for _, entry := range mc.entries {
		metrics.CurrentTokens += entry.TotalTokens
		metrics.CurrentCost += entry.CostUSD
	}

	// 计算进度
	now := time.Now()
	elapsed := now.Sub(mc.sessionStart)
	if elapsed < 0 {
		elapsed = 0
	}

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
		costLastHour := sumCost(lastHour)

		// 按实际时间窗口计算平均速率
		actualHours := math.Min(1.0, time.Since(mc.sessionStart).Hours())
		if actualHours > 0 {
			metrics.TokensPerHour = float64(tokensLastHour) / actualHours
			metrics.CostPerHour = costLastHour / actualHours
		}

		// 燃烧率：最近一小时的平均每分钟消耗
		metrics.BurnRate = float64(tokensLastHour) / 60.0
	}

	// 如果没有最近一小时的数据，使用整个会话的平均值
	if len(lastHour) == 0 && len(mc.entries) > 0 {
		sessionDuration := time.Since(mc.sessionStart)
		if sessionDuration > 0 {
			minutes := sessionDuration.Minutes()
			if minutes > 0 {
				metrics.TokensPerMinute = float64(metrics.CurrentTokens) / minutes
				metrics.CostPerMinute = metrics.CurrentCost / minutes
				metrics.TokensPerHour = metrics.TokensPerMinute * 60
				metrics.CostPerHour = metrics.CostPerMinute * 60
				metrics.BurnRate = metrics.TokensPerMinute
			}
		}
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
	} else {
		metrics.ProjectedTokens = metrics.CurrentTokens
		metrics.ProjectedCost = metrics.CurrentCost
	}

	// 计算资源耗尽时间
	if limit := mc.getPlanLimit(); limit > 0 && metrics.CostPerMinute > 0 {
		remainingBudget := limit - metrics.CurrentCost
		if remainingBudget > 0 {
			minutesToLimit := remainingBudget / metrics.CostPerMinute
			if minutesToLimit > 0 {
				metrics.PredictedEndTime = now.Add(time.Duration(minutesToLimit) * time.Minute)
			}
		}
	}

	// 计算置信度（基于数据点数量和时间范围）
	dataPoints := len(mc.entries)
	timeRange := time.Since(mc.sessionStart).Minutes()

	// 更多数据点和更长时间范围提高置信度
	baseConfidence := math.Min(100, float64(dataPoints)/10.0*100)
	timeConfidence := math.Min(100, timeRange/60.0*100) // 1小时数据为100%

	metrics.ConfidenceLevel = math.Min(100, (baseConfidence+timeConfidence)/2)
}

// calculateModelDistribution 计算模型分布
func (mc *MetricsCalculator) calculateModelDistribution(metrics *RealtimeMetrics) {
	modelStats := make(map[string]ModelMetrics)

	for _, entry := range mc.entries {
		stat, exists := modelStats[entry.Model]
		if !exists {
			stat = ModelMetrics{
				LastUsed: entry.Timestamp,
			}
		}

		stat.TokenCount += entry.TotalTokens
		stat.Cost += entry.CostUSD
		if entry.Timestamp.After(stat.LastUsed) {
			stat.LastUsed = entry.Timestamp
		}

		modelStats[entry.Model] = stat
	}

	// 计算百分比
	for model, stat := range modelStats {
		if metrics.CurrentTokens > 0 {
			stat.Percentage = float64(stat.TokenCount) / float64(metrics.CurrentTokens) * 100
		}
		modelStats[model] = stat
	}

	metrics.ModelDistribution = modelStats
}

// getEntriesInWindow 获取时间窗口内的条目
func (mc *MetricsCalculator) getEntriesInWindow(start, end time.Time) []models.UsageEntry {
	var result []models.UsageEntry
	for _, entry := range mc.entries {
		if (entry.Timestamp.After(start) || entry.Timestamp.Equal(start)) &&
			entry.Timestamp.Before(end) {
			result = append(result, entry)
		}
	}
	return result
}

// getPlanLimit 获取订阅计划限额
func (mc *MetricsCalculator) getPlanLimit() float64 {
	if mc.config == nil {
		return 0
	}

	switch mc.config.Subscription.Plan {
	case "pro":
		return 18.00
	case "max5":
		return 35.00
	case "max20":
		return 140.00
	case "custom":
		return mc.config.Subscription.CustomCostLimit
	default:
		return 0
	}
}

// sumTokens 计算条目列表的总tokens
func sumTokens(entries []models.UsageEntry) int {
	total := 0
	for _, entry := range entries {
		total += entry.TotalTokens
	}
	return total
}

// sumCost 计算条目列表的总成本
func sumCost(entries []models.UsageEntry) float64 {
	total := 0.0
	for _, entry := range entries {
		total += entry.CostUSD
	}
	return total
}

// GetBurnRate 获取指定时间窗口的燃烧率
func (mc *MetricsCalculator) GetBurnRate(duration time.Duration) float64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	now := time.Now()
	entries := mc.getEntriesInWindow(now.Add(-duration), now)

	if len(entries) == 0 {
		return 0
	}

	totalTokens := sumTokens(entries)
	return float64(totalTokens) / duration.Minutes()
}

// GetProjections 获取详细的预测分析
func (mc *MetricsCalculator) GetProjections() *RealtimeMetrics {
	return mc.Calculate()
}

// Reset 重置计算器状态
func (mc *MetricsCalculator) Reset(sessionStart time.Time) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.sessionStart = sessionStart
	mc.entries = make([]models.UsageEntry, 0)
	mc.cachedMetrics = nil
	mc.lastCalculated = time.Time{}
}

// GetEntryCount 获取当前条目数量
func (mc *MetricsCalculator) GetEntryCount() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return len(mc.entries)
}
