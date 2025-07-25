package components

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// formatNumber 格式化大数字
func formatNumber(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}


// formatCurrency 格式化货币
func formatCurrency(amount float64) string {
	if amount >= 1000 {
		return fmt.Sprintf("$%.1fK", amount/1000)
	}
	return fmt.Sprintf("$%.2f", amount)
}

// formatDuration 格式化时长
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

// formatPercentage 格式化百分比
func formatPercentage(value float64, precision int) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "0.0%"
	}

	format := fmt.Sprintf("%%.%df%%%%", precision)
	return fmt.Sprintf(format, value)
}

// formatRate 格式化速率
func formatRate(rate float64, unit string) string {
	if rate >= 1000 {
		return fmt.Sprintf("%.1fK %s", rate/1000, unit)
	} else if rate >= 100 {
		return fmt.Sprintf("%.0f %s", rate, unit)
	} else if rate >= 10 {
		return fmt.Sprintf("%.1f %s", rate, unit)
	}
	return fmt.Sprintf("%.2f %s", rate, unit)
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// padString 填充字符串到指定长度
func padString(s string, length int, padLeft bool) string {
	if len(s) >= length {
		return s
	}

	padding := strings.Repeat(" ", length-len(s))
	if padLeft {
		return padding + s
	}
	return s + padding
}

// formatBurnRate 格式化燃烧率
func formatBurnRate(tokensPerMinute float64) string {
	if tokensPerMinute == 0 {
		return "0 tok/min"
	}

	if tokensPerMinute >= 1000 {
		return fmt.Sprintf("%.1fK tok/min", tokensPerMinute/1000)
	}

	return fmt.Sprintf("%.1f tok/min", tokensPerMinute)
}

// formatTokenCount 格式化 token 数量
func formatTokenCount(tokens int) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.2fM", float64(tokens)/1000000)
	} else if tokens >= 1000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}

// formatConfidence 格式化置信度
func formatConfidence(confidence float64) string {
	if confidence >= 90 {
		return fmt.Sprintf("%.0f%% (High)", confidence)
	} else if confidence >= 70 {
		return fmt.Sprintf("%.0f%% (Med)", confidence)
	} else if confidence >= 50 {
		return fmt.Sprintf("%.0f%% (Low)", confidence)
	}
	return fmt.Sprintf("%.0f%% (Poor)", confidence)
}

// formatChange 格式化变化值
func formatChange(current, projected float64) string {
	if projected == current || current == 0 {
		return "—"
	}

	change := projected - current
	percentage := (change / current) * 100

	if math.Abs(percentage) < 0.1 {
		return "~0%"
	}

	arrow := "↑"
	if change < 0 {
		arrow = "↓"
		percentage = -percentage
	}

	return fmt.Sprintf("%s %.1f%%", arrow, percentage)
}

// formatModelName 格式化模型名称
func formatModelName(model string, maxLength int) string {
	// 简化常见模型名称
	simplified := map[string]string{
		"claude-3-5-sonnet-20241022": "Claude 3.5 Sonnet",
		"claude-3-5-haiku-20241022":  "Claude 3.5 Haiku",
		"claude-3-opus-20240229":     "Claude 3 Opus",
		"claude-3-sonnet-20240229":   "Claude 3 Sonnet",
		"claude-3-haiku-20240307":    "Claude 3 Haiku",
		"gpt-4-turbo":                "GPT-4 Turbo",
		"gpt-4":                      "GPT-4",
		"gpt-3.5-turbo":              "GPT-3.5 Turbo",
	}

	if simplified, exists := simplified[model]; exists {
		model = simplified
	}

	return truncateString(model, maxLength)
}

// formatTimestamp 格式化时间戳
func formatTimestamp(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else {
		return t.Format("Jan 2 15:04")
	}
}

// formatSize 格式化文件大小
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// formatProgress 格式化进度信息
func formatProgress(current, total float64) string {
	if total == 0 {
		return "0/0 (0%)"
	}

	percentage := (current / total) * 100
	return fmt.Sprintf("%.0f/%.0f (%.1f%%)", current, total, percentage)
}

// formatETA 格式化预计完成时间
func formatETA(eta time.Time) string {
	if eta.IsZero() {
		return "Unknown"
	}

	now := time.Now()
	if eta.Before(now) {
		return "Completed"
	}

	remaining := eta.Sub(now)
	return fmt.Sprintf("in %s", formatDuration(remaining))
}

// NumberFormatter 数字格式化器
type NumberFormatter struct {
	ThousandsSep string
	DecimalSep   string
	Precision    int
}

// NewNumberFormatter 创建数字格式化器
func NewNumberFormatter() *NumberFormatter {
	return &NumberFormatter{
		ThousandsSep: ",",
		DecimalSep:   ".",
		Precision:    2,
	}
}

// Format 格式化数字
func (nf *NumberFormatter) Format(value float64) string {
	// 简化实现，不添加千位分隔符
	format := fmt.Sprintf("%%.%df", nf.Precision)
	return fmt.Sprintf(format, value)
}

// FormatInt 格式化整数
func (nf *NumberFormatter) FormatInt(value int64) string {
	// 简化实现
	return fmt.Sprintf("%d", value)
}
