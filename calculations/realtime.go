package calculations

import (
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
