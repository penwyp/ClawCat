package calculations

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/models"
)

// AggregationView 聚合视图类型
type AggregationView string

const (
	DailyView   AggregationView = "daily"
	WeeklyView  AggregationView = "weekly"
	MonthlyView AggregationView = "monthly"
	CustomView  AggregationView = "custom"
)

// AggregatedData 聚合数据
type AggregatedData struct {
	Period   TimePeriod                       `json:"period"`
	Entries  int                              `json:"entries"`
	Tokens   TokenStats                       `json:"tokens"`
	Cost     CostStats                        `json:"cost"`
	Models   map[string]AggregationModelStats `json:"models"`
	Sessions []SessionSummary                 `json:"sessions"`
	Patterns UsagePattern                     `json:"patterns"`
}

// TimePeriod 时间段
type TimePeriod struct {
	Start time.Time       `json:"start"`
	End   time.Time       `json:"end"`
	Label string          `json:"label"` // e.g., "2024-01-15", "Week 3", "January 2024"
	Type  AggregationView `json:"type"`
}

// TokenStats token 统计
type TokenStats struct {
	Total    int       `json:"total"`
	Input    int       `json:"input"`
	Output   int       `json:"output"`
	Cache    int       `json:"cache"`
	Average  float64   `json:"average"`
	Peak     int       `json:"peak"`
	PeakTime time.Time `json:"peak_time"`
	Min      int       `json:"min"`
}

// CostStats 成本统计
type CostStats struct {
	Total     float64            `json:"total"`
	Average   float64            `json:"average"`
	Min       float64            `json:"min"`
	Max       float64            `json:"max"`
	Breakdown map[string]float64 `json:"breakdown"` // 按模型分解
}

// AggregationModelStats 模型使用统计（聚合专用）
type AggregationModelStats struct {
	Count  int     `json:"count"`
	Tokens int     `json:"tokens"`
	Cost   float64 `json:"cost"`
}

// SessionSummary 会话摘要
type SessionSummary struct {
	ID       string        `json:"id"`
	Start    time.Time     `json:"start"`
	Duration time.Duration `json:"duration"`
	Tokens   int           `json:"tokens"`
	Cost     float64       `json:"cost"`
}

// UsagePattern 使用模式
type UsagePattern struct {
	PeakHours []int     `json:"peak_hours"` // 高峰时段（小时）
	PeakDays  []string  `json:"peak_days"`  // 高峰日期（星期几）
	Trend     TrendType `json:"trend"`      // 上升、下降、稳定
	Anomalies []Anomaly `json:"anomalies"`  // 异常使用
}

// TrendType 趋势类型
type TrendType string

const (
	TrendUp     TrendType = "up"
	TrendDown   TrendType = "down"
	TrendStable TrendType = "stable"
)

// Anomaly 异常使用
type Anomaly struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Expected  float64   `json:"expected"`
	Severity  string    `json:"severity"`
}

// AggregationEngine 数据聚合引擎
type AggregationEngine struct {
	mu       sync.RWMutex
	entries  []models.UsageEntry
	timezone *time.Location
	config   *config.Config
	cache    *AggregationCache
}

// AggregationCache 聚合数据缓存
type AggregationCache struct {
	mu      sync.RWMutex
	cache   map[string]CacheEntry
	maxSize int
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Data      []AggregatedData
	Timestamp time.Time
	TTL       time.Duration
}

// NewAggregationEngine 创建聚合引擎
func NewAggregationEngine(entries []models.UsageEntry, cfg *config.Config) *AggregationEngine {
	timezone := time.Local
	if cfg != nil && cfg.App.Timezone != "" {
		if tz, err := time.LoadLocation(cfg.App.Timezone); err == nil {
			timezone = tz
		}
	}

	return &AggregationEngine{
		entries:  entries,
		timezone: timezone,
		config:   cfg,
		cache:    NewAggregationCache(100),
	}
}

// NewAggregationCache 创建聚合缓存
func NewAggregationCache(maxSize int) *AggregationCache {
	return &AggregationCache{
		cache:   make(map[string]CacheEntry),
		maxSize: maxSize,
	}
}

// Aggregate 执行聚合操作
func (ae *AggregationEngine) Aggregate(view AggregationView, start, end time.Time) ([]AggregatedData, error) {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	// 检查缓存
	cacheKey := fmt.Sprintf("%s_%s_%s", view, start.Format("20060102"), end.Format("20060102"))
	if cached, ok := ae.cache.Get(cacheKey); ok {
		return cached, nil
	}

	// 过滤时间范围内的数据
	filtered := ae.filterByTimeRange(start, end)
	if len(filtered) == 0 {
		return []AggregatedData{}, nil
	}

	// 根据视图类型分组
	var grouped map[string][]models.UsageEntry

	switch view {
	case DailyView:
		grouped = ae.groupByDay(filtered)
	case WeeklyView:
		grouped = ae.groupByWeek(filtered)
	case MonthlyView:
		grouped = ae.groupByMonth(filtered)
	default:
		return nil, fmt.Errorf("unsupported view: %s", view)
	}

	// 计算每个分组的统计数据
	results := make([]AggregatedData, 0, len(grouped))
	for periodKey, entries := range grouped {
		data := ae.calculateStats(entries, periodKey, view)
		results = append(results, data)
	}

	// 按时间排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Period.Start.Before(results[j].Period.Start)
	})

	// 缓存结果
	ae.cache.Set(cacheKey, results, time.Hour)

	return results, nil
}

// filterByTimeRange 过滤时间范围内的数据
func (ae *AggregationEngine) filterByTimeRange(start, end time.Time) []models.UsageEntry {
	var filtered []models.UsageEntry
	for _, entry := range ae.entries {
		if entry.Timestamp.After(start) && entry.Timestamp.Before(end) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// groupByDay 按天分组
func (ae *AggregationEngine) groupByDay(entries []models.UsageEntry) map[string][]models.UsageEntry {
	grouped := make(map[string][]models.UsageEntry)

	for _, entry := range entries {
		// 转换到用户时区
		localTime := entry.Timestamp.In(ae.timezone)
		dayKey := localTime.Format("2006-01-02")
		grouped[dayKey] = append(grouped[dayKey], entry)
	}

	return grouped
}

// groupByWeek 按周分组
func (ae *AggregationEngine) groupByWeek(entries []models.UsageEntry) map[string][]models.UsageEntry {
	grouped := make(map[string][]models.UsageEntry)

	for _, entry := range entries {
		localTime := entry.Timestamp.In(ae.timezone)
		year, week := localTime.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", year, week)
		grouped[weekKey] = append(grouped[weekKey], entry)
	}

	return grouped
}

// groupByMonth 按月分组
func (ae *AggregationEngine) groupByMonth(entries []models.UsageEntry) map[string][]models.UsageEntry {
	grouped := make(map[string][]models.UsageEntry)

	for _, entry := range entries {
		localTime := entry.Timestamp.In(ae.timezone)
		monthKey := localTime.Format("2006-01")
		grouped[monthKey] = append(grouped[monthKey], entry)
	}

	return grouped
}

// calculateStats 计算统计数据
func (ae *AggregationEngine) calculateStats(entries []models.UsageEntry, periodKey string, view AggregationView) AggregatedData {
	if len(entries) == 0 {
		return AggregatedData{
			Period: ae.parsePeriod(periodKey, view),
			Models: make(map[string]AggregationModelStats),
		}
	}

	data := AggregatedData{
		Period:  ae.parsePeriod(periodKey, view),
		Entries: len(entries),
		Models:  make(map[string]AggregationModelStats),
	}

	// 初始化统计
	tokenStats := TokenStats{Min: int(^uint(0) >> 1)} // Max int
	costStats := CostStats{
		Min:       float64(^uint(0) >> 1), // Max float64
		Breakdown: make(map[string]float64),
	}

	// 遍历计算
	for _, entry := range entries {
		// Token 统计
		tokenStats.Total += entry.TotalTokens
		tokenStats.Input += entry.InputTokens
		tokenStats.Output += entry.OutputTokens
		tokenStats.Cache += entry.CacheCreationTokens + entry.CacheReadTokens

		if entry.TotalTokens > tokenStats.Peak {
			tokenStats.Peak = entry.TotalTokens
			tokenStats.PeakTime = entry.Timestamp
		}

		if entry.TotalTokens < tokenStats.Min {
			tokenStats.Min = entry.TotalTokens
		}

		// 成本统计
		costStats.Total += entry.CostUSD
		if entry.CostUSD > costStats.Max {
			costStats.Max = entry.CostUSD
		}
		if entry.CostUSD < costStats.Min {
			costStats.Min = entry.CostUSD
		}

		// 模型统计
		modelStat := data.Models[entry.Model]
		modelStat.Count++
		modelStat.Tokens += entry.TotalTokens
		modelStat.Cost += entry.CostUSD
		data.Models[entry.Model] = modelStat

		// 成本分解
		costStats.Breakdown[entry.Model] += entry.CostUSD
	}

	// 计算平均值
	if len(entries) > 0 {
		tokenStats.Average = float64(tokenStats.Total) / float64(len(entries))
		costStats.Average = costStats.Total / float64(len(entries))
	}

	data.Tokens = tokenStats
	data.Cost = costStats

	return data
}

// parsePeriod 解析时间段
func (ae *AggregationEngine) parsePeriod(key string, view AggregationView) TimePeriod {
	period := TimePeriod{
		Label: key,
		Type:  view,
	}

	switch view {
	case DailyView:
		if t, err := time.ParseInLocation("2006-01-02", key, ae.timezone); err == nil {
			period.Start = t
			period.End = t.Add(24 * time.Hour).Add(-time.Nanosecond)
		}
	case WeeklyView:
		// 解析 "2024-W03" 格式
		var year, week int
		if _, err := fmt.Sscanf(key, "%d-W%d", &year, &week); err == nil {
			period.Start = weekStart(year, week, ae.timezone)
			period.End = period.Start.Add(7 * 24 * time.Hour).Add(-time.Nanosecond)
			period.Label = fmt.Sprintf("Week %d, %d", week, year)
		}
	case MonthlyView:
		if t, err := time.ParseInLocation("2006-01", key, ae.timezone); err == nil {
			period.Start = t
			period.End = t.AddDate(0, 1, 0).Add(-time.Nanosecond)
			period.Label = t.Format("January 2006")
		}
	}

	return period
}

// DetectPatterns 检测使用模式
func (ae *AggregationEngine) DetectPatterns(aggregated []AggregatedData) UsagePattern {
	if len(aggregated) < 7 { // 需要至少一周的数据
		return UsagePattern{}
	}

	pattern := UsagePattern{
		PeakHours: ae.findPeakHours(aggregated),
		PeakDays:  ae.findPeakDays(aggregated),
		Trend:     ae.detectTrend(aggregated),
		Anomalies: ae.detectAnomalies(aggregated),
	}

	return pattern
}

// findPeakHours 查找高峰时段
func (ae *AggregationEngine) findPeakHours(aggregated []AggregatedData) []int {
	hourlyUsage := make(map[int]int)

	// 统计每小时的使用量
	for _, entry := range ae.entries {
		hour := entry.Timestamp.In(ae.timezone).Hour()
		hourlyUsage[hour] += entry.TotalTokens
	}

	// 找出高峰时段（使用量前25%）
	type hourUsage struct {
		hour  int
		usage int
	}

	var hours []hourUsage
	for h, u := range hourlyUsage {
		hours = append(hours, hourUsage{hour: h, usage: u})
	}

	sort.Slice(hours, func(i, j int) bool {
		return hours[i].usage > hours[j].usage
	})

	// 返回前25%的小时
	peakCount := len(hours) / 4
	if peakCount < 1 {
		peakCount = 1
	}

	var peakHours []int
	for i := 0; i < peakCount && i < len(hours); i++ {
		peakHours = append(peakHours, hours[i].hour)
	}

	return peakHours
}

// findPeakDays 查找高峰日期
func (ae *AggregationEngine) findPeakDays(aggregated []AggregatedData) []string {
	dayUsage := make(map[string]int)

	for _, data := range aggregated {
		dayName := data.Period.Start.Weekday().String()
		dayUsage[dayName] += data.Tokens.Total
	}

	// 找出使用量最高的日期
	type dayUsageStruct struct {
		day   string
		usage int
	}

	var days []dayUsageStruct
	for d, u := range dayUsage {
		days = append(days, dayUsageStruct{day: d, usage: u})
	}

	sort.Slice(days, func(i, j int) bool {
		return days[i].usage > days[j].usage
	})

	// 返回使用量高于平均值的日期
	if len(days) == 0 {
		return []string{}
	}

	totalUsage := 0
	for _, d := range days {
		totalUsage += d.usage
	}
	avgUsage := totalUsage / len(days)

	var peakDays []string
	for _, d := range days {
		if d.usage > avgUsage {
			peakDays = append(peakDays, d.day)
		}
	}

	return peakDays
}

// detectTrend 检测趋势
func (ae *AggregationEngine) detectTrend(aggregated []AggregatedData) TrendType {
	if len(aggregated) < 3 {
		return TrendStable
	}

	// 使用线性回归检测趋势
	n := len(aggregated)
	var sumX, sumY, sumXY, sumX2 float64

	for i, data := range aggregated {
		x := float64(i)
		y := float64(data.Tokens.Total)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// 计算斜率
	slope := (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)

	// 判断趋势
	threshold := 0.1 // 阈值可调
	if slope > threshold {
		return TrendUp
	} else if slope < -threshold {
		return TrendDown
	}
	return TrendStable
}

// detectAnomalies 检测异常
func (ae *AggregationEngine) detectAnomalies(aggregated []AggregatedData) []Anomaly {
	if len(aggregated) < 5 {
		return []Anomaly{}
	}

	// 计算平均值和标准差
	var values []float64
	for _, data := range aggregated {
		values = append(values, float64(data.Tokens.Total))
	}

	mean := ae.calculateMean(values)
	stdDev := ae.calculateStdDev(values, mean)

	// 检测异常值（超过2个标准差）
	var anomalies []Anomaly
	threshold := 2.0

	for _, data := range aggregated {
		value := float64(data.Tokens.Total)
		deviation := (value - mean) / stdDev

		if deviation > threshold || deviation < -threshold {
			anomalies = append(anomalies, Anomaly{
				Timestamp: data.Period.Start,
				Value:     value,
				Expected:  mean,
				Severity:  ae.getAnomalySeverity(deviation),
			})
		}
	}

	return anomalies
}

// calculateMean 计算平均值
func (ae *AggregationEngine) calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateStdDev 计算标准差
func (ae *AggregationEngine) calculateStdDev(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	sumSquares := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)-1))
}

// getAnomalySeverity 获取异常严重程度
func (ae *AggregationEngine) getAnomalySeverity(deviation float64) string {
	absDeviation := deviation
	if deviation < 0 {
		absDeviation = -deviation
	}

	if absDeviation > 3.0 {
		return "critical"
	} else if absDeviation > 2.5 {
		return "high"
	} else if absDeviation > 2.0 {
		return "medium"
	}
	return "low"
}

// weekStart 计算指定年和周的开始时间
func weekStart(year, week int, loc *time.Location) time.Time {
	// 计算该年第一天
	jan1 := time.Date(year, 1, 1, 0, 0, 0, 0, loc)

	// 找到第一个星期一
	jan1Weekday := int(jan1.Weekday())
	if jan1Weekday == 0 {
		jan1Weekday = 7 // 将周日从0改为7
	}

	firstMonday := jan1.AddDate(0, 0, 8-jan1Weekday)

	// 计算指定周的开始
	return firstMonday.AddDate(0, 0, (week-1)*7)
}

// Get 获取缓存数据
func (c *AggregationCache) Get(key string) ([]AggregatedData, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Since(entry.Timestamp) > entry.TTL {
		delete(c.cache, key)
		return nil, false
	}

	return entry.Data, true
}

// Set 设置缓存数据
func (c *AggregationCache) Set(key string, data []AggregatedData, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果缓存已满，删除最旧的条目
	if len(c.cache) >= c.maxSize {
		var oldestKey string
		var oldestTime time.Time = time.Now()

		for k, v := range c.cache {
			if v.Timestamp.Before(oldestTime) {
				oldestTime = v.Timestamp
				oldestKey = k
			}
		}

		if oldestKey != "" {
			delete(c.cache, oldestKey)
		}
	}

	c.cache[key] = CacheEntry{
		Data:      data,
		Timestamp: time.Now(),
		TTL:       ttl,
	}
}
