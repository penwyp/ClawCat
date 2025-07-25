package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/penwyp/ClawCat/calculations"
	"github.com/penwyp/ClawCat/ui/components"
)

// AggregationView 聚合视图
type AggregationView struct {
	// 视图状态
	viewType  calculations.AggregationView
	dateRange DateRange
	data      []calculations.AggregatedData
	loading   bool
	err       error

	// UI 组件
	table   *components.AggregationTable
	chart   *components.UsageChart
	summary *SummaryCards

	// 交互状态
	selected    int
	showChart   bool
	showDetails bool

	// 布局
	width  int
	height int
	styles Styles

	// 键绑定
	keys AggregationKeyMap
}

// DateRange 日期范围
type DateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// SummaryCards 摘要卡片
type SummaryCards struct {
	totalTokens    int
	totalCost      float64
	avgDailyTokens int
	peakDay        string
	confidence     float64
}

// AggregationKeyMap 聚合视图键绑定
type AggregationKeyMap struct {
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	ToggleChart key.Binding
	ToggleView  key.Binding
	NextPeriod  key.Binding
	PrevPeriod  key.Binding
	Export      key.Binding
	Refresh     key.Binding
	Help        key.Binding
	Quit        key.Binding
}

// DefaultAggregationKeys 默认键绑定
func DefaultAggregationKeys() AggregationKeyMap {
	return AggregationKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "previous"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "next"),
		),
		ToggleChart: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "toggle chart"),
		),
		ToggleView: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "cycle view"),
		),
		NextPeriod: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next period"),
		),
		PrevPeriod: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prev period"),
		),
		Export: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "export"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc"),
			key.WithHelp("q", "quit"),
		),
	}
}

// NewAggregationView 创建聚合视图
func NewAggregationView() *AggregationView {
	return &AggregationView{
		viewType: calculations.DailyView,
		dateRange: DateRange{
			Start: time.Now().AddDate(0, 0, -30), // 默认最近30天
			End:   time.Now(),
		},
		table:       components.NewAggregationTable(),
		chart:       components.NewUsageChart(),
		summary:     &SummaryCards{},
		showChart:   true,
		showDetails: false,
		styles:      NewStyles(DefaultTheme()),
		keys:        DefaultAggregationKeys(),
	}
}

// Init 初始化视图
func (av *AggregationView) Init() tea.Cmd {
	return av.refreshData()
}

// Update 更新视图
func (av *AggregationView) Update(msg tea.Msg) (*AggregationView, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		av.width = msg.Width
		av.height = msg.Height
		av.updateLayout()

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, av.keys.Quit):
			return av, tea.Quit

		case key.Matches(msg, av.keys.ToggleView):
			av.cycleViewType()
			return av, av.refreshData()

		case key.Matches(msg, av.keys.ToggleChart):
			av.showChart = !av.showChart

		case key.Matches(msg, av.keys.NextPeriod):
			av.nextPeriod()
			return av, av.refreshData()

		case key.Matches(msg, av.keys.PrevPeriod):
			av.previousPeriod()
			return av, av.refreshData()

		case key.Matches(msg, av.keys.Refresh):
			return av, av.refreshData()

		case key.Matches(msg, av.keys.Export):
			return av, av.exportData()

		case key.Matches(msg, av.keys.Up):
			if av.selected > 0 {
				av.selected--
			}

		case key.Matches(msg, av.keys.Down):
			if av.selected < len(av.data)-1 {
				av.selected++
			}

		case key.Matches(msg, av.keys.Left):
			av.table.PreviousPage()

		case key.Matches(msg, av.keys.Right):
			av.table.NextPage()
		}

	case AggregationDataMsg:
		av.data = msg.Data
		av.loading = false
		av.err = msg.Error
		av.updateComponents()

	case ExportCompleteMsg:
		// 处理导出完成消息
		if msg.Error != nil {
			av.err = msg.Error
		}
	}

	return av, cmd
}

// View 渲染视图
func (av *AggregationView) View() string {
	if av.width == 0 || av.height == 0 {
		return av.styles.Faint().Render("Loading aggregation view...")
	}

	if av.loading {
		return av.renderLoading()
	}

	if av.err != nil {
		return av.renderError()
	}

	// 标题栏
	header := av.renderHeader()

	// 视图选择器
	viewSelector := av.renderViewSelector()

	// 主要内容区域
	content := av.renderContent()

	// 统计摘要
	summary := av.renderSummary()

	// 帮助信息
	help := av.renderHelp()

	// 组合所有部分
	sections := []string{
		header,
		viewSelector,
		content,
		summary,
		help,
	}

	return av.styles.Content.
		Width(av.width).
		Height(av.height).
		Render(strings.Join(sections, "\n\n"))
}

// renderHeader 渲染标题
func (av *AggregationView) renderHeader() string {
	title := av.styles.Title.Render("📊 Usage History")

	dateRange := fmt.Sprintf("%s - %s",
		av.dateRange.Start.Format("Jan 2, 2006"),
		av.dateRange.End.Format("Jan 2, 2006"),
	)

	subtitle := av.styles.Subtitle.Render(dateRange)

	return lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
}

// renderViewSelector 渲染视图选择器
func (av *AggregationView) renderViewSelector() string {
	views := []struct {
		Type  calculations.AggregationView
		Label string
	}{
		{calculations.DailyView, "Daily"},
		{calculations.WeeklyView, "Weekly"},
		{calculations.MonthlyView, "Monthly"},
	}

	buttons := make([]string, len(views))
	for i, v := range views {
		style := av.styles.Button
		if v.Type == av.viewType {
			style = av.styles.ButtonActive()
		}
		buttons[i] = style.Render(v.Label)
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, buttons...)
}

// renderContent 渲染主要内容
func (av *AggregationView) renderContent() string {
	if len(av.data) == 0 {
		return av.styles.Faint().Render("No data available for the selected period")
	}

	// 根据屏幕宽度决定布局
	if av.width > 120 {
		// 宽屏：并排显示表格和图表
		table := av.table.Render(av.width/2 - 2)

		var chart string
		if av.showChart {
			chart = av.chart.Render(av.width/2 - 2)
		} else {
			chart = av.styles.Faint().Render("Chart hidden (press 'c' to show)")
		}

		return lipgloss.JoinHorizontal(lipgloss.Top, table, "  ", chart)
	} else {
		// 窄屏：垂直排列
		table := av.table.Render(av.width - 4)

		if av.showChart {
			chart := av.chart.Render(av.width - 4)
			return lipgloss.JoinVertical(lipgloss.Left, table, chart)
		}

		return table
	}
}

// renderSummary 渲染统计摘要
func (av *AggregationView) renderSummary() string {
	if len(av.data) == 0 {
		return ""
	}

	summary := av.calculateSummary()

	cards := []string{
		av.renderSummaryCard("Total Tokens", formatNumber(summary.totalTokens), av.styles.Info),
		av.renderSummaryCard("Total Cost", fmt.Sprintf("$%.2f", summary.totalCost), av.styles.Warning),
		av.renderSummaryCard("Avg Daily", formatNumber(summary.avgDailyTokens), av.styles.Success),
		av.renderSummaryCard("Peak Day", summary.peakDay, av.styles.Normal),
	}

	return av.styles.Box().Render(
		lipgloss.JoinHorizontal(lipgloss.Center, cards...),
	)
}

// renderSummaryCard 渲染摘要卡片
func (av *AggregationView) renderSummaryCard(title, value string, style lipgloss.Style) string {
	card := fmt.Sprintf("%s\n%s",
		style.Bold(true).Render(title),
		style.Faint(true).Render(value),
	)

	return av.styles.Card().
		Width(av.width/4 - 2).
		Render(card)
}

// renderHelp 渲染帮助信息
func (av *AggregationView) renderHelp() string {
	helpItems := []string{
		"v: cycle view",
		"c: toggle chart",
		"n/p: next/prev period",
		"r: refresh",
		"e: export",
		"q: quit",
	}

	return av.styles.Help().Render(strings.Join(helpItems, " • "))
}

// renderLoading 渲染加载状态
func (av *AggregationView) renderLoading() string {
	spinner := "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏" // 简单的旋转动画
	frame := int(time.Now().UnixNano()/100000000) % len(spinner)

	loading := fmt.Sprintf("%c Loading aggregation data...", rune(spinner[frame]))

	return av.styles.Info.
		Width(av.width).
		Height(av.height).
		AlignHorizontal(lipgloss.Center).
		AlignVertical(lipgloss.Center).
		Render(loading)
}

// renderError 渲染错误状态
func (av *AggregationView) renderError() string {
	errorMsg := fmt.Sprintf("❌ Error loading data: %s\n\nPress 'r' to retry", av.err.Error())

	return av.styles.Error.
		Width(av.width).
		Height(av.height).
		AlignHorizontal(lipgloss.Center).
		AlignVertical(lipgloss.Center).
		Render(errorMsg)
}

// updateLayout 更新布局
func (av *AggregationView) updateLayout() {
	if av.table != nil {
		av.table.SetWidth(av.width)
	}
	if av.chart != nil {
		av.chart.SetWidth(av.width)
	}
}

// updateComponents 更新组件数据
func (av *AggregationView) updateComponents() {
	if av.table != nil {
		av.table.Update(av.data)
	}
	if av.chart != nil {
		av.chart.Update(av.data)
	}
}

// calculateSummary 计算摘要统计
func (av *AggregationView) calculateSummary() *SummaryCards {
	summary := &SummaryCards{}

	if len(av.data) == 0 {
		return summary
	}

	var peakTokens int
	var peakDay string

	for _, data := range av.data {
		summary.totalTokens += data.Tokens.Total
		summary.totalCost += data.Cost.Total

		if data.Tokens.Total > peakTokens {
			peakTokens = data.Tokens.Total
			peakDay = data.Period.Label
		}
	}

	if len(av.data) > 0 {
		summary.avgDailyTokens = summary.totalTokens / len(av.data)
	}

	summary.peakDay = peakDay
	summary.confidence = av.calculateConfidence()

	return summary
}

// calculateConfidence 计算数据置信度
func (av *AggregationView) calculateConfidence() float64 {
	if len(av.data) == 0 {
		return 0
	}

	// 基于数据点数量和时间跨度计算置信度
	dataPoints := len(av.data)
	timeSpan := av.dateRange.End.Sub(av.dateRange.Start).Hours() / 24

	// 简单的置信度计算：数据点密度
	density := float64(dataPoints) / timeSpan
	confidence := density * 20 // 调整系数

	if confidence > 100 {
		confidence = 100
	}

	return confidence
}

// cycleViewType 切换视图类型
func (av *AggregationView) cycleViewType() {
	switch av.viewType {
	case calculations.DailyView:
		av.viewType = calculations.WeeklyView
	case calculations.WeeklyView:
		av.viewType = calculations.MonthlyView
	case calculations.MonthlyView:
		av.viewType = calculations.DailyView
	}
}

// nextPeriod 下一个时间段
func (av *AggregationView) nextPeriod() {
	duration := av.dateRange.End.Sub(av.dateRange.Start)
	av.dateRange.Start = av.dateRange.End
	av.dateRange.End = av.dateRange.Start.Add(duration)
}

// previousPeriod 上一个时间段
func (av *AggregationView) previousPeriod() {
	duration := av.dateRange.End.Sub(av.dateRange.Start)
	av.dateRange.End = av.dateRange.Start
	av.dateRange.Start = av.dateRange.End.Add(-duration)
}

// refreshData 刷新数据
func (av *AggregationView) refreshData() tea.Cmd {
	av.loading = true
	av.err = nil

	return func() tea.Msg {
		// 这里应该调用实际的数据加载逻辑
		// 暂时返回模拟数据
		return AggregationDataMsg{
			Data:  generateMockAggregationData(),
			Error: nil,
		}
	}
}

// exportData 导出数据
func (av *AggregationView) exportData() tea.Cmd {
	return func() tea.Msg {
		// 实现数据导出逻辑
		// 这里可以导出为 CSV, JSON 等格式
		return ExportCompleteMsg{
			Format: "csv",
			Path:   "aggregation_export.csv",
			Error:  nil,
		}
	}
}

// formatNumber 格式化数字
func formatNumber(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// 消息类型
type AggregationDataMsg struct {
	Data  []calculations.AggregatedData
	Error error
}

type ExportCompleteMsg struct {
	Format string
	Path   string
	Error  error
}

// generateMockAggregationData 生成模拟数据（用于演示）
func generateMockAggregationData() []calculations.AggregatedData {
	var data []calculations.AggregatedData
	baseTime := time.Now().AddDate(0, 0, -7)

	for i := 0; i < 7; i++ {
		day := baseTime.AddDate(0, 0, i)

		aggregated := calculations.AggregatedData{
			Period: calculations.TimePeriod{
				Start: day,
				End:   day.Add(24 * time.Hour),
				Label: day.Format("Jan 2"),
				Type:  calculations.DailyView,
			},
			Entries: 10 + i*2,
			Tokens: calculations.TokenStats{
				Total:   5000 + i*1000,
				Input:   2000 + i*400,
				Output:  3000 + i*600,
				Average: 500 + float64(i*100),
			},
			Cost: calculations.CostStats{
				Total:   float64(15 + i*3),
				Average: float64(1.5 + float64(i)*0.3),
				Min:     0.5,
				Max:     float64(3 + i),
			},
			Models: map[string]calculations.AggregationModelStats{
				"claude-3-opus": {
					Count:  5 + i,
					Tokens: 3000 + i*600,
					Cost:   float64(10 + i*2),
				},
				"claude-3-sonnet": {
					Count:  3 + i,
					Tokens: 2000 + i*400,
					Cost:   float64(5 + i),
				},
			},
		}

		data = append(data, aggregated)
	}

	return data
}
