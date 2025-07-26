package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/claudecat/calculations"
	"github.com/penwyp/claudecat/models"
)

// MonitorView represents the Claude Code Usage Monitor view
// This view matches the exact layout of Claude-Code-Usage-Monitor
type MonitorView struct {
	config          Config
	stats           Statistics
	metrics         *calculations.RealtimeMetrics
	blocks          []models.SessionBlock
	width           int
	height          int
	styles          Styles
	p90Calculator   *calculations.P90Calculator
	timezone        string
	timeFormat      string // "12h" or "24h"
	plan            string
	tokenLimit      int
	costLimitP90    float64
	messagesLimitP90 int
}

// NewMonitorView creates a new monitor view
func NewMonitorView(config Config) *MonitorView {
	timezone := config.Timezone
	if timezone == "" || timezone == "auto" {
		timezone = "Asia/Shanghai"
	}

	timeFormat := config.TimeFormat
	if timeFormat == "" || timeFormat == "auto" {
		timeFormat = "24h"
	}

	return &MonitorView{
		config:          config,
		styles:          NewStyles(GetThemeByName(config.Theme)),
		p90Calculator:   calculations.NewP90Calculator(),
		timezone:        timezone,
		timeFormat:      timeFormat,
		plan:            strings.ToLower(config.SubscriptionPlan),
	}
}

// Init initializes the monitor view
func (v *MonitorView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the monitor view
func (v *MonitorView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v, nil
}

// View renders the monitor display
func (v *MonitorView) View() string {
	var lines []string

	// Header
	lines = append(lines, v.renderHeader()...)
	lines = append(lines, "")

	// Main content based on state
	if v.metrics == nil || v.metrics.CurrentTokens == 0 {
		lines = append(lines, v.renderNoActiveSession()...)
	} else {
		lines = append(lines, v.renderActiveSession()...)
	}

	// Footer
	lines = append(lines, v.renderFooter())

	return strings.Join(lines, "\n")
}

// renderHeader renders the header section
func (v *MonitorView) renderHeader() []string {
	sparkles := "✦ ✧ ✦ ✧"
	title := "CLAUDE CODE USAGE MONITOR"
	separator := strings.Repeat("=", 60)

	plan := v.plan
	if plan == "" {
		plan = "pro"
	}

	timezone := v.timezone
	if timezone == "" {
		timezone = "Europe/Warsaw"
	}

	return []string{
		fmt.Sprintf("%s %s %s", sparkles, title, sparkles),
		separator,
		fmt.Sprintf("[ %s | %s ]", strings.ToLower(plan), strings.ToLower(timezone)),
	}
}

// renderNoActiveSession renders the display when there's no active session
func (v *MonitorView) renderNoActiveSession() []string {
	var lines []string

	// Empty progress bar
	emptyBar := v.renderWideProgressBar(0, "🟨")
	lines = append(lines, fmt.Sprintf("📊 Token Usage:    %s", emptyBar))
	lines = append(lines, "")

	// Stats with zero values
	lines = append(lines, fmt.Sprintf("🎯 Tokens:         0 / ~%s (0 left)", v.formatNumber(v.tokenLimit)))
	lines = append(lines, "🔥 Burn Rate:      0.0 tokens/min")
	lines = append(lines, "💲 Cost Rate:      $0.00 $/min")
	lines = append(lines, "📨 Sent Messages:  0 messages")
	lines = append(lines, "")

	return lines
}

// renderActiveSession renders the display for an active session
func (v *MonitorView) renderActiveSession() []string {
	var lines []string

	// Calculate burn rate once for the entire function
	burnRate := v.calculateBurnRate()

	// Calculate percentages
	tokenUsage := float64(v.metrics.CurrentTokens) / float64(v.tokenLimit) * 100
	costUsage := v.metrics.CurrentCost / v.costLimitP90 * 100
	
	// Get message count from current active session
	messageCount := 0
	if len(v.blocks) > 0 {
		for _, block := range v.blocks {
			if block.IsActive {
				messageCount = block.SentMessagesCount
				break
			}
		}
	}
	messagesUsage := float64(messageCount) / float64(v.messagesLimitP90) * 100

	// Time calculations
	sessionStart := v.metrics.SessionStart
	if sessionStart.IsZero() && len(v.blocks) > 0 {
		for _, block := range v.blocks {
			if block.IsActive {
				sessionStart = block.StartTime
				break
			}
		}
	}
	
	elapsed := time.Since(sessionStart).Minutes()
	totalMinutes := 300.0 // 5 hours
	timePercentage := (elapsed / totalMinutes) * 100
	timeRemaining := totalMinutes - elapsed

	// For custom plan, show P90 limits section
	if v.plan == "custom" || v.plan == "pro" || v.plan == "max5" || v.plan == "max20" {
		lines = append(lines, "📊 Session-Based Dynamic Limits")
		lines = append(lines, "Based on your historical usage patterns when hitting limits (P90)")
		lines = append(lines, strings.Repeat("─", 60))
		
		// Cost Usage
		costBar := v.renderWideProgressBar(costUsage, v.getColorIndicator(costUsage))
		lines = append(lines, fmt.Sprintf("💰 Cost Usage:           %s %5.1f%%    $%.2f / $%.2f",
			costBar, costUsage, v.metrics.CurrentCost, v.costLimitP90))
		lines = append(lines, "")
		
		// Token Usage
		tokenBar := v.renderWideProgressBar(tokenUsage, v.getColorIndicator(tokenUsage))
		lines = append(lines, fmt.Sprintf("📊 Token Usage:          %s %5.1f%%    %s / %s",
			tokenBar, tokenUsage, v.formatNumber(v.metrics.CurrentTokens), v.formatNumber(v.tokenLimit)))
		lines = append(lines, "")
		
		// Messages Usage
		messagesBar := v.renderWideProgressBar(messagesUsage, v.getColorIndicator(messagesUsage))
		lines = append(lines, fmt.Sprintf("📨 Messages Usage:       %s %5.1f%%    %d / %d",
			messagesBar, messagesUsage, messageCount, v.messagesLimitP90))
		lines = append(lines, strings.Repeat("─", 60))
		
		// Time to Reset
		timeBar := v.renderWideProgressBar(timePercentage, "")
		hours := int(timeRemaining / 60)
		mins := int(timeRemaining) % 60
		lines = append(lines, fmt.Sprintf("⏱️  Time to Reset:       %s %dh %dm", timeBar, hours, mins))
		lines = append(lines, "")
		
		// Model Distribution
		modelBar := v.renderModelDistribution()
		lines = append(lines, fmt.Sprintf("🤖 Model Distribution:   %s", modelBar))
		lines = append(lines, strings.Repeat("─", 60))
		
		// Burn Rate
		velocityEmoji := v.getVelocityEmoji(burnRate)
		lines = append(lines, fmt.Sprintf("🔥 Burn Rate:              %.1f tokens/min %s", burnRate, velocityEmoji))
		
		// Cost Rate
		costRate := v.calculateCostRate()
		lines = append(lines, fmt.Sprintf("💲 Cost Rate:              $%.4f $/min", costRate))
	} else {
		// Simple view for other plans
		tokenBar := v.renderProgressBar(tokenUsage)
		lines = append(lines, fmt.Sprintf("📊 Token Usage:    %s", tokenBar))
		lines = append(lines, "")
		
		lines = append(lines, fmt.Sprintf("🎯 Tokens:         %s / ~%s (%s left)",
			v.formatNumber(v.metrics.CurrentTokens),
			v.formatNumber(v.tokenLimit),
			v.formatNumber(v.tokenLimit-v.metrics.CurrentTokens)))
		
		velocityEmoji := v.getVelocityEmoji(burnRate)
		lines = append(lines, fmt.Sprintf("🔥 Burn Rate:      %.1f tokens/min %s", burnRate, velocityEmoji))
		
		lines = append(lines, fmt.Sprintf("💲 Session Cost:   $%.2f", v.metrics.CurrentCost))
		lines = append(lines, fmt.Sprintf("📨 Sent Messages:  %d messages", messageCount))
		
		if modelBar := v.renderModelDistribution(); modelBar != "" {
			lines = append(lines, fmt.Sprintf("🤖 Model Usage:    %s", modelBar))
		}
		lines = append(lines, "")
		
		timeBar := v.renderTimeProgress(elapsed, totalMinutes)
		lines = append(lines, fmt.Sprintf("⏱️  Time to Reset:  %s", timeBar))
		lines = append(lines, "")
	}

	// Predictions
	lines = append(lines, "")
	lines = append(lines, "🔮 Predictions:")
	
	// Calculate when tokens will run out
	if burnRate > 0 {
		minutesUntilOut := float64(v.tokenLimit-v.metrics.CurrentTokens) / burnRate
		runOutTime := time.Now().Add(time.Duration(minutesUntilOut) * time.Minute)
		lines = append(lines, fmt.Sprintf("   Tokens will run out: %s", v.formatTime(runOutTime)))
	} else {
		lines = append(lines, "   Tokens will run out: --:--:--")
	}
	
	// Reset time
	resetTime := sessionStart.Add(5 * time.Hour)
	lines = append(lines, fmt.Sprintf("   Limit resets at:     %s", v.formatTime(resetTime)))
	lines = append(lines, "")

	return lines
}

// renderFooter renders the footer
func (v *MonitorView) renderFooter() string {
	currentTime := v.formatTime(time.Now())
	
	statusIcon := "🟨" // Yellow for no active session
	statusText := "No active session"
	
	if v.metrics != nil && v.metrics.CurrentTokens > 0 {
		statusIcon = "🟢" // Green for active session
		statusText = "Active session"
	}
	
	return fmt.Sprintf("⏰ %s 📝 %s | Ctrl+C to exit %s", currentTime, statusText, statusIcon)
}

// renderWideProgressBar renders a 50-character wide progress bar
func (v *MonitorView) renderWideProgressBar(percentage float64, colorIndicator string) string {
	width := 50
	filled := int(percentage * float64(width) / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat(" ", width-filled)
	
	if colorIndicator == "" {
		return fmt.Sprintf("[%s]", bar)
	}
	return fmt.Sprintf("%s [%s]", colorIndicator, bar)
}

// renderProgressBar renders a standard progress bar
func (v *MonitorView) renderProgressBar(percentage float64) string {
	width := 20
	filled := int(percentage * float64(width) / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %.1f%%", bar, percentage)
}

// renderTimeProgress renders time progress bar
func (v *MonitorView) renderTimeProgress(elapsed, total float64) string {
	percentage := (elapsed / total) * 100
	if percentage > 100 {
		percentage = 100
	}
	
	width := 20
	filled := int(percentage * float64(width) / 100)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	
	remaining := total - elapsed
	hours := int(remaining / 60)
	mins := int(remaining) % 60
	
	return fmt.Sprintf("[%s] %dh %dm", bar, hours, mins)
}

// renderModelDistribution renders model usage distribution
func (v *MonitorView) renderModelDistribution() string {
	if v.metrics == nil || len(v.metrics.ModelDistribution) == 0 {
		return "[No model data]"
	}

	var parts []string
	for model, metrics := range v.metrics.ModelDistribution {
		// Simplify model names
		displayModel := model
		if strings.Contains(model, "opus") {
			displayModel = "claude-3-opus"
		} else if strings.Contains(model, "sonnet") {
			displayModel = "claude-3-sonnet"
		} else if strings.Contains(model, "haiku") {
			displayModel = "claude-3-haiku"
		}
		
		// Calculate percentage based on token count
		percentage := float64(0)
		if v.metrics.CurrentTokens > 0 {
			percentage = float64(metrics.TokenCount) / float64(v.metrics.CurrentTokens) * 100
		}
		
		parts = append(parts, fmt.Sprintf("%s: %.0f%%", displayModel, percentage))
	}
	
	return fmt.Sprintf("[%s]", strings.Join(parts, "] ["))
}

// getColorIndicator returns the appropriate color indicator based on percentage
func (v *MonitorView) getColorIndicator(percentage float64) string {
	if percentage < 50 {
		return "🟢"
	} else if percentage < 80 {
		return "🟡"
	} else {
		return "🔴"
	}
}

// getVelocityEmoji returns the appropriate velocity emoji based on burn rate
func (v *MonitorView) getVelocityEmoji(burnRate float64) string {
	if burnRate < 50 {
		return "🐌" // Snail - very slow
	} else if burnRate < 100 {
		return "🚶" // Walking - slow
	} else if burnRate < 200 {
		return "🏃" // Running - moderate
	} else if burnRate < 500 {
		return "🚗" // Car - fast
	} else {
		return "🚀" // Rocket - very fast
	}
}

// calculateBurnRate calculates the current burn rate in tokens/min
func (v *MonitorView) calculateBurnRate() float64 {
	if v.blocks == nil || len(v.blocks) == 0 {
		return 0.0
	}

	// Use the burn rate calculator to get hourly burn rate
	calculator := calculations.NewBurnRateCalculator()
	return calculator.CalculateHourlyBurnRate(v.blocks, time.Now())
}

// calculateCostRate calculates the cost rate in $/min
func (v *MonitorView) calculateCostRate() float64 {
	if v.metrics == nil || v.metrics.SessionStart.IsZero() {
		return 0.0
	}

	elapsed := time.Since(v.metrics.SessionStart).Minutes()
	if elapsed <= 0 {
		return 0.0
	}

	return v.metrics.CurrentCost / elapsed
}

// formatNumber formats large numbers with K/M suffixes
func (v *MonitorView) formatNumber(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.0fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.0fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// formatTime formats time according to the configured format
func (v *MonitorView) formatTime(t time.Time) string {
	// Convert to configured timezone
	loc, err := time.LoadLocation(v.timezone)
	if err != nil {
		loc = time.UTC
	}
	t = t.In(loc)

	if v.timeFormat == "24h" {
		return t.Format("15:04:05")
	}
	return t.Format("3:04:05 PM")
}

// UpdateStats updates the view statistics
func (v *MonitorView) UpdateStats(stats Statistics) {
	v.stats = stats
}

// UpdateMetrics updates real-time metrics
func (v *MonitorView) UpdateMetrics(metrics *calculations.RealtimeMetrics) {
	v.metrics = metrics
}

// UpdateBlocks updates session blocks for calculations
func (v *MonitorView) UpdateBlocks(blocks []models.SessionBlock) {
	v.blocks = blocks
	
	// Calculate P90 limits if on custom plan
	if v.plan == "custom" {
		v.tokenLimit = v.p90Calculator.CalculateP90Limit(blocks, true)
		v.costLimitP90 = v.p90Calculator.GetCostP90(blocks)
		v.messagesLimitP90 = v.p90Calculator.GetMessagesP90(blocks)
	} else {
		// Set fixed limits based on plan
		switch v.plan {
		case "pro":
			v.tokenLimit = 1000000
			v.costLimitP90 = 18.0
			v.messagesLimitP90 = 1500
		case "max5":
			v.tokenLimit = 2000000
			v.costLimitP90 = 35.0
			v.messagesLimitP90 = 3000
		case "max20":
			v.tokenLimit = 8000000
			v.costLimitP90 = 140.0
			v.messagesLimitP90 = 12000
		default:
			v.tokenLimit = 1000000
			v.costLimitP90 = 18.0
			v.messagesLimitP90 = 1500
		}
	}
}

// Resize updates the view dimensions
func (v *MonitorView) Resize(width, height int) {
	v.width = width
	v.height = height
}

// UpdateConfig updates the view configuration
func (v *MonitorView) UpdateConfig(config Config) {
	v.config = config
	v.styles = NewStyles(GetThemeByName(config.Theme))
	v.plan = strings.ToLower(config.SubscriptionPlan)
	
	if config.Timezone != "" && config.Timezone != "auto" {
		v.timezone = config.Timezone
	}
	if config.TimeFormat != "" && config.TimeFormat != "auto" {
		v.timeFormat = config.TimeFormat
	}
}