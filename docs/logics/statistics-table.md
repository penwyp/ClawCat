# 统计表格功能开发计划

## 1. 功能概述

统计表格是 claudecat Dashboard 的核心组件之一，用于展示当前会话的实时统计数据。通过对比当前值和预测值，展示燃烧率指标，以及模型使用分布，为用户提供全面的数据洞察。表格需要具备响应式设计，能够适应不同的终端窗口大小。

### 1.1 核心功能

- **当前值 vs 预测值对比**: 展示实时数据和基于当前速率的预测
- **燃烧率指标**: tokens/分钟、成本/小时等速率指标
- **模型分布**: 各模型使用占比的可视化展示
- **响应式布局**: 自动适应不同终端尺寸

## 2. 技术设计

### 2.1 数据结构

```go
// StatisticsTable 统计表格组件
type StatisticsTable struct {
    metrics     *calculations.RealtimeMetrics
    stats       TableStatistics
    width       int
    height      int
    styles      Styles
    layout      TableLayout
}

// TableStatistics 表格统计数据
type TableStatistics struct {
    // 当前值
    CurrentTokens       int
    CurrentCost         float64
    CurrentMessages     int
    CurrentDuration     time.Duration
    
    // 预测值
    ProjectedTokens     int
    ProjectedCost       float64
    ProjectedMessages   int
    ConfidenceLevel     float64
    
    // 速率指标
    TokensPerMinute     float64
    TokensPerHour       float64
    CostPerMinute       float64
    CostPerHour         float64
    MessagesPerHour     float64
    
    // 模型分布
    ModelDistribution   []ModelUsage
}

// ModelUsage 模型使用情况
type ModelUsage struct {
    Model       string
    TokenCount  int
    Percentage  float64
    Cost        float64
    Color       lipgloss.Color
}

// TableLayout 表格布局配置
type TableLayout struct {
    ShowHeaders     bool
    ShowBorders     bool
    ColumnWidths    []int
    Alignment       []Alignment
    CompactMode     bool
}
```

### 2.2 表格渲染器设计

```go
// TableRenderer 表格渲染器接口
type TableRenderer interface {
    Render() string
    SetData(stats TableStatistics)
    SetWidth(width int)
    SetLayout(layout TableLayout)
}

// ResponsiveTable 响应式表格
type ResponsiveTable struct {
    baseWidth   int
    minWidth    int
    columns     []Column
    rows        []Row
    styles      TableStyles
}

// Column 表格列定义
type Column struct {
    Key         string
    Title       string
    Width       int
    MinWidth    int
    Priority    int // 用于响应式隐藏
    Formatter   func(interface{}) string
    Alignment   Alignment
}

// Row 表格行
type Row struct {
    Cells       []Cell
    IsHeader    bool
    IsSeparator bool
    Style       lipgloss.Style
}

// Cell 表格单元格
type Cell struct {
    Content     string
    Style       lipgloss.Style
    Colspan     int
}
```

## 3. 实现步骤

### 3.1 创建统计表格组件

**文件**: `ui/components/statistics_table.go`

```go
package components

import (
    "fmt"
    "strings"
    "github.com/charmbracelet/lipgloss"
    "github.com/penwyp/claudecat/calculations"
)

// NewStatisticsTable 创建统计表格
func NewStatisticsTable(width int) *StatisticsTable {
    return &StatisticsTable{
        width:  width,
        styles: NewStyles(DefaultTheme()),
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
        Alignment:    []Alignment{AlignLeft, AlignRight, AlignRight, AlignRight},
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
        ProjectedTokens:  metrics.ProjectedTokens,
        ProjectedCost:    metrics.ProjectedCost,
        ConfidenceLevel:  metrics.ConfidenceLevel,
        
        // 速率指标
        TokensPerMinute:  metrics.TokensPerMinute,
        TokensPerHour:    metrics.TokensPerHour,
        CostPerMinute:    metrics.CostPerMinute,
        CostPerHour:      metrics.CostPerHour,
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
    
    tables = append(tables, modelTable)
    
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
    
    builder.WriteString(st.styles.Subtitle.Render("⚡ Burn Rate Metrics\n"))
    
    // 使用两列布局
    leftCol := []string{
        fmt.Sprintf("Tokens/min: %.1f", st.stats.TokensPerMinute),
        fmt.Sprintf("Tokens/hr:  %.0f", st.stats.TokensPerHour),
    }
    
    rightCol := []string{
        fmt.Sprintf("Cost/min: $%.3f", st.stats.CostPerMinute),
        fmt.Sprintf("Cost/hr:  $%.2f", st.stats.CostPerHour),
    }
    
    // 应用样式
    for i := range leftCol {
        left := st.styles.Normal.Render(leftCol[i])
        right := st.styles.Normal.Render(rightCol[i])
        
        // 如果燃烧率过高，使用警告颜色
        if st.stats.TokensPerMinute > 200 {
            left = st.styles.Warning.Render(leftCol[i])
        }
        if st.stats.CostPerHour > 5.0 {
            right = st.styles.Warning.Render(rightCol[i])
        }
        
        builder.WriteString(fmt.Sprintf("%-30s %s\n", left, right))
    }
    
    return st.styles.Faint.Box.Render(builder.String())
}

// renderModelDistribution 渲染模型分布
func (st *StatisticsTable) renderModelDistribution() string {
    if len(st.stats.ModelDistribution) == 0 {
        return ""
    }
    
    builder := strings.Builder{}
    builder.WriteString(st.styles.Subtitle.Render("🤖 Model Distribution\n"))
    
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
        bar := strings.Repeat("█", barWidth) + strings.Repeat("░", maxBarWidth-barWidth)
        
        // 应用颜色
        coloredBar := lipgloss.NewStyle().
            Foreground(model.Color).
            Render(bar)
        
        // 添加 token 数量
        stats := fmt.Sprintf(" %s tokens", formatNumber(model.TokenCount))
        
        builder.WriteString(fmt.Sprintf("%s\n%s%s\n\n", 
            label, coloredBar, 
            st.styles.Faint.Render(stats),
        ))
    }
    
    return builder.String()
}
```

### 3.2 响应式表格实现

**文件**: `ui/components/responsive_table.go`

```go
package components

import (
    "fmt"
    "strings"
    "github.com/charmbracelet/lipgloss"
)

// ResponsiveTable 响应式表格实现
type ResponsiveTable struct {
    width       int
    columns     []Column
    rows        [][]interface{}
    styles      TableStyles
    visibleCols []int
}

// TableStyles 表格样式
type TableStyles struct {
    Header      lipgloss.Style
    Cell        lipgloss.Style
    Border      lipgloss.Style
    Separator   string
}

// NewResponsiveTable 创建响应式表格
func NewResponsiveTable(width int) *ResponsiveTable {
    return &ResponsiveTable{
        width:  width,
        styles: DefaultTableStyles(),
        rows:   make([][]interface{}, 0),
    }
}

// DefaultTableStyles 默认表格样式
func DefaultTableStyles() TableStyles {
    return TableStyles{
        Header:    lipgloss.NewStyle().Bold(true).Underline(true),
        Cell:      lipgloss.NewStyle(),
        Border:    lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
        Separator: "│",
    }
}

// SetColumns 设置表格列
func (rt *ResponsiveTable) SetColumns(columns []Column) {
    rt.columns = columns
    rt.calculateVisibleColumns()
}

// AddRow 添加数据行
func (rt *ResponsiveTable) AddRow(data []interface{}) {
    rt.rows = append(rt.rows, data)
}

// calculateVisibleColumns 计算可见列
func (rt *ResponsiveTable) calculateVisibleColumns() {
    rt.visibleCols = []int{}
    usedWidth := 0
    
    // 按优先级排序列
    sorted := make([]int, len(rt.columns))
    for i := range sorted {
        sorted[i] = i
    }
    
    // 优先显示高优先级列
    for _, idx := range sorted {
        col := rt.columns[idx]
        if usedWidth+col.MinWidth+3 <= rt.width { // +3 for separators
            rt.visibleCols = append(rt.visibleCols, idx)
            usedWidth += col.MinWidth + 3
        }
    }
}

// Render 渲染表格
func (rt *ResponsiveTable) Render() string {
    if len(rt.rows) == 0 {
        return "No data available"
    }
    
    // 计算列宽
    colWidths := rt.calculateColumnWidths()
    
    var lines []string
    
    // 渲染表头
    if len(rt.columns) > 0 {
        header := rt.renderHeader(colWidths)
        lines = append(lines, header)
        lines = append(lines, rt.renderSeparator(colWidths))
    }
    
    // 渲染数据行
    for _, row := range rt.rows {
        lines = append(lines, rt.renderRow(row, colWidths))
    }
    
    return strings.Join(lines, "\n")
}

// calculateColumnWidths 计算实际列宽
func (rt *ResponsiveTable) calculateColumnWidths() []int {
    widths := make([]int, len(rt.visibleCols))
    totalMinWidth := 0
    
    // 计算最小宽度总和
    for i, colIdx := range rt.visibleCols {
        widths[i] = rt.columns[colIdx].MinWidth
        totalMinWidth += widths[i]
    }
    
    // 分配剩余空间
    remainingSpace := rt.width - totalMinWidth - len(rt.visibleCols)*3
    if remainingSpace > 0 {
        // 平均分配剩余空间
        extra := remainingSpace / len(rt.visibleCols)
        for i := range widths {
            widths[i] += extra
        }
    }
    
    return widths
}

// renderHeader 渲染表头
func (rt *ResponsiveTable) renderHeader(widths []int) string {
    parts := []string{}
    
    for i, colIdx := range rt.visibleCols {
        col := rt.columns[colIdx]
        content := truncateString(col.Title, widths[i])
        aligned := rt.alignText(content, widths[i], col.Alignment)
        parts = append(parts, rt.styles.Header.Render(aligned))
    }
    
    return rt.styles.Border.Render(rt.styles.Separator) + 
           strings.Join(parts, rt.styles.Border.Render(rt.styles.Separator)) + 
           rt.styles.Border.Render(rt.styles.Separator)
}

// renderRow 渲染数据行
func (rt *ResponsiveTable) renderRow(row []interface{}, widths []int) string {
    parts := []string{}
    
    for i, colIdx := range rt.visibleCols {
        if colIdx < len(row) {
            content := fmt.Sprintf("%v", row[colIdx])
            content = truncateString(content, widths[i])
            
            // 应用对齐
            col := rt.columns[colIdx]
            aligned := rt.alignText(content, widths[i], col.Alignment)
            
            // 应用样式
            styled := rt.styles.Cell.Render(aligned)
            parts = append(parts, styled)
        } else {
            // 空单元格
            parts = append(parts, strings.Repeat(" ", widths[i]))
        }
    }
    
    return rt.styles.Border.Render(rt.styles.Separator) + 
           strings.Join(parts, rt.styles.Border.Render(rt.styles.Separator)) + 
           rt.styles.Border.Render(rt.styles.Separator)
}

// renderSeparator 渲染分隔线
func (rt *ResponsiveTable) renderSeparator(widths []int) string {
    parts := []string{}
    
    for _, width := range widths {
        parts = append(parts, strings.Repeat("─", width))
    }
    
    return rt.styles.Border.Render("├") + 
           strings.Join(parts, rt.styles.Border.Render("┼")) + 
           rt.styles.Border.Render("┤")
}

// alignText 文本对齐
func (rt *ResponsiveTable) alignText(text string, width int, align Alignment) string {
    padding := width - len(text)
    if padding <= 0 {
        return text
    }
    
    switch align {
    case AlignRight:
        return strings.Repeat(" ", padding) + text
    case AlignCenter:
        left := padding / 2
        right := padding - left
        return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
    default: // AlignLeft
        return text + strings.Repeat(" ", padding)
    }
}
```

### 3.3 数据格式化工具

**文件**: `ui/components/formatters.go`

```go
package components

import (
    "fmt"
    "time"
    "github.com/charmbracelet/lipgloss"
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

// formatDuration 格式化时长
func formatDuration(d time.Duration) string {
    hours := int(d.Hours())
    minutes := int(d.Minutes()) % 60
    
    if hours > 0 {
        return fmt.Sprintf("%dh %dm", hours, minutes)
    }
    return fmt.Sprintf("%dm", minutes)
}

// formatChange 格式化变化值
func (st *StatisticsTable) formatChange(current, projected int) string {
    if projected == current {
        return "—"
    }
    
    change := projected - current
    percentage := float64(change) / float64(current) * 100
    
    arrow := "↑"
    style := st.styles.Success
    if change < 0 {
        arrow = "↓"
        style = st.styles.Error
    }
    
    return style.Render(fmt.Sprintf("%s %.0f%%", arrow, percentage))
}

// formatCostChange 格式化成本变化
func (st *StatisticsTable) formatCostChange(current, projected float64) string {
    if projected == current {
        return "—"
    }
    
    change := projected - current
    
    style := st.styles.Warning
    if change > 5.0 { // 超过 $5 增长
        style = st.styles.Error
    }
    
    return style.Render(fmt.Sprintf("+$%.2f", change))
}

// formatTimeRemaining 格式化剩余时间
func (st *StatisticsTable) formatTimeRemaining() string {
    remaining := 5*time.Hour - st.stats.CurrentDuration
    
    if remaining <= 0 {
        return st.styles.Error.Render("Expired")
    }
    
    style := st.styles.Normal
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
```

## 4. 测试计划

### 4.1 单元测试

```go
// ui/components/statistics_table_test.go

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
                CurrentTokens:   50000,
                ProjectedTokens: 75000,
                CurrentCost:     10.50,
                ProjectedCost:   15.75,
                TokensPerMinute: 100,
                CostPerHour:     3.60,
            },
            validate: func(t *testing.T, output string) {
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
                CurrentTokens: 25000,
                CurrentCost:   5.25,
            },
            validate: func(t *testing.T, output string) {
                // 在紧凑模式下，某些列可能被隐藏
                assert.Contains(t, output, "25.0K")
                assert.Contains(t, output, "$5.25")
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            table := NewStatisticsTable(tt.width)
            table.Update(tt.metrics)
            
            output := table.Render()
            tt.validate(t, output)
        })
    }
}

func TestResponsiveTable_ColumnVisibility(t *testing.T) {
    table := NewResponsiveTable(50)
    
    columns := []Column{
        {Key: "a", Title: "A", MinWidth: 10, Priority: 1},
        {Key: "b", Title: "B", MinWidth: 10, Priority: 2},
        {Key: "c", Title: "C", MinWidth: 10, Priority: 3},
        {Key: "d", Title: "D", MinWidth: 10, Priority: 4},
    }
    
    table.SetColumns(columns)
    
    // 在宽度 50 的情况下，应该只能显示部分列
    assert.Less(t, len(table.visibleCols), len(columns))
    
    // 高优先级列应该被显示
    assert.Contains(t, table.visibleCols, 0) // Priority 1
    assert.Contains(t, table.visibleCols, 1) // Priority 2
}
```

### 4.2 视觉测试

```go
// ui/components/statistics_visual_test.go

func TestStatisticsTable_VisualOutput(t *testing.T) {
    // 创建测试数据
    metrics := &calculations.RealtimeMetrics{
        SessionStart:    time.Now().Add(-2 * time.Hour),
        CurrentTokens:   125000,
        ProjectedTokens: 187500,
        CurrentCost:     21.50,
        ProjectedCost:   32.25,
        TokensPerMinute: 156.25,
        TokensPerHour:   9375,
        CostPerMinute:   0.027,
        CostPerHour:     1.62,
        ModelDistribution: map[string]calculations.ModelMetrics{
            "claude-3-opus": {
                TokenCount: 87500,
                Cost:       15.05,
            },
            "claude-3-sonnet": {
                TokenCount: 37500,
                Cost:       6.45,
            },
        },
    }
    
    // 测试不同宽度
    widths := []int{60, 80, 100, 120}
    
    for _, width := range widths {
        t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
            table := NewStatisticsTable(width)
            table.Update(metrics)
            
            output := table.Render()
            
            t.Logf("\n=== Width: %d ===\n%s\n", width, output)
            
            // 基本验证
            assert.NotEmpty(t, output)
            assert.Contains(t, output, "Statistics Overview")
        })
    }
}
```

### 4.3 性能测试

```go
// ui/components/statistics_bench_test.go

func BenchmarkStatisticsTable_Render(b *testing.B) {
    table := NewStatisticsTable(100)
    metrics := generateComplexMetrics()
    table.Update(metrics)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = table.Render()
    }
}

func BenchmarkResponsiveTable_LargeDataSet(b *testing.B) {
    table := NewResponsiveTable(120)
    
    // 设置多列
    columns := make([]Column, 10)
    for i := range columns {
        columns[i] = Column{
            Key:      fmt.Sprintf("col%d", i),
            Title:    fmt.Sprintf("Column %d", i),
            MinWidth: 10,
            Priority: i + 1,
        }
    }
    table.SetColumns(columns)
    
    // 添加多行数据
    for i := 0; i < 100; i++ {
        row := make([]interface{}, 10)
        for j := range row {
            row[j] = fmt.Sprintf("Cell %d-%d", i, j)
        }
        table.AddRow(row)
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = table.Render()
    }
}
```

## 5. 高级功能

### 5.1 交互式表格

```go
// InteractiveTable 支持交互的表格
type InteractiveTable struct {
    *StatisticsTable
    selectedRow    int
    selectedCol    int
    sortColumn     int
    sortAscending  bool
}

// HandleKeyPress 处理键盘输入
func (it *InteractiveTable) HandleKeyPress(key tea.KeyMsg) tea.Cmd {
    switch key.String() {
    case "up", "k":
        it.selectedRow--
    case "down", "j":
        it.selectedRow++
    case "left", "h":
        it.selectedCol--
    case "right", "l":
        it.selectedCol++
    case "s":
        it.toggleSort()
    case "enter":
        return it.handleSelection()
    }
    
    it.clampSelection()
    return nil
}

// toggleSort 切换排序
func (it *InteractiveTable) toggleSort() {
    if it.sortColumn == it.selectedCol {
        it.sortAscending = !it.sortAscending
    } else {
        it.sortColumn = it.selectedCol
        it.sortAscending = true
    }
    
    it.sortData()
}
```

### 5.2 数据导出

```go
// ExportFormat 导出格式
type ExportFormat string

const (
    ExportCSV      ExportFormat = "csv"
    ExportJSON     ExportFormat = "json"
    ExportMarkdown ExportFormat = "markdown"
)

// Export 导出表格数据
func (st *StatisticsTable) Export(format ExportFormat) (string, error) {
    switch format {
    case ExportCSV:
        return st.exportCSV()
    case ExportJSON:
        return st.exportJSON()
    case ExportMarkdown:
        return st.exportMarkdown()
    default:
        return "", fmt.Errorf("unsupported format: %s", format)
    }
}

// exportCSV 导出为 CSV
func (st *StatisticsTable) exportCSV() (string, error) {
    var buf bytes.Buffer
    writer := csv.NewWriter(&buf)
    
    // 写入表头
    headers := []string{"Metric", "Current", "Projected", "Change"}
    if err := writer.Write(headers); err != nil {
        return "", err
    }
    
    // 写入数据
    rows := [][]string{
        {"Tokens", 
         fmt.Sprintf("%d", st.stats.CurrentTokens),
         fmt.Sprintf("%d", st.stats.ProjectedTokens),
         fmt.Sprintf("%.1f%%", calculateChangePercent(st.stats.CurrentTokens, st.stats.ProjectedTokens)),
        },
        {"Cost",
         fmt.Sprintf("%.2f", st.stats.CurrentCost),
         fmt.Sprintf("%.2f", st.stats.ProjectedCost),
         fmt.Sprintf("%.1f%%", calculateChangePercent(int(st.stats.CurrentCost*100), int(st.stats.ProjectedCost*100))),
        },
    }
    
    for _, row := range rows {
        if err := writer.Write(row); err != nil {
            return "", err
        }
    }
    
    writer.Flush()
    return buf.String(), writer.Error()
}

// exportMarkdown 导出为 Markdown
func (st *StatisticsTable) exportMarkdown() (string, error) {
    var buf bytes.Buffer
    
    fmt.Fprintf(&buf, "# Statistics Report\n\n")
    fmt.Fprintf(&buf, "Generated: %s\n\n", time.Now().Format(time.RFC3339))
    
    // 主要统计表格
    fmt.Fprintf(&buf, "## Main Statistics\n\n")
    fmt.Fprintf(&buf, "| Metric | Current | Projected | Change |\n")
    fmt.Fprintf(&buf, "|--------|---------|-----------|--------|\n")
    fmt.Fprintf(&buf, "| Tokens | %s | %s | %s |\n",
        formatNumber(st.stats.CurrentTokens),
        formatNumber(st.stats.ProjectedTokens),
        st.formatChange(st.stats.CurrentTokens, st.stats.ProjectedTokens),
    )
    fmt.Fprintf(&buf, "| Cost | $%.2f | $%.2f | %s |\n",
        st.stats.CurrentCost,
        st.stats.ProjectedCost,
        st.formatCostChange(st.stats.CurrentCost, st.stats.ProjectedCost),
    )
    
    // 燃烧率
    fmt.Fprintf(&buf, "\n## Burn Rate\n\n")
    fmt.Fprintf(&buf, "- Tokens per minute: %.1f\n", st.stats.TokensPerMinute)
    fmt.Fprintf(&buf, "- Tokens per hour: %.0f\n", st.stats.TokensPerHour)
    fmt.Fprintf(&buf, "- Cost per hour: $%.2f\n", st.stats.CostPerHour)
    
    // 模型分布
    if len(st.stats.ModelDistribution) > 0 {
        fmt.Fprintf(&buf, "\n## Model Distribution\n\n")
        fmt.Fprintf(&buf, "| Model | Tokens | Percentage | Cost |\n")
        fmt.Fprintf(&buf, "|-------|--------|------------|------|\n")
        
        for _, model := range st.stats.ModelDistribution {
            fmt.Fprintf(&buf, "| %s | %s | %.1f%% | $%.2f |\n",
                model.Model,
                formatNumber(model.TokenCount),
                model.Percentage,
                model.Cost,
            )
        }
    }
    
    return buf.String(), nil
}
```

## 6. 样式和主题

### 6.1 表格主题

```go
// TableTheme 表格主题
type TableTheme struct {
    HeaderStyle     lipgloss.Style
    CellStyle       lipgloss.Style
    BorderStyle     lipgloss.Style
    HighlightStyle  lipgloss.Style
    WarningStyle    lipgloss.Style
    ErrorStyle      lipgloss.Style
    BorderChars     BorderCharSet
}

// BorderCharSet 边框字符集
type BorderCharSet struct {
    Top         string
    Bottom      string
    Left        string
    Right       string
    TopLeft     string
    TopRight    string
    BottomLeft  string
    BottomRight string
    Cross       string
    Horizontal  string
    Vertical    string
}

// 预定义主题
var (
    DefaultTableTheme = TableTheme{
        HeaderStyle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")),
        CellStyle:    lipgloss.NewStyle(),
        BorderStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
        BorderChars:  ASCIIBorderChars,
    }
    
    MinimalTableTheme = TableTheme{
        HeaderStyle:  lipgloss.NewStyle().Bold(true),
        CellStyle:    lipgloss.NewStyle(),
        BorderStyle:  lipgloss.NewStyle().Faint(true),
        BorderChars:  MinimalBorderChars,
    }
    
    ASCIIBorderChars = BorderCharSet{
        Top:         "-",
        Bottom:      "-",
        Left:        "|",
        Right:       "|",
        TopLeft:     "+",
        TopRight:    "+",
        BottomLeft:  "+",
        BottomRight: "+",
        Cross:       "+",
        Horizontal:  "-",
        Vertical:    "|",
    }
    
    UnicodeBorderChars = BorderCharSet{
        Top:         "─",
        Bottom:      "─",
        Left:        "│",
        Right:       "│",
        TopLeft:     "┌",
        TopRight:    "┐",
        BottomLeft:  "└",
        BottomRight: "┘",
        Cross:       "┼",
        Horizontal:  "─",
        Vertical:    "│",
    }
)
```

## 7. 配置选项

### 7.1 表格配置

```yaml
# config.yaml
ui:
  statistics:
    show_current_values: true
    show_projected_values: true
    show_burn_rate: true
    show_model_distribution: true
    table_style: "unicode"  # ascii, unicode, minimal
    compact_mode: false
    update_interval: 1s
    decimal_places:
      tokens: 0
      cost: 2
      percentage: 1
```

### 7.2 配置应用

```go
func (st *StatisticsTable) ApplyConfig(config TableConfig) {
    if config.CompactMode {
        st.layout.CompactMode = true
    }
    
    if config.TableStyle == "ascii" {
        st.styles.BorderChars = ASCIIBorderChars
    }
    
    // 应用小数位数设置
    st.formatters = Formatters{
        TokenDecimals:      config.DecimalPlaces.Tokens,
        CostDecimals:       config.DecimalPlaces.Cost,
        PercentageDecimals: config.DecimalPlaces.Percentage,
    }
}
```

## 8. 错误处理

### 8.1 数据验证

```go
func (st *StatisticsTable) validateData() error {
    if st.stats.CurrentTokens < 0 {
        return fmt.Errorf("negative token count: %d", st.stats.CurrentTokens)
    }
    
    if st.stats.CurrentCost < 0 {
        return fmt.Errorf("negative cost: %.2f", st.stats.CurrentCost)
    }
    
    if st.stats.ProjectedTokens < st.stats.CurrentTokens {
        // 预测值不应小于当前值（在正常情况下）
        st.logger.Warn("Projected tokens less than current tokens")
    }
    
    return nil
}
```

### 8.2 渲染降级

```go
func (st *StatisticsTable) Render() string {
    defer func() {
        if r := recover(); r != nil {
            st.logger.Error("Table render panic", "error", r)
            // 返回简化版本
            return st.renderSimplified()
        }
    }()
    
    if err := st.validateData(); err != nil {
        return st.renderError(err)
    }
    
    return st.renderNormal()
}

func (st *StatisticsTable) renderSimplified() string {
    return fmt.Sprintf(
        "Tokens: %d (projected: %d)\n"+
        "Cost: $%.2f (projected: $%.2f)\n"+
        "Burn rate: %.1f tokens/min",
        st.stats.CurrentTokens,
        st.stats.ProjectedTokens,
        st.stats.CurrentCost,
        st.stats.ProjectedCost,
        st.stats.TokensPerMinute,
    )
}
```

## 9. 部署清单

- [ ] 实现 `ui/components/statistics_table.go`
- [ ] 实现 `ui/components/responsive_table.go`
- [ ] 实现 `ui/components/formatters.go`
- [ ] 添加表格主题系统
- [ ] 集成到 Dashboard
- [ ] 编写单元测试
- [ ] 编写视觉测试
- [ ] 性能优化
- [ ] 添加交互功能（可选）
- [ ] 实现数据导出
- [ ] 更新配置选项
- [ ] 编写用户文档

## 10. 未来增强

- 实时数据图表集成
- 历史数据对比视图
- 自定义指标定义
- 数据钻取功能
- 警报阈值配置
- 表格模板系统
- 与外部监控系统集成
- 移动端适配优化