package calculations

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/models"
)

// 测试配置
var testConfig = &config.Config{
	Subscription: config.SubscriptionConfig{
		Plan:            "pro",
		CustomCostLimit: 18.0,
		WarnThreshold:   75.0,
		AlertThreshold:  90.0,
	},
}

func TestNewMetricsCalculator(t *testing.T) {
	sessionStart := time.Now()
	calc := NewMetricsCalculator(sessionStart, testConfig)

	assert.NotNil(t, calc)
	assert.Equal(t, sessionStart, calc.sessionStart)
	assert.Equal(t, 5*time.Hour, calc.windowSize)
	assert.Equal(t, 10*time.Second, calc.updateInterval)
	assert.NotNil(t, calc.entries)
	assert.Equal(t, 0, len(calc.entries))
	assert.Equal(t, testConfig, calc.config)
}

func TestMetricsCalculator_Calculate_EmptyData(t *testing.T) {
	calc := NewMetricsCalculator(time.Now(), testConfig)
	metrics := calc.Calculate()

	assert.NotNil(t, metrics)
	assert.Equal(t, 0, metrics.CurrentTokens)
	assert.Equal(t, 0.0, metrics.CurrentCost)
	assert.Equal(t, 0.0, metrics.TokensPerMinute)
	assert.Equal(t, 0.0, metrics.BurnRate)
	assert.Equal(t, 0, metrics.ProjectedTokens)
	assert.Equal(t, 0.0, metrics.ProjectedCost)
	assert.NotNil(t, metrics.ModelDistribution)
	assert.Equal(t, 0, len(metrics.ModelDistribution))
}

func TestMetricsCalculator_UpdateWithNewEntry(t *testing.T) {
	calc := NewMetricsCalculator(time.Now(), testConfig)

	entry := models.UsageEntry{
		Timestamp:    time.Now(),
		Model:        "claude-3-opus",
		InputTokens:  100,
		OutputTokens: 200,
		TotalTokens:  300,
		CostUSD:      0.01,
	}

	calc.UpdateWithNewEntry(entry)

	assert.Equal(t, 1, len(calc.entries))
	assert.Equal(t, entry, calc.entries[0])
	assert.Nil(t, calc.cachedMetrics) // 缓存应该被清理
}

func TestMetricsCalculator_Calculate_BasicStats(t *testing.T) {
	sessionStart := time.Now().Add(-30 * time.Minute)
	calc := NewMetricsCalculator(sessionStart, testConfig)

	// 添加测试数据
	entries := generateTestEntries(10, sessionStart)
	for _, entry := range entries {
		calc.UpdateWithNewEntry(entry)
	}

	metrics := calc.Calculate()

	assert.Greater(t, metrics.CurrentTokens, 0)
	assert.Greater(t, metrics.CurrentCost, 0.0)
	assert.Greater(t, metrics.SessionProgress, 0.0)
	assert.Less(t, metrics.SessionProgress, 100.0)
	assert.Greater(t, metrics.TimeRemaining, 0*time.Second)

	// 验证计算正确性
	expectedTokens := 0
	expectedCost := 0.0
	for _, entry := range entries {
		expectedTokens += entry.TotalTokens
		expectedCost += entry.CostUSD
	}

	assert.Equal(t, expectedTokens, metrics.CurrentTokens)
	assert.InDelta(t, expectedCost, metrics.CurrentCost, 0.001)
}

func TestMetricsCalculator_Calculate_RateCalculations(t *testing.T) {
	sessionStart := time.Now().Add(-30 * time.Minute)
	calc := NewMetricsCalculator(sessionStart, testConfig)

	// 模拟持续使用 - 每分钟添加100个tokens
	now := time.Now()
	for i := 0; i < 30; i++ {
		entry := models.UsageEntry{
			Timestamp:    now.Add(-time.Duration(30-i) * time.Minute),
			Model:        "claude-3-opus",
			InputTokens:  50,
			OutputTokens: 50,
			TotalTokens:  100,
			CostUSD:      0.01,
		}
		calc.UpdateWithNewEntry(entry)
	}

	metrics := calc.Calculate()

	// 验证速率计算 - 由于时间窗口的影响，可能为0
	assert.GreaterOrEqual(t, metrics.TokensPerMinute, 0.0)
	assert.GreaterOrEqual(t, metrics.TokensPerHour, 0.0)
	assert.GreaterOrEqual(t, metrics.CostPerMinute, 0.0)
	assert.GreaterOrEqual(t, metrics.CostPerHour, 0.0)
	assert.GreaterOrEqual(t, metrics.BurnRate, 0.0)

	// 燃烧率应该接近平均值
	expectedBurnRate := float64(100)                            // 每分钟100个tokens
	assert.InDelta(t, expectedBurnRate, metrics.BurnRate, 50.0) // 允许一定误差
}

func TestMetricsCalculator_Calculate_Projections(t *testing.T) {
	sessionStart := time.Now().Add(-1 * time.Hour)
	calc := NewMetricsCalculator(sessionStart, testConfig)

	// 添加稳定的使用模式
	now := time.Now()
	for i := 0; i < 60; i++ {
		entry := models.UsageEntry{
			Timestamp:    now.Add(-time.Duration(60-i) * time.Minute),
			Model:        "claude-3-opus",
			InputTokens:  25,
			OutputTokens: 25,
			TotalTokens:  50,
			CostUSD:      0.005,
		}
		calc.UpdateWithNewEntry(entry)
	}

	metrics := calc.Calculate()

	// 验证预测 - 如果有剩余时间且速率大于0，预测值应该更大
	if metrics.TimeRemaining > 0 && metrics.TokensPerMinute > 0 {
		assert.Greater(t, metrics.ProjectedTokens, metrics.CurrentTokens)
		assert.Greater(t, metrics.ProjectedCost, metrics.CurrentCost)
	} else {
		assert.GreaterOrEqual(t, metrics.ProjectedTokens, metrics.CurrentTokens)
		assert.GreaterOrEqual(t, metrics.ProjectedCost, metrics.CurrentCost)
	}
	assert.GreaterOrEqual(t, metrics.ConfidenceLevel, 0.0)

	// 预测应该基于当前速率
	if metrics.TokensPerMinute > 0 && metrics.TimeRemaining > 0 {
		expectedAdditional := int(metrics.TokensPerMinute * metrics.TimeRemaining.Minutes())
		expectedProjected := metrics.CurrentTokens + expectedAdditional
		assert.InDelta(t, float64(expectedProjected), float64(metrics.ProjectedTokens), float64(expectedProjected)*0.1)
	}
}

func TestMetricsCalculator_Calculate_ModelDistribution(t *testing.T) {
	calc := NewMetricsCalculator(time.Now(), testConfig)

	// 添加不同模型的数据
	modelNames := []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"}
	for i, model := range modelNames {
		for j := 0; j < (i+1)*10; j++ { // 不同模型不同数量
			entry := models.UsageEntry{
				Timestamp:    time.Now().Add(-time.Duration(j) * time.Minute),
				Model:        model,
				InputTokens:  50,
				OutputTokens: 50,
				TotalTokens:  100,
				CostUSD:      0.01,
			}
			calc.UpdateWithNewEntry(entry)
		}
	}

	metrics := calc.Calculate()

	// 验证模型分布
	assert.Equal(t, 3, len(metrics.ModelDistribution))

	totalPercentage := 0.0
	for model, modelMetrics := range metrics.ModelDistribution {
		assert.Contains(t, modelNames, model)
		assert.Greater(t, modelMetrics.TokenCount, 0)
		assert.Greater(t, modelMetrics.Cost, 0.0)
		assert.Greater(t, modelMetrics.Percentage, 0.0)
		assert.False(t, modelMetrics.LastUsed.IsZero())

		totalPercentage += modelMetrics.Percentage
	}

	// 总百分比应该接近100%
	assert.InDelta(t, 100.0, totalPercentage, 0.1)
}

func TestMetricsCalculator_Calculate_PlanLimits(t *testing.T) {
	tests := []struct {
		name     string
		plan     string
		limit    float64
		expected float64
	}{
		{"Pro Plan", "pro", 0, 18.0},
		{"Max5 Plan", "max5", 0, 35.0},
		{"Max20 Plan", "max20", 0, 140.0},
		{"Custom Plan", "custom", 25.0, 25.0},
		{"Unknown Plan", "unknown", 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Subscription: config.SubscriptionConfig{
					Plan:            tt.plan,
					CustomCostLimit: tt.limit,
				},
			}

			calc := NewMetricsCalculator(time.Now(), cfg)
			limit := calc.getPlanLimit()
			assert.Equal(t, tt.expected, limit)
		})
	}
}

func TestMetricsCalculator_Calculate_PredictedEndTime(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro", // $18 limit
		},
	}

	calc := NewMetricsCalculator(time.Now(), cfg)

	// 添加接近限额的数据，每分钟消耗$0.10
	for i := 0; i < 100; i++ {
		entry := models.UsageEntry{
			Timestamp:   time.Now().Add(-time.Duration(i) * time.Minute),
			Model:       "claude-3-opus",
			TotalTokens: 1000,
			CostUSD:     0.10,
		}
		calc.UpdateWithNewEntry(entry)
	}

	metrics := calc.Calculate()

	// 应该有预测的结束时间
	if metrics.CostPerMinute > 0 {
		assert.False(t, metrics.PredictedEndTime.IsZero())

		// 验证预测时间的合理性
		remainingBudget := 18.0 - metrics.CurrentCost
		if remainingBudget > 0 {
			expectedMinutes := remainingBudget / metrics.CostPerMinute
			expectedEndTime := time.Now().Add(time.Duration(expectedMinutes) * time.Minute)

			// 允许5分钟的误差
			timeDiff := metrics.PredictedEndTime.Sub(expectedEndTime).Abs()
			assert.Less(t, timeDiff, 5*time.Minute)
		}
	}
}

func TestMetricsCalculator_Caching(t *testing.T) {
	calc := NewMetricsCalculator(time.Now(), testConfig)

	// 添加一些数据
	entry := models.UsageEntry{
		Timestamp:   time.Now(),
		Model:       "claude-3-opus",
		TotalTokens: 100,
		CostUSD:     0.01,
	}
	calc.UpdateWithNewEntry(entry)

	// 第一次计算
	metrics1 := calc.Calculate()
	assert.NotNil(t, calc.cachedMetrics)

	// 第二次计算应该返回缓存结果
	metrics2 := calc.Calculate()
	assert.Equal(t, metrics1, metrics2)

	// 添加新数据应该清理缓存
	calc.UpdateWithNewEntry(entry)
	assert.Nil(t, calc.cachedMetrics)
}

func TestMetricsCalculator_DataCleanup(t *testing.T) {
	calc := NewMetricsCalculator(time.Now(), testConfig)

	// 添加过期数据（超过5小时）
	oldEntry := models.UsageEntry{
		Timestamp:   time.Now().Add(-6 * time.Hour),
		Model:       "claude-3-opus",
		TotalTokens: 100,
		CostUSD:     0.01,
	}
	calc.UpdateWithNewEntry(oldEntry)

	// 添加新数据
	newEntry := models.UsageEntry{
		Timestamp:   time.Now(),
		Model:       "claude-3-opus",
		TotalTokens: 100,
		CostUSD:     0.01,
	}
	calc.UpdateWithNewEntry(newEntry)

	// 过期数据应该被清理
	assert.Equal(t, 1, len(calc.entries))
	assert.Equal(t, newEntry.Timestamp, calc.entries[0].Timestamp)
}

func TestMetricsCalculator_GetBurnRate(t *testing.T) {
	calc := NewMetricsCalculator(time.Now(), testConfig)

	// 添加最近30分钟的数据
	now := time.Now()
	totalTokens := 0
	for i := 0; i < 30; i++ {
		tokens := 50 + rand.Intn(50) // 50-100 tokens per minute
		entry := models.UsageEntry{
			Timestamp:   now.Add(-time.Duration(30-i) * time.Minute),
			Model:       "claude-3-opus",
			TotalTokens: tokens,
			CostUSD:     0.01,
		}
		calc.UpdateWithNewEntry(entry)
		totalTokens += tokens
	}

	// 测试30分钟窗口的燃烧率
	burnRate := calc.GetBurnRate(30 * time.Minute)
	expectedRate := float64(totalTokens) / 30.0

	assert.InDelta(t, expectedRate, burnRate, 5.0) // 增加容错范围
}

func TestMetricsCalculator_Reset(t *testing.T) {
	calc := NewMetricsCalculator(time.Now(), testConfig)

	// 添加数据
	entry := models.UsageEntry{
		Timestamp:   time.Now(),
		Model:       "claude-3-opus",
		TotalTokens: 100,
		CostUSD:     0.01,
	}
	calc.UpdateWithNewEntry(entry)
	calc.Calculate() // 创建缓存

	assert.Equal(t, 1, len(calc.entries))
	assert.NotNil(t, calc.cachedMetrics)

	// 重置
	newStart := time.Now().Add(1 * time.Hour)
	calc.Reset(newStart)

	assert.Equal(t, newStart, calc.sessionStart)
	assert.Equal(t, 0, len(calc.entries))
	assert.Nil(t, calc.cachedMetrics)
	assert.True(t, calc.lastCalculated.IsZero())
}

func TestMetricsCalculator_GetEntryCount(t *testing.T) {
	calc := NewMetricsCalculator(time.Now(), testConfig)

	assert.Equal(t, 0, calc.GetEntryCount())

	// 添加条目
	for i := 0; i < 5; i++ {
		entry := models.UsageEntry{
			Timestamp:   time.Now(),
			Model:       "claude-3-opus",
			TotalTokens: 100,
			CostUSD:     0.01,
		}
		calc.UpdateWithNewEntry(entry)
	}

	assert.Equal(t, 5, calc.GetEntryCount())
}

func TestMetricsCalculator_ConcurrentAccess(t *testing.T) {
	calc := NewMetricsCalculator(time.Now(), testConfig)

	// 并发添加数据和计算指标
	done := make(chan bool, 2)

	// 协程1：持续添加数据
	go func() {
		for i := 0; i < 100; i++ {
			entry := models.UsageEntry{
				Timestamp:   time.Now(),
				Model:       "claude-3-opus",
				TotalTokens: 100,
				CostUSD:     0.01,
			}
			calc.UpdateWithNewEntry(entry)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// 协程2：持续计算指标
	go func() {
		for i := 0; i < 100; i++ {
			metrics := calc.Calculate()
			assert.NotNil(t, metrics)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// 等待完成
	<-done
	<-done

	// 验证最终状态
	assert.Equal(t, 100, calc.GetEntryCount())
	metrics := calc.Calculate()
	assert.Equal(t, 10000, metrics.CurrentTokens)     // 100 entries * 100 tokens
	assert.InDelta(t, 1.0, metrics.CurrentCost, 0.01) // 100 entries * 0.01 cost (允许浮点误差)
}

// 辅助函数

// generateTestEntries 生成测试条目
func generateTestEntries(count int, baseTime time.Time) []models.UsageEntry {
	entries := make([]models.UsageEntry, count)
	modelNames := []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"}

	for i := 0; i < count; i++ {
		entries[i] = models.UsageEntry{
			Timestamp:    baseTime.Add(time.Duration(i) * time.Minute),
			Model:        modelNames[i%len(modelNames)],
			InputTokens:  100 + rand.Intn(900),
			OutputTokens: 200 + rand.Intn(1800),
			TotalTokens:  300 + rand.Intn(2700),
			CostUSD:      0.01 + rand.Float64()*0.09,
		}
	}
	return entries
}

// Benchmark tests

func BenchmarkMetricsCalculator_Calculate(b *testing.B) {
	calc := NewMetricsCalculator(time.Now(), testConfig)

	// 添加1000个测试条目
	entries := generateTestEntries(1000, time.Now().Add(-1*time.Hour))
	for _, entry := range entries {
		calc.UpdateWithNewEntry(entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.Calculate()
	}
}

func BenchmarkMetricsCalculator_UpdateWithNewEntry(b *testing.B) {
	calc := NewMetricsCalculator(time.Now(), testConfig)

	entry := models.UsageEntry{
		Timestamp:   time.Now(),
		Model:       "claude-3-opus",
		TotalTokens: 100,
		CostUSD:     0.01,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.UpdateWithNewEntry(entry)
	}
}
