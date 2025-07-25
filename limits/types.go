package limits

import (
	"time"
)

// PlanType 计划类型
type PlanType string

const (
	PlanFree   PlanType = "free"
	PlanPro    PlanType = "pro"
	PlanMax5   PlanType = "max5"
	PlanMax20  PlanType = "max20"
	PlanCustom PlanType = "custom"
)

// SubscriptionPlan 订阅计划
type SubscriptionPlan struct {
	Name          string         `json:"name"`
	Type          PlanType       `json:"type"`
	CostLimit     float64        `json:"cost_limit"`
	TokenLimit    int64          `json:"token_limit"`
	CustomLimit   bool           `json:"custom_limit"`
	Features      []string       `json:"features"`
	WarningLevels []WarningLevel `json:"warning_levels"`
	ResetCycle    ResetCycle     `json:"reset_cycle"`
}

// WarningLevel 警告级别
type WarningLevel struct {
	Threshold float64  `json:"threshold"` // 百分比阈值
	Message   string   `json:"message"`
	Severity  Severity `json:"severity"`
	Actions   []Action `json:"actions"`
}

// Severity 严重程度
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Action 触发的动作
type Action struct {
	Type   ActionType             `json:"type"`
	Config map[string]interface{} `json:"config"`
}

// ActionType 动作类型
type ActionType string

const (
	ActionNotify   ActionType = "notify"
	ActionLog      ActionType = "log"
	ActionThrottle ActionType = "throttle"
	ActionBlock    ActionType = "block"
	ActionWebhook  ActionType = "webhook"
)

// ResetCycle 重置周期
type ResetCycle string

const (
	ResetCycleDaily   ResetCycle = "daily"
	ResetCycleWeekly  ResetCycle = "weekly"
	ResetCycleMonthly ResetCycle = "monthly"
)

// LimitStatus 限额状态
type LimitStatus struct {
	Plan            SubscriptionPlan `json:"plan"`
	CurrentUsage    Usage            `json:"current_usage"`
	Percentage      float64          `json:"percentage"`
	TimeToReset     time.Duration    `json:"time_to_reset"`
	WarningLevel    *WarningLevel    `json:"warning_level"`
	Recommendations []string         `json:"recommendations"`
}

// Usage 使用情况
type Usage struct {
	Tokens     int64     `json:"tokens"`
	Cost       float64   `json:"cost"`
	StartTime  time.Time `json:"start_time"`
	LastUpdate time.Time `json:"last_update"`
}

// HistoricalUsage 历史使用记录
type HistoricalUsage struct {
	Date   time.Time `json:"date"`
	Tokens int64     `json:"tokens"`
	Cost   float64   `json:"cost"`
}

// Distribution 数据分布统计
type Distribution struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	P25    float64 `json:"p25"`
	P75    float64 `json:"p75"`
	P90    float64 `json:"p90"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
	StdDev float64 `json:"std_dev"`
}

// UsagePatterns 使用模式
type UsagePatterns struct {
	PeakHours []int     `json:"peak_hours"`
	PeakDays  []string  `json:"peak_days"`
	Trend     TrendType `json:"trend"`
	Anomalies []Anomaly `json:"anomalies"`
}

// TrendType 趋势类型
type TrendType string

const (
	TrendIncreasing TrendType = "increasing"
	TrendDecreasing TrendType = "decreasing"
	TrendStable     TrendType = "stable"
)

// Anomaly 异常使用
type Anomaly struct {
	Date      time.Time `json:"date"`
	Value     float64   `json:"value"`
	Expected  float64   `json:"expected"`
	Deviation float64   `json:"deviation"`
}

// PredictedUsage 预测使用量
type PredictedUsage struct {
	Date            time.Time `json:"date"`
	PredictedCost   float64   `json:"predicted_cost"`
	ConfidenceLevel float64   `json:"confidence_level"`
}
