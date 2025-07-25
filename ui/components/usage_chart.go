package components

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/penwyp/ClawCat/calculations"
)

// UsageChart 使用量图表组件
type UsageChart struct {
	data      []calculations.AggregatedData
	chartType ChartType
	width     int
	height    int
	styles    ChartStyles
}

// ChartType 图表类型
type ChartType string

const (
	BarChart  ChartType = "bar"
	LineChart ChartType = "line"
	AreaChart ChartType = "area"
)

// ChartStyles 图表样式
type ChartStyles struct {
	Title     lipgloss.Style
	Axis      lipgloss.Style
	Bar       lipgloss.Style
	Line      lipgloss.Style
	Grid      lipgloss.Style
	Legend    lipgloss.Style
	Value     lipgloss.Style
	Highlight lipgloss.Style
}

// NewUsageChart 创建使用量图表
func NewUsageChart() *UsageChart {
	return &UsageChart{
		chartType: BarChart,
		width:     60,
		height:    20,
		styles:    DefaultChartStyles(),
	}
}

// DefaultChartStyles 默认图表样式
func DefaultChartStyles() ChartStyles {
	return ChartStyles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			MarginBottom(1),
		Axis: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
		Bar: lipgloss.NewStyle().
			Background(lipgloss.Color("63")).
			Foreground(lipgloss.Color("15")),
		Line: lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")),
		Grid: lipgloss.NewStyle().
			Foreground(lipgloss.Color("237")),
		Legend: lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			MarginTop(1),
		Value: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Bold(true),
		Highlight: lipgloss.NewStyle().
			Background(lipgloss.Color("99")).
			Foreground(lipgloss.Color("15")),
	}
}

// Update 更新图表数据
func (uc *UsageChart) Update(data []calculations.AggregatedData) {
	uc.data = data
}

// SetWidth 设置图表宽度
func (uc *UsageChart) SetWidth(width int) {
	uc.width = width
}

// SetHeight 设置图表高度
func (uc *UsageChart) SetHeight(height int) {
	uc.height = height
}

// SetChartType 设置图表类型
func (uc *UsageChart) SetChartType(chartType ChartType) {
	uc.chartType = chartType
}

// Render 渲染图表
func (uc *UsageChart) Render(width int) string {
	uc.width = width

	if len(uc.data) == 0 {
		return uc.styles.Title.Render("No data to display")
	}

	switch uc.chartType {
	case BarChart:
		return uc.renderBarChart()
	case LineChart:
		return uc.renderLineChart()
	case AreaChart:
		return uc.renderAreaChart()
	default:
		return uc.renderBarChart()
	}
}

// renderBarChart 渲染柱状图
func (uc *UsageChart) renderBarChart() string {
	if len(uc.data) == 0 {
		return uc.styles.Title.Render("No data available")
	}

	// 计算图表区域尺寸
	chartWidth := uc.width - 10  // 留出空间给标签和值
	chartHeight := uc.height - 4 // 留出空间给标题和坐标轴

	if chartWidth < 10 || chartHeight < 5 {
		return uc.renderCompactChart()
	}

	// 找出最大值用于缩放
	maxValue := 0
	for _, data := range uc.data {
		if data.Tokens.Total > maxValue {
			maxValue = data.Tokens.Total
		}
	}

	if maxValue == 0 {
		return uc.styles.Title.Render("No token usage data")
	}

	var sections []string

	// 标题
	title := uc.styles.Title.Render("📊 Token Usage")
	sections = append(sections, title)

	// 构建图表
	barWidth := chartWidth / len(uc.data)
	if barWidth < 1 {
		barWidth = 1
	}

	// 渲染每一行（从上到下）
	for row := chartHeight - 1; row >= 0; row-- {
		var line strings.Builder

		// Y轴标签
		threshold := float64(row) / float64(chartHeight-1)
		yValue := int(float64(maxValue) * threshold)
		yLabel := fmt.Sprintf("%6s ", uc.formatValue(yValue))
		line.WriteString(uc.styles.Axis.Render(yLabel))

		// 绘制柱状图
		for i, data := range uc.data {
			barHeight := float64(data.Tokens.Total) / float64(maxValue) * float64(chartHeight)

			if float64(row) <= barHeight {
				// 绘制柱子
				bar := strings.Repeat("█", barWidth)
				if i < len(uc.data)-1 {
					bar += " "
				}
				line.WriteString(uc.styles.Bar.Render(bar))
			} else {
				// 空白区域
				spaces := strings.Repeat(" ", barWidth)
				if i < len(uc.data)-1 {
					spaces += " "
				}
				line.WriteString(spaces)
			}
		}

		sections = append(sections, line.String())
	}

	// X轴标签
	var xAxis strings.Builder
	xAxis.WriteString(strings.Repeat(" ", 7)) // 对齐Y轴标签

	for i, data := range uc.data {
		label := uc.truncateLabel(data.Period.Label, barWidth)
		if len(label) < barWidth {
			padding := (barWidth - len(label)) / 2
			label = strings.Repeat(" ", padding) + label + strings.Repeat(" ", barWidth-len(label)-padding)
		}
		xAxis.WriteString(uc.styles.Axis.Render(label))
		if i < len(uc.data)-1 {
			xAxis.WriteString(" ")
		}
	}

	sections = append(sections, xAxis.String())

	// 图例
	legend := uc.renderLegend()
	if legend != "" {
		sections = append(sections, legend)
	}

	return strings.Join(sections, "\n")
}

// renderLineChart 渲染线图
func (uc *UsageChart) renderLineChart() string {
	if len(uc.data) == 0 {
		return uc.styles.Title.Render("No data available")
	}

	// 简化的线图实现
	chartWidth := uc.width - 10
	chartHeight := uc.height - 4

	if chartWidth < 10 || chartHeight < 5 {
		return uc.renderCompactChart()
	}

	maxValue := 0
	for _, data := range uc.data {
		if data.Tokens.Total > maxValue {
			maxValue = data.Tokens.Total
		}
	}

	if maxValue == 0 {
		return uc.styles.Title.Render("No token usage data")
	}

	var sections []string

	// 标题
	title := uc.styles.Title.Render("📈 Token Usage Trend")
	sections = append(sections, title)

	// 创建画布
	canvas := make([][]rune, chartHeight)
	for i := range canvas {
		canvas[i] = make([]rune, chartWidth)
		for j := range canvas[i] {
			canvas[i][j] = ' '
		}
	}

	// 绘制数据点和线条
	points := make([]struct{ x, y int }, len(uc.data))
	for i, data := range uc.data {
		x := int(float64(i) / float64(len(uc.data)-1) * float64(chartWidth-1))
		y := chartHeight - 1 - int(float64(data.Tokens.Total)/float64(maxValue)*float64(chartHeight-1))
		points[i] = struct{ x, y int }{x, y}

		// 标记数据点
		if x < chartWidth && y >= 0 && y < chartHeight {
			canvas[y][x] = '●'
		}
	}

	// 连接数据点
	for i := 1; i < len(points); i++ {
		uc.drawLine(canvas, points[i-1].x, points[i-1].y, points[i].x, points[i].y)
	}

	// 渲染画布
	for row := 0; row < chartHeight; row++ {
		var line strings.Builder

		// Y轴标签
		threshold := float64(chartHeight-1-row) / float64(chartHeight-1)
		yValue := int(float64(maxValue) * threshold)
		yLabel := fmt.Sprintf("%6s ", uc.formatValue(yValue))
		line.WriteString(uc.styles.Axis.Render(yLabel))

		// 画布内容
		lineContent := string(canvas[row])
		line.WriteString(uc.styles.Line.Render(lineContent))

		sections = append(sections, line.String())
	}

	// X轴标签
	if len(uc.data) > 0 {
		var xAxis strings.Builder
		xAxis.WriteString(strings.Repeat(" ", 7))

		firstLabel := uc.truncateLabel(uc.data[0].Period.Label, 8)
		lastLabel := uc.truncateLabel(uc.data[len(uc.data)-1].Period.Label, 8)

		xAxis.WriteString(uc.styles.Axis.Render(firstLabel))
		if chartWidth > 16 {
			padding := chartWidth - len(firstLabel) - len(lastLabel)
			xAxis.WriteString(strings.Repeat(" ", padding))
		}
		xAxis.WriteString(uc.styles.Axis.Render(lastLabel))

		sections = append(sections, xAxis.String())
	}

	// 图例
	legend := uc.renderLegend()
	if legend != "" {
		sections = append(sections, legend)
	}

	return strings.Join(sections, "\n")
}

// renderAreaChart 渲染面积图
func (uc *UsageChart) renderAreaChart() string {
	// 面积图是线图的变体，填充线下方的区域
	if len(uc.data) == 0 {
		return uc.styles.Title.Render("No data available")
	}

	chartWidth := uc.width - 10
	chartHeight := uc.height - 4

	if chartWidth < 10 || chartHeight < 5 {
		return uc.renderCompactChart()
	}

	maxValue := 0
	for _, data := range uc.data {
		if data.Tokens.Total > maxValue {
			maxValue = data.Tokens.Total
		}
	}

	if maxValue == 0 {
		return uc.styles.Title.Render("No token usage data")
	}

	var sections []string

	// 标题
	title := uc.styles.Title.Render("📊 Token Usage Area")
	sections = append(sections, title)

	// 创建画布
	canvas := make([][]rune, chartHeight)
	for i := range canvas {
		canvas[i] = make([]rune, chartWidth)
		for j := range canvas[i] {
			canvas[i][j] = ' '
		}
	}

	// 绘制面积图
	for i, data := range uc.data {
		x := int(float64(i) / float64(len(uc.data)-1) * float64(chartWidth-1))
		barHeight := int(float64(data.Tokens.Total) / float64(maxValue) * float64(chartHeight-1))

		// 填充列
		for y := chartHeight - 1; y > chartHeight-1-barHeight; y-- {
			if x < chartWidth && y >= 0 && y < chartHeight {
				if y == chartHeight-1-barHeight {
					canvas[y][x] = '▀' // 顶部
				} else {
					canvas[y][x] = '█' // 填充
				}
			}
		}
	}

	// 渲染画布
	for row := 0; row < chartHeight; row++ {
		var line strings.Builder

		// Y轴标签
		threshold := float64(chartHeight-1-row) / float64(chartHeight-1)
		yValue := int(float64(maxValue) * threshold)
		yLabel := fmt.Sprintf("%6s ", uc.formatValue(yValue))
		line.WriteString(uc.styles.Axis.Render(yLabel))

		// 画布内容
		lineContent := string(canvas[row])
		line.WriteString(uc.styles.Bar.Render(lineContent))

		sections = append(sections, line.String())
	}

	// X轴标签
	var xAxis strings.Builder
	xAxis.WriteString(strings.Repeat(" ", 7))

	for i, data := range uc.data {
		x := int(float64(i) / float64(len(uc.data)-1) * float64(chartWidth-1))
		if i == 0 || i == len(uc.data)-1 || x%10 == 0 { // 只显示部分标签
			label := uc.truncateLabel(data.Period.Label, 6)
			xAxis.WriteString(uc.styles.Axis.Render(label))
		}
	}

	sections = append(sections, xAxis.String())

	// 图例
	legend := uc.renderLegend()
	if legend != "" {
		sections = append(sections, legend)
	}

	return strings.Join(sections, "\n")
}

// renderCompactChart 渲染紧凑图表
func (uc *UsageChart) renderCompactChart() string {
	if len(uc.data) == 0 {
		return "No data"
	}

	// 简单的迷你图表
	maxValue := 0
	for _, data := range uc.data {
		if data.Tokens.Total > maxValue {
			maxValue = data.Tokens.Total
		}
	}

	if maxValue == 0 {
		return "No usage"
	}

	var chart strings.Builder
	chart.WriteString("📊 ")

	// 使用Unicode块字符创建迷你柱状图
	for _, data := range uc.data {
		height := float64(data.Tokens.Total) / float64(maxValue)

		if height > 0.75 {
			chart.WriteRune('█')
		} else if height > 0.5 {
			chart.WriteRune('▆')
		} else if height > 0.25 {
			chart.WriteRune('▄')
		} else if height > 0 {
			chart.WriteRune('▂')
		} else {
			chart.WriteRune('_')
		}
	}

	chart.WriteString(fmt.Sprintf(" Max: %s", uc.formatValue(maxValue)))

	return uc.styles.Title.Render(chart.String())
}

// renderLegend 渲染图例
func (uc *UsageChart) renderLegend() string {
	if len(uc.data) == 0 {
		return ""
	}

	// 计算统计信息
	total := 0
	max := 0
	min := math.MaxInt32

	for _, data := range uc.data {
		total += data.Tokens.Total
		if data.Tokens.Total > max {
			max = data.Tokens.Total
		}
		if data.Tokens.Total < min {
			min = data.Tokens.Total
		}
	}

	avg := total / len(uc.data)

	legend := fmt.Sprintf("Total: %s | Avg: %s | Max: %s | Min: %s",
		uc.formatValue(total),
		uc.formatValue(avg),
		uc.formatValue(max),
		uc.formatValue(min),
	)

	return uc.styles.Legend.Render(legend)
}

// drawLine 在画布上绘制线条
func (uc *UsageChart) drawLine(canvas [][]rune, x1, y1, x2, y2 int) {
	dx := x2 - x1
	dy := y2 - y1

	if dx == 0 && dy == 0 {
		return
	}

	steps := int(math.Max(math.Abs(float64(dx)), math.Abs(float64(dy))))

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := int(float64(x1) + t*float64(dx))
		y := int(float64(y1) + t*float64(dy))

		if x >= 0 && x < len(canvas[0]) && y >= 0 && y < len(canvas) {
			if canvas[y][x] == ' ' {
				// 根据线条方向选择字符
				if dx > 0 && dy == 0 {
					canvas[y][x] = '─'
				} else if dx == 0 && dy > 0 {
					canvas[y][x] = '│'
				} else if dx > 0 && dy > 0 {
					canvas[y][x] = '╱'
				} else if dx > 0 && dy < 0 {
					canvas[y][x] = '╲'
				} else {
					canvas[y][x] = '·'
				}
			}
		}
	}
}

// formatValue 格式化数值
func (uc *UsageChart) formatValue(value int) string {
	if value >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(value)/1000000)
	} else if value >= 1000 {
		return fmt.Sprintf("%.1fK", float64(value)/1000)
	}
	return fmt.Sprintf("%d", value)
}

// truncateLabel 截断标签
func (uc *UsageChart) truncateLabel(label string, maxLen int) string {
	if len(label) <= maxLen {
		return label
	}

	if maxLen <= 3 {
		return label[:maxLen]
	}

	return label[:maxLen-3] + "..."
}

// GetStats 获取图表统计信息
func (uc *UsageChart) GetStats() ChartStats {
	if len(uc.data) == 0 {
		return ChartStats{}
	}

	stats := ChartStats{
		DataPoints: len(uc.data),
		ChartType:  string(uc.chartType),
	}

	// 计算统计
	total := 0
	max := 0
	min := math.MaxInt32
	costs := 0.0

	for _, data := range uc.data {
		total += data.Tokens.Total
		costs += data.Cost.Total

		if data.Tokens.Total > max {
			max = data.Tokens.Total
		}
		if data.Tokens.Total < min {
			min = data.Tokens.Total
		}
	}

	stats.TotalTokens = total
	stats.TotalCost = costs
	stats.MaxTokens = max
	stats.MinTokens = min
	stats.AvgTokens = total / len(uc.data)

	return stats
}

// ChartStats 图表统计信息
type ChartStats struct {
	DataPoints  int     `json:"data_points"`
	ChartType   string  `json:"chart_type"`
	TotalTokens int     `json:"total_tokens"`
	TotalCost   float64 `json:"total_cost"`
	MaxTokens   int     `json:"max_tokens"`
	MinTokens   int     `json:"min_tokens"`
	AvgTokens   int     `json:"avg_tokens"`
}

// ToggleChartType 切换图表类型
func (uc *UsageChart) ToggleChartType() {
	switch uc.chartType {
	case BarChart:
		uc.chartType = LineChart
	case LineChart:
		uc.chartType = AreaChart
	case AreaChart:
		uc.chartType = BarChart
	default:
		uc.chartType = BarChart
	}
}

// GetChartType 获取当前图表类型
func (uc *UsageChart) GetChartType() ChartType {
	return uc.chartType
}

// RenderMiniChart 渲染迷你图表（用于嵌入其他组件）
func (uc *UsageChart) RenderMiniChart(width int) string {
	if len(uc.data) == 0 || width < 10 {
		return "No data"
	}

	maxValue := 0
	for _, data := range uc.data {
		if data.Tokens.Total > maxValue {
			maxValue = data.Tokens.Total
		}
	}

	if maxValue == 0 {
		return "No usage"
	}

	var chart strings.Builder
	availableWidth := width - 2 // 留出边框空间

	// 计算每个数据点的宽度
	pointWidth := float64(availableWidth) / float64(len(uc.data))

	if pointWidth >= 1 {
		// 每个数据点至少占用1个字符
		for _, data := range uc.data {
			height := float64(data.Tokens.Total) / float64(maxValue)

			if height > 0.75 {
				chart.WriteRune('█')
			} else if height > 0.5 {
				chart.WriteRune('▆')
			} else if height > 0.25 {
				chart.WriteRune('▄')
			} else if height > 0 {
				chart.WriteRune('▂')
			} else {
				chart.WriteRune('_')
			}
		}
	} else {
		// 数据点太多，需要采样
		sampleSize := availableWidth
		step := float64(len(uc.data)) / float64(sampleSize)

		for i := 0; i < sampleSize; i++ {
			dataIndex := int(float64(i) * step)
			if dataIndex >= len(uc.data) {
				dataIndex = len(uc.data) - 1
			}

			data := uc.data[dataIndex]
			height := float64(data.Tokens.Total) / float64(maxValue)

			if height > 0.75 {
				chart.WriteRune('█')
			} else if height > 0.5 {
				chart.WriteRune('▆')
			} else if height > 0.25 {
				chart.WriteRune('▄')
			} else if height > 0 {
				chart.WriteRune('▂')
			} else {
				chart.WriteRune('_')
			}
		}
	}

	return chart.String()
}
