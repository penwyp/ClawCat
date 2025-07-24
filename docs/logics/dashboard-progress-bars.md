# Dashboard 进度条组件开发计划

## 1. 功能概述

Dashboard 进度条组件是 ClawCat 的核心可视化组件，用于直观展示用户的资源使用情况。通过三个主要进度条（Token 使用、成本使用、时间进度）和动态颜色编码，让用户一目了然地了解当前会话的资源消耗状态。

### 1.1 核心组件

- **Token 使用进度条**: 显示当前 token 使用量与限额的比例
- **成本使用进度条**: 显示当前成本与计划限额的比例  
- **时间进度条**: 显示 5 小时会话窗口的时间进度
- **动态颜色系统**: 根据使用率自动调整颜色（绿→黄→橙→红）

## 2. 技术设计

### 2.1 组件架构

```go
// ProgressBar 进度条基础组件
type ProgressBar struct {
    Label       string      // 标签文本
    Current     float64     // 当前值
    Max         float64     // 最大值
    Percentage  float64     // 百分比（0-100）
    Width       int         // 进度条宽度
    ShowValue   bool        // 是否显示数值
    ShowPercent bool        // 是否显示百分比
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

// ProgressSection 进度条区域组件
type ProgressSection struct {
    TokenProgress   *ProgressBar
    CostProgress    *ProgressBar
    TimeProgress    *ProgressBar
    styles          Styles
    width           int
    height          int
}
```

### 2.2 颜色系统设计

```go
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
```

## 3. 实现步骤

### 3.1 创建进度条基础组件

**文件**: `ui/components/progress_bar.go`

```go
package components

import (
    "fmt"
    "strings"
    "github.com/charmbracelet/lipgloss"
)

// NewProgressBar 创建新的进度条
func NewProgressBar(label string, current, max float64) *ProgressBar {
    percentage := 0.0
    if max > 0 {
        percentage = (current / max) * 100
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
    emptyLength := pb.Width - fillLength
    
    // 构建进度条
    bar := fmt.Sprintf("%s%s%s%s",
        pb.Style.BarBracketStart,
        strings.Repeat(pb.Style.BarChar, fillLength),
        strings.Repeat(pb.Style.EmptyChar, emptyLength),
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
        pb.Percentage = (current / pb.Max) * 100
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
```

### 3.2 创建进度条区域组件

**文件**: `ui/components/progress_section.go`

```go
package components

import (
    "fmt"
    "strings"
    "time"
    "github.com/charmbracelet/lipgloss"
    "github.com/penwyp/ClawCat/calculations"
)

// NewProgressSection 创建进度条区域
func NewProgressSection(width int) *ProgressSection {
    return &ProgressSection{
        width:  width,
        styles: NewStyles(DefaultTheme()),
    }
}

// Update 更新进度条数据
func (ps *ProgressSection) Update(metrics *calculations.RealtimeMetrics, limits Limits) {
    // 更新 Token 进度条
    ps.TokenProgress = ps.createTokenProgress(metrics, limits)
    
    // 更新成本进度条
    ps.CostProgress = ps.createCostProgress(metrics, limits)
    
    // 更新时间进度条
    ps.TimeProgress = ps.createTimeProgress(metrics)
}

// createTokenProgress 创建 Token 进度条
func (ps *ProgressSection) createTokenProgress(metrics *calculations.RealtimeMetrics, limits Limits) *ProgressBar {
    pb := NewProgressBar(
        "Token Usage",
        float64(metrics.CurrentTokens),
        float64(limits.TokenLimit),
    )
    
    // 设置动态颜色
    colorScheme := DefaultColorScheme
    pb.Color = colorScheme.GetProgressColor(pb.Percentage)
    
    // 自定义显示格式
    pb.Style.ValueStyle = lipgloss.NewStyle().Bold(true)
    
    // 根据屏幕宽度调整进度条宽度
    barWidth := ps.calculateBarWidth()
    pb.SetWidth(barWidth)
    
    return pb
}

// createCostProgress 创建成本进度条
func (ps *ProgressSection) createCostProgress(metrics *calculations.RealtimeMetrics, limits Limits) *ProgressBar {
    pb := NewProgressBar(
        "Cost Usage",
        metrics.CurrentCost,
        limits.CostLimit,
    )
    
    // 设置动态颜色
    colorScheme := DefaultColorScheme
    pb.Color = colorScheme.GetProgressColor(pb.Percentage)
    
    // 成本显示特殊格式
    pb.Style.ValueStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#FFD700")) // 金色
    
    barWidth := ps.calculateBarWidth()
    pb.SetWidth(barWidth)
    
    return pb
}

// createTimeProgress 创建时间进度条
func (ps *ProgressSection) createTimeProgress(metrics *calculations.RealtimeMetrics) *ProgressBar {
    elapsed := time.Since(metrics.SessionStart)
    total := 5 * time.Hour
    
    pb := NewProgressBar(
        "Time Elapsed",
        elapsed.Minutes(),
        total.Minutes(),
    )
    
    // 时间进度条使用渐变颜色
    pb.Color = ps.getTimeProgressColor(pb.Percentage)
    
    // 自定义时间显示
    pb.ShowValue = false // 使用自定义格式
    
    barWidth := ps.calculateBarWidth()
    pb.SetWidth(barWidth)
    
    return pb
}

// Render 渲染进度条区域
func (ps *ProgressSection) Render() string {
    if ps.width == 0 {
        return "Loading progress..."
    }
    
    // 区域标题
    title := ps.styles.SectionTitle.Render("📊 Progress Overview")
    
    // 渲染各个进度条
    var progressBars []string
    
    if ps.TokenProgress != nil {
        progressBars = append(progressBars, ps.renderProgressWithInfo(ps.TokenProgress))
    }
    
    if ps.CostProgress != nil {
        progressBars = append(progressBars, ps.renderProgressWithInfo(ps.CostProgress))
    }
    
    if ps.TimeProgress != nil {
        progressBars = append(progressBars, ps.renderTimeProgress())
    }
    
    // 组合所有元素
    content := strings.Join(append([]string{title}, progressBars...), "\n\n")
    
    // 添加边框
    return ps.styles.Box.
        Width(ps.width).
        Render(content)
}

// renderProgressWithInfo 渲染带附加信息的进度条
func (ps *ProgressSection) renderProgressWithInfo(pb *ProgressBar) string {
    progressBar := pb.Render()
    
    // 添加附加信息
    var info string
    switch pb.Label {
    case "Token Usage":
        if pb.Percentage > 90 {
            info = ps.styles.Error.Render("⚠️ Approaching token limit!")
        } else if pb.Percentage > 75 {
            info = ps.styles.Warning.Render("⚡ High token usage")
        }
    case "Cost Usage":
        if pb.Percentage > 90 {
            info = ps.styles.Error.Render("💸 Budget alert!")
        } else if pb.Percentage > 75 {
            info = ps.styles.Warning.Render("💰 Monitor spending")
        }
    }
    
    if info != "" {
        return progressBar + "\n" + info
    }
    return progressBar
}

// renderTimeProgress 渲染时间进度条（特殊格式）
func (ps *ProgressSection) renderTimeProgress() string {
    if ps.TimeProgress == nil {
        return ""
    }
    
    // 基础进度条
    bar := ps.TimeProgress.Render()
    
    // 添加时间显示
    elapsed := time.Duration(ps.TimeProgress.Current) * time.Minute
    remaining := time.Duration(ps.TimeProgress.Max-ps.TimeProgress.Current) * time.Minute
    
    timeInfo := fmt.Sprintf("%s elapsed, %s remaining",
        formatDuration(elapsed),
        formatDuration(remaining),
    )
    
    timeStyle := lipgloss.NewStyle().Faint(true)
    
    return bar + "\n" + timeStyle.Render(timeInfo)
}

// calculateBarWidth 计算进度条宽度
func (ps *ProgressSection) calculateBarWidth() int {
    // 预留空间给标签、数值和边距
    reservedSpace := 40
    availableWidth := ps.width - reservedSpace
    
    if availableWidth < 20 {
        return 20
    }
    if availableWidth > 60 {
        return 60
    }
    
    return availableWidth
}

// getTimeProgressColor 获取时间进度条颜色
func (ps *ProgressSection) getTimeProgressColor(percentage float64) lipgloss.Color {
    // 时间进度条使用蓝色系渐变
    if percentage < 25 {
        return lipgloss.Color("#00BFFF") // 深天蓝
    } else if percentage < 50 {
        return lipgloss.Color("#1E90FF") // 道奇蓝
    } else if percentage < 75 {
        return lipgloss.Color("#4169E1") // 皇家蓝
    } else {
        return lipgloss.Color("#6A5ACD") // 石板蓝
    }
}

// formatDuration 格式化时间显示
func formatDuration(d time.Duration) string {
    hours := int(d.Hours())
    minutes := int(d.Minutes()) % 60
    
    if hours > 0 {
        return fmt.Sprintf("%dh %dm", hours, minutes)
    }
    return fmt.Sprintf("%dm", minutes)
}
```

### 3.3 集成到 Dashboard

**文件**: `ui/dashboard_enhanced.go`

```go
package ui

import (
    "github.com/penwyp/ClawCat/ui/components"
    "github.com/penwyp/ClawCat/calculations"
)

// EnhancedDashboardView 增强的 Dashboard 视图
type EnhancedDashboardView struct {
    *DashboardView
    progressSection *components.ProgressSection
    metrics         *calculations.RealtimeMetrics
    limits          components.Limits
}

// NewEnhancedDashboardView 创建增强的 Dashboard
func NewEnhancedDashboardView(config Config) *EnhancedDashboardView {
    return &EnhancedDashboardView{
        DashboardView:   NewDashboardView(),
        progressSection: components.NewProgressSection(0),
        limits:          getLimitsFromConfig(config),
    }
}

// UpdateMetrics 更新指标和进度条
func (d *EnhancedDashboardView) UpdateMetrics(metrics *calculations.RealtimeMetrics) {
    d.metrics = metrics
    d.progressSection.Update(metrics, d.limits)
}

// View 渲染增强的 Dashboard
func (d *EnhancedDashboardView) View() string {
    if d.width == 0 || d.height == 0 {
        return "Dashboard loading..."
    }
    
    // 更新进度条宽度
    d.progressSection.width = d.width - 4
    
    // 渲染各个部分
    header := d.renderHeader()
    progress := d.progressSection.Render()
    metrics := d.renderMetrics()
    charts := d.renderCharts()
    footer := d.renderFooter()
    
    // 组合所有部分
    content := strings.Join([]string{
        header,
        progress,  // 新增进度条区域
        metrics,
        charts,
        footer,
    }, "\n\n")
    
    return d.styles.Content.
        Width(d.width - 4).
        Height(d.height - 4).
        Render(content)
}

// getLimitsFromConfig 从配置获取限制值
func getLimitsFromConfig(config Config) components.Limits {
    limits := components.Limits{
        CostLimit: 18.00, // 默认 Pro 计划
    }
    
    switch config.Subscription.Plan {
    case "pro":
        limits.CostLimit = 18.00
        limits.TokenLimit = 1000000 // 估算值
    case "max5":
        limits.CostLimit = 35.00
        limits.TokenLimit = 2000000
    case "max20":
        limits.CostLimit = 140.00
        limits.TokenLimit = 8000000
    case "custom":
        // 使用 P90 计算
        limits.CostLimit = config.Subscription.CustomLimit
        limits.TokenLimit = int(config.Subscription.CustomTokenLimit)
    }
    
    return limits
}
```

## 4. 测试计划

### 4.1 单元测试

```go
// ui/components/progress_bar_test.go

func TestProgressBar_Render(t *testing.T) {
    tests := []struct {
        name     string
        current  float64
        max      float64
        width    int
        expected string
    }{
        {
            name:    "50% progress",
            current: 50,
            max:     100,
            width:   20,
            expected: "contains ██████████░░░░░░░░░░",
        },
        {
            name:    "100% progress",
            current: 100,
            max:     100,
            width:   20,
            expected: "contains ████████████████████",
        },
        {
            name:    "0% progress",
            current: 0,
            max:     100,
            width:   20,
            expected: "contains ░░░░░░░░░░░░░░░░░░░░",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            pb := NewProgressBar("Test", tt.current, tt.max)
            pb.SetWidth(tt.width)
            
            result := pb.Render()
            
            assert.Contains(t, result, "Test")
            assert.Contains(t, result, fmt.Sprintf("%.1f%%", (tt.current/tt.max)*100))
        })
    }
}

func TestProgressColorScheme_GetProgressColor(t *testing.T) {
    scheme := DefaultColorScheme
    
    tests := []struct {
        percentage float64
        expected   lipgloss.Color
    }{
        {25, "#00ff00"},  // 绿色
        {60, "#ffff00"},  // 黄色
        {80, "#ff8800"},  // 橙色
        {95, "#ff0000"},  // 红色
    }
    
    for _, tt := range tests {
        t.Run(fmt.Sprintf("%.0f%%", tt.percentage), func(t *testing.T) {
            color := scheme.GetProgressColor(tt.percentage)
            assert.Equal(t, tt.expected, color)
        })
    }
}
```

### 4.2 视觉测试

```go
// ui/components/progress_visual_test.go

func TestProgressSection_VisualOutput(t *testing.T) {
    // 创建测试数据
    metrics := &calculations.RealtimeMetrics{
        SessionStart:    time.Now().Add(-2 * time.Hour),
        CurrentTokens:   75000,
        CurrentCost:     12.50,
        SessionProgress: 40.0,
    }
    
    limits := components.Limits{
        TokenLimit: 100000,
        CostLimit:  18.00,
    }
    
    // 创建进度条区域
    section := components.NewProgressSection(80)
    section.Update(metrics, limits)
    
    // 渲染并输出（用于视觉检查）
    output := section.Render()
    
    t.Log("\n" + output)
    
    // 验证输出包含关键元素
    assert.Contains(t, output, "Token Usage")
    assert.Contains(t, output, "Cost Usage")
    assert.Contains(t, output, "Time Elapsed")
    assert.Contains(t, output, "75.0%") // Token 使用率
    assert.Contains(t, output, "69.4%") // 成本使用率
}
```

### 4.3 性能测试

```go
// ui/components/progress_bench_test.go

func BenchmarkProgressBar_Render(b *testing.B) {
    pb := NewProgressBar("Benchmark", 50, 100)
    pb.SetWidth(40)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = pb.Render()
    }
}

func BenchmarkProgressSection_Update(b *testing.B) {
    section := components.NewProgressSection(100)
    metrics := generateTestMetrics()
    limits := components.Limits{TokenLimit: 100000, CostLimit: 18.00}
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        section.Update(metrics, limits)
    }
}
```

## 5. 样式和主题

### 5.1 主题支持

```go
// ui/themes/progress_themes.go

type ProgressTheme struct {
    ColorScheme     ProgressColorScheme
    BarChar         string
    EmptyChar       string
    BorderStyle     lipgloss.Style
    LabelStyle      lipgloss.Style
}

var ProgressThemes = map[string]ProgressTheme{
    "default": {
        ColorScheme: DefaultColorScheme,
        BarChar:     "█",
        EmptyChar:   "░",
    },
    "ascii": {
        ColorScheme: DefaultColorScheme,
        BarChar:     "#",
        EmptyChar:   "-",
    },
    "minimal": {
        ColorScheme: MinimalColorScheme,
        BarChar:     "=",
        EmptyChar:   " ",
    },
    "fancy": {
        ColorScheme: DefaultColorScheme,
        BarChar:     "▰",
        EmptyChar:   "▱",
    },
}
```

### 5.2 响应式设计

```go
// 自动调整进度条宽度
func (ps *ProgressSection) adaptToScreenSize() {
    if ps.width < 80 {
        // 小屏幕模式
        ps.useCompactLayout()
    } else if ps.width < 120 {
        // 中等屏幕
        ps.useNormalLayout()
    } else {
        // 大屏幕
        ps.useExpandedLayout()
    }
}

func (ps *ProgressSection) useCompactLayout() {
    // 隐藏数值，只显示百分比
    if ps.TokenProgress != nil {
        ps.TokenProgress.ShowValue = false
        ps.TokenProgress.SetWidth(20)
    }
    // ... 其他进度条类似处理
}
```

## 6. 动画效果

### 6.1 平滑过渡

```go
// SmoothProgressBar 支持平滑过渡的进度条
type SmoothProgressBar struct {
    *ProgressBar
    targetValue    float64
    animationSpeed float64
}

// AnimateToValue 平滑过渡到目标值
func (spb *SmoothProgressBar) AnimateToValue(target float64) tea.Cmd {
    spb.targetValue = target
    
    return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
        return AnimationTickMsg{
            ProgressBar: spb,
            Time:        t,
        }
    })
}

// Update 更新动画状态
func (spb *SmoothProgressBar) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case AnimationTickMsg:
        if msg.ProgressBar == spb {
            diff := spb.targetValue - spb.Current
            if math.Abs(diff) > 0.1 {
                // 继续动画
                spb.Current += diff * spb.animationSpeed
                return spb.AnimateToValue(spb.targetValue)
            }
            // 动画结束
            spb.Current = spb.targetValue
        }
    }
    return nil
}
```

## 7. 可访问性

### 7.1 屏幕阅读器支持

```go
// 为进度条添加 ARIA 标签
func (pb *ProgressBar) GetAccessibilityText() string {
    return fmt.Sprintf(
        "%s: %.1f%% complete, %s out of %s",
        pb.Label,
        pb.Percentage,
        formatAccessibleValue(pb.Current),
        formatAccessibleValue(pb.Max),
    )
}
```

### 7.2 键盘导航

```go
// 支持键盘焦点切换
func (ps *ProgressSection) HandleKeyPress(key tea.KeyMsg) tea.Cmd {
    switch key.String() {
    case "tab":
        ps.focusNext()
    case "shift+tab":
        ps.focusPrevious()
    case "?":
        return ps.showProgressHelp()
    }
    return nil
}
```

## 8. 配置选项

### 8.1 用户配置

```yaml
# config.yaml
ui:
  progress:
    show_token_progress: true
    show_cost_progress: true
    show_time_progress: true
    animation_enabled: true
    animation_speed: 0.15
    color_scheme: "default"
    compact_mode: false
    update_frequency: 100ms
```

### 8.2 配置应用

```go
func (ps *ProgressSection) ApplyConfig(config UIConfig) {
    if !config.Progress.ShowTokenProgress {
        ps.TokenProgress = nil
    }
    
    if config.Progress.CompactMode {
        ps.useCompactLayout()
    }
    
    if config.Progress.AnimationEnabled {
        ps.enableAnimations()
    }
}
```

## 9. 错误处理

### 9.1 边界情况

```go
func (pb *ProgressBar) safeDivision() float64 {
    if pb.Max == 0 {
        return 0
    }
    percentage := (pb.Current / pb.Max) * 100
    
    // 限制在 0-100 范围内
    if percentage < 0 {
        return 0
    }
    if percentage > 100 {
        return 100
    }
    
    return percentage
}
```

### 9.2 降级处理

```go
func (ps *ProgressSection) Render() string {
    defer func() {
        if r := recover(); r != nil {
            // 降级到简单文本输出
            return ps.renderFallback()
        }
    }()
    
    // 正常渲染逻辑
    return ps.renderNormal()
}

func (ps *ProgressSection) renderFallback() string {
    return fmt.Sprintf(
        "Token: %.0f%% | Cost: %.0f%% | Time: %.0f%%",
        ps.TokenProgress.Percentage,
        ps.CostProgress.Percentage,
        ps.TimeProgress.Percentage,
    )
}
```

## 10. 部署清单

- [ ] 实现 `ui/components/progress_bar.go`
- [ ] 实现 `ui/components/progress_section.go`
- [ ] 实现颜色系统和主题
- [ ] 集成到现有 Dashboard
- [ ] 编写单元测试
- [ ] 编写视觉测试
- [ ] 性能优化
- [ ] 添加动画效果（可选）
- [ ] 完善配置选项
- [ ] 更新文档
- [ ] 代码审查

## 11. 未来增强

- 自定义进度条样式
- 多段进度条（显示不同模型的使用）
- 历史进度对比
- 进度条工具提示
- 导出进度快照
- 声音/振动提醒（接近限额时）
- 进度条小组件（系统托盘）