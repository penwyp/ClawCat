package components

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/penwyp/claudecat/calculations"
)

// StatisticsTable 统计表格组件
type StatisticsTable struct {
	metrics *calculations.RealtimeMetrics
	stats   TableStatistics
	width   int
	height  int
	styles  ProgressStyles
	layout  TableLayout
}

// TableStatistics 表格统计数据
type TableStatistics struct {
	// 当前值
	CurrentTokens   int
	CurrentCost     float64
	CurrentMessages int
	CurrentDuration time.Duration

	// 预测值
	ProjectedTokens   int
	ProjectedCost     float64
	ProjectedMessages int
	ConfidenceLevel   float64

	// 速率指标
	TokensPerMinute float64
	TokensPerHour   float64
	CostPerMinute   float64
	CostPerHour     float64
	MessagesPerHour float64

	// 模型分布
	ModelDistribution []ModelUsage
}

// ModelUsage 模型使用情况
type ModelUsage struct {
	Model      string
	TokenCount int
	Percentage float64
	Cost       float64
	Color      lipgloss.Color
}

// TableLayout 表格布局配置
type TableLayout struct {
	ShowHeaders  bool
	ShowBorders  bool
	ColumnWidths []int
	CompactMode  bool
}

// NewStatisticsTable 创建统计表格
func NewStatisticsTable(width int) *StatisticsTable {
	return &StatisticsTable{
		width:  width,
		styles: DefaultProgressStyles(),
		layout: DefaultTableLayout(),
	}
}

// DefaultTableLayout 默认表格布局
func DefaultTableLayout() TableLayout {
	return TableLayout{
		ShowHeaders:  true,
		ShowBorders:  true,
		CompactMode:  false,
		ColumnWidths: []int{20, 15, 15, 15}, // 自动调整
	}
}

// Update 更新表格数据
func (st *StatisticsTable) Update(metrics *calculations.RealtimeMetrics) {
	st.metrics = metrics
	st.stats = st.calculateStatistics(metrics)
	st.adjustLayout()
}

// calculateStatistics 计算统计数据
func (st *StatisticsTable) calculateStatistics(metrics *calculations.RealtimeMetrics) TableStatistics {
	stats := TableStatistics{
		// 当前值
		CurrentTokens:   metrics.CurrentTokens,
		CurrentCost:     metrics.CurrentCost,
		CurrentDuration: time.Since(metrics.SessionStart),

		// 预测值
		ProjectedTokens: metrics.ProjectedTokens,
		ProjectedCost:   metrics.ProjectedCost,
		ConfidenceLevel: metrics.ConfidenceLevel,

		// 速率指标
		TokensPerMinute: metrics.TokensPerMinute,
		TokensPerHour:   metrics.TokensPerHour,
		CostPerMinute:   metrics.CostPerMinute,
		CostPerHour:     metrics.CostPerHour,
	}

	// 计算模型分布
	stats.ModelDistribution = st.calculateModelDistribution(metrics)

	return stats
}

// Render 渲染统计表格
func (st *StatisticsTable) Render() string {
	if st.width == 0 {
		return "Loading statistics..."
	}

	// 构建表格部分
	mainTable := st.renderMainStatistics()
	rateTable := st.renderRateMetrics()
	modelTable := st.renderModelDistribution()

	// 组合所有表格
	tables := []string{mainTable}

	if st.width > 80 { // 宽屏显示更多信息
		tables = append(tables, rateTable)
	}

	if len(st.stats.ModelDistribution) > 0 {
		tables = append(tables, modelTable)
	}

	// 添加标题
	title := st.styles.SectionTitle.Render("📈 Statistics Overview")

	content := strings.Join(append([]string{title}, tables...), "\n\n")

	return st.styles.Box.
		Width(st.width).
		Render(content)
}

// renderMainStatistics 渲染主要统计表格
func (st *StatisticsTable) renderMainStatistics() string {
	table := NewResponsiveTable(st.width - 4)

	// 定义列
	columns := []Column{
		{Key: "metric", Title: "Metric", MinWidth: 15, Priority: 1},
		{Key: "current", Title: "Current", MinWidth: 12, Priority: 1},
		{Key: "projected", Title: "Projected", MinWidth: 12, Priority: 2},
		{Key: "change", Title: "Change", MinWidth: 10, Priority: 3},
	}

	table.SetColumns(columns)

	// 添加数据行
	rows := [][]interface{}{
		{"Tokens",
			formatNumber(st.stats.CurrentTokens),
			formatNumber(st.stats.ProjectedTokens),
			st.formatChange(st.stats.CurrentTokens, st.stats.ProjectedTokens),
		},
		{"Cost",
			fmt.Sprintf("$%.2f", st.stats.CurrentCost),
			fmt.Sprintf("$%.2f", st.stats.ProjectedCost),
			st.formatCostChange(st.stats.CurrentCost, st.stats.ProjectedCost),
		},
		{"Duration",
			formatDuration(st.stats.CurrentDuration),
			"5h 0m",
			st.formatTimeRemaining(),
		},
	}

	for _, row := range rows {
		table.AddRow(row)
	}

	return table.Render()
}

// renderRateMetrics 渲染速率指标表格
func (st *StatisticsTable) renderRateMetrics() string {
	// 创建简化的速率表格
	builder := strings.Builder{}

	subtitleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981"))
	builder.WriteString(subtitleStyle.Render("⚡ Burn Rate Metrics") + "\n")

	// 使用两列布局
	leftCol := []string{
		fmt.Sprintf("Tokens/min: %.1f", st.stats.TokensPerMinute),
		fmt.Sprintf("Tokens/hr:  %.0f", st.stats.TokensPerHour),
	}

	rightCol := []string{
		fmt.Sprintf("Cost/min: $%.3f", st.stats.CostPerMinute),
		fmt.Sprintf("Cost/hr:  $%.2f", st.stats.CostPerHour),
	}

	normalStyle := lipgloss.NewStyle()
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))

	// 应用样式
	for i := range leftCol {
		leftStyle := normalStyle
		rightStyle := normalStyle

		// 如果燃烧率过高，使用警告颜色
		if st.stats.TokensPerMinute > 200 {
			leftStyle = warningStyle
		}
		if st.stats.CostPerHour > 5.0 {
			rightStyle = warningStyle
		}

		left := leftStyle.Render(leftCol[i])
		right := rightStyle.Render(rightCol[i])

		builder.WriteString(fmt.Sprintf("%-30s %s\n", left, right))
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#6B7280")).
		Padding(0, 1)

	return boxStyle.Render(builder.String())
}

// renderModelDistribution 渲染模型分布
func (st *StatisticsTable) renderModelDistribution() string {
	if len(st.stats.ModelDistribution) == 0 {
		return ""
	}

	builder := strings.Builder{}
	subtitleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5CF6"))
	builder.WriteString(subtitleStyle.Render("🤖 Model Distribution") + "\n")

	// 计算条形图宽度
	maxBarWidth := st.width - 40
	if maxBarWidth < 20 {
		maxBarWidth = 20
	}

	for _, model := range st.stats.ModelDistribution {
		// 模型名称和百分比
		label := fmt.Sprintf("%-20s %5.1f%%",
			truncateString(model.Model, 20),
			model.Percentage,
		)

		// 条形图
		barWidth := int(float64(maxBarWidth) * model.Percentage / 100)
		if barWidth > maxBarWidth {
			barWidth = maxBarWidth
		}
		bar := strings.Repeat("█", barWidth) + strings.Repeat("░", maxBarWidth-barWidth)

		// 应用颜色
		coloredBar := lipgloss.NewStyle().
			Foreground(model.Color).
			Render(bar)

		// 添加 token 数量
		faintStyle := lipgloss.NewStyle().Faint(true)
		stats := fmt.Sprintf(" %s tokens", formatNumber(model.TokenCount))

		builder.WriteString(fmt.Sprintf("%s\n%s%s\n\n",
			label, coloredBar,
			faintStyle.Render(stats),
		))
	}

	return builder.String()
}

// adjustLayout 调整布局
func (st *StatisticsTable) adjustLayout() {
	if st.width < 80 {
		st.layout.CompactMode = true
	}
}

// formatChange 格式化变化值
func (st *StatisticsTable) formatChange(current, projected int) string {
	if projected == current || current == 0 {
		return "—"
	}

	change := projected - current
	percentage := float64(change) / float64(current) * 100

	arrow := "↑"
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")) // 绿色
	if change < 0 {
		arrow = "↓"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")) // 红色
	}

	return style.Render(fmt.Sprintf("%s %.0f%%", arrow, percentage))
}

// formatCostChange 格式化成本变化
func (st *StatisticsTable) formatCostChange(current, projected float64) string {
	if projected == current || current == 0 {
		return "—"
	}

	change := projected - current

	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")) // 橙色
	if change > 5.0 {                                                  // 超过 $5 增长
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")) // 红色
	}

	return style.Render(fmt.Sprintf("+$%.2f", change))
}

// formatTimeRemaining 格式化剩余时间
func (st *StatisticsTable) formatTimeRemaining() string {
	remaining := 5*time.Hour - st.stats.CurrentDuration

	if remaining <= 0 {
		return st.styles.Error.Render("Expired")
	}

	style := lipgloss.NewStyle()
	if remaining < 30*time.Minute {
		style = st.styles.Warning
	} else if remaining < 10*time.Minute {
		style = st.styles.Error
	}

	return style.Render(formatDuration(remaining))
}

// calculateModelDistribution 计算模型分布
func (st *StatisticsTable) calculateModelDistribution(metrics *calculations.RealtimeMetrics) []ModelUsage {
	models := []ModelUsage{}
	colors := []lipgloss.Color{
		"#FF6B6B", // 红色
		"#4ECDC4", // 青色
		"#45B7D1", // 蓝色
		"#FFA07A", // 浅橙
		"#98D8C8", // 薄荷绿
		"#DDA0DD", // 梅红
		"#F0E68C", // 卡其色
		"#87CEEB", // 天蓝色
	}

	totalTokens := metrics.CurrentTokens
	colorIndex := 0

	for model, stats := range metrics.ModelDistribution {
		percentage := 0.0
		if totalTokens > 0 {
			percentage = float64(stats.TokenCount) / float64(totalTokens) * 100
		}

		usage := ModelUsage{
			Model:      model,
			TokenCount: stats.TokenCount,
			Percentage: percentage,
			Cost:       stats.Cost,
			Color:      colors[colorIndex%len(colors)],
		}

		models = append(models, usage)
		colorIndex++
	}

	// 按使用率排序
	sort.Slice(models, func(i, j int) bool {
		return models[i].Percentage > models[j].Percentage
	})

	return models
}

// SetWidth 设置表格宽度
func (st *StatisticsTable) SetWidth(width int) {
	st.width = width
}

// SetHeight 设置表格高度
func (st *StatisticsTable) SetHeight(height int) {
	st.height = height
}

// GetSummary 获取统计摘要
func (st *StatisticsTable) GetSummary() string {
	if st.metrics == nil {
		return "No statistics available"
	}

	return fmt.Sprintf(
		"Current: %s tokens, $%.2f | Projected: %s tokens, $%.2f",
		formatNumber(st.stats.CurrentTokens),
		st.stats.CurrentCost,
		formatNumber(st.stats.ProjectedTokens),
		st.stats.ProjectedCost,
	)
}
