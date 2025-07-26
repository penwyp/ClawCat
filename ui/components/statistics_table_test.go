package components

import (
	"fmt"
	"testing"
	"time"

	"github.com/penwyp/claudecat/calculations"
	"github.com/stretchr/testify/assert"
)

func TestStatisticsTable_Render(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		metrics  *calculations.RealtimeMetrics
		validate func(t *testing.T, output string)
	}{
		{
			name:  "basic table",
			width: 80,
			metrics: &calculations.RealtimeMetrics{
				SessionStart:      time.Now().Add(-2 * time.Hour),
				CurrentTokens:     50000,
				ProjectedTokens:   75000,
				CurrentCost:       10.50,
				ProjectedCost:     15.75,
				TokensPerMinute:   100,
				TokensPerHour:     6000,
				CostPerHour:       3.60,
				ModelDistribution: make(map[string]calculations.ModelMetrics),
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "Statistics Overview")
				assert.Contains(t, output, "Current")
				assert.Contains(t, output, "Projected")
				assert.Contains(t, output, "50.0K")
				assert.Contains(t, output, "75.0K")
				assert.Contains(t, output, "$10.50")
				assert.Contains(t, output, "$15.75")
			},
		},
		{
			name:  "compact mode",
			width: 50,
			metrics: &calculations.RealtimeMetrics{
				SessionStart:      time.Now().Add(-1 * time.Hour),
				CurrentTokens:     25000,
				CurrentCost:       5.25,
				ProjectedTokens:   30000,
				ProjectedCost:     6.30,
				ModelDistribution: make(map[string]calculations.ModelMetrics),
			},
			validate: func(t *testing.T, output string) {
				// 在紧凑模式下，仍应包含基本信息
				assert.Contains(t, output, "25.0K")
				assert.Contains(t, output, "$5.25")
				assert.NotEmpty(t, output)
			},
		},
		{
			name:  "with model distribution",
			width: 100,
			metrics: &calculations.RealtimeMetrics{
				SessionStart:    time.Now().Add(-1 * time.Hour),
				CurrentTokens:   100000,
				CurrentCost:     15.00,
				ProjectedTokens: 120000,
				ProjectedCost:   18.00,
				ModelDistribution: map[string]calculations.ModelMetrics{
					"claude-3-opus": {
						TokenCount: 60000,
						Cost:       9.00,
						Percentage: 60.0,
					},
					"claude-3-sonnet": {
						TokenCount: 40000,
						Cost:       6.00,
						Percentage: 40.0,
					},
				},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "Model Distribution")
				assert.Contains(t, output, "claude-3-opus")
				assert.Contains(t, output, "claude-3-sonnet")
				assert.Contains(t, output, "60.0%")
				assert.Contains(t, output, "40.0%")
			},
		},
		{
			name:  "zero values",
			width: 80,
			metrics: &calculations.RealtimeMetrics{
				SessionStart:      time.Now(),
				CurrentTokens:     0,
				CurrentCost:       0,
				ProjectedTokens:   0,
				ProjectedCost:     0,
				ModelDistribution: make(map[string]calculations.ModelMetrics),
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "Statistics Overview")
				assert.NotContains(t, output, "Model Distribution") // 应该没有模型分布
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := NewStatisticsTable(tt.width)
			table.Update(tt.metrics)

			output := table.Render()
			t.Logf("\n=== %s (width: %d) ===\n%s\n", tt.name, tt.width, output)

			tt.validate(t, output)
			assert.NotEmpty(t, output)
		})
	}
}

func TestStatisticsTable_Update(t *testing.T) {
	table := NewStatisticsTable(80)

	metrics := &calculations.RealtimeMetrics{
		SessionStart:    time.Now().Add(-30 * time.Minute),
		CurrentTokens:   1500,
		CurrentCost:     2.50,
		ProjectedTokens: 3000,
		ProjectedCost:   5.00,
		TokensPerMinute: 50,
		CostPerMinute:   0.083,
		ModelDistribution: map[string]calculations.ModelMetrics{
			"claude-3-sonnet": {
				TokenCount: 1500,
				Cost:       2.50,
				Percentage: 100.0,
			},
		},
	}

	table.Update(metrics)

	assert.Equal(t, 1500, table.stats.CurrentTokens)
	assert.Equal(t, 2.50, table.stats.CurrentCost)
	assert.Equal(t, 3000, table.stats.ProjectedTokens)
	assert.Equal(t, 5.00, table.stats.ProjectedCost)
	assert.Len(t, table.stats.ModelDistribution, 1)
}

func TestStatisticsTable_FormatChange(t *testing.T) {
	table := NewStatisticsTable(80)

	tests := []struct {
		name      string
		current   int
		projected int
		contains  []string
	}{
		{
			name:      "increase",
			current:   1000,
			projected: 1500,
			contains:  []string{"↑", "50%"},
		},
		{
			name:      "decrease",
			current:   1500,
			projected: 1000,
			contains:  []string{"↓", "33%"},
		},
		{
			name:      "no change",
			current:   1000,
			projected: 1000,
			contains:  []string{"—"},
		},
		{
			name:      "zero current",
			current:   0,
			projected: 1000,
			contains:  []string{"—"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := table.formatChange(tt.current, tt.projected)
			t.Logf("formatChange(%d, %d) = %s", tt.current, tt.projected, result)

			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestStatisticsTable_ModelDistribution(t *testing.T) {
	table := NewStatisticsTable(100)

	metrics := &calculations.RealtimeMetrics{
		SessionStart:  time.Now().Add(-1 * time.Hour),
		CurrentTokens: 10000,
		CurrentCost:   15.00,
		ModelDistribution: map[string]calculations.ModelMetrics{
			"claude-3-opus": {
				TokenCount: 6000,
				Cost:       9.00,
			},
			"claude-3-sonnet": {
				TokenCount: 4000,
				Cost:       6.00,
			},
		},
	}

	table.Update(metrics)

	// 验证模型分布计算
	assert.Len(t, table.stats.ModelDistribution, 2)

	// 验证按使用率排序
	assert.Equal(t, "claude-3-opus", table.stats.ModelDistribution[0].Model)
	assert.Equal(t, "claude-3-sonnet", table.stats.ModelDistribution[1].Model)

	// 验证百分比计算
	assert.Equal(t, 60.0, table.stats.ModelDistribution[0].Percentage)
	assert.Equal(t, 40.0, table.stats.ModelDistribution[1].Percentage)
}

func TestStatisticsTable_GetSummary(t *testing.T) {
	table := NewStatisticsTable(80)

	metrics := &calculations.RealtimeMetrics{
		SessionStart:      time.Now().Add(-1 * time.Hour),
		CurrentTokens:     5000,
		CurrentCost:       7.50,
		ProjectedTokens:   8000,
		ProjectedCost:     12.00,
		ModelDistribution: make(map[string]calculations.ModelMetrics),
	}

	table.Update(metrics)

	summary := table.GetSummary()
	t.Logf("Summary: %s", summary)

	assert.Contains(t, summary, "5.0K tokens")
	assert.Contains(t, summary, "$7.50")
	assert.Contains(t, summary, "8.0K tokens")
	assert.Contains(t, summary, "$12.00")
}

func TestStatisticsTable_RenderRateMetrics(t *testing.T) {
	table := NewStatisticsTable(100)

	metrics := &calculations.RealtimeMetrics{
		SessionStart:      time.Now().Add(-30 * time.Minute),
		CurrentTokens:     3000,
		CurrentCost:       4.50,
		TokensPerMinute:   100,
		TokensPerHour:     6000,
		CostPerMinute:     0.15,
		CostPerHour:       9.00,
		ModelDistribution: make(map[string]calculations.ModelMetrics),
	}

	table.Update(metrics)

	output := table.Render()

	// 在宽屏模式下应该包含燃烧率指标
	assert.Contains(t, output, "Burn Rate Metrics")
	assert.Contains(t, output, "Tokens/min: 100.0")
	assert.Contains(t, output, "Cost/hr:  $9.00")
}

func TestStatisticsTable_ResponsiveLayout(t *testing.T) {
	metrics := &calculations.RealtimeMetrics{
		SessionStart:      time.Now().Add(-1 * time.Hour),
		CurrentTokens:     1000,
		CurrentCost:       1.50,
		ProjectedTokens:   1500,
		ProjectedCost:     2.25,
		ModelDistribution: make(map[string]calculations.ModelMetrics),
	}

	widths := []int{50, 80, 120}

	for _, width := range widths {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			table := NewStatisticsTable(width)
			table.Update(metrics)

			output := table.Render()
			t.Logf("\n=== Width: %d ===\n%s\n", width, output)

			// 基本验证
			assert.NotEmpty(t, output)
			assert.Contains(t, output, "Statistics Overview")

			// 宽度大于80时才显示燃烧率
			if width > 80 {
				assert.Contains(t, output, "Burn Rate Metrics")
			}
		})
	}
}
