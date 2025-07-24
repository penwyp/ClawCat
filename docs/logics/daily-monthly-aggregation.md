# 日/月聚合视图开发计划

## 1. 功能概述

日/月聚合视图为用户提供历史数据的汇总和分析功能。通过按天、周、月等时间维度聚合数据，帮助用户了解长期使用趋势、识别使用模式，并进行成本预算管理。

### 1.1 核心功能

- **日度聚合**: 按天统计 token 使用量、成本、消息数等
- **月度汇总**: 月度总计、日均使用量、成本趋势
- **历史对比**: 不同时期的数据对比分析
- **使用模式**: 识别高峰时段、常用模型等
- **导出报表**: 支持导出聚合数据用于进一步分析

## 2. 技术设计

### 2.1 数据结构

```go
// AggregationView 聚合视图类型
type AggregationView string

const (
    DailyView   AggregationView = "daily"
    WeeklyView  AggregationView = "weekly"
    MonthlyView AggregationView = "monthly"
    CustomView  AggregationView = "custom"
)

// AggregatedData 聚合数据
type AggregatedData struct {
    Period      TimePeriod
    Entries     int
    Tokens      TokenStats
    Cost        CostStats
    Models      map[string]ModelStats
    Sessions    []SessionSummary
    Patterns    UsagePattern
}

// TimePeriod 时间段
type TimePeriod struct {
    Start       time.Time
    End         time.Time
    Label       string // e.g., "2024-01-15", "Week 3", "January 2024"
    Type        AggregationView
}

// TokenStats token 统计
type TokenStats struct {
    Total       int
    Input       int
    Output      int
    Cache       int
    Average     float64
    Peak        int
    PeakTime    time.Time
}

// CostStats 成本统计
type CostStats struct {
    Total       float64
    Average     float64
    Min         float64
    Max         float64
    Breakdown   map[string]float64 // 按模型分解
}

// UsagePattern 使用模式
type UsagePattern struct {
    PeakHours   []int     // 高峰时段（小时）
    PeakDays    []string  // 高峰日期（星期几）
    Trend       TrendType // 上升、下降、稳定
    Anomalies   []Anomaly // 异常使用
}
```

### 2.2 聚合引擎

```go
// AggregationEngine 数据聚合引擎
type AggregationEngine struct {
    entries     []models.UsageEntry
    timezone    *time.Location
    config      *config.Config
    cache       *AggregationCache
}

// Aggregate 执行聚合
func (ae *AggregationEngine) Aggregate(view AggregationView, start, end time.Time) ([]AggregatedData, error)

// GroupByDay 按天分组
func (ae *AggregationEngine) GroupByDay() map[string][]models.UsageEntry

// GroupByWeek 按周分组
func (ae *AggregationEngine) GroupByWeek() map[string][]models.UsageEntry

// GroupByMonth 按月分组
func (ae *AggregationEngine) GroupByMonth() map[string][]models.UsageEntry

// CalculateStats 计算统计数据
func (ae *AggregationEngine) CalculateStats(entries []models.UsageEntry) AggregatedData

// DetectPatterns 检测使用模式
func (ae *AggregationEngine) DetectPatterns(aggregated []AggregatedData) UsagePattern
```

## 3. 实现步骤

### 3.1 创建聚合引擎

**文件**: `calculations/aggregation.go`

```go
package calculations

import (
    "fmt"
    "sort"
    "time"
    "github.com/penwyp/ClawCat/models"
)

// NewAggregationEngine 创建聚合引擎
func NewAggregationEngine(entries []models.UsageEntry, config *config.Config) *AggregationEngine {
    return &AggregationEngine{
        entries:  entries,
        timezone: config.GetTimezone(),
        config:   config,
        cache:    NewAggregationCache(),
    }
}

// Aggregate 执行聚合操作
func (ae *AggregationEngine) Aggregate(view AggregationView, start, end time.Time) ([]AggregatedData, error) {
    // 检查缓存
    cacheKey := fmt.Sprintf("%s_%s_%s", view, start.Format("20060102"), end.Format("20060102"))
    if cached, ok := ae.cache.Get(cacheKey); ok {
        return cached, nil
    }
    
    // 过滤时间范围内的数据
    filtered := ae.filterByTimeRange(start, end)
    
    // 根据视图类型分组
    var grouped map[string][]models.UsageEntry
    switch view {
    case DailyView:
        grouped = ae.groupByDay(filtered)
    case WeeklyView:
        grouped = ae.groupByWeek(filtered)
    case MonthlyView:
        grouped = ae.groupByMonth(filtered)
    default:
        return nil, fmt.Errorf("unsupported view: %s", view)
    }
    
    // 计算每个分组的统计数据
    results := make([]AggregatedData, 0, len(grouped))
    for period, entries := range grouped {
        data := ae.calculateStats(entries, period, view)
        results = append(results, data)
    }
    
    // 按时间排序
    sort.Slice(results, func(i, j int) bool {
        return results[i].Period.Start.Before(results[j].Period.Start)
    })
    
    // 缓存结果
    ae.cache.Set(cacheKey, results)
    
    return results, nil
}

// groupByDay 按天分组
func (ae *AggregationEngine) groupByDay(entries []models.UsageEntry) map[string][]models.UsageEntry {
    grouped := make(map[string][]models.UsageEntry)
    
    for _, entry := range entries {
        // 转换到用户时区
        localTime := entry.Timestamp.In(ae.timezone)
        dayKey := localTime.Format("2006-01-02")
        grouped[dayKey] = append(grouped[dayKey], entry)
    }
    
    return grouped
}

// groupByWeek 按周分组
func (ae *AggregationEngine) groupByWeek(entries []models.UsageEntry) map[string][]models.UsageEntry {
    grouped := make(map[string][]models.UsageEntry)
    
    for _, entry := range entries {
        localTime := entry.Timestamp.In(ae.timezone)
        year, week := localTime.ISOWeek()
        weekKey := fmt.Sprintf("%d-W%02d", year, week)
        grouped[weekKey] = append(grouped[weekKey], entry)
    }
    
    return grouped
}

// groupByMonth 按月分组
func (ae *AggregationEngine) groupByMonth(entries []models.UsageEntry) map[string][]models.UsageEntry {
    grouped := make(map[string][]models.UsageEntry)
    
    for _, entry := range entries {
        localTime := entry.Timestamp.In(ae.timezone)
        monthKey := localTime.Format("2006-01")
        grouped[monthKey] = append(grouped[monthKey], entry)
    }
    
    return grouped
}

// calculateStats 计算统计数据
func (ae *AggregationEngine) calculateStats(entries []models.UsageEntry, periodKey string, view AggregationView) AggregatedData {
    if len(entries) == 0 {
        return AggregatedData{Period: ae.parsePeriod(periodKey, view)}
    }
    
    data := AggregatedData{
        Period:  ae.parsePeriod(periodKey, view),
        Entries: len(entries),
        Models:  make(map[string]ModelStats),
    }
    
    // 初始化统计
    tokenStats := TokenStats{Min: int(^uint(0) >> 1)} // Max int
    costStats := CostStats{Min: float64(^uint(0) >> 1)}
    
    // 遍历计算
    for _, entry := range entries {
        // Token 统计
        tokenStats.Total += entry.TotalTokens
        tokenStats.Input += entry.InputTokens
        tokenStats.Output += entry.OutputTokens
        tokenStats.Cache += entry.CacheCreationTokens + entry.CacheReadTokens
        
        if entry.TotalTokens > tokenStats.Peak {
            tokenStats.Peak = entry.TotalTokens
            tokenStats.PeakTime = entry.Timestamp
        }
        
        // 成本统计
        costStats.Total += entry.CostUSD
        if entry.CostUSD > costStats.Max {
            costStats.Max = entry.CostUSD
        }
        if entry.CostUSD < costStats.Min {
            costStats.Min = entry.CostUSD
        }
        
        // 模型统计
        modelStat := data.Models[entry.Model]
        modelStat.Count++
        modelStat.Tokens += entry.TotalTokens
        modelStat.Cost += entry.CostUSD
        data.Models[entry.Model] = modelStat
    }
    
    // 计算平均值
    if len(entries) > 0 {
        tokenStats.Average = float64(tokenStats.Total) / float64(len(entries))
        costStats.Average = costStats.Total / float64(len(entries))
    }
    
    // 计算成本分解
    costStats.Breakdown = make(map[string]float64)
    for model, stats := range data.Models {
        costStats.Breakdown[model] = stats.Cost
    }
    
    data.Tokens = tokenStats
    data.Cost = costStats
    
    return data
}

// parsePeriod 解析时间段
func (ae *AggregationEngine) parsePeriod(key string, view AggregationView) TimePeriod {
    period := TimePeriod{
        Label: key,
        Type:  view,
    }
    
    switch view {
    case DailyView:
        if t, err := time.ParseInLocation("2006-01-02", key, ae.timezone); err == nil {
            period.Start = t
            period.End = t.Add(24 * time.Hour).Add(-time.Nanosecond)
        }
    case WeeklyView:
        // 解析 "2024-W03" 格式
        var year, week int
        if _, err := fmt.Sscanf(key, "%d-W%d", &year, &week); err == nil {
            period.Start = weekStart(year, week, ae.timezone)
            period.End = period.Start.Add(7 * 24 * time.Hour).Add(-time.Nanosecond)
            period.Label = fmt.Sprintf("Week %d, %d", week, year)
        }
    case MonthlyView:
        if t, err := time.ParseInLocation("2006-01", key, ae.timezone); err == nil {
            period.Start = t
            period.End = t.AddDate(0, 1, 0).Add(-time.Nanosecond)
            period.Label = t.Format("January 2006")
        }
    }
    
    return period
}
```

### 3.2 创建聚合视图 UI

**文件**: `ui/aggregation_view.go`

```go
package ui

import (
    "fmt"
    "strings"
    "time"
    "github.com/charmbracelet/lipgloss"
    "github.com/penwyp/ClawCat/calculations"
)

// AggregationView UI 聚合视图
type AggregationView struct {
    viewType    calculations.AggregationView
    dateRange   DateRange
    data        []calculations.AggregatedData
    table       *AggregationTable
    chart       *UsageChart
    selected    int
    width       int
    height      int
    styles      Styles
}

// DateRange 日期范围
type DateRange struct {
    Start time.Time
    End   time.Time
}

// NewAggregationView 创建聚合视图
func NewAggregationView() *AggregationView {
    return &AggregationView{
        viewType: calculations.DailyView,
        dateRange: DateRange{
            Start: time.Now().AddDate(0, 0, -30), // 默认最近30天
            End:   time.Now(),
        },
        table:  NewAggregationTable(),
        chart:  NewUsageChart(),
        styles: NewStyles(DefaultTheme()),
    }
}

// Update 更新视图数据
func (av *AggregationView) Update(data []calculations.AggregatedData) {
    av.data = data
    av.table.Update(data)
    av.chart.Update(data)
}

// View 渲染视图
func (av *AggregationView) View() string {
    if av.width == 0 || av.height == 0 {
        return "Loading aggregation view..."
    }
    
    // 标题栏
    header := av.renderHeader()
    
    // 视图切换器
    viewSelector := av.renderViewSelector()
    
    // 主要内容区域
    content := av.renderContent()
    
    // 统计摘要
    summary := av.renderSummary()
    
    // 组合所有部分
    sections := []string{
        header,
        viewSelector,
        content,
        summary,
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
            style = av.styles.ButtonActive
        }
        buttons[i] = style.Render(v.Label)
    }
    
    return lipgloss.JoinHorizontal(lipgloss.Center, buttons...)
}

// renderContent 渲染主要内容
func (av *AggregationView) renderContent() string {
    if len(av.data) == 0 {
        return av.styles.Faint.Render("No data available for the selected period")
    }
    
    // 根据屏幕宽度决定布局
    if av.width > 120 {
        // 宽屏：并排显示表格和图表
        table := av.table.Render(av.width/2 - 2)
        chart := av.chart.Render(av.width/2 - 2)
        return lipgloss.JoinHorizontal(lipgloss.Top, table, "  ", chart)
    } else {
        // 窄屏：垂直排列
        table := av.table.Render(av.width - 4)
        chart := av.chart.Render(av.width - 4)
        return lipgloss.JoinVertical(lipgloss.Left, table, chart)
    }
}

// renderSummary 渲染统计摘要
func (av *AggregationView) renderSummary() string {
    if len(av.data) == 0 {
        return ""
    }
    
    summary := av.calculateSummary()
    
    cards := []string{
        av.renderSummaryCard("Total Tokens", formatNumber(summary.TotalTokens), av.styles.Info),
        av.renderSummaryCard("Total Cost", fmt.Sprintf("$%.2f", summary.TotalCost), av.styles.Warning),
        av.renderSummaryCard("Avg Daily", formatNumber(summary.AvgDailyTokens), av.styles.Success),
        av.renderSummaryCard("Peak Day", summary.PeakDay, av.styles.Normal),
    }
    
    return av.styles.Box.Render(
        lipgloss.JoinHorizontal(lipgloss.Center, cards...),
    )
}

// calculateSummary 计算摘要统计
func (av *AggregationView) calculateSummary() Summary {
    summary := Summary{}
    
    for _, data := range av.data {
        summary.TotalTokens += data.Tokens.Total
        summary.TotalCost += data.Cost.Total
        
        if data.Tokens.Total > summary.PeakTokens {
            summary.PeakTokens = data.Tokens.Total
            summary.PeakDay = data.Period.Label
        }
    }
    
    if len(av.data) > 0 {
        summary.AvgDailyTokens = summary.TotalTokens / len(av.data)
        summary.AvgDailyCost = summary.TotalCost / float64(len(av.data))
    }
    
    return summary
}
```

### 3.3 创建聚合表格组件

**文件**: `ui/components/aggregation_table.go`

```go
package components

import (
    "fmt"
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
    styles     TableStyles
}

// NewAggregationTable 创建聚合表格
func NewAggregationTable() *AggregationTable {
    return &AggregationTable{
        pageSize: 10,
        styles:   DefaultTableStyles(),
    }
}

// Update 更新表格数据
func (at *AggregationTable) Update(data []calculations.AggregatedData) {
    at.data = data
    at.page = 0
}

// Render 渲染表格
func (at *AggregationTable) Render(width int) string {
    if len(at.data) == 0 {
        return "No data to display"
    }
    
    // 创建响应式表格
    table := NewResponsiveTable(width)
    
    // 定义列
    columns := []Column{
        {Key: "date", Title: "Date", MinWidth: 12, Priority: 1},
        {Key: "entries", Title: "Messages", MinWidth: 10, Priority: 3},
        {Key: "tokens", Title: "Tokens", MinWidth: 12, Priority: 2},
        {Key: "cost", Title: "Cost", MinWidth: 10, Priority: 2},
        {Key: "avg", Title: "Avg/Msg", MinWidth: 10, Priority: 4},
        {Key: "model", Title: "Top Model", MinWidth: 15, Priority: 5},
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
            formatNumber(data.Tokens.Total),
            fmt.Sprintf("$%.2f", data.Cost.Total),
            formatNumber(avgTokens),
            topModel,
        }
        
        table.AddRow(row)
    }
    
    // 添加分页信息
    pageInfo := at.renderPageInfo()
    
    return strings.Join([]string{
        table.Render(),
        pageInfo,
    }, "\n")
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
    
    return topModel
}

// renderPageInfo 渲染分页信息
func (at *AggregationTable) renderPageInfo() string {
    totalPages := (len(at.data) + at.pageSize - 1) / at.pageSize
    
    info := fmt.Sprintf("Page %d of %d", at.page+1, totalPages)
    
    nav := []string{}
    if at.page > 0 {
        nav = append(nav, "← Previous")
    }
    if at.page < totalPages-1 {
        nav = append(nav, "Next →")
    }
    
    return at.styles.Faint.Render(
        fmt.Sprintf("%s  %s", info, strings.Join(nav, " | ")),
    )
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
        switch at.sortColumn {
        case 1: // Messages
            if at.sortAsc {
                return at.data[i].Entries < at.data[j].Entries
            }
            return at.data[i].Entries > at.data[j].Entries
        case 2: // Tokens
            if at.sortAsc {
                return at.data[i].Tokens.Total < at.data[j].Tokens.Total
            }
            return at.data[i].Tokens.Total > at.data[j].Tokens.Total
        case 3: // Cost
            if at.sortAsc {
                return at.data[i].Cost.Total < at.data[j].Cost.Total
            }
            return at.data[i].Cost.Total > at.data[j].Cost.Total
        default: // Date
            if at.sortAsc {
                return at.data[i].Period.Start.Before(at.data[j].Period.Start)
            }
            return at.data[i].Period.Start.After(at.data[j].Period.Start)
        }
    })
}
```

### 3.4 创建使用趋势图表

**文件**: `ui/components/usage_chart.go`

```go
package components

import (
    "fmt"
    "math"
    "strings"
    "github.com/charmbracelet/lipgloss"
    "github.com/penwyp/ClawCat/calculations"
)

// UsageChart 使用趋势图表
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

// NewUsageChart 创建使用图表
func NewUsageChart() *UsageChart {
    return &UsageChart{
        chartType: BarChart,
        height:    10,
        styles:    DefaultChartStyles(),
    }
}

// Update 更新图表数据
func (uc *UsageChart) Update(data []calculations.AggregatedData) {
    uc.data = data
}

// Render 渲染图表
func (uc *UsageChart) Render(width int) string {
    uc.width = width
    
    if len(uc.data) == 0 {
        return "No data for chart"
    }
    
    title := uc.styles.Title.Render("Token Usage Trend")
    
    var chart string
    switch uc.chartType {
    case LineChart:
        chart = uc.renderLineChart()
    case AreaChart:
        chart = uc.renderAreaChart()
    default:
        chart = uc.renderBarChart()
    }
    
    legend := uc.renderLegend()
    
    return strings.Join([]string{title, chart, legend}, "\n")
}

// renderBarChart 渲染柱状图
func (uc *UsageChart) renderBarChart() string {
    // 找出最大值用于缩放
    maxTokens := 0
    for _, data := range uc.data {
        if data.Tokens.Total > maxTokens {
            maxTokens = data.Tokens.Total
        }
    }
    
    if maxTokens == 0 {
        return "No token usage"
    }
    
    // 计算每个数据点的宽度
    barWidth := uc.width / len(uc.data)
    if barWidth < 3 {
        barWidth = 3
    }
    
    // 构建图表
    lines := make([]string, uc.height)
    
    for row := 0; row < uc.height; row++ {
        line := strings.Builder{}
        threshold := float64(maxTokens) * float64(uc.height-row) / float64(uc.height)
        
        for _, data := range uc.data {
            // 计算这个数据点是否应该在这一行显示
            if float64(data.Tokens.Total) >= threshold {
                // 根据使用量选择颜色
                color := uc.getBarColor(float64(data.Tokens.Total) / float64(maxTokens))
                bar := strings.Repeat("█", barWidth-1) + " "
                line.WriteString(lipgloss.NewStyle().Foreground(color).Render(bar))
            } else {
                line.WriteString(strings.Repeat(" ", barWidth))
            }
        }
        
        lines[row] = line.String()
    }
    
    // 添加 X 轴
    xAxis := uc.renderXAxis()
    lines = append(lines, xAxis)
    
    return uc.styles.Chart.Render(strings.Join(lines, "\n"))
}

// renderLineChart 渲染折线图
func (uc *UsageChart) renderLineChart() string {
    if len(uc.data) < 2 {
        return uc.renderBarChart() // 数据点太少，降级到柱状图
    }
    
    // 找出最大值和最小值
    maxTokens, minTokens := 0, int(^uint(0)>>1)
    for _, data := range uc.data {
        if data.Tokens.Total > maxTokens {
            maxTokens = data.Tokens.Total
        }
        if data.Tokens.Total < minTokens {
            minTokens = data.Tokens.Total
        }
    }
    
    // 创建画布
    canvas := make([][]rune, uc.height)
    for i := range canvas {
        canvas[i] = make([]rune, uc.width)
        for j := range canvas[i] {
            canvas[i][j] = ' '
        }
    }
    
    // 绘制数据点和连线
    points := make([]Point, len(uc.data))
    for i, data := range uc.data {
        x := i * (uc.width - 1) / (len(uc.data) - 1)
        y := uc.height - 1 - int(float64(data.Tokens.Total-minTokens)/float64(maxTokens-minTokens)*float64(uc.height-1))
        points[i] = Point{X: x, Y: y}
        
        // 绘制数据点
        if x >= 0 && x < uc.width && y >= 0 && y < uc.height {
            canvas[y][x] = '●'
        }
    }
    
    // 连接点
    for i := 1; i < len(points); i++ {
        uc.drawLine(canvas, points[i-1], points[i])
    }
    
    // 转换为字符串
    lines := make([]string, uc.height)
    for i, row := range canvas {
        lines[i] = string(row)
    }
    
    return uc.styles.Chart.Render(strings.Join(lines, "\n"))
}

// drawLine 在画布上绘制线段
func (uc *UsageChart) drawLine(canvas [][]rune, p1, p2 Point) {
    dx := abs(p2.X - p1.X)
    dy := abs(p2.Y - p1.Y)
    sx := 1
    if p1.X > p2.X {
        sx = -1
    }
    sy := 1
    if p1.Y > p2.Y {
        sy = -1
    }
    err := dx - dy
    
    x, y := p1.X, p1.Y
    
    for {
        if x >= 0 && x < uc.width && y >= 0 && y < uc.height {
            if canvas[y][x] == ' ' {
                canvas[y][x] = '─'
            }
        }
        
        if x == p2.X && y == p2.Y {
            break
        }
        
        e2 := 2 * err
        if e2 > -dy {
            err -= dy
            x += sx
        }
        if e2 < dx {
            err += dx
            y += sy
        }
    }
}

// renderXAxis 渲染 X 轴标签
func (uc *UsageChart) renderXAxis() string {
    if len(uc.data) == 0 {
        return ""
    }
    
    // 简化标签显示
    labels := make([]string, len(uc.data))
    for i, data := range uc.data {
        // 根据数据类型选择合适的标签格式
        switch data.Period.Type {
        case calculations.DailyView:
            labels[i] = data.Period.Start.Format("01/02")
        case calculations.WeeklyView:
            labels[i] = data.Period.Start.Format("W") + fmt.Sprintf("%d", data.Period.Start.Day()/7+1)
        case calculations.MonthlyView:
            labels[i] = data.Period.Start.Format("Jan")
        default:
            labels[i] = fmt.Sprintf("%d", i+1)
        }
    }
    
    // 根据可用空间决定显示哪些标签
    labelWidth := uc.width / len(labels)
    displayLabels := make([]string, len(labels))
    
    for i, label := range labels {
        if i%(len(labels)/5+1) == 0 || i == len(labels)-1 { // 只显示部分标签
            displayLabels[i] = label
        }
    }
    
    return uc.formatXAxisLabels(displayLabels, labelWidth)
}

// getBarColor 根据使用率获取颜色
func (uc *UsageChart) getBarColor(usage float64) lipgloss.Color {
    if usage > 0.8 {
        return lipgloss.Color("#FF6B6B") // 红色
    } else if usage > 0.6 {
        return lipgloss.Color("#FFD93D") // 黄色
    } else if usage > 0.4 {
        return lipgloss.Color("#6BCF7F") // 绿色
    }
    return lipgloss.Color("#4ECDC4") // 青色
}

// renderLegend 渲染图例
func (uc *UsageChart) renderLegend() string {
    total := 0
    maxDay := ""
    maxTokens := 0
    
    for _, data := range uc.data {
        total += data.Tokens.Total
        if data.Tokens.Total > maxTokens {
            maxTokens = data.Tokens.Total
            maxDay = data.Period.Label
        }
    }
    
    avg := 0
    if len(uc.data) > 0 {
        avg = total / len(uc.data)
    }
    
    legend := fmt.Sprintf(
        "Total: %s | Average: %s | Peak: %s on %s",
        formatNumber(total),
        formatNumber(avg),
        formatNumber(maxTokens),
        maxDay,
    )
    
    return uc.styles.Legend.Render(legend)
}
```

## 4. 测试计划

### 4.1 单元测试

```go
// calculations/aggregation_test.go

func TestAggregationEngine_GroupByDay(t *testing.T) {
    // 创建测试数据
    entries := []models.UsageEntry{
        {Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), TotalTokens: 100},
        {Timestamp: time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC), TotalTokens: 200},
        {Timestamp: time.Date(2024, 1, 16, 9, 0, 0, 0, time.UTC), TotalTokens: 150},
    }
    
    engine := NewAggregationEngine(entries, testConfig)
    
    grouped := engine.groupByDay(entries)
    
    assert.Len(t, grouped, 2)
    assert.Len(t, grouped["2024-01-15"], 2)
    assert.Len(t, grouped["2024-01-16"], 1)
}

func TestAggregationEngine_CalculateStats(t *testing.T) {
    entries := []models.UsageEntry{
        {
            Model:       "claude-3-opus",
            TotalTokens: 1000,
            CostUSD:     0.15,
        },
        {
            Model:       "claude-3-sonnet",
            TotalTokens: 500,
            CostUSD:     0.05,
        },
    }
    
    engine := NewAggregationEngine(entries, testConfig)
    stats := engine.calculateStats(entries, "2024-01-15", calculations.DailyView)
    
    assert.Equal(t, 1500, stats.Tokens.Total)
    assert.Equal(t, 0.20, stats.Cost.Total)
    assert.Equal(t, 750.0, stats.Tokens.Average)
    assert.Len(t, stats.Models, 2)
}

func TestAggregationEngine_DetectPatterns(t *testing.T) {
    // 创建有模式的测试数据
    var entries []models.UsageEntry
    
    // 模拟工作日高使用，周末低使用
    for day := 1; day <= 30; day++ {
        date := time.Date(2024, 1, day, 0, 0, 0, 0, time.UTC)
        tokensBase := 1000
        
        // 周末使用量降低
        if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
            tokensBase = 200
        }
        
        // 添加一天内的多个条目
        for hour := 9; hour <= 17; hour++ {
            entries = append(entries, models.UsageEntry{
                Timestamp:   date.Add(time.Duration(hour) * time.Hour),
                TotalTokens: tokensBase + rand.Intn(500),
            })
        }
    }
    
    engine := NewAggregationEngine(entries, testConfig)
    aggregated, _ := engine.Aggregate(calculations.DailyView, entries[0].Timestamp, entries[len(entries)-1].Timestamp)
    
    patterns := engine.DetectPatterns(aggregated)
    
    // 验证检测到的模式
    assert.Contains(t, patterns.PeakHours, 12) // 中午应该是高峰
    assert.NotContains(t, patterns.PeakDays, "Saturday")
    assert.NotContains(t, patterns.PeakDays, "Sunday")
}
```

### 4.2 集成测试

```go
// integration/aggregation_view_test.go

func TestAggregationView_Integration(t *testing.T) {
    // 创建测试应用
    app := setupTestApp(t)
    
    // 生成测试数据
    generateTestData(app, 60) // 60天的数据
    
    // 创建聚合视图
    view := ui.NewAggregationView()
    
    // 测试不同聚合级别
    t.Run("daily aggregation", func(t *testing.T) {
        view.SetViewType(calculations.DailyView)
        view.SetDateRange(time.Now().AddDate(0, 0, -30), time.Now())
        
        output := view.View()
        
        assert.Contains(t, output, "Daily")
        assert.Contains(t, output, "Date")
        assert.Contains(t, output, "Tokens")
    })
    
    t.Run("weekly aggregation", func(t *testing.T) {
        view.SetViewType(calculations.WeeklyView)
        
        output := view.View()
        
        assert.Contains(t, output, "Weekly")
        assert.Contains(t, output, "Week")
    })
    
    t.Run("monthly aggregation", func(t *testing.T) {
        view.SetViewType(calculations.MonthlyView)
        
        output := view.View()
        
        assert.Contains(t, output, "Monthly")
        assert.Contains(t, output, "January")
    })
}
```

## 5. 性能优化

### 5.1 数据缓存

```go
// AggregationCache 聚合数据缓存
type AggregationCache struct {
    cache *lru.Cache
    mu    sync.RWMutex
}

func NewAggregationCache() *AggregationCache {
    cache, _ := lru.New(100) // 缓存100个聚合结果
    return &AggregationCache{
        cache: cache,
    }
}

func (ac *AggregationCache) Get(key string) ([]AggregatedData, bool) {
    ac.mu.RLock()
    defer ac.mu.RUnlock()
    
    if val, ok := ac.cache.Get(key); ok {
        return val.([]AggregatedData), true
    }
    return nil, false
}

func (ac *AggregationCache) Set(key string, data []AggregatedData) {
    ac.mu.Lock()
    defer ac.mu.Unlock()
    
    ac.cache.Add(key, data)
}
```

### 5.2 增量聚合

```go
// IncrementalAggregator 增量聚合器
type IncrementalAggregator struct {
    lastProcessed time.Time
    baseData      []AggregatedData
}

func (ia *IncrementalAggregator) UpdateIncremental(newEntries []models.UsageEntry) []AggregatedData {
    // 只处理新数据
    var toProcess []models.UsageEntry
    for _, entry := range newEntries {
        if entry.Timestamp.After(ia.lastProcessed) {
            toProcess = append(toProcess, entry)
        }
    }
    
    if len(toProcess) == 0 {
        return ia.baseData
    }
    
    // 聚合新数据
    engine := NewAggregationEngine(toProcess, nil)
    newAggregated, _ := engine.Aggregate(DailyView, toProcess[0].Timestamp, toProcess[len(toProcess)-1].Timestamp)
    
    // 合并到基础数据
    merged := ia.mergeAggregatedData(ia.baseData, newAggregated)
    
    ia.baseData = merged
    ia.lastProcessed = toProcess[len(toProcess)-1].Timestamp
    
    return merged
}
```

## 6. 导出功能

### 6.1 报表导出

```go
// ReportExporter 报表导出器
type ReportExporter struct {
    data     []AggregatedData
    metadata ReportMetadata
}

// ExportHTML 导出 HTML 报表
func (re *ReportExporter) ExportHTML() (string, error) {
    tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>ClawCat Usage Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        .summary { background-color: #e8f4f8; padding: 15px; margin-bottom: 20px; }
        .chart { margin: 20px 0; }
    </style>
</head>
<body>
    <h1>Usage Report</h1>
    <div class="summary">
        <h2>Summary</h2>
        <p>Period: {{.StartDate}} - {{.EndDate}}</p>
        <p>Total Tokens: {{.TotalTokens}}</p>
        <p>Total Cost: ${{.TotalCost}}</p>
    </div>
    
    <h2>Daily Usage</h2>
    <table>
        <tr>
            <th>Date</th>
            <th>Messages</th>
            <th>Tokens</th>
            <th>Cost</th>
            <th>Top Model</th>
        </tr>
        {{range .Data}}
        <tr>
            <td>{{.Period.Label}}</td>
            <td>{{.Entries}}</td>
            <td>{{.Tokens.Total}}</td>
            <td>${{.Cost.Total}}</td>
            <td>{{.TopModel}}</td>
        </tr>
        {{end}}
    </table>
</body>
</html>
    `
    
    // 渲染模板
    t, err := template.New("report").Parse(tmpl)
    if err != nil {
        return "", err
    }
    
    var buf bytes.Buffer
    err = t.Execute(&buf, re.prepareTemplateData())
    
    return buf.String(), err
}

// ExportPDF 导出 PDF 报表
func (re *ReportExporter) ExportPDF() ([]byte, error) {
    // 使用第三方库如 gofpdf
    // 实现 PDF 生成逻辑
    return nil, nil
}
```

## 7. 配置选项

### 7.1 聚合配置

```yaml
# config.yaml
aggregation:
  default_view: "daily"
  default_range: 30  # 天
  timezone: "America/New_York"
  cache:
    enabled: true
    ttl: 1h
  export:
    formats: ["csv", "json", "html", "pdf"]
    include_charts: true
  patterns:
    detect_anomalies: true
    anomaly_threshold: 2.5  # 标准差
```

## 8. 错误处理

### 8.1 数据完整性检查

```go
func (ae *AggregationEngine) validateData() error {
    if len(ae.entries) == 0 {
        return fmt.Errorf("no data to aggregate")
    }
    
    // 检查时间戳顺序
    for i := 1; i < len(ae.entries); i++ {
        if ae.entries[i].Timestamp.Before(ae.entries[i-1].Timestamp) {
            return fmt.Errorf("entries not sorted by timestamp")
        }
    }
    
    return nil
}
```

## 9. 部署清单

- [ ] 实现 `calculations/aggregation.go`
- [ ] 实现 `ui/aggregation_view.go`
- [ ] 实现 `ui/components/aggregation_table.go`
- [ ] 实现 `ui/components/usage_chart.go`
- [ ] 添加缓存机制
- [ ] 实现增量聚合
- [ ] 添加模式检测
- [ ] 实现导出功能
- [ ] 编写测试
- [ ] 性能优化
- [ ] 集成到主界面
- [ ] 更新文档

## 10. 未来增强

- 机器学习异常检测
- 预测性分析
- 自定义聚合维度
- 实时流式聚合
- 分布式聚合计算
- 自定义报表模板
- 定时报表邮件
- 数据钻取功能
- 与 BI 工具集成