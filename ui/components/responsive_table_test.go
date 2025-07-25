package components

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

	t.Logf("Visible columns: %v", table.visibleCols)
}

func TestResponsiveTable_Render(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		columns  []Column
		rows     [][]interface{}
		validate func(t *testing.T, output string)
	}{
		{
			name:  "basic table",
			width: 60,
			columns: []Column{
				{Key: "name", Title: "Name", MinWidth: 15, Priority: 1, Alignment: AlignLeft},
				{Key: "value", Title: "Value", MinWidth: 10, Priority: 2, Alignment: AlignRight},
				{Key: "status", Title: "Status", MinWidth: 10, Priority: 3, Alignment: AlignCenter},
			},
			rows: [][]interface{}{
				{"Alice", 100, "Active"},
				{"Bob", 200, "Inactive"},
				{"Charlie", 300, "Active"},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "Name")
				assert.Contains(t, output, "Value")
				assert.Contains(t, output, "Alice")
				assert.Contains(t, output, "Bob")
				assert.Contains(t, output, "Charlie")
				assert.Contains(t, output, "100")
				assert.Contains(t, output, "200")
				assert.Contains(t, output, "300")
			},
		},
		{
			name:  "narrow table",
			width: 30,
			columns: []Column{
				{Key: "name", Title: "Name", MinWidth: 10, Priority: 1},
				{Key: "value", Title: "Value", MinWidth: 8, Priority: 2},
				{Key: "extra", Title: "Extra", MinWidth: 10, Priority: 3},
			},
			rows: [][]interface{}{
				{"Test", 42, "Hidden"},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "Name")
				assert.Contains(t, output, "Test")
				// 在窄表格中，低优先级列可能被隐藏
			},
		},
		{
			name:    "empty table",
			width:   50,
			columns: []Column{{Key: "test", Title: "Test", MinWidth: 10, Priority: 1}},
			rows:    [][]interface{}{},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "No data available")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := NewResponsiveTable(tt.width)
			table.SetColumns(tt.columns)

			for _, row := range tt.rows {
				table.AddRow(row)
			}

			output := table.Render()
			t.Logf("\n=== %s (width: %d) ===\n%s\n", tt.name, tt.width, output)

			tt.validate(t, output)
		})
	}
}

func TestResponsiveTable_AlignText(t *testing.T) {
	table := NewResponsiveTable(50)

	tests := []struct {
		text      string
		width     int
		alignment Alignment
		expected  string
	}{
		{"Hello", 10, AlignLeft, "Hello     "},
		{"Hello", 10, AlignRight, "     Hello"},
		{"Hello", 10, AlignCenter, "  Hello   "},
		{"Hello", 5, AlignLeft, "Hello"}, // No padding needed
		{"Hello", 3, AlignLeft, "Hello"}, // Text longer than width
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%d_%d", tt.text, tt.width, tt.alignment), func(t *testing.T) {
			result := table.alignText(tt.text, tt.width, tt.alignment)
			assert.Equal(t, tt.expected, result)
			t.Logf("alignText('%s', %d, %d) = '%s'", tt.text, tt.width, tt.alignment, result)
		})
	}
}

func TestResponsiveTable_CalculateColumnWidths(t *testing.T) {
	tests := []struct {
		name         string
		tableWidth   int
		columns      []Column
		expectedCols int // 期望的可见列数
	}{
		{
			name:       "wide table",
			tableWidth: 100,
			columns: []Column{
				{Key: "a", Title: "A", MinWidth: 15, Priority: 1},
				{Key: "b", Title: "B", MinWidth: 15, Priority: 2},
				{Key: "c", Title: "C", MinWidth: 15, Priority: 3},
				{Key: "d", Title: "D", MinWidth: 15, Priority: 4},
			},
			expectedCols: 4, // 所有列都应该可见
		},
		{
			name:       "narrow table",
			tableWidth: 50,
			columns: []Column{
				{Key: "a", Title: "A", MinWidth: 15, Priority: 1},
				{Key: "b", Title: "B", MinWidth: 15, Priority: 2},
				{Key: "c", Title: "C", MinWidth: 15, Priority: 3},
				{Key: "d", Title: "D", MinWidth: 15, Priority: 4},
			},
			expectedCols: 2, // 只有高优先级列可见
		},
		{
			name:       "very narrow table",
			tableWidth: 25,
			columns: []Column{
				{Key: "a", Title: "A", MinWidth: 10, Priority: 1},
				{Key: "b", Title: "B", MinWidth: 10, Priority: 2},
				{Key: "c", Title: "C", MinWidth: 10, Priority: 3},
			},
			expectedCols: 1, // 只有最高优先级列可见
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := NewResponsiveTable(tt.tableWidth)
			table.SetColumns(tt.columns)

			assert.Equal(t, tt.expectedCols, len(table.visibleCols))
			t.Logf("Width: %d, Visible columns: %d/%d", tt.tableWidth, len(table.visibleCols), len(tt.columns))

			if len(table.visibleCols) > 0 {
				widths := table.calculateColumnWidths()
				assert.Len(t, widths, len(table.visibleCols))

				// 验证宽度分配合理
				totalWidth := 0
				for _, width := range widths {
					assert.Greater(t, width, 0)
					totalWidth += width
				}
				
				// 总宽度应该接近表格宽度（考虑分隔符）
				separatorSpace := len(table.visibleCols) * 3
				assert.LessOrEqual(t, totalWidth+separatorSpace, tt.tableWidth+5) // +5 为容差
			}
		})
	}
}

func TestResponsiveTable_SetWidth(t *testing.T) {
	table := NewResponsiveTable(100)

	columns := []Column{
		{Key: "a", Title: "A", MinWidth: 20, Priority: 1},
		{Key: "b", Title: "B", MinWidth: 20, Priority: 2},
		{Key: "c", Title: "C", MinWidth: 20, Priority: 3},
	}
	table.SetColumns(columns)

	// 初始状态：所有列可见
	assert.Equal(t, 3, len(table.visibleCols))

	// 缩小宽度
	table.SetWidth(50)
	assert.Less(t, len(table.visibleCols), 3)

	// 扩大宽度
	table.SetWidth(120)
	assert.Equal(t, 3, len(table.visibleCols))
}

func TestResponsiveTable_Clear(t *testing.T) {
	table := NewResponsiveTable(50)
	
	columns := []Column{
		{Key: "test", Title: "Test", MinWidth: 10, Priority: 1},
	}
	table.SetColumns(columns)

	// 添加数据
	table.AddRow([]interface{}{"row1"})
	table.AddRow([]interface{}{"row2"})
	assert.Len(t, table.rows, 2)

	// 清空数据
	table.Clear()
	assert.Len(t, table.rows, 0)

	// 清空后渲染应该显示无数据
	output := table.Render()
	assert.Contains(t, output, "No data available")
}

func TestResponsiveTable_TruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"Hello World", 15, "Hello World"},     // 不需要截断
		{"Hello World", 10, "Hello W..."},      // 需要截断
		{"Hello World", 5, "He..."},            // 短截断
		{"Hello World", 3, "Hel"},              // 极短截断
		{"Hi", 5, "Hi"},                        // 输入比限制短
		{"", 5, ""},                            // 空字符串
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%d", tt.input, tt.maxLen), func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
			assert.LessOrEqual(t, len(result), tt.maxLen)
		})
	}
}

func TestResponsiveTable_WithNilData(t *testing.T) {
	table := NewResponsiveTable(50)
	
	columns := []Column{
		{Key: "name", Title: "Name", MinWidth: 10, Priority: 1},
		{Key: "value", Title: "Value", MinWidth: 10, Priority: 2},
	}
	table.SetColumns(columns)

	// 添加包含 nil 值的行
	table.AddRow([]interface{}{"Alice", nil})
	table.AddRow([]interface{}{nil, 42})
	table.AddRow([]interface{}{"Bob", "test"})

	output := table.Render()
	t.Logf("\n=== Table with nil values ===\n%s\n", output)

	// 应该能正常渲染，不崩溃
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "Bob")
	assert.Contains(t, output, "42")
	assert.Contains(t, output, "test")
}

func TestResponsiveTable_EdgeCases(t *testing.T) {
	t.Run("zero width", func(t *testing.T) {
		table := NewResponsiveTable(0)
		columns := []Column{
			{Key: "test", Title: "Test", MinWidth: 10, Priority: 1},
		}
		table.SetColumns(columns)
		
		// 零宽度时不应该有可见列
		assert.Equal(t, 0, len(table.visibleCols))
	})

	t.Run("no columns", func(t *testing.T) {
		table := NewResponsiveTable(50)
		table.AddRow([]interface{}{"data"})
		
		output := table.Render()
		assert.Contains(t, output, "No data available")
	})

	t.Run("column without min width", func(t *testing.T) {
		table := NewResponsiveTable(50)
		columns := []Column{
			{Key: "test", Title: "Test", Priority: 1}, // MinWidth = 0
		}
		table.SetColumns(columns)
		
		// 应该使用默认最小宽度
		assert.Greater(t, len(table.visibleCols), 0)
	})
}