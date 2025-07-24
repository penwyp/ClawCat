# 订阅计划限制功能开发计划

## 1. 功能概述

订阅计划限制功能是 ClawCat 的核心安全机制，用于监控和控制用户的资源使用，防止超出订阅计划的限额。该功能包括多种订阅计划支持、实时限额监控、智能警告系统以及基于历史数据的 P90 自动限额计算。

### 1.1 核心功能

- **多计划支持**: Pro ($18)、Max5 ($35)、Max20 ($140)、Custom (P90)
- **实时监控**: 持续跟踪当前使用量与限额的关系
- **智能警告**: 分级警告系统（75%、90%、95%）
- **P90 计算**: 基于历史数据自动计算合理限额
- **限额管理**: 配置界面和动态调整功能

## 2. 技术设计

### 2.1 数据结构

```go
// SubscriptionPlan 订阅计划
type SubscriptionPlan struct {
    Name            string
    Type            PlanType
    CostLimit       float64
    TokenLimit      int64
    CustomLimit     bool
    Features        []string
    WarningLevels   []WarningLevel
    ResetCycle      ResetCycle
}

// PlanType 计划类型
type PlanType string

const (
    PlanFree   PlanType = "free"
    PlanPro    PlanType = "pro"
    PlanMax5   PlanType = "max5"
    PlanMax20  PlanType = "max20"
    PlanCustom PlanType = "custom"
)

// WarningLevel 警告级别
type WarningLevel struct {
    Threshold   float64 // 百分比阈值
    Message     string
    Severity    Severity
    Actions     []Action
}

// Severity 严重程度
type Severity string

const (
    SeverityInfo    Severity = "info"
    SeverityWarning Severity = "warning"
    SeverityError   Severity = "error"
    SeverityCritical Severity = "critical"
)

// Action 触发的动作
type Action struct {
    Type        ActionType
    Config      map[string]interface{}
}

// ActionType 动作类型
type ActionType string

const (
    ActionNotify    ActionType = "notify"
    ActionLog       ActionType = "log"
    ActionThrottle  ActionType = "throttle"
    ActionBlock     ActionType = "block"
    ActionWebhook   ActionType = "webhook"
)

// LimitStatus 限额状态
type LimitStatus struct {
    Plan            SubscriptionPlan
    CurrentUsage    Usage
    Percentage      float64
    TimeToReset     time.Duration
    WarningLevel    *WarningLevel
    Recommendations []string
}

// Usage 使用情况
type Usage struct {
    Tokens      int64
    Cost        float64
    StartTime   time.Time
    LastUpdate  time.Time
}
```

### 2.2 限额管理器

```go
// LimitManager 限额管理器
type LimitManager struct {
    plan            SubscriptionPlan
    usage           Usage
    history         []HistoricalUsage
    notifier        Notifier
    config          *config.Config
    mu              sync.RWMutex
    warningHistory  map[Severity]time.Time
    p90Calculator   *P90Calculator
}

// 主要方法
func (lm *LimitManager) CheckUsage(entry models.UsageEntry) (*LimitStatus, error)
func (lm *LimitManager) UpdateUsage(tokens int64, cost float64) error
func (lm *LimitManager) GetStatus() *LimitStatus
func (lm *LimitManager) SetPlan(plan PlanType) error
func (lm *LimitManager) CalculateP90Limit() (float64, error)
func (lm *LimitManager) ResetUsage() error
func (lm *LimitManager) GetRecommendations() []string
```

## 3. 实现步骤

### 3.1 创建限额管理器

**文件**: `limits/manager.go`

```go
package limits

import (
    "fmt"
    "sync"
    "time"
    "github.com/penwyp/ClawCat/models"
    "github.com/penwyp/ClawCat/config"
)

// 预定义计划
var (
    PlanDefinitions = map[PlanType]SubscriptionPlan{
        PlanPro: {
            Name:       "Pro",
            Type:       PlanPro,
            CostLimit:  18.00,
            TokenLimit: 1000000, // 估算值
            Features:   []string{"5-hour sessions", "All models", "Priority support"},
            WarningLevels: []WarningLevel{
                {Threshold: 75, Message: "You've used 75% of your Pro plan limit", Severity: SeverityInfo},
                {Threshold: 90, Message: "⚠️ 90% of limit reached! Consider upgrading", Severity: SeverityWarning},
                {Threshold: 95, Message: "🚨 95% limit! Usage will be blocked soon", Severity: SeverityError},
                {Threshold: 100, Message: "❌ Limit reached! Upgrade to continue", Severity: SeverityCritical},
            },
            ResetCycle: ResetCycleMonthly,
        },
        PlanMax5: {
            Name:       "Max-5",
            Type:       PlanMax5,
            CostLimit:  35.00,
            TokenLimit: 2000000,
            Features:   []string{"5-hour sessions", "All models", "Priority support", "Advanced analytics"},
            WarningLevels: defaultWarningLevels(35.00),
            ResetCycle: ResetCycleMonthly,
        },
        PlanMax20: {
            Name:       "Max-20",
            Type:       PlanMax20,
            CostLimit:  140.00,
            TokenLimit: 8000000,
            Features:   []string{"5-hour sessions", "All models", "Priority support", "Advanced analytics", "Team features"},
            WarningLevels: defaultWarningLevels(140.00),
            ResetCycle: ResetCycleMonthly,
        },
    }
)

// NewLimitManager 创建限额管理器
func NewLimitManager(config *config.Config) (*LimitManager, error) {
    planType := PlanType(config.Subscription.Plan)
    plan, ok := PlanDefinitions[planType]
    
    if !ok {
        if planType == PlanCustom {
            // 创建自定义计划
            plan = createCustomPlan(config)
        } else {
            return nil, fmt.Errorf("unknown plan type: %s", planType)
        }
    }
    
    lm := &LimitManager{
        plan:           plan,
        config:         config,
        notifier:       NewNotifier(config),
        warningHistory: make(map[Severity]time.Time),
        p90Calculator:  NewP90Calculator(),
    }
    
    // 加载历史使用数据
    if err := lm.loadHistoricalUsage(); err != nil {
        return nil, fmt.Errorf("failed to load historical usage: %w", err)
    }
    
    // 如果是自定义计划，计算 P90 限额
    if planType == PlanCustom {
        if limit, err := lm.CalculateP90Limit(); err == nil {
            lm.plan.CostLimit = limit
        }
    }
    
    return lm, nil
}

// CheckUsage 检查使用情况
func (lm *LimitManager) CheckUsage(entry models.UsageEntry) (*LimitStatus, error) {
    lm.mu.Lock()
    defer lm.mu.Unlock()
    
    // 更新使用量
    lm.usage.Tokens += int64(entry.TotalTokens)
    lm.usage.Cost += entry.CostUSD
    lm.usage.LastUpdate = time.Now()
    
    // 计算使用百分比
    percentage := (lm.usage.Cost / lm.plan.CostLimit) * 100
    
    // 获取当前状态
    status := &LimitStatus{
        Plan:         lm.plan,
        CurrentUsage: lm.usage,
        Percentage:   percentage,
        TimeToReset:  lm.calculateTimeToReset(),
    }
    
    // 检查警告级别
    for _, level := range lm.plan.WarningLevels {
        if percentage >= level.Threshold {
            status.WarningLevel = &level
            
            // 触发警告动作
            if lm.shouldTriggerWarning(level) {
                go lm.triggerWarningActions(level, status)
            }
        }
    }
    
    // 生成建议
    status.Recommendations = lm.GetRecommendations()
    
    return status, nil
}

// shouldTriggerWarning 判断是否应该触发警告
func (lm *LimitManager) shouldTriggerWarning(level WarningLevel) bool {
    lastTriggered, exists := lm.warningHistory[level.Severity]
    if !exists {
        return true
    }
    
    // 避免频繁警告，每个级别至少间隔1小时
    cooldown := time.Hour
    if level.Severity == SeverityCritical {
        cooldown = 15 * time.Minute
    }
    
    return time.Since(lastTriggered) > cooldown
}

// triggerWarningActions 触发警告动作
func (lm *LimitManager) triggerWarningActions(level WarningLevel, status *LimitStatus) {
    lm.warningHistory[level.Severity] = time.Now()
    
    for _, action := range level.Actions {
        switch action.Type {
        case ActionNotify:
            lm.notifier.SendNotification(level.Message, level.Severity)
        case ActionLog:
            lm.logWarning(level, status)
        case ActionWebhook:
            lm.sendWebhook(action.Config, status)
        case ActionThrottle:
            lm.applyThrottling(action.Config)
        }
    }
}

// CalculateP90Limit 计算 P90 限额
func (lm *LimitManager) CalculateP90Limit() (float64, error) {
    if len(lm.history) < 10 {
        return 0, fmt.Errorf("insufficient historical data: need at least 10 data points")
    }
    
    // 收集历史成本数据
    costs := make([]float64, len(lm.history))
    for i, h := range lm.history {
        costs[i] = h.Cost
    }
    
    // 计算 P90
    p90 := lm.p90Calculator.Calculate(costs)
    
    // 添加 10% 的缓冲
    return p90 * 1.1, nil
}

// GetRecommendations 获取使用建议
func (lm *LimitManager) GetRecommendations() []string {
    lm.mu.RLock()
    defer lm.mu.RUnlock()
    
    recommendations := []string{}
    percentage := (lm.usage.Cost / lm.plan.CostLimit) * 100
    
    if percentage > 90 {
        recommendations = append(recommendations, 
            "Consider upgrading to a higher plan",
            "Review your token usage patterns",
            "Use caching to reduce token consumption",
        )
    } else if percentage > 75 {
        recommendations = append(recommendations,
            "Monitor your usage closely",
            "Plan your remaining tasks carefully",
        )
    }
    
    // 基于使用模式的建议
    if lm.hasHighBurnRate() {
        recommendations = append(recommendations,
            "Your burn rate is high - consider spreading usage",
        )
    }
    
    if lm.hasFrequentSpikes() {
        recommendations = append(recommendations,
            "Frequent usage spikes detected - consider batch processing",
        )
    }
    
    return recommendations
}

// calculateTimeToReset 计算重置时间
func (lm *LimitManager) calculateTimeToReset() time.Duration {
    now := time.Now()
    
    switch lm.plan.ResetCycle {
    case ResetCycleDaily:
        tomorrow := now.Add(24 * time.Hour)
        reset := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, now.Location())
        return reset.Sub(now)
        
    case ResetCycleWeekly:
        // 找到下周一
        daysUntilMonday := (8 - int(now.Weekday())) % 7
        if daysUntilMonday == 0 {
            daysUntilMonday = 7
        }
        reset := now.AddDate(0, 0, daysUntilMonday)
        reset = time.Date(reset.Year(), reset.Month(), reset.Day(), 0, 0, 0, 0, now.Location())
        return reset.Sub(now)
        
    case ResetCycleMonthly:
        // 下月1日
        nextMonth := now.AddDate(0, 1, 0)
        reset := time.Date(nextMonth.Year(), nextMonth.Month(), 1, 0, 0, 0, 0, now.Location())
        return reset.Sub(now)
        
    default:
        return 0
    }
}
```

### 3.2 P90 计算器实现

**文件**: `limits/p90_calculator.go`

```go
package limits

import (
    "math"
    "sort"
)

// P90Calculator P90 百分位计算器
type P90Calculator struct {
    windowSize int
    minSamples int
}

// NewP90Calculator 创建 P90 计算器
func NewP90Calculator() *P90Calculator {
    return &P90Calculator{
        windowSize: 30, // 30天窗口
        minSamples: 10, // 最少10个样本
    }
}

// Calculate 计算 P90 值
func (p *P90Calculator) Calculate(values []float64) float64 {
    if len(values) == 0 {
        return 0
    }
    
    // 复制并排序
    sorted := make([]float64, len(values))
    copy(sorted, values)
    sort.Float64s(sorted)
    
    // 计算 90 百分位的位置
    index := int(math.Ceil(0.9 * float64(len(sorted))))
    if index >= len(sorted) {
        index = len(sorted) - 1
    }
    
    return sorted[index]
}

// CalculateWithOutlierRemoval 计算 P90 并移除异常值
func (p *P90Calculator) CalculateWithOutlierRemoval(values []float64) float64 {
    if len(values) < p.minSamples {
        return p.Calculate(values)
    }
    
    // 计算 IQR（四分位距）
    q1 := p.percentile(values, 25)
    q3 := p.percentile(values, 75)
    iqr := q3 - q1
    
    // 定义异常值边界
    lowerBound := q1 - 1.5*iqr
    upperBound := q3 + 1.5*iqr
    
    // 过滤异常值
    filtered := []float64{}
    for _, v := range values {
        if v >= lowerBound && v <= upperBound {
            filtered = append(filtered, v)
        }
    }
    
    return p.Calculate(filtered)
}

// percentile 计算百分位数
func (p *P90Calculator) percentile(values []float64, percentile float64) float64 {
    sorted := make([]float64, len(values))
    copy(sorted, values)
    sort.Float64s(sorted)
    
    index := (percentile / 100) * float64(len(sorted)-1)
    lower := math.Floor(index)
    upper := math.Ceil(index)
    
    if lower == upper {
        return sorted[int(index)]
    }
    
    // 线性插值
    return sorted[int(lower)]*(upper-index) + sorted[int(upper)]*(index-lower)
}

// AnalyzeDistribution 分析数据分布
func (p *P90Calculator) AnalyzeDistribution(values []float64) Distribution {
    if len(values) == 0 {
        return Distribution{}
    }
    
    sorted := make([]float64, len(values))
    copy(sorted, values)
    sort.Float64s(sorted)
    
    return Distribution{
        Min:    sorted[0],
        Max:    sorted[len(sorted)-1],
        Mean:   p.mean(values),
        Median: p.percentile(values, 50),
        P25:    p.percentile(values, 25),
        P75:    p.percentile(values, 75),
        P90:    p.percentile(values, 90),
        P95:    p.percentile(values, 95),
        P99:    p.percentile(values, 99),
        StdDev: p.stdDev(values),
    }
}

// mean 计算平均值
func (p *P90Calculator) mean(values []float64) float64 {
    if len(values) == 0 {
        return 0
    }
    
    sum := 0.0
    for _, v := range values {
        sum += v
    }
    
    return sum / float64(len(values))
}

// stdDev 计算标准差
func (p *P90Calculator) stdDev(values []float64) float64 {
    if len(values) < 2 {
        return 0
    }
    
    mean := p.mean(values)
    sumSquares := 0.0
    
    for _, v := range values {
        diff := v - mean
        sumSquares += diff * diff
    }
    
    variance := sumSquares / float64(len(values)-1)
    return math.Sqrt(variance)
}
```

### 3.3 通知系统实现

**文件**: `limits/notifier.go`

```go
package limits

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os/exec"
    "runtime"
    "github.com/penwyp/ClawCat/config"
)

// Notifier 通知器
type Notifier struct {
    config       *config.Config
    enabledTypes []NotificationType
}

// NotificationType 通知类型
type NotificationType string

const (
    NotifyDesktop NotificationType = "desktop"
    NotifySound   NotificationType = "sound"
    NotifyWebhook NotificationType = "webhook"
    NotifyEmail   NotificationType = "email"
)

// NewNotifier 创建通知器
func NewNotifier(config *config.Config) Notifier {
    return Notifier{
        config:       config,
        enabledTypes: config.Limits.Notifications,
    }
}

// SendNotification 发送通知
func (n *Notifier) SendNotification(message string, severity Severity) error {
    var errors []error
    
    for _, notifType := range n.enabledTypes {
        var err error
        
        switch notifType {
        case NotifyDesktop:
            err = n.sendDesktopNotification(message, severity)
        case NotifySound:
            err = n.playSound(severity)
        case NotifyWebhook:
            err = n.sendWebhookNotification(message, severity)
        case NotifyEmail:
            err = n.sendEmailNotification(message, severity)
        }
        
        if err != nil {
            errors = append(errors, err)
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("notification errors: %v", errors)
    }
    
    return nil
}

// sendDesktopNotification 发送桌面通知
func (n *Notifier) sendDesktopNotification(message string, severity Severity) error {
    title := fmt.Sprintf("ClawCat - %s", severity)
    
    switch runtime.GOOS {
    case "darwin":
        // macOS
        script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
        return exec.Command("osascript", "-e", script).Run()
        
    case "linux":
        // Linux (需要 notify-send)
        icon := n.getIconForSeverity(severity)
        return exec.Command("notify-send", "-i", icon, title, message).Run()
        
    case "windows":
        // Windows (使用 PowerShell)
        ps := fmt.Sprintf(`
            Add-Type -AssemblyName System.Windows.Forms
            $notification = New-Object System.Windows.Forms.NotifyIcon
            $notification.Icon = [System.Drawing.SystemIcons]::Information
            $notification.BalloonTipTitle = "%s"
            $notification.BalloonTipText = "%s"
            $notification.Visible = $true
            $notification.ShowBalloonTip(10000)
        `, title, message)
        return exec.Command("powershell", "-Command", ps).Run()
        
    default:
        return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
    }
}

// playSound 播放提示音
func (n *Notifier) playSound(severity Severity) error {
    soundFile := n.getSoundForSeverity(severity)
    
    switch runtime.GOOS {
    case "darwin":
        return exec.Command("afplay", soundFile).Run()
    case "linux":
        return exec.Command("paplay", soundFile).Run()
    case "windows":
        return exec.Command("powershell", "-c", fmt.Sprintf("(New-Object Media.SoundPlayer '%s').PlaySync()", soundFile)).Run()
    default:
        return nil
    }
}

// sendWebhookNotification 发送 Webhook 通知
func (n *Notifier) sendWebhookNotification(message string, severity Severity) error {
    webhookURL := n.config.Limits.WebhookURL
    if webhookURL == "" {
        return nil
    }
    
    payload := map[string]interface{}{
        "message":   message,
        "severity":  severity,
        "timestamp": time.Now().Unix(),
        "usage": map[string]interface{}{
            "cost":       n.getCurrentCost(),
            "tokens":     n.getCurrentTokens(),
            "percentage": n.getUsagePercentage(),
        },
    }
    
    jsonData, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    
    resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned status %d", resp.StatusCode)
    }
    
    return nil
}
```

### 3.4 限额 UI 组件

**文件**: `ui/components/limit_display.go`

```go
package components

import (
    "fmt"
    "strings"
    "github.com/charmbracelet/lipgloss"
    "github.com/penwyp/ClawCat/limits"
)

// LimitDisplay 限额显示组件
type LimitDisplay struct {
    status    *limits.LimitStatus
    styles    Styles
    width     int
    expanded  bool
}

// NewLimitDisplay 创建限额显示组件
func NewLimitDisplay() *LimitDisplay {
    return &LimitDisplay{
        styles:   NewStyles(DefaultTheme()),
        expanded: false,
    }
}

// Update 更新限额状态
func (ld *LimitDisplay) Update(status *limits.LimitStatus) {
    ld.status = status
}

// Render 渲染限额显示
func (ld *LimitDisplay) Render() string {
    if ld.status == nil {
        return "Loading limit status..."
    }
    
    if ld.expanded {
        return ld.renderExpanded()
    }
    return ld.renderCompact()
}

// renderCompact 渲染紧凑视图
func (ld *LimitDisplay) renderCompact() string {
    // 计划名称和使用百分比
    planBadge := ld.renderPlanBadge()
    usageBar := ld.renderMiniProgressBar()
    
    // 警告信息
    warning := ""
    if ld.status.WarningLevel != nil {
        style := ld.getWarningStyle(ld.status.WarningLevel.Severity)
        warning = style.Render(ld.status.WarningLevel.Message)
    }
    
    parts := []string{planBadge, usageBar}
    if warning != "" {
        parts = append(parts, warning)
    }
    
    return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

// renderExpanded 渲染展开视图
func (ld *LimitDisplay) renderExpanded() string {
    sections := []string{
        ld.renderHeader(),
        ld.renderUsageDetails(),
        ld.renderProgressSection(),
    }
    
    if ld.status.WarningLevel != nil {
        sections = append(sections, ld.renderWarning())
    }
    
    if len(ld.status.Recommendations) > 0 {
        sections = append(sections, ld.renderRecommendations())
    }
    
    content := strings.Join(sections, "\n\n")
    
    return ld.styles.Box.
        Width(ld.width).
        Render(content)
}

// renderHeader 渲染标题
func (ld *LimitDisplay) renderHeader() string {
    title := fmt.Sprintf("💳 %s Plan", ld.status.Plan.Name)
    subtitle := fmt.Sprintf("Resets in %s", formatDuration(ld.status.TimeToReset))
    
    return lipgloss.JoinVertical(
        lipgloss.Left,
        ld.styles.Title.Render(title),
        ld.styles.Subtitle.Render(subtitle),
    )
}

// renderUsageDetails 渲染使用详情
func (ld *LimitDisplay) renderUsageDetails() string {
    details := []string{
        fmt.Sprintf("💰 Cost: $%.2f / $%.2f", 
            ld.status.CurrentUsage.Cost, 
            ld.status.Plan.CostLimit,
        ),
        fmt.Sprintf("🔢 Tokens: %s / %s",
            formatNumber(ld.status.CurrentUsage.Tokens),
            formatNumber(ld.status.Plan.TokenLimit),
        ),
        fmt.Sprintf("📊 Usage: %.1f%%", ld.status.Percentage),
    }
    
    // 根据使用率应用颜色
    style := ld.styles.Normal
    if ld.status.Percentage > 90 {
        style = ld.styles.Error
    } else if ld.status.Percentage > 75 {
        style = ld.styles.Warning
    }
    
    return style.Render(strings.Join(details, "\n"))
}

// renderProgressSection 渲染进度条区域
func (ld *LimitDisplay) renderProgressSection() string {
    // 成本进度条
    costProgress := NewProgressBar(
        "Cost Usage",
        ld.status.CurrentUsage.Cost,
        ld.status.Plan.CostLimit,
    )
    costProgress.Color = ld.getProgressColor(ld.status.Percentage)
    costProgress.SetWidth(ld.width - 10)
    
    // Token 进度条（如果有限制）
    var tokenBar string
    if ld.status.Plan.TokenLimit > 0 {
        tokenProgress := NewProgressBar(
            "Token Usage",
            float64(ld.status.CurrentUsage.Tokens),
            float64(ld.status.Plan.TokenLimit),
        )
        tokenProgress.Color = ld.getProgressColor(
            float64(ld.status.CurrentUsage.Tokens) / float64(ld.status.Plan.TokenLimit) * 100,
        )
        tokenProgress.SetWidth(ld.width - 10)
        tokenBar = tokenProgress.Render()
    }
    
    parts := []string{costProgress.Render()}
    if tokenBar != "" {
        parts = append(parts, tokenBar)
    }
    
    return strings.Join(parts, "\n")
}

// renderWarning 渲染警告
func (ld *LimitDisplay) renderWarning() string {
    level := ld.status.WarningLevel
    style := ld.getWarningStyle(level.Severity)
    
    icon := ld.getWarningIcon(level.Severity)
    message := fmt.Sprintf("%s %s", icon, level.Message)
    
    return style.
        Padding(1).
        Margin(0, 1).
        Render(message)
}

// renderRecommendations 渲染建议
func (ld *LimitDisplay) renderRecommendations() string {
    title := ld.styles.Subtitle.Render("💡 Recommendations")
    
    items := []string{}
    for _, rec := range ld.status.Recommendations {
        items = append(items, fmt.Sprintf("• %s", rec))
    }
    
    content := ld.styles.Faint.Render(strings.Join(items, "\n"))
    
    return strings.Join([]string{title, content}, "\n")
}

// renderPlanBadge 渲染计划徽章
func (ld *LimitDisplay) renderPlanBadge() string {
    badge := fmt.Sprintf(" %s ", ld.status.Plan.Name)
    
    style := ld.styles.Badge
    if ld.status.Percentage > 90 {
        style = style.Background(lipgloss.Color("#FF0000"))
    } else if ld.status.Percentage > 75 {
        style = style.Background(lipgloss.Color("#FFA500"))
    }
    
    return style.Render(badge)
}

// renderMiniProgressBar 渲染迷你进度条
func (ld *LimitDisplay) renderMiniProgressBar() string {
    width := 20
    filled := int(ld.status.Percentage / 100 * float64(width))
    
    bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
    percentage := fmt.Sprintf(" %.0f%%", ld.status.Percentage)
    
    color := ld.getProgressColor(ld.status.Percentage)
    
    return lipgloss.NewStyle().
        Foreground(color).
        Render(bar + percentage)
}

// getWarningStyle 获取警告样式
func (ld *LimitDisplay) getWarningStyle(severity limits.Severity) lipgloss.Style {
    switch severity {
    case limits.SeverityInfo:
        return ld.styles.Info
    case limits.SeverityWarning:
        return ld.styles.Warning
    case limits.SeverityError:
        return ld.styles.Error
    case limits.SeverityCritical:
        return ld.styles.Error.Bold(true).Blink(true)
    default:
        return ld.styles.Normal
    }
}

// getWarningIcon 获取警告图标
func (ld *LimitDisplay) getWarningIcon(severity limits.Severity) string {
    switch severity {
    case limits.SeverityInfo:
        return "ℹ️"
    case limits.SeverityWarning:
        return "⚠️"
    case limits.SeverityError:
        return "🚨"
    case limits.SeverityCritical:
        return "❌"
    default:
        return "•"
    }
}

// getProgressColor 获取进度条颜色
func (ld *LimitDisplay) getProgressColor(percentage float64) lipgloss.Color {
    if percentage >= 95 {
        return lipgloss.Color("#FF0000") // 红色
    } else if percentage >= 90 {
        return lipgloss.Color("#FF4500") // 橙红色
    } else if percentage >= 75 {
        return lipgloss.Color("#FFA500") // 橙色
    } else if percentage >= 50 {
        return lipgloss.Color("#FFD700") // 金色
    }
    return lipgloss.Color("#00FF00") // 绿色
}
```

## 4. 测试计划

### 4.1 单元测试

```go
// limits/manager_test.go

func TestLimitManager_CheckUsage(t *testing.T) {
    tests := []struct {
        name           string
        plan           PlanType
        currentCost    float64
        expectedLevel  *Severity
        expectedAction bool
    }{
        {
            name:           "under 75%",
            plan:           PlanPro,
            currentCost:    10.00,
            expectedLevel:  nil,
            expectedAction: false,
        },
        {
            name:           "at 75%",
            plan:           PlanPro,
            currentCost:    13.50,
            expectedLevel:  &SeverityInfo,
            expectedAction: true,
        },
        {
            name:           "at 90%",
            plan:           PlanPro,
            currentCost:    16.20,
            expectedLevel:  &SeverityWarning,
            expectedAction: true,
        },
        {
            name:           "over limit",
            plan:           PlanPro,
            currentCost:    20.00,
            expectedLevel:  &SeverityCritical,
            expectedAction: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            config := &config.Config{
                Subscription: config.Subscription{
                    Plan: string(tt.plan),
                },
            }
            
            manager, err := NewLimitManager(config)
            assert.NoError(t, err)
            
            // 设置初始使用量
            manager.usage.Cost = tt.currentCost
            
            // 创建测试条目
            entry := models.UsageEntry{
                TotalTokens: 100,
                CostUSD:     0.01,
            }
            
            status, err := manager.CheckUsage(entry)
            assert.NoError(t, err)
            
            if tt.expectedLevel != nil {
                assert.NotNil(t, status.WarningLevel)
                assert.Equal(t, *tt.expectedLevel, status.WarningLevel.Severity)
            } else {
                assert.Nil(t, status.WarningLevel)
            }
        })
    }
}

func TestP90Calculator_Calculate(t *testing.T) {
    calc := NewP90Calculator()
    
    tests := []struct {
        name     string
        values   []float64
        expected float64
    }{
        {
            name:     "simple sequence",
            values:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
            expected: 9,
        },
        {
            name:     "with outliers",
            values:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 100},
            expected: 100, // P90 包含异常值
        },
        {
            name:     "real usage pattern",
            values:   []float64{15.2, 18.5, 12.3, 20.1, 16.8, 14.5, 19.2, 13.7, 17.5, 22.3},
            expected: 20.1,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := calc.Calculate(tt.values)
            assert.InDelta(t, tt.expected, result, 0.1)
        })
    }
}

func TestLimitManager_Recommendations(t *testing.T) {
    manager := &LimitManager{
        plan: PlanDefinitions[PlanPro],
        usage: Usage{
            Cost: 16.50, // 91.7%
        },
    }
    
    recommendations := manager.GetRecommendations()
    
    assert.NotEmpty(t, recommendations)
    assert.Contains(t, recommendations[0], "upgrade")
}
```

### 4.2 集成测试

```go
// integration/limits_test.go

func TestLimitsIntegration(t *testing.T) {
    // 创建测试环境
    app := setupTestApp(t)
    
    t.Run("warning escalation", func(t *testing.T) {
        // 模拟逐渐增加的使用量
        costs := []float64{5.0, 10.0, 13.5, 16.2, 17.1, 18.0}
        expectedSeverities := []string{"", "", "info", "warning", "error", "critical"}
        
        for i, cost := range costs {
            // 添加使用量
            entry := models.UsageEntry{
                TotalTokens: 1000,
                CostUSD:     cost - getPreviousCost(costs, i),
            }
            
            status, err := app.LimitManager.CheckUsage(entry)
            assert.NoError(t, err)
            
            if expectedSeverities[i] != "" {
                assert.NotNil(t, status.WarningLevel)
                assert.Equal(t, expectedSeverities[i], string(status.WarningLevel.Severity))
            }
        }
    })
    
    t.Run("P90 calculation with history", func(t *testing.T) {
        // 生成30天的历史数据
        history := generateHistoricalUsage(30)
        
        manager := &LimitManager{
            history:       history,
            p90Calculator: NewP90Calculator(),
        }
        
        limit, err := manager.CalculateP90Limit()
        assert.NoError(t, err)
        assert.Greater(t, limit, 0.0)
        
        // P90 应该覆盖90%的历史使用
        covered := 0
        for _, h := range history {
            if h.Cost <= limit {
                covered++
            }
        }
        
        coverage := float64(covered) / float64(len(history))
        assert.GreaterOrEqual(t, coverage, 0.85) // 允许一些误差
    })
}
```

## 5. 配置选项

### 5.1 限额配置

```yaml
# config.yaml
limits:
  # 订阅计划
  subscription:
    plan: "pro"  # free, pro, max5, max20, custom
    custom_limit: 0  # 自定义限额（仅 custom 计划）
    
  # 警告配置
  warnings:
    enabled: true
    thresholds:
      - level: 75
        severity: "info"
        actions: ["notify", "log"]
      - level: 90
        severity: "warning"
        actions: ["notify", "log", "webhook"]
      - level: 95
        severity: "error"
        actions: ["notify", "log", "webhook", "sound"]
      - level: 100
        severity: "critical"
        actions: ["notify", "log", "webhook", "sound", "block"]
    
  # 通知配置
  notifications:
    - "desktop"
    - "sound"
    - "webhook"
  webhook_url: "https://example.com/webhook"
  
  # P90 配置
  p90:
    enabled: true
    window_days: 30
    min_samples: 10
    outlier_removal: true
    buffer_percentage: 10
    
  # 重置周期
  reset_cycle: "monthly"  # daily, weekly, monthly
  
  # 限流配置
  throttling:
    enabled: false
    rate_limit: 100  # requests per minute
    burst: 20
```

## 6. 监控和分析

### 6.1 使用分析

```go
// UsageAnalyzer 使用分析器
type UsageAnalyzer struct {
    history []HistoricalUsage
}

// AnalyzePatterns 分析使用模式
func (ua *UsageAnalyzer) AnalyzePatterns() UsagePatterns {
    patterns := UsagePatterns{}
    
    // 分析日内模式
    hourlyUsage := ua.groupByHour()
    patterns.PeakHours = ua.findPeakHours(hourlyUsage)
    
    // 分析周内模式
    dailyUsage := ua.groupByDayOfWeek()
    patterns.PeakDays = ua.findPeakDays(dailyUsage)
    
    // 检测使用趋势
    patterns.Trend = ua.detectTrend()
    
    // 检测异常
    patterns.Anomalies = ua.detectAnomalies()
    
    return patterns
}

// PredictFutureUsage 预测未来使用
func (ua *UsageAnalyzer) PredictFutureUsage(days int) []PredictedUsage {
    // 使用简单的移动平均或更复杂的预测模型
    // 这里使用加权移动平均
    predictions := []PredictedUsage{}
    
    weights := []float64{0.5, 0.3, 0.2} // 最近的数据权重更高
    
    for i := 0; i < days; i++ {
        prediction := ua.weightedAverage(weights)
        predictions = append(predictions, PredictedUsage{
            Date:            time.Now().AddDate(0, 0, i+1),
            PredictedCost:   prediction,
            ConfidenceLevel: ua.calculateConfidence(i),
        })
    }
    
    return predictions
}
```

## 7. 错误处理

### 7.1 降级策略

```go
func (lm *LimitManager) CheckUsageWithFallback(entry models.UsageEntry) (*LimitStatus, error) {
    // 尝试正常检查
    status, err := lm.CheckUsage(entry)
    if err == nil {
        return status, nil
    }
    
    // 降级到基本检查
    lm.logger.Error("Failed to check usage, using fallback", "error", err)
    
    return &LimitStatus{
        Plan:         lm.plan,
        CurrentUsage: lm.usage,
        Percentage:   (lm.usage.Cost / lm.plan.CostLimit) * 100,
        WarningLevel: lm.getBasicWarningLevel(),
    }, nil
}
```

## 8. 部署清单

- [ ] 实现 `limits/manager.go`
- [ ] 实现 `limits/p90_calculator.go`
- [ ] 实现 `limits/notifier.go`
- [ ] 实现 `ui/components/limit_display.go`
- [ ] 添加预定义计划配置
- [ ] 实现警告系统
- [ ] 集成通知功能
- [ ] 添加 P90 计算
- [ ] 编写测试
- [ ] 集成到主应用
- [ ] 更新配置选项
- [ ] 编写用户文档

## 9. 未来增强

- 多用户/团队限额管理
- 基于 AI 的使用预测
- 自动计划推荐
- 成本优化建议
- API 限额集成
- 自定义警告规则
- 限额共享池
- 预付费模式支持
- 与计费系统集成