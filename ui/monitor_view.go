package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/penwyp/claudecat/calculations"
	"github.com/penwyp/claudecat/models"
)

// MonitorView represents the Claude Code Usage Monitor view
// This view matches the exact layout of Claude-Code-Usage-Monitor
type MonitorView struct {
	config           Config
	stats            Statistics
	metrics          *calculations.RealtimeMetrics
	blocks           []models.SessionBlock
	width            int
	height           int
	styles           Styles
	p90Calculator    *calculations.P90Calculator
	timezone         string
	timeFormat       string // "12h" or "24h"
	plan             string
	tokenLimit       int
	costLimitP90     float64
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
		config:        config,
		styles:        NewStyles(GetThemeByName(config.Theme)),
		p90Calculator: calculations.NewP90Calculator(),
		timezone:      timezone,
		timeFormat:    timeFormat,
		plan:          strings.ToLower(config.SubscriptionPlan),
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
	hasActiveSession := false
	if v.blocks != nil {
		for _, block := range v.blocks {
			if block.IsActive {
				hasActiveSession = true
				break
			}
		}
	}

	if hasActiveSession {
		lines = append(lines, v.renderActiveSession()...)
	} else {
		lines = append(lines, v.renderNoActiveSession()...)
	}

	// Footer
	lines = append(lines, v.renderFooter())

	return strings.Join(lines, "\n")
}

// renderHeader renders the header section
func (v *MonitorView) renderHeader() []string {
	sparkles := "✦ ✧ ✦ ✧"
	title := "CLAUDE CODE USAGE MONITOR"
	separator := strings.Repeat("═", 60)

	plan := v.plan
	if plan == "" {
		plan = "pro"
	}

	timezone := v.timezone
	if timezone == "" {
		timezone = "Europe/Warsaw"
	}

	// Create styled header
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFD700")).
		Bold(true)

	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#444444"))

	planStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00D9FF")).
		Bold(true)

	return []string{
		fmt.Sprintf("%s %s %s", sparkles, titleStyle.Render(title), sparkles),
		separatorStyle.Render(separator),
		fmt.Sprintf("[ %s | %s ]", planStyle.Render(strings.ToLower(plan)), strings.ToLower(timezone)),
	}
}

// renderNoActiveSession renders the display when there's no active session
func (v *MonitorView) renderNoActiveSession() []string {
	var lines []string

	// Show metrics from the most recent session if available
	tokensUsed := 0
	costUsed := 0.0
	messagesUsed := 0

	if v.metrics != nil {
		tokensUsed = v.metrics.CurrentTokens
		costUsed = v.metrics.CurrentCost
		// Get message count from metrics or recent inactive session
		if len(v.blocks) > 0 {
			// Find the most recent session (active or not)
			for i := len(v.blocks) - 1; i >= 0; i-- {
				if !v.blocks[i].IsGap {
					messagesUsed = v.blocks[i].SentMessagesCount
					break
				}
			}
		}
	}

	// Calculate usage percentage
	tokenUsage := 0.0
	if v.tokenLimit > 0 && tokensUsed > 0 {
		tokenUsage = float64(tokensUsed) / float64(v.tokenLimit) * 100
	}

	// Progress bar
	progressBar := v.renderWideProgressBar(tokenUsage, "🟨")
	lines = append(lines, fmt.Sprintf("📊 Token Usage:    %s", progressBar))
	lines = append(lines, "")

	// Stats - show actual values if any tokens were used
	if tokensUsed > 0 {
		lines = append(lines, fmt.Sprintf("🎯 Tokens:         %s / ~%s (%s left)",
			v.formatNumber(tokensUsed),
			v.formatNumber(v.tokenLimit),
			v.formatNumber(v.tokenLimit-tokensUsed)))
		lines = append(lines, fmt.Sprintf("💲 Session Cost:   $%.2f", costUsed))
		lines = append(lines, fmt.Sprintf("📨 Sent Messages:  %d messages", messagesUsed))
	} else {
		lines = append(lines, fmt.Sprintf("🎯 Tokens:         0 / ~%s (0 left)", v.formatNumber(v.tokenLimit)))
		lines = append(lines, "💲 Session Cost:   $0.00")
		lines = append(lines, "📨 Sent Messages:  0 messages")
	}

	lines = append(lines, "🔥 Burn Rate:      0.0 tokens/min")
	lines = append(lines, "💵 Cost Rate:      $0.00 $/min")
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

	// Styles for different elements
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))

	// Always show the unified format regardless of plan
	lines = append(lines, "")
	lines = append(lines, "")

	// Cost Usage
	costIndicator := v.getColorIndicator(costUsage)
	costBar := v.renderWideProgressBar(costUsage, "")
	lines = append(lines, fmt.Sprintf("💰 Cost Usage:           %s %s %.1f%%    $%.2f / $%.2f",
		costIndicator, costBar, costUsage, v.metrics.CurrentCost, v.costLimitP90))
	lines = append(lines, "")

	// Token Usage
	tokenIndicator := v.getColorIndicator(tokenUsage)
	tokenBar := v.renderWideProgressBar(tokenUsage, "")
	lines = append(lines, fmt.Sprintf("📊 Token Usage:          %s %s %.1f%%    %s / %s",
		tokenIndicator, tokenBar, tokenUsage,
		v.formatNumberWithCommas(v.metrics.CurrentTokens),
		v.formatNumberWithCommas(v.tokenLimit)))
	lines = append(lines, "")

	// Messages Usage
	messagesIndicator := v.getColorIndicator(messagesUsage)
	messagesBar := v.renderWideProgressBar(messagesUsage, "")
	lines = append(lines, fmt.Sprintf("📨 Messages Usage:       %s %s %.1f%%    %d / %s",
		messagesIndicator, messagesBar, messagesUsage, messageCount,
		v.formatNumberWithCommas(v.messagesLimitP90)))
	lines = append(lines, separatorStyle.Render(strings.Repeat("─", 60)))

	// Time to Reset
	timeIndicator := ""
	if timePercentage >= 60 {
		timeIndicator = "🟡"
	} else {
		timeIndicator = "  "
	}
	timeBar := v.renderWideProgressBar(timePercentage, "")
	hours := int(timeRemaining / 60)
	mins := int(timeRemaining) % 60
	lines = append(lines, fmt.Sprintf("⏱️  Time to Reset:       %s %s %dh %dm",
		timeIndicator, timeBar, hours, mins))
	lines = append(lines, "")

	// Model Distribution
	modelBar := v.renderModelDistributionSimple()
	lines = append(lines, fmt.Sprintf("🤖 Model Distribution:   🤖 %s", modelBar))
	lines = append(lines, separatorStyle.Render(strings.Repeat("─", 60)))

	// Burn Rate with arrow emoji
	lines = append(lines, fmt.Sprintf("🔥 Burn Rate:              %.1f tokens/min ➡️", burnRate))

	// Cost Rate
	costRate := v.calculateCostRate()
	lines = append(lines, fmt.Sprintf("💲 Cost Rate:              $%.4f $/min", costRate))

	// Predictions
	lines = append(lines, "")
	lines = append(lines, "🔮 Predictions:")

	// Calculate when tokens will run out
	if burnRate > 0 {
		minutesUntilOut := float64(v.tokenLimit-v.metrics.CurrentTokens) / burnRate
		runOutTime := time.Now().Add(time.Duration(minutesUntilOut) * time.Minute)
		lines = append(lines, fmt.Sprintf("   Tokens will run out: %s", v.formatTimeShort(runOutTime)))
	} else {
		lines = append(lines, "   Tokens will run out: --:--")
	}

	// Reset time
	resetTime := sessionStart.Add(5 * time.Hour)
	lines = append(lines, fmt.Sprintf("   Limit resets at:     %s", v.formatTimeShort(resetTime)))
	lines = append(lines, "")

	// Check if cost limit will be exceeded before reset
	if burnRate > 0 && costRate > 0 {
		// Calculate if cost limit will be exceeded before session reset
		minutesUntilCostLimit := (v.costLimitP90 - v.metrics.CurrentCost) / costRate
		minutesUntilReset := timeRemaining

		if minutesUntilCostLimit < minutesUntilReset {
			lines = append(lines, "⏰ Cost limit will be exceeded before reset!")
		}
	}

	return lines
}

// renderFooter renders the footer
func (v *MonitorView) renderFooter() string {
	currentTime := v.formatTime(time.Now())

	statusIcon := "🟨" // Yellow for no active session
	statusText := "No active session"
	statusColor := lipgloss.Color("#FFAA00")

	// Check for active sessions based on blocks
	hasActiveSession := false
	if v.blocks != nil {
		for _, block := range v.blocks {
			if block.IsActive {
				hasActiveSession = true
				break
			}
		}
	}

	if hasActiveSession {
		statusIcon = "🟢" // Green for active session
		statusText = "Active session"
		statusColor = lipgloss.Color("#00FF00")
	}

	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	statusStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)
	exitStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Italic(true)

	return fmt.Sprintf("⏰ %s 📝 %s | %s %s",
		timeStyle.Render(currentTime),
		statusStyle.Render(statusText),
		exitStyle.Render("Ctrl+C to exit"),
		statusIcon)
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

	// Create color styles based on percentage
	var barStyle lipgloss.Style
	if percentage >= 80 {
		barStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	} else if percentage >= 50 {
		barStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
	} else {
		barStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	}

	// Use filled blocks and dotted pattern for empty space
	filledBar := barStyle.Render(strings.Repeat("█", filled))
	emptyBar := lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render(strings.Repeat("⋯", width-filled))
	bar := filledBar + emptyBar

	if colorIndicator == "" {
		return fmt.Sprintf("[%s]", bar)
	}
	return fmt.Sprintf("%s [%s]", colorIndicator, bar)
}

// renderModelDistributionSimple renders a simplified model distribution for the main view
func (v *MonitorView) renderModelDistributionSimple() string {
	if v.metrics == nil || len(v.metrics.ModelDistribution) == 0 {
		return "[No model data]"
	}

	// Find the dominant model
	maxModel := ""
	maxPercentage := 0.0

	for model, metrics := range v.metrics.ModelDistribution {
		percentage := 0.0
		if v.metrics.CurrentTokens > 0 {
			percentage = float64(metrics.TokenCount) / float64(v.metrics.CurrentTokens) * 100
		}
		if percentage > maxPercentage {
			maxPercentage = percentage
			maxModel = model
		}
	}

	// Get model display name
	displayName := "Unknown"
	if strings.Contains(maxModel, "opus") {
		displayName = "Opus"
	} else if strings.Contains(maxModel, "sonnet") {
		displayName = "Sonnet"
	} else if strings.Contains(maxModel, "haiku") {
		displayName = "Haiku"
	}

	// Create the progress bar
	width := 50
	filled := int(maxPercentage * float64(width) / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("⋯", width-filled)

	return fmt.Sprintf("[%s] %s %.1f%%", bar, displayName, maxPercentage)
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

// formatNumberWithCommas formats numbers with commas for thousands
func (v *MonitorView) formatNumberWithCommas(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Convert to string and add commas
	str := fmt.Sprintf("%d", n)
	result := ""
	for i, digit := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(digit)
	}
	return result
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

// formatTimeShort formats time in short format (HH:MM)
func (v *MonitorView) formatTimeShort(t time.Time) string {
	// Convert to configured timezone
	loc, err := time.LoadLocation(v.timezone)
	if err != nil {
		loc = time.UTC
	}
	t = t.In(loc)

	if v.timeFormat == "24h" {
		return t.Format("15:04")
	}
	return t.Format("3:04 PM")
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
