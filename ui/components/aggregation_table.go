package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/penwyp/ClawCat/calculations"
)

// AggregationTable 聚合数据表格
type AggregationTable struct {
	data       []calculations.AggregatedData
	sortColumn int
	sortAsc    bool
	page       int
	pageSize   int
	width      int
	styles     TableStyles
}

// TableStyles 表格样式
type TableStyles struct {
	Header    lipgloss.Style
	Cell      lipgloss.Style
	Border    lipgloss.Style
	Highlight lipgloss.Style
	Faint     lipgloss.Style
}

// NewAggregationTable 创建聚合表格
func NewAggregationTable() *AggregationTable {
	return &AggregationTable{
		pageSize: 10,
		styles:   DefaultTableStyles(),
	}
}

// DefaultTableStyles 默认表格样式
func DefaultTableStyles() TableStyles {
	return TableStyles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("240")),
		Cell: lipgloss.NewStyle().
			Padding(0, 1),
		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
		Highlight: lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("15")),
		Faint: lipgloss.NewStyle().
			Faint(true),
	}
}

// Update 更新表格数据
func (at *AggregationTable) Update(data []calculations.AggregatedData) {
	at.data = data
	at.page = 0
}

// SetWidth 设置表格宽度
func (at *AggregationTable) SetWidth(width int) {
	at.width = width
}

// Render 渲染表格
func (at *AggregationTable) Render(width int) string {
	at.width = width
	
	if len(at.data) == 0 {
		return at.styles.Faint.Render("No data to display")
	}

	// 创建响应式表格
	table := NewResponsiveTable(width)

	// 定义列
	columns := []Column{
		{Key: "date", Title: "Date", MinWidth: 12, Priority: 1, Alignment: AlignLeft},
		{Key: "entries", Title: "Messages", MinWidth: 10, Priority: 3, Alignment: AlignRight},
		{Key: "tokens", Title: "Tokens", MinWidth: 12, Priority: 2, Alignment: AlignRight},
		{Key: "cost", Title: "Cost", MinWidth: 10, Priority: 2, Alignment: AlignRight},
		{Key: "avg", Title: "Avg/Msg", MinWidth: 10, Priority: 4, Alignment: AlignRight},
		{Key: "model", Title: "Top Model", MinWidth: 15, Priority: 5, Alignment: AlignLeft},
	}

	table.SetColumns(columns)

	// 添加数据行
	start := at.page * at.pageSize
	end := start + at.pageSize
	if end > len(at.data) {
		end = len(at.data)
	}

	for i := start; i < end; i++ {
		data := at.data[i]

		// 找出使用最多的模型
		topModel := at.getTopModel(data.Models)

		// 计算平均每条消息的 token
		avgTokens := 0
		if data.Entries > 0 {
			avgTokens = data.Tokens.Total / data.Entries
		}

		row := []interface{}{
			data.Period.Label,
			fmt.Sprintf("%d", data.Entries),
			at.formatTokens(data.Tokens.Total),
			fmt.Sprintf("$%.2f", data.Cost.Total),
			at.formatTokens(avgTokens),
			topModel,
		}

		table.AddRow(row)
	}

	// 渲染表格
	tableContent := table.Render()

	// 添加分页信息
	pageInfo := at.renderPageInfo()

	// 添加排序指示器
	sortInfo := at.renderSortInfo()

	sections := []string{tableContent}
	if pageInfo != "" {
		sections = append(sections, pageInfo)
	}
	if sortInfo != "" {
		sections = append(sections, sortInfo)
	}

	return strings.Join(sections, "\n")
}

// getTopModel 获取使用最多的模型
func (at *AggregationTable) getTopModel(models map[string]calculations.ModelStats) string {
	var topModel string
	var maxTokens int

	for model, stats := range models {
		if stats.Tokens > maxTokens {
			maxTokens = stats.Tokens
			topModel = model
		}
	}

	// 简化模型名称显示
	if strings.Contains(topModel, "claude-3-") {
		topModel = strings.TrimPrefix(topModel, "claude-3-")
	}

	// 截断过长的模型名
	if len(topModel) > 12 {
		topModel = topModel[:9] + "..."
	}

	return topModel
}

// formatTokens 格式化 token 数量
func (at *AggregationTable) formatTokens(tokens int) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	} else if tokens >= 1000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}

// renderPageInfo 渲染分页信息
func (at *AggregationTable) renderPageInfo() string {
	if len(at.data) <= at.pageSize {
		return ""
	}

	totalPages := (len(at.data) + at.pageSize - 1) / at.pageSize
	info := fmt.Sprintf("Page %d of %d", at.page+1, totalPages)

	nav := []string{}
	if at.page > 0 {
		nav = append(nav, "← Previous")
	}
	if at.page < totalPages-1 {
		nav = append(nav, "Next →")
	}

	if len(nav) > 0 {
		info += "  " + strings.Join(nav, " | ")
	}

	return at.styles.Faint.Render(info)
}

// renderSortInfo 渲染排序信息
func (at *AggregationTable) renderSortInfo() string {
	if at.sortColumn == 0 {
		return ""
	}

	columns := []string{"Date", "Messages", "Tokens", "Cost", "Avg/Msg", "Top Model"}
	if at.sortColumn < len(columns) {
		direction := "↑"
		if !at.sortAsc {
			direction = "↓"
		}
		sortInfo := fmt.Sprintf("Sorted by %s %s", columns[at.sortColumn], direction)
		return at.styles.Faint.Render(sortInfo)
	}

	return ""
}

// NextPage 下一页
func (at *AggregationTable) NextPage() {
	totalPages := (len(at.data) + at.pageSize - 1) / at.pageSize
	if at.page < totalPages-1 {
		at.page++
	}
}

// PreviousPage 上一页
func (at *AggregationTable) PreviousPage() {
	if at.page > 0 {
		at.page--
	}
}

// Sort 排序
func (at *AggregationTable) Sort(column int) {
	if at.sortColumn == column {
		at.sortAsc = !at.sortAsc
	} else {
		at.sortColumn = column
		at.sortAsc = true
	}

	// 执行排序
	sort.Slice(at.data, func(i, j int) bool {
		var less bool
		
		switch at.sortColumn {
		case 0: // Date
			less = at.data[i].Period.Start.Before(at.data[j].Period.Start)
		case 1: // Messages
			less = at.data[i].Entries < at.data[j].Entries
		case 2: // Tokens
			less = at.data[i].Tokens.Total < at.data[j].Tokens.Total
		case 3: // Cost
			less = at.data[i].Cost.Total < at.data[j].Cost.Total
		case 4: // Avg/Msg
			avgI := 0
			if at.data[i].Entries > 0 {
				avgI = at.data[i].Tokens.Total / at.data[i].Entries
			}
			avgJ := 0
			if at.data[j].Entries > 0 {
				avgJ = at.data[j].Tokens.Total / at.data[j].Entries
			}
			less = avgI < avgJ
		case 5: // Top Model
			modelI := at.getTopModel(at.data[i].Models)
			modelJ := at.getTopModel(at.data[j].Models)
			less = strings.Compare(modelI, modelJ) < 0
		default:
			less = at.data[i].Period.Start.Before(at.data[j].Period.Start)
		}

		if at.sortAsc {
			return less
		}
		return !less
	})
}

// GetSelectedData 获取选中的数据
func (at *AggregationTable) GetSelectedData(selectedIndex int) *calculations.AggregatedData {
	start := at.page * at.pageSize
	actualIndex := start + selectedIndex
	
	if actualIndex >= 0 && actualIndex < len(at.data) {
		return &at.data[actualIndex]
	}
	
	return nil
}

// RenderDetailView 渲染详细视图
func (at *AggregationTable) RenderDetailView(data *calculations.AggregatedData, width int) string {
	if data == nil {
		return at.styles.Faint.Render("No data selected")
	}

	var sections []string

	// 基本信息
	basicInfo := fmt.Sprintf("📅 %s\n💬 %d messages\n🔢 %s tokens\n💰 $%.2f",
		data.Period.Label,
		data.Entries,
		at.formatTokens(data.Tokens.Total),
		data.Cost.Total,
	)
	sections = append(sections, at.styles.Header.Render("Basic Information"))
	sections = append(sections, basicInfo)

	// Token 详情
	if data.Tokens.Total > 0 {
		tokenDetails := fmt.Sprintf("Input: %s\nOutput: %s\nCached: %s\nPeak: %s",
			at.formatTokens(data.Tokens.Input),
			at.formatTokens(data.Tokens.Output),
			at.formatTokens(data.Tokens.Cache),
			at.formatTokens(data.Tokens.Peak),
		)
		sections = append(sections, at.styles.Header.Render("Token Breakdown"))
		sections = append(sections, tokenDetails)
	}

	// 模型分布
	if len(data.Models) > 0 {
		sections = append(sections, at.styles.Header.Render("Model Distribution"))
		
		// 按使用量排序模型
		type modelUsage struct {
			name   string
			stats  calculations.ModelStats
			percent float64
		}

		var models []modelUsage
		for name, stats := range data.Models {
			percent := 0.0
			if data.Tokens.Total > 0 {
				percent = float64(stats.Tokens) / float64(data.Tokens.Total) * 100
			}
			models = append(models, modelUsage{name, stats, percent})
		}

		sort.Slice(models, func(i, j int) bool {
			return models[i].stats.Tokens > models[j].stats.Tokens
		})

		for _, model := range models {
			modelInfo := fmt.Sprintf("%s: %s tokens (%.1f%%) - $%.2f",
				at.simplifyModelName(model.name),
				at.formatTokens(model.stats.Tokens),
				model.percent,
				model.stats.Cost,
			)
			sections = append(sections, modelInfo)
		}
	}

	// 成本分析
	if data.Cost.Total > 0 {
		costAnalysis := fmt.Sprintf("Average: $%.4f\nMin: $%.4f\nMax: $%.4f",
			data.Cost.Average,
			data.Cost.Min,
			data.Cost.Max,
		)
		sections = append(sections, at.styles.Header.Render("Cost Analysis"))
		sections = append(sections, costAnalysis)
	}

	content := strings.Join(sections, "\n\n")
	
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1).
		Width(width - 4).
		Render(content)
}

// simplifyModelName 简化模型名称
func (at *AggregationTable) simplifyModelName(name string) string {
	// 移除常见前缀
	name = strings.TrimPrefix(name, "claude-3-")
	name = strings.TrimPrefix(name, "claude-")
	
	// 首字母大写
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	
	return name
}

// GetStats 获取表格统计信息
func (at *AggregationTable) GetStats() TableStats {
	if len(at.data) == 0 {
		return TableStats{}
	}

	stats := TableStats{
		TotalEntries: len(at.data),
		TotalPages:   (len(at.data) + at.pageSize - 1) / at.pageSize,
		CurrentPage:  at.page + 1,
	}

	// 计算汇总统计
	for _, data := range at.data {
		stats.TotalTokens += data.Tokens.Total
		stats.TotalCost += data.Cost.Total
		stats.TotalMessages += data.Entries
	}

	if len(at.data) > 0 {
		stats.AvgTokensPerEntry = stats.TotalTokens / len(at.data)
		stats.AvgCostPerEntry = stats.TotalCost / float64(len(at.data))
	}

	return stats
}

// TableStats 表格统计信息
type TableStats struct {
	TotalEntries      int     `json:"total_entries"`
	TotalPages        int     `json:"total_pages"`
	CurrentPage       int     `json:"current_page"`
	TotalTokens       int     `json:"total_tokens"`
	TotalCost         float64 `json:"total_cost"`
	TotalMessages     int     `json:"total_messages"`
	AvgTokensPerEntry int     `json:"avg_tokens_per_entry"`
	AvgCostPerEntry   float64 `json:"avg_cost_per_entry"`
}

// Export 导出表格数据
func (at *AggregationTable) Export(format ExportFormat) (string, error) {
	switch format {
	case ExportCSV:
		return at.exportCSV()
	case ExportJSON:
		return at.exportJSON()
	case ExportTSV:
		return at.exportTSV()
	default:
		return "", fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportCSV 导出为 CSV
func (at *AggregationTable) exportCSV() (string, error) {
	var lines []string
	
	// CSV 头
	headers := []string{"Date", "Messages", "Tokens", "Cost", "Avg_Tokens_Per_Message", "Top_Model"}
	lines = append(lines, strings.Join(headers, ","))
	
	// 数据行
	for _, data := range at.data {
		avgTokens := 0
		if data.Entries > 0 {
			avgTokens = data.Tokens.Total / data.Entries
		}
		
		topModel := at.getTopModel(data.Models)
		
		row := []string{
			fmt.Sprintf("\"%s\"", data.Period.Label),
			fmt.Sprintf("%d", data.Entries),
			fmt.Sprintf("%d", data.Tokens.Total),
			fmt.Sprintf("%.2f", data.Cost.Total),
			fmt.Sprintf("%d", avgTokens),
			fmt.Sprintf("\"%s\"", topModel),
		}
		
		lines = append(lines, strings.Join(row, ","))
	}
	
	return strings.Join(lines, "\n"), nil
}

// exportJSON 导出为 JSON
func (at *AggregationTable) exportJSON() (string, error) {
	// 简化的 JSON 导出
	var jsonLines []string
	jsonLines = append(jsonLines, "{")
	jsonLines = append(jsonLines, `  "data": [`)
	
	for i, data := range at.data {
		avgTokens := 0
		if data.Entries > 0 {
			avgTokens = data.Tokens.Total / data.Entries
		}
		
		topModel := at.getTopModel(data.Models)
		
		jsonLine := fmt.Sprintf(`    {`+
			`"date": "%s", `+
			`"messages": %d, `+
			`"tokens": %d, `+
			`"cost": %.2f, `+
			`"avg_tokens_per_message": %d, `+
			`"top_model": "%s"`+
			`}`,
			data.Period.Label,
			data.Entries,
			data.Tokens.Total,
			data.Cost.Total,
			avgTokens,
			topModel,
		)
		
		if i < len(at.data)-1 {
			jsonLine += ","
		}
		
		jsonLines = append(jsonLines, jsonLine)
	}
	
	jsonLines = append(jsonLines, "  ]")
	jsonLines = append(jsonLines, "}")
	
	return strings.Join(jsonLines, "\n"), nil
}

// exportTSV 导出为 TSV
func (at *AggregationTable) exportTSV() (string, error) {
	var lines []string
	
	// TSV 头
	headers := []string{"Date", "Messages", "Tokens", "Cost", "Avg_Tokens_Per_Message", "Top_Model"}
	lines = append(lines, strings.Join(headers, "\t"))
	
	// 数据行
	for _, data := range at.data {
		avgTokens := 0
		if data.Entries > 0 {
			avgTokens = data.Tokens.Total / data.Entries
		}
		
		topModel := at.getTopModel(data.Models)
		
		row := []string{
			data.Period.Label,
			fmt.Sprintf("%d", data.Entries),
			fmt.Sprintf("%d", data.Tokens.Total),
			fmt.Sprintf("%.2f", data.Cost.Total),
			fmt.Sprintf("%d", avgTokens),
			topModel,
		}
		
		lines = append(lines, strings.Join(row, "\t"))
	}
	
	return strings.Join(lines, "\n"), nil
}

// ExportFormat 导出格式
type ExportFormat string

const (
	ExportCSV  ExportFormat = "csv"
	ExportJSON ExportFormat = "json"
	ExportTSV  ExportFormat = "tsv"
)