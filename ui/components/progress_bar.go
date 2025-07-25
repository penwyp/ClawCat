package components

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ProgressBar 进度条基础组件
type ProgressBar struct {
	Label       string         // 标签文本
	Current     float64        // 当前值
	Max         float64        // 最大值
	Percentage  float64        // 百分比（0-100）
	Width       int            // 进度条宽度
	ShowValue   bool           // 是否显示数值
	ShowPercent bool           // 是否显示百分比
	Color       lipgloss.Color // 进度条颜色
	Style       ProgressBarStyle // 样式配置
}

// ProgressBarStyle 进度条样式
type ProgressBarStyle struct {
	BarChar         string // 进度字符
	EmptyChar       string // 空白字符
	BarBracketStart string // 开始括号
	BarBracketEnd   string // 结束括号
	LabelStyle      lipgloss.Style
	ValueStyle      lipgloss.Style
	PercentStyle    lipgloss.Style
}

// ColorThreshold 颜色阈值配置
type ColorThreshold struct {
	Value float64
	Color lipgloss.Color
}

// ProgressColorScheme 进度条颜色方案
type ProgressColorScheme struct {
	Thresholds []ColorThreshold
	Default    lipgloss.Color
}

// 默认颜色方案
var DefaultColorScheme = ProgressColorScheme{
	Thresholds: []ColorThreshold{
		{Value: 0, Color: "#00ff00"},    // 绿色 0-50%
		{Value: 50, Color: "#ffff00"},   // 黄色 50-75%
		{Value: 75, Color: "#ff8800"},   // 橙色 75-90%
		{Value: 90, Color: "#ff0000"},   // 红色 90-100%
	},
	Default: "#00ff00",
}

// GetProgressColor 根据百分比获取颜色
func (pcs ProgressColorScheme) GetProgressColor(percentage float64) lipgloss.Color {
	for i := len(pcs.Thresholds) - 1; i >= 0; i-- {
		if percentage >= pcs.Thresholds[i].Value {
			return pcs.Thresholds[i].Color
		}
	}
	return pcs.Default
}

// NewProgressBar 创建新的进度条
func NewProgressBar(label string, current, max float64) *ProgressBar {
	percentage := 0.0
	if max > 0 {
		percentage = math.Min(100, (current/max)*100)
	}

	return &ProgressBar{
		Label:       label,
		Current:     current,
		Max:         max,
		Percentage:  percentage,
		Width:       40,
		ShowValue:   true,
		ShowPercent: true,
		Style:       DefaultProgressBarStyle(),
	}
}

// DefaultProgressBarStyle 默认进度条样式
func DefaultProgressBarStyle() ProgressBarStyle {
	return ProgressBarStyle{
		BarChar:         "█",
		EmptyChar:       "░",
		BarBracketStart: "[",
		BarBracketEnd:   "]",
		LabelStyle:      lipgloss.NewStyle().Bold(true),
		ValueStyle:      lipgloss.NewStyle(),
		PercentStyle:    lipgloss.NewStyle().Faint(true),
	}
}

// Render 渲染进度条
func (pb *ProgressBar) Render() string {
	// 计算填充长度
	fillLength := int(float64(pb.Width) * pb.Percentage / 100)
	if fillLength > pb.Width {
		fillLength = pb.Width
	}
	if fillLength < 0 {
		fillLength = 0
	}
	emptyLength := pb.Width - fillLength

	// 构建进度条
	barContent := strings.Repeat(pb.Style.BarChar, fillLength) +
		strings.Repeat(pb.Style.EmptyChar, emptyLength)

	bar := fmt.Sprintf("%s%s%s",
		pb.Style.BarBracketStart,
		barContent,
		pb.Style.BarBracketEnd,
	)

	// 应用颜色
	if pb.Color != "" {
		barStyle := lipgloss.NewStyle().Foreground(pb.Color)
		bar = barStyle.Render(bar)
	}

	// 构建完整输出
	parts := []string{
		pb.Style.LabelStyle.Render(pb.Label),
		bar,
	}

	// 添加数值显示
	if pb.ShowValue {
		value := formatValue(pb.Current, pb.Max)
		parts = append(parts, pb.Style.ValueStyle.Render(value))
	}

	// 添加百分比显示
	if pb.ShowPercent {
		percent := fmt.Sprintf("%.1f%%", pb.Percentage)
		parts = append(parts, pb.Style.PercentStyle.Render(percent))
	}

	return strings.Join(parts, " ")
}

// SetWidth 设置进度条宽度
func (pb *ProgressBar) SetWidth(width int) {
	if width < 10 {
		width = 10
	}
	pb.Width = width
}

// Update 更新进度条数值
func (pb *ProgressBar) Update(current float64) {
	pb.Current = current
	if pb.Max > 0 {
		pb.Percentage = math.Min(100, (current/pb.Max)*100)
	}
}

// SetColor 设置进度条颜色
func (pb *ProgressBar) SetColor(color lipgloss.Color) {
	pb.Color = color
}

// SetMax 设置最大值
func (pb *ProgressBar) SetMax(max float64) {
	pb.Max = max
	if pb.Max > 0 {
		pb.Percentage = math.Min(100, (pb.Current/pb.Max)*100)
	}
}

// formatValue 格式化数值显示
func formatValue(current, max float64) string {
	// Token 显示
	if max > 1000000 {
		return fmt.Sprintf("%.1fM/%.1fM", current/1000000, max/1000000)
	} else if max > 1000 {
		return fmt.Sprintf("%.1fK/%.1fK", current/1000, max/1000)
	}

	// 成本显示
	if max < 1000 {
		return fmt.Sprintf("$%.2f/$%.2f", current, max)
	}

	return fmt.Sprintf("%.0f/%.0f", current, max)
}

// IsOverLimit 检查是否超出限制
func (pb *ProgressBar) IsOverLimit() bool {
	if pb.Max == 0 {
		return false
	}
	return pb.Current > pb.Max
}

// GetStatus 获取进度条状态
func (pb *ProgressBar) GetStatus() string {
	if pb.Percentage >= 90 {
		return "critical"
	} else if pb.Percentage >= 75 {
		return "warning"
	} else if pb.Percentage >= 50 {
		return "moderate"
	}
	return "normal"
}