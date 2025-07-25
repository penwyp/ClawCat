package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Alignment 文本对齐方式
type Alignment int

const (
	AlignLeft Alignment = iota
	AlignCenter
	AlignRight
)

// Column 表格列定义
type Column struct {
	Key       string
	Title     string
	Width     int
	MinWidth  int
	Priority  int // 用于响应式隐藏
	Alignment Alignment
}

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
	Header    lipgloss.Style
	Cell      lipgloss.Style
	Border    lipgloss.Style
	Separator string
	Highlight lipgloss.Style
	Faint     lipgloss.Style
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
		Header:    lipgloss.NewStyle().Bold(true).Underline(true).Foreground(lipgloss.Color("#8B5CF6")),
		Cell:      lipgloss.NewStyle(),
		Border:    lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")),
		Separator: "│",
		Highlight: lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("15")),
		Faint:     lipgloss.NewStyle().Faint(true),
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

	// 按优先级排序列索引
	indices := make([]int, len(rt.columns))
	for i := range indices {
		indices[i] = i
	}

	// 简单排序：优先级高的在前
	for i := 0; i < len(indices)-1; i++ {
		for j := i + 1; j < len(indices); j++ {
			if rt.columns[indices[i]].Priority > rt.columns[indices[j]].Priority {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	// 优先显示高优先级列
	for _, idx := range indices {
		col := rt.columns[idx]
		columnWidth := col.MinWidth
		if columnWidth == 0 {
			columnWidth = 10 // 默认最小宽度
		}

		// +3 为分隔符留出空间
		if usedWidth+columnWidth+3 <= rt.width {
			rt.visibleCols = append(rt.visibleCols, idx)
			usedWidth += columnWidth + 3
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
		minWidth := rt.columns[colIdx].MinWidth
		if minWidth == 0 {
			minWidth = 10
		}
		widths[i] = minWidth
		totalMinWidth += widths[i]
	}

	// 分配剩余空间
	separatorSpace := len(rt.visibleCols) * 3 // 每列3个字符的分隔符空间
	remainingSpace := rt.width - totalMinWidth - separatorSpace
	if remainingSpace > 0 && len(rt.visibleCols) > 0 {
		// 平均分配剩余空间
		extra := remainingSpace / len(rt.visibleCols)
		remainder := remainingSpace % len(rt.visibleCols)

		for i := range widths {
			widths[i] += extra
			if i < remainder {
				widths[i]++ // 分配余数
			}
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
		content := ""
		if colIdx < len(row) && row[colIdx] != nil {
			content = fmt.Sprintf("%v", row[colIdx])
		}

		content = truncateString(content, widths[i])

		// 应用对齐
		col := rt.columns[colIdx]
		aligned := rt.alignText(content, widths[i], col.Alignment)

		// 应用样式
		styled := rt.styles.Cell.Render(aligned)
		parts = append(parts, styled)
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
	textLen := len(text)
	if textLen >= width {
		return text
	}

	padding := width - textLen

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

// SetWidth 设置表格宽度
func (rt *ResponsiveTable) SetWidth(width int) {
	rt.width = width
	rt.calculateVisibleColumns()
}

// GetVisibleColumns 获取可见列数量
func (rt *ResponsiveTable) GetVisibleColumns() int {
	return len(rt.visibleCols)
}

// Clear 清空表格数据
func (rt *ResponsiveTable) Clear() {
	rt.rows = make([][]interface{}, 0)
}

// SetStyles 设置表格样式
func (rt *ResponsiveTable) SetStyles(styles TableStyles) {
	rt.styles = styles
}
