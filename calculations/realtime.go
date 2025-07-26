package calculations

import (
	"math"
	"sync"
	"time"

	"github.com/penwyp/claudecat/config"
	"github.com/penwyp/claudecat/models"
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

	// 新增性能指标
	PerformanceMetrics PerformanceMetrics `json:"performance_metrics"`
	EfficiencyMetrics  EfficiencyMetrics  `json:"efficiency_metrics"`
	HealthMetrics      HealthMetrics      `json:"health_metrics"`
	TrendMetrics       TrendMetrics       `json:"trend_metrics"`
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	RequestCount        int     `json:"request_count"`
	AverageLatency      float64 `json:"average_latency_ms"`
	TokensPerRequest    float64 `json:"tokens_per_request"`
	RequestsPerMinute   float64 `json:"requests_per_minute"`
	PeakRequestsPerMin  float64 `json:"peak_requests_per_minute"`
	ResponseTimeP95     float64 `json:"response_time_p95"`
	ThroughputTokensSec float64 `json:"throughput_tokens_per_sec"`
}

// EfficiencyMetrics 效率指标
type EfficiencyMetrics struct {
	CostPerRequest     float64            `json:"cost_per_request"`
	CostPerToken       float64            `json:"cost_per_token"`
	TokensPerDollar    float64            `json:"tokens_per_dollar"`
	EfficiencyScore    float64            `json:"efficiency_score"` // 0-100, 综合效率评分
	ModelEfficiency    map[string]float64 `json:"model_efficiency"` // 每个模型的效率
	PeakEfficiencyTime time.Time          `json:"peak_efficiency_time"`
	WasteIndex         float64            `json:"waste_index"` // 浪费指数
}

// HealthMetrics 健康指标
type HealthMetrics struct {
	ErrorRate        float64 `json:"error_rate"`        // 错误率百分比
	RetryCount       int     `json:"retry_count"`       // 重试次数
	ConnectionHealth int     `json:"connection_health"` // 连接健康度 0-100
	CacheHitRate     float64 `json:"cache_hit_rate"`    // 缓存命中率
	ProcessingSpeed  float64 `json:"processing_speed"`  // 处理速度 tokens/sec
	SystemLoad       float64 `json:"system_load"`       // 系统负载
	MemoryUsage      float64 `json:"memory_usage"`      // 内存使用率百分比
}

// TrendMetrics 趋势指标
type TrendMetrics struct {
	HourlyGrowthRate    float64   `json:"hourly_growth_rate"`    // 每小时增长率
	PeakUsageHour       int       `json:"peak_usage_hour"`       // 峰值使用时间
	UsagePattern        string    `json:"usage_pattern"`         // 使用模式: "steady", "burst", "declining"
	SeasonalTrend       float64   `json:"seasonal_trend"`        // 季节性趋势
	PredictedPeakTime   time.Time `json:"predicted_peak_time"`   // 预测峰值时间
	BurnRateTrend       string    `json:"burn_rate_trend"`       // 燃烧率趋势: "up", "down", "stable"
	CostEfficiencyTrend string    `json:"cost_efficiency_trend"` // 成本效率趋势
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

	// 新增指标计算
	mc.calculatePerformanceMetrics(metrics, now)
	mc.calculateEfficiencyMetrics(metrics, now)
	mc.calculateHealthMetrics(metrics, now)
	mc.calculateTrendMetrics(metrics, now)

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

// calculatePerformanceMetrics 计算性能指标
func (mc *MetricsCalculator) calculatePerformanceMetrics(metrics *RealtimeMetrics, now time.Time) {
	perf := PerformanceMetrics{
		RequestCount: len(mc.entries),
	}

	if len(mc.entries) == 0 {
		metrics.PerformanceMetrics = perf
		return
	}

	// 计算平均延迟和响应时间
	var totalLatency float64
	var latencies []float64
	var totalTokens int

	for _, entry := range mc.entries {
		// 模拟延迟计算 (基于token数量估算)
		estimatedLatency := float64(entry.TotalTokens) * 0.1 // 假设每token 0.1ms
		totalLatency += estimatedLatency
		latencies = append(latencies, estimatedLatency)
		totalTokens += entry.TotalTokens
	}

	perf.AverageLatency = totalLatency / float64(len(mc.entries))

	// 计算P95响应时间
	if len(latencies) > 0 {
		// 简单排序获取P95
		for i := 0; i < len(latencies)-1; i++ {
			for j := 0; j < len(latencies)-i-1; j++ {
				if latencies[j] > latencies[j+1] {
					latencies[j], latencies[j+1] = latencies[j+1], latencies[j]
				}
			}
		}
		p95Index := int(float64(len(latencies)) * 0.95)
		if p95Index >= len(latencies) {
			p95Index = len(latencies) - 1
		}
		perf.ResponseTimeP95 = latencies[p95Index]
	}

	// 计算请求速率
	sessionDuration := now.Sub(mc.sessionStart)
	if sessionDuration > 0 {
		minutes := sessionDuration.Minutes()
		if minutes > 0 {
			perf.RequestsPerMinute = float64(len(mc.entries)) / minutes
		}

		seconds := sessionDuration.Seconds()
		if seconds > 0 {
			perf.ThroughputTokensSec = float64(totalTokens) / seconds
		}
	}

	// 计算tokens per request
	if len(mc.entries) > 0 {
		perf.TokensPerRequest = float64(totalTokens) / float64(len(mc.entries))
	}

	// 计算峰值请求数（最近5分钟窗口）
	recentWindow := now.Add(-5 * time.Minute)
	recentRequests := 0
	for _, entry := range mc.entries {
		if entry.Timestamp.After(recentWindow) {
			recentRequests++
		}
	}
	perf.PeakRequestsPerMin = float64(recentRequests) / 5.0

	metrics.PerformanceMetrics = perf
}

// calculateEfficiencyMetrics 计算效率指标
func (mc *MetricsCalculator) calculateEfficiencyMetrics(metrics *RealtimeMetrics, now time.Time) {
	eff := EfficiencyMetrics{
		ModelEfficiency: make(map[string]float64),
	}

	if metrics.CurrentCost == 0 || metrics.CurrentTokens == 0 || len(mc.entries) == 0 {
		metrics.EfficiencyMetrics = eff
		return
	}

	// 基础效率指标
	eff.CostPerRequest = metrics.CurrentCost / float64(len(mc.entries))
	eff.CostPerToken = metrics.CurrentCost / float64(metrics.CurrentTokens)
	eff.TokensPerDollar = float64(metrics.CurrentTokens) / metrics.CurrentCost

	// 计算模型效率
	for model, modelMetrics := range metrics.ModelDistribution {
		if modelMetrics.Cost > 0 {
			efficiency := float64(modelMetrics.TokenCount) / modelMetrics.Cost
			eff.ModelEfficiency[model] = efficiency
		}
	}

	// 计算综合效率评分 (0-100)
	// 基于tokens per dollar，与行业基准比较
	baselineEfficiency := 1000.0 // 假设基准为1000 tokens/$
	eff.EfficiencyScore = math.Min(100, (eff.TokensPerDollar/baselineEfficiency)*100)

	// 计算浪费指数
	if metrics.ProjectedCost > 0 && metrics.CurrentCost > 0 {
		// 如果预测成本远超当前成本，说明可能有浪费
		wasteRatio := (metrics.ProjectedCost - metrics.CurrentCost) / metrics.CurrentCost
		eff.WasteIndex = math.Min(100, wasteRatio*100)
	}

	// 记录峰值效率时间
	if mc.cachedMetrics != nil && eff.EfficiencyScore > mc.cachedMetrics.EfficiencyMetrics.EfficiencyScore {
		eff.PeakEfficiencyTime = now
	} else if mc.cachedMetrics != nil {
		eff.PeakEfficiencyTime = mc.cachedMetrics.EfficiencyMetrics.PeakEfficiencyTime
	}

	metrics.EfficiencyMetrics = eff
}

// calculateHealthMetrics 计算健康指标
func (mc *MetricsCalculator) calculateHealthMetrics(metrics *RealtimeMetrics, now time.Time) {
	health := HealthMetrics{
		ConnectionHealth: 100,  // 默认健康
		CacheHitRate:     85.0, // 假设85%缓存命中率
		SystemLoad:       0.5,  // 假设50%系统负载
		MemoryUsage:      60.0, // 假设60%内存使用
	}

	if len(mc.entries) == 0 {
		metrics.HealthMetrics = health
		return
	}

	// 计算处理速度
	sessionDuration := now.Sub(mc.sessionStart)
	if sessionDuration.Seconds() > 0 {
		health.ProcessingSpeed = float64(metrics.CurrentTokens) / sessionDuration.Seconds()
	}

	// 模拟错误率计算（基于entries的时间间隔）
	var intervals []time.Duration
	for i := 1; i < len(mc.entries); i++ {
		interval := mc.entries[i].Timestamp.Sub(mc.entries[i-1].Timestamp)
		intervals = append(intervals, interval)
	}

	// 如果间隔过大，可能存在错误或重试
	errorCount := 0
	for _, interval := range intervals {
		if interval > 30*time.Second { // 超过30秒认为可能有问题
			errorCount++
		}
	}

	if len(intervals) > 0 {
		health.ErrorRate = (float64(errorCount) / float64(len(intervals))) * 100
	}

	// 连接健康度基于错误率
	health.ConnectionHealth = int(100 - health.ErrorRate)
	if health.ConnectionHealth < 0 {
		health.ConnectionHealth = 0
	}

	metrics.HealthMetrics = health
}

// calculateTrendMetrics 计算趋势指标
func (mc *MetricsCalculator) calculateTrendMetrics(metrics *RealtimeMetrics, now time.Time) {
	trend := TrendMetrics{
		UsagePattern:        "steady",
		BurnRateTrend:       "stable",
		CostEfficiencyTrend: "stable",
	}

	if len(mc.entries) < 2 {
		metrics.TrendMetrics = trend
		return
	}

	// 计算每小时增长率
	sessionHours := now.Sub(mc.sessionStart).Hours()
	if sessionHours > 1 && mc.cachedMetrics != nil {
		oldTokens := mc.cachedMetrics.CurrentTokens
		if oldTokens > 0 {
			growth := float64(metrics.CurrentTokens-oldTokens) / float64(oldTokens) * 100
			trend.HourlyGrowthRate = growth / sessionHours
		}
	}

	// 分析使用模式
	if len(mc.entries) >= 3 {
		// 检查最近几个条目的间隔
		recent := mc.entries[len(mc.entries)-3:]
		intervals := make([]float64, 0, 2)
		for i := 1; i < len(recent); i++ {
			interval := recent[i].Timestamp.Sub(recent[i-1].Timestamp).Minutes()
			intervals = append(intervals, interval)
		}

		if len(intervals) >= 2 {
			avgInterval := (intervals[0] + intervals[1]) / 2
			if avgInterval < 1 { // 小于1分钟间隔
				trend.UsagePattern = "burst"
			} else if avgInterval > 10 { // 大于10分钟间隔
				trend.UsagePattern = "declining"
			}
		}
	}

	// 燃烧率趋势
	if mc.cachedMetrics != nil {
		currentBurnRate := metrics.BurnRate
		previousBurnRate := mc.cachedMetrics.BurnRate

		if currentBurnRate > previousBurnRate*1.1 {
			trend.BurnRateTrend = "up"
		} else if currentBurnRate < previousBurnRate*0.9 {
			trend.BurnRateTrend = "down"
		}
	}

	// 成本效率趋势
	if mc.cachedMetrics != nil {
		currentEfficiency := metrics.EfficiencyMetrics.EfficiencyScore
		previousEfficiency := mc.cachedMetrics.EfficiencyMetrics.EfficiencyScore

		if currentEfficiency > previousEfficiency+5 {
			trend.CostEfficiencyTrend = "improving"
		} else if currentEfficiency < previousEfficiency-5 {
			trend.CostEfficiencyTrend = "declining"
		}
	}

	// 峰值使用时间
	trend.PeakUsageHour = now.Hour()

	// 预测峰值时间（基于当前趋势）
	if trend.HourlyGrowthRate > 0 {
		hoursToNext := 24 - float64(now.Hour())
		trend.PredictedPeakTime = now.Add(time.Duration(hoursToNext) * time.Hour)
	}

	metrics.TrendMetrics = trend
}
