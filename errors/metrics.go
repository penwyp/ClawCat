package errors

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/penwyp/claudecat/logging"
)

// ErrorMetrics 错误指标
type ErrorMetrics struct {
	// 计数器
	TotalErrors      *Counter
	ErrorsByType     map[ErrorType]*Counter
	ErrorsBySeverity map[ErrorSeverity]*Counter
	RecoverySuccess  *Counter
	RecoveryFailed   *Counter

	// 速率
	ErrorRate    *Rate
	RecoveryRate *Rate

	// 延迟
	RecoveryLatency *Histogram

	// 状态
	CircuitBreakerState *Gauge
	DegradedMode        *Gauge
}

// NewErrorMetrics 创建错误指标
func NewErrorMetrics() *ErrorMetrics {
	return &ErrorMetrics{
		TotalErrors:         NewCounter(),
		ErrorsByType:        make(map[ErrorType]*Counter),
		ErrorsBySeverity:    make(map[ErrorSeverity]*Counter),
		RecoverySuccess:     NewCounter(),
		RecoveryFailed:      NewCounter(),
		ErrorRate:           NewRate(),
		RecoveryRate:        NewRate(),
		RecoveryLatency:     NewHistogram(),
		CircuitBreakerState: NewGauge(),
		DegradedMode:        NewGauge(),
	}
}

// Counter 计数器
type Counter struct {
	value int64
}

// NewCounter 创建计数器
func NewCounter() *Counter {
	return &Counter{}
}

// Inc 增加计数
func (c *Counter) Inc() {
	atomic.AddInt64(&c.value, 1)
}

// Add 增加指定值
func (c *Counter) Add(delta int64) {
	atomic.AddInt64(&c.value, delta)
}

// Value 获取当前值
func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// Reset 重置计数器
func (c *Counter) Reset() {
	atomic.StoreInt64(&c.value, 0)
}

// Rate 速率计算器
type Rate struct {
	events    []time.Time
	mu        sync.Mutex
	window    time.Duration
	lastClean time.Time
}

// NewRate 创建速率计算器
func NewRate() *Rate {
	return &Rate{
		events: make([]time.Time, 0),
		window: 1 * time.Minute,
	}
}

// Mark 记录事件
func (r *Rate) Mark() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	r.events = append(r.events, now)

	// 定期清理过期事件
	if now.Sub(r.lastClean) > 10*time.Second {
		r.cleanup(now)
		r.lastClean = now
	}
}

// Rate 计算当前速率 (events/second)
func (r *Rate) Rate() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	r.cleanup(now)

	if len(r.events) == 0 {
		return 0
	}

	duration := r.window.Seconds()
	return float64(len(r.events)) / duration
}

// cleanup 清理过期事件
func (r *Rate) cleanup(now time.Time) {
	cutoff := now.Add(-r.window)

	// 移除过期事件
	newEvents := make([]time.Time, 0, len(r.events))
	for _, event := range r.events {
		if event.After(cutoff) {
			newEvents = append(newEvents, event)
		}
	}
	r.events = newEvents
}

// Histogram 直方图
type Histogram struct {
	buckets []Bucket
	mu      sync.Mutex
}

// Bucket 直方图桶
type Bucket struct {
	UpperBound float64
	Count      int64
}

// NewHistogram 创建直方图
func NewHistogram() *Histogram {
	buckets := []Bucket{
		{UpperBound: 0.001, Count: 0}, // 1ms
		{UpperBound: 0.01, Count: 0},  // 10ms
		{UpperBound: 0.1, Count: 0},   // 100ms
		{UpperBound: 1.0, Count: 0},   // 1s
		{UpperBound: 10.0, Count: 0},  // 10s
	}

	return &Histogram{
		buckets: buckets,
	}
}

// Observe 记录观测值
func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i := range h.buckets {
		if value <= h.buckets[i].UpperBound {
			atomic.AddInt64(&h.buckets[i].Count, 1)
			break
		}
	}
}

// GetBuckets 获取桶信息
func (h *Histogram) GetBuckets() []Bucket {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make([]Bucket, len(h.buckets))
	for i, bucket := range h.buckets {
		result[i] = Bucket{
			UpperBound: bucket.UpperBound,
			Count:      atomic.LoadInt64(&bucket.Count),
		}
	}

	return result
}

// Gauge 仪表盘
type Gauge struct {
	value int64
}

// NewGauge 创建仪表盘
func NewGauge() *Gauge {
	return &Gauge{}
}

// Set 设置值
func (g *Gauge) Set(value float64) {
	atomic.StoreInt64(&g.value, int64(value))
}

// Inc 增加
func (g *Gauge) Inc() {
	atomic.AddInt64(&g.value, 1)
}

// Dec 减少
func (g *Gauge) Dec() {
	atomic.AddInt64(&g.value, -1)
}

// Value 获取当前值
func (g *Gauge) Value() float64 {
	return float64(atomic.LoadInt64(&g.value))
}

// RingBuffer 环形缓冲区
type RingBuffer struct {
	items    []interface{}
	head     int
	tail     int
	size     int
	capacity int
	mu       sync.RWMutex
}

// NewRingBuffer 创建环形缓冲区
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		items:    make([]interface{}, capacity),
		capacity: capacity,
	}
}

// Add 添加项目
func (rb *RingBuffer) Add(item interface{}) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.items[rb.tail] = item
	rb.tail = (rb.tail + 1) % rb.capacity

	if rb.size < rb.capacity {
		rb.size++
	} else {
		rb.head = (rb.head + 1) % rb.capacity
	}
}

// CountRecent 计算最近时间内的项目数量
func (rb *RingBuffer) CountRecent(duration time.Duration) int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	count := 0

	for i := 0; i < rb.size; i++ {
		index := (rb.head + i) % rb.capacity
		if item, ok := rb.items[index].(*RecoverableError); ok {
			if item.Timestamp.After(cutoff) {
				count++
			}
		}
	}

	return count
}

// Size 获取当前大小
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

// AlertManager 告警管理器
type AlertManager struct {
	channels []AlertChannel
	mu       sync.RWMutex
}

// AlertChannel 告警渠道接口
type AlertChannel interface {
	Send(alert Alert) error
}

// Alert 告警
type Alert struct {
	Type      AlertType
	Severity  AlertSeverity
	Message   string
	Timestamp time.Time
	Context   map[string]interface{}
}

// AlertType 告警类型
type AlertType string

const (
	AlertTypeErrorRate       AlertType = "error_rate"
	AlertTypeCriticalErrors  AlertType = "critical_errors"
	AlertTypeRecoveryFailure AlertType = "recovery_failure"
)

// AlertSeverity 告警严重程度
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityCritical AlertSeverity = "critical"
)

// NewAlertManager 创建告警管理器
func NewAlertManager() *AlertManager {
	return &AlertManager{
		channels: make([]AlertChannel, 0),
	}
}

// AddChannel 添加告警渠道
func (am *AlertManager) AddChannel(channel AlertChannel) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.channels = append(am.channels, channel)
}

// SendAlert 发送告警
func (am *AlertManager) SendAlert(err *RecoverableError, context *ErrorContext) {
	alert := Alert{
		Type:      AlertTypeErrorRate, // 简化实现
		Severity:  am.mapSeverity(err.Severity),
		Message:   err.Message,
		Timestamp: time.Now(),
		Context: map[string]interface{}{
			"error_type": err.Type,
			"component":  context.Component,
		},
	}

	am.mu.RLock()
	channels := make([]AlertChannel, len(am.channels))
	copy(channels, am.channels)
	am.mu.RUnlock()

	for _, channel := range channels {
		go func(ch AlertChannel) {
			if err := ch.Send(alert); err != nil {
				// 记录告警发送失败
				if logging.GetLogger() != nil {
					logging.GetLogger().Errorf("Failed to send alert: %v", err)
				}
			}
		}(channel)
	}
}

// mapSeverity 映射严重程度
func (am *AlertManager) mapSeverity(severity ErrorSeverity) AlertSeverity {
	switch severity {
	case SeverityLow:
		return AlertSeverityInfo
	case SeverityMedium:
		return AlertSeverityWarning
	case SeverityHigh:
		return AlertSeverityHigh
	case SeverityCritical:
		return AlertSeverityCritical
	default:
		return AlertSeverityInfo
	}
}
