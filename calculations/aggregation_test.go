package calculations

import (
	"math/rand"
	"testing"
	"time"

	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/models"
	"github.com/stretchr/testify/assert"
)

func TestAggregationEngine_GroupByDay(t *testing.T) {
	// 创建测试数据
	entries := []models.UsageEntry{
		{Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), TotalTokens: 100},
		{Timestamp: time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC), TotalTokens: 200},
		{Timestamp: time.Date(2024, 1, 16, 9, 0, 0, 0, time.UTC), TotalTokens: 150},
	}

	testConfig := &config.Config{App: config.AppConfig{Timezone: "UTC"}}
	engine := NewAggregationEngine(entries, testConfig)

	grouped := engine.groupByDay(entries)

	assert.Len(t, grouped, 2)
	assert.Len(t, grouped["2024-01-15"], 2)
	assert.Len(t, grouped["2024-01-16"], 1)
}

func TestAggregationEngine_GroupByWeek(t *testing.T) {
	entries := []models.UsageEntry{
		{Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), TotalTokens: 100}, // Week 3
		{Timestamp: time.Date(2024, 1, 22, 14, 0, 0, 0, time.UTC), TotalTokens: 200}, // Week 4
		{Timestamp: time.Date(2024, 1, 29, 9, 0, 0, 0, time.UTC), TotalTokens: 150},  // Week 5
	}

	testConfig := &config.Config{App: config.AppConfig{Timezone: "UTC"}}
	engine := NewAggregationEngine(entries, testConfig)

	grouped := engine.groupByWeek(entries)

	// 验证分组结果
	assert.GreaterOrEqual(t, len(grouped), 2) // 至少有2个不同的周
}

func TestAggregationEngine_GroupByMonth(t *testing.T) {
	entries := []models.UsageEntry{
		{Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), TotalTokens: 100},
		{Timestamp: time.Date(2024, 1, 25, 14, 0, 0, 0, time.UTC), TotalTokens: 200},
		{Timestamp: time.Date(2024, 2, 5, 9, 0, 0, 0, time.UTC), TotalTokens: 150},
	}

	testConfig := &config.Config{App: config.AppConfig{Timezone: "UTC"}}
	engine := NewAggregationEngine(entries, testConfig)

	grouped := engine.groupByMonth(entries)

	assert.Len(t, grouped, 2)
	assert.Len(t, grouped["2024-01"], 2)
	assert.Len(t, grouped["2024-02"], 1)
}

func TestAggregationEngine_CalculateStats(t *testing.T) {
	entries := []models.UsageEntry{
		{
			Model:        "claude-3-opus",
			TotalTokens:  1000,
			InputTokens:  400,
			OutputTokens: 600,
			CostUSD:      0.15,
			Timestamp:    time.Now(),
		},
		{
			Model:        "claude-3-sonnet",
			TotalTokens:  500,
			InputTokens:  200,
			OutputTokens: 300,
			CostUSD:      0.05,
			Timestamp:    time.Now(),
		},
	}

	testConfig := &config.Config{App: config.AppConfig{Timezone: "UTC"}}
	engine := NewAggregationEngine(entries, testConfig)
	stats := engine.calculateStats(entries, "2024-01-15", DailyView)

	assert.Equal(t, 1500, stats.Tokens.Total)
	assert.Equal(t, 600, stats.Tokens.Input)
	assert.Equal(t, 900, stats.Tokens.Output)
	assert.InDelta(t, 0.20, stats.Cost.Total, 0.01)
	assert.Equal(t, 750.0, stats.Tokens.Average)
	assert.Len(t, stats.Models, 2)

	// 验证模型统计
	opusStats := stats.Models["claude-3-opus"]
	assert.Equal(t, 1, opusStats.Count)
	assert.Equal(t, 1000, opusStats.Tokens)
	assert.InDelta(t, 0.15, opusStats.Cost, 0.01)

	sonnetStats := stats.Models["claude-3-sonnet"]
	assert.Equal(t, 1, sonnetStats.Count)
	assert.Equal(t, 500, sonnetStats.Tokens)
	assert.InDelta(t, 0.05, sonnetStats.Cost, 0.01)
}

func TestAggregationEngine_Aggregate(t *testing.T) {
	// 创建跨越多天的测试数据
	entries := generateAggregationTestEntries(30)
	testConfig := &config.Config{App: config.AppConfig{Timezone: "UTC"}}
	engine := NewAggregationEngine(entries, testConfig)

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	t.Run("daily aggregation", func(t *testing.T) {
		result, err := engine.Aggregate(DailyView, start, end)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)

		// 验证按时间排序
		for i := 1; i < len(result); i++ {
			assert.True(t, result[i-1].Period.Start.Before(result[i].Period.Start) ||
				result[i-1].Period.Start.Equal(result[i].Period.Start))
		}
	})

	t.Run("weekly aggregation", func(t *testing.T) {
		result, err := engine.Aggregate(WeeklyView, start, end)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)

		// 周聚合的数据点应该少于日聚合
		dailyResult, _ := engine.Aggregate(DailyView, start, end)
		assert.LessOrEqual(t, len(result), len(dailyResult))
	})

	t.Run("monthly aggregation", func(t *testing.T) {
		result, err := engine.Aggregate(MonthlyView, start, end)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)
		assert.LessOrEqual(t, len(result), 2) // 最多2个月
	})
}

func TestAggregationEngine_DetectPatterns(t *testing.T) {
	// 创建有模式的测试数据
	entries := generatePatternedEntries()
	testConfig := &config.Config{App: config.AppConfig{Timezone: "UTC"}}
	engine := NewAggregationEngine(entries, testConfig)

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	aggregated, err := engine.Aggregate(DailyView, start, end)
	assert.NoError(t, err)

	patterns := engine.DetectPatterns(aggregated)

	// 验证模式检测结果
	assert.NotEmpty(t, patterns.PeakHours)
	assert.NotEmpty(t, patterns.PeakDays)
	assert.Contains(t, []TrendType{TrendUp, TrendDown, TrendStable}, patterns.Trend)
}

func TestAggregationCache(t *testing.T) {
	cache := NewAggregationCache(10)

	// 测试数据
	testData := []AggregatedData{
		{
			Period: TimePeriod{
				Start: time.Now(),
				End:   time.Now().Add(time.Hour),
				Label: "test",
				Type:  DailyView,
			},
			Entries: 5,
			Tokens:  TokenStats{Total: 1000},
		},
	}

	// 测试设置和获取
	key := "test_key"
	cache.Set(key, testData, time.Minute)

	retrieved, found := cache.Get(key)
	assert.True(t, found)
	assert.Len(t, retrieved, 1)
	assert.Equal(t, testData[0].Entries, retrieved[0].Entries)

	// 测试过期
	cache.Set(key, testData, time.Nanosecond)
	time.Sleep(time.Millisecond) // 确保过期

	_, found = cache.Get(key)
	assert.False(t, found)
}

func TestAggregationEngine_DetectTrend(t *testing.T) {
	testConfig := &config.Config{App: config.AppConfig{Timezone: "UTC"}}
	engine := NewAggregationEngine([]models.UsageEntry{}, testConfig)

	t.Run("upward trend", func(t *testing.T) {
		// 创建上升趋势的数据
		aggregated := []AggregatedData{
			{Tokens: TokenStats{Total: 100}},
			{Tokens: TokenStats{Total: 200}},
			{Tokens: TokenStats{Total: 300}},
			{Tokens: TokenStats{Total: 400}},
			{Tokens: TokenStats{Total: 500}},
		}

		trend := engine.detectTrend(aggregated)
		assert.Equal(t, TrendUp, trend)
	})

	t.Run("downward trend", func(t *testing.T) {
		// 创建下降趋势的数据
		aggregated := []AggregatedData{
			{Tokens: TokenStats{Total: 500}},
			{Tokens: TokenStats{Total: 400}},
			{Tokens: TokenStats{Total: 300}},
			{Tokens: TokenStats{Total: 200}},
			{Tokens: TokenStats{Total: 100}},
		}

		trend := engine.detectTrend(aggregated)
		assert.Equal(t, TrendDown, trend)
	})

	t.Run("stable trend", func(t *testing.T) {
		// 创建稳定趋势的数据
		aggregated := []AggregatedData{
			{Tokens: TokenStats{Total: 300}},
			{Tokens: TokenStats{Total: 295}},
			{Tokens: TokenStats{Total: 305}},
			{Tokens: TokenStats{Total: 300}},
			{Tokens: TokenStats{Total: 298}},
		}

		trend := engine.detectTrend(aggregated)
		assert.Equal(t, TrendStable, trend)
	})
}

func TestAggregationEngine_FilterByTimeRange(t *testing.T) {
	entries := []models.UsageEntry{
		{Timestamp: time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC), TotalTokens: 100},
		{Timestamp: time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC), TotalTokens: 200},
		{Timestamp: time.Date(2024, 1, 20, 9, 0, 0, 0, time.UTC), TotalTokens: 150},
		{Timestamp: time.Date(2024, 1, 25, 16, 0, 0, 0, time.UTC), TotalTokens: 300},
	}

	testConfig := &config.Config{App: config.AppConfig{Timezone: "UTC"}}
	engine := NewAggregationEngine(entries, testConfig)

	start := time.Date(2024, 1, 12, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 22, 0, 0, 0, 0, time.UTC)

	filtered := engine.filterByTimeRange(start, end)

	assert.Len(t, filtered, 2) // 应该过滤出1/15和1/20的数据
	assert.Equal(t, 200, filtered[0].TotalTokens)
	assert.Equal(t, 150, filtered[1].TotalTokens)
}

// 测试辅助函数
func generateAggregationTestEntries(days int) []models.UsageEntry {
	entries := make([]models.UsageEntry, 0, days*5)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	modelNames := []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"}

	for i := 0; i < days; i++ {
		day := baseTime.AddDate(0, 0, i)

		// 每天生成3-7个条目
		entriesPerDay := 3 + rand.Intn(5)

		for j := 0; j < entriesPerDay; j++ {
			timestamp := day.Add(time.Duration(8+rand.Intn(12)) * time.Hour) // 8-20点之间

			entry := models.UsageEntry{
				Timestamp:    timestamp,
				Model:        modelNames[rand.Intn(len(modelNames))],
				InputTokens:  100 + rand.Intn(900),
				OutputTokens: 200 + rand.Intn(1800),
				TotalTokens:  300 + rand.Intn(2700),
				CostUSD:      0.01 + rand.Float64()*0.09,
			}

			entries = append(entries, entry)
		}
	}

	return entries
}

func generatePatternedEntries() []models.UsageEntry {
	entries := make([]models.UsageEntry, 0)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 30; i++ {
		day := baseTime.AddDate(0, 0, i)
		tokensBase := 1000

		// 周末使用量降低
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			tokensBase = 200
		}

		// 添加一天内的多个条目，高峰时间在12-14点
		for hour := 9; hour <= 17; hour++ {
			tokens := tokensBase
			if hour >= 12 && hour <= 14 {
				tokens = int(float64(tokens) * 1.5) // 高峰时段增加50%
			}

			entries = append(entries, models.UsageEntry{
				Timestamp:   day.Add(time.Duration(hour) * time.Hour),
				Model:       "claude-3-opus",
				TotalTokens: tokens + rand.Intn(500),
				CostUSD:     float64(tokens) * 0.000015, // 简化的成本计算
			})
		}
	}

	return entries
}

func BenchmarkAggregationEngine_Aggregate(b *testing.B) {
	entries := generateAggregationTestEntries(365) // 一年的数据
	testConfig := &config.Config{App: config.AppConfig{Timezone: "UTC"}}
	engine := NewAggregationEngine(entries, testConfig)

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Aggregate(DailyView, start, end)
	}
}

func BenchmarkAggregationEngine_DetectPatterns(b *testing.B) {
	entries := generatePatternedEntries()
	testConfig := &config.Config{App: config.AppConfig{Timezone: "UTC"}}
	engine := NewAggregationEngine(entries, testConfig)

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	aggregated, _ := engine.Aggregate(DailyView, start, end)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.DetectPatterns(aggregated)
	}
}
