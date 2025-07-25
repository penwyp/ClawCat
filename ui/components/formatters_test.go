package components

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{12345, "12.3K"},
		{999999, "1000.0K"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
		{12345678, "12.3M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatNumber(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{0, "0m"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m"},
		{5 * time.Minute, "5m"},
		{65 * time.Minute, "1h 5m"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{25 * time.Hour, "25h 0m"},
		{-5 * time.Minute, "0m"}, // 负数处理
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "$0.00"},
		{1.23, "$1.23"},
		{9.99, "$9.99"},
		{99.99, "$99.99"},
		{123.45, "$123.45"},
		{999.99, "$999.99"},
		{1000, "$1.0K"},
		{1500, "$1.5K"},
		{12345, "$12.3K"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatCurrency(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatPercentage(t *testing.T) {
	tests := []struct {
		value     float64
		precision int
		expected  string
	}{
		{0, 1, "0.0%"},
		{25.5, 1, "25.5%"},
		{100, 1, "100.0%"},
		{33.333, 2, "33.33%"},
		{66.666, 0, "67%"},
		{math.NaN(), 1, "0.0%"},   // NaN 处理
		{math.Inf(1), 1, "0.0%"},  // Inf 处理
		{math.Inf(-1), 1, "0.0%"}, // -Inf 处理
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatPercentage(tt.value, tt.precision)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatRate(t *testing.T) {
	tests := []struct {
		rate     float64
		unit     string
		expected string
	}{
		{0, "tok/min", "0.00 tok/min"},
		{5.5, "tok/min", "5.50 tok/min"},
		{15.7, "tok/min", "15.7 tok/min"},
		{150, "tok/min", "150 tok/min"},
		{1500, "tok/min", "1.5K tok/min"},
		{12345, "req/s", "12.3K req/s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatRate(tt.rate, tt.unit)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"Hello", 10, "Hello"},
		{"Hello World", 10, "Hello W..."},
		{"Hello World", 5, "He..."},
		{"Hello World", 3, "Hel"},
		{"Hi", 5, "Hi"},
		{"", 5, ""},
		{"Test", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
			assert.LessOrEqual(t, len(result), tt.maxLen)
		})
	}
}

func TestFormatBurnRate(t *testing.T) {
	tests := []struct {
		rate     float64
		expected string
	}{
		{0, "0 tok/min"},
		{5.5, "5.5 tok/min"},
		{150.7, "150.7 tok/min"},
		{1500, "1.5K tok/min"},
		{12345, "12.3K tok/min"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBurnRate(tt.rate)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTokenCount(t *testing.T) {
	tests := []struct {
		tokens   int
		expected string
	}{
		{0, "0"},
		{500, "500"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{999999, "1000.0K"},
		{1000000, "1.00M"},
		{2500000, "2.50M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatTokenCount(tt.tokens)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatConfidence(t *testing.T) {
	tests := []struct {
		confidence float64
		expected   string
	}{
		{95, "95% (High)"},
		{85, "85% (Med)"},
		{65, "65% (Low)"},
		{45, "45% (Poor)"},
		{0, "0% (Poor)"},
		{100, "100% (High)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatConfidence(tt.confidence)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatChange(t *testing.T) {
	tests := []struct {
		current   float64
		projected float64
		expected  string
	}{
		{100, 150, "↑ 50.0%"},
		{150, 100, "↓ 33.3%"},
		{100, 100, "—"},
		{0, 100, "—"},
		{100, 100.05, "~0%"}, // 很小的变化
		{100, 99.95, "~0%"},  // 很小的变化
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatChange(tt.current, tt.projected)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatModelName(t *testing.T) {
	tests := []struct {
		model     string
		maxLength int
		expected  string
	}{
		{"claude-3-5-sonnet-20241022", 20, "Claude 3.5 Sonnet"},
		{"claude-3-opus-20240229", 15, "Claude 3 Opus"},
		{"gpt-4-turbo", 10, "GPT-4 Turbo"},
		{"some-very-long-model-name", 10, "some-ve..."},
		{"short", 20, "short"},
		{"unknown-model", 15, "unknown-model"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := formatModelName(tt.model, tt.maxLength)
			assert.Equal(t, tt.expected, result)
			assert.LessOrEqual(t, len(result), tt.maxLength)
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	now := time.Now()

	tests := []struct {
		timestamp time.Time
		expected  string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-2 * time.Hour), "2h ago"},
		{now.Add(-25 * time.Hour), now.Add(-25 * time.Hour).Format("Jan 2 15:04")},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatTimestamp(tt.timestamp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatSize(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatProgress(t *testing.T) {
	tests := []struct {
		current  float64
		total    float64
		expected string
	}{
		{50, 100, "50/100 (50.0%)"},
		{75, 100, "75/100 (75.0%)"},
		{0, 100, "0/100 (0.0%)"},
		{100, 100, "100/100 (100.0%)"},
		{50, 0, "0/0 (0%)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatProgress(tt.current, tt.total)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatETA(t *testing.T) {
	now := time.Now()

	tests := []struct {
		eta      time.Time
		expected string
	}{
		{time.Time{}, "Unknown"},
		{now.Add(-1 * time.Hour), "Completed"},
		{now.Add(30 * time.Minute), "in 30m"},
		{now.Add(2 * time.Hour), "in 2h 0m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatETA(tt.eta)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumberFormatter(t *testing.T) {
	formatter := NewNumberFormatter()

	tests := []struct {
		value    float64
		expected string
	}{
		{0, "0.00"},
		{1.5, "1.50"},
		{123.456, "123.46"},
		{-45.67, "-45.67"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatter.Format(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumberFormatter_FormatInt(t *testing.T) {
	formatter := NewNumberFormatter()

	tests := []struct {
		value    int64
		expected string
	}{
		{0, "0"},
		{42, "42"},
		{-123, "-123"},
		{999999, "999999"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatter.FormatInt(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPadString(t *testing.T) {
	tests := []struct {
		input    string
		length   int
		padLeft  bool
		expected string
	}{
		{"Hello", 10, false, "Hello     "},
		{"Hello", 10, true, "     Hello"},
		{"Hello", 5, false, "Hello"},
		{"Hello", 3, false, "Hello"}, // 输入长度大于目标长度
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := padString(tt.input, tt.length, tt.padLeft)
			assert.Equal(t, tt.expected, result)
		})
	}
}
