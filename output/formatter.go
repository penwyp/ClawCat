package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/penwyp/claudecat/calculations"
	"github.com/penwyp/claudecat/models"
)

// ConsoleFormatter formats data for console output
type ConsoleFormatter struct {
	plan             string
	timezone         string
	timeFormat       string
	tokenLimit       int
	costLimitP90     float64
	messagesLimitP90 int
	p90Calculator    *calculations.P90Calculator
}

// NewConsoleFormatter creates a new console formatter
func NewConsoleFormatter(plan, timezone, timeFormat string) *ConsoleFormatter {
	if timezone == "" || timezone == "auto" {
		timezone = "Asia/Shanghai"
	}
	if timeFormat == "" || timeFormat == "auto" {
		timeFormat = "24h"
	}

	return &ConsoleFormatter{
		plan:          strings.ToLower(plan),
		timezone:      timezone,
		timeFormat:    timeFormat,
		p90Calculator: calculations.NewP90Calculator(),
	}
}

// Format formats the monitoring data for console output
func (f *ConsoleFormatter) Format(metrics *calculations.RealtimeMetrics, blocks []models.SessionBlock) string {
	f.updateLimits(blocks)

	var lines []string
	lines = append(lines, f.renderHeader()...)
	lines = append(lines, "")

	// Check if there's an active session
	hasActiveSession := false
	if blocks != nil {
		for _, block := range blocks {
			if block.IsActive {
				hasActiveSession = true
				break
			}
		}
	}

	if hasActiveSession && metrics != nil {
		lines = append(lines, f.renderActiveSession(metrics, blocks)...)
	} else {
		lines = append(lines, f.renderNoActiveSession(metrics, blocks)...)
	}

	lines = append(lines, f.renderFooter(hasActiveSession))

	return strings.Join(lines, "\n")
}

// renderHeader renders the header section
func (f *ConsoleFormatter) renderHeader() []string {
	sparkles := "✦ ✧ ✦ ✧"
	title := "CLAUDE CODE USAGE MONITOR"
	separator := strings.Repeat("=", 60)

	plan := f.plan
	if plan == "" {
		plan = "pro"
	}

	return []string{
		fmt.Sprintf("%s %s %s", sparkles, title, sparkles),
		separator,
		fmt.Sprintf("[ %s | %s ]", plan, strings.ToLower(f.timezone)),
	}
}

// renderNoActiveSession renders the display when there's no active session
func (f *ConsoleFormatter) renderNoActiveSession(metrics *calculations.RealtimeMetrics, blocks []models.SessionBlock) []string {
	var lines []string

	// Show metrics from the most recent session if available
	tokensUsed := 0
	costUsed := 0.0
	messagesUsed := 0

	if metrics != nil {
		tokensUsed = metrics.CurrentTokens
		costUsed = metrics.CurrentCost
		// Get message count from metrics or recent inactive session
		if len(blocks) > 0 {
			// Find the most recent session (active or not)
			for i := len(blocks) - 1; i >= 0; i-- {
				if !blocks[i].IsGap {
					messagesUsed = blocks[i].SentMessagesCount
					break
				}
			}
		}
	}

	// Calculate usage percentage
	tokenUsage := 0.0
	if f.tokenLimit > 0 && tokensUsed > 0 {
		tokenUsage = float64(tokensUsed) / float64(f.tokenLimit) * 100
	}

	// Progress bar
	progressBar := f.renderWideProgressBar(tokenUsage, "🟨")
	lines = append(lines, fmt.Sprintf("📊 Token Usage:    %s", progressBar))
	lines = append(lines, "")

	// Stats - show actual values if any tokens were used
	if tokensUsed > 0 {
		lines = append(lines, fmt.Sprintf("🎯 Tokens:         %s / ~%s (%s left)",
			f.formatNumber(tokensUsed),
			f.formatNumber(f.tokenLimit),
			f.formatNumber(f.tokenLimit-tokensUsed)))
		lines = append(lines, fmt.Sprintf("💲 Session Cost:   $%.2f", costUsed))
		lines = append(lines, fmt.Sprintf("📨 Sent Messages:  %d messages", messagesUsed))
	} else {
		lines = append(lines, fmt.Sprintf("🎯 Tokens:         0 / ~%s (0 left)", f.formatNumber(f.tokenLimit)))
		lines = append(lines, "💲 Session Cost:   $0.00")
		lines = append(lines, "📨 Sent Messages:  0 messages")
	}

	lines = append(lines, "🔥 Burn Rate:      0.0 tokens/min")
	lines = append(lines, "💵 Cost Rate:      $0.00 $/min")
	lines = append(lines, "")

	return lines
}

// renderActiveSession renders the display for an active session
func (f *ConsoleFormatter) renderActiveSession(metrics *calculations.RealtimeMetrics, blocks []models.SessionBlock) []string {
	var lines []string

	// Calculate burn rate
	burnRate := f.calculateBurnRate(blocks)

	// Calculate percentages
	tokenUsage := float64(metrics.CurrentTokens) / float64(f.tokenLimit) * 100
	costUsage := metrics.CurrentCost / f.costLimitP90 * 100

	// Get message count from current active session
	messageCount := 0
	if len(blocks) > 0 {
		for _, block := range blocks {
			if block.IsActive {
				messageCount = block.SentMessagesCount
				break
			}
		}
	}
	messagesUsage := float64(messageCount) / float64(f.messagesLimitP90) * 100

	// Time calculations
	sessionStart := metrics.SessionStart
	if sessionStart.IsZero() && len(blocks) > 0 {
		for _, block := range blocks {
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

	lines = append(lines, "")
	lines = append(lines, "")

	// Cost Usage
	costIndicator := f.getColorIndicator(costUsage)
	costBar := f.renderWideProgressBar(costUsage, "")
	lines = append(lines, fmt.Sprintf("💰 Cost Usage:           %s %s %5.1f%%    $%.2f / $%.2f",
		costIndicator, costBar, costUsage, metrics.CurrentCost, f.costLimitP90))
	lines = append(lines, "")

	// Token Usage
	tokenIndicator := f.getColorIndicator(tokenUsage)
	tokenBar := f.renderWideProgressBar(tokenUsage, "")
	lines = append(lines, fmt.Sprintf("📊 Token Usage:          %s %s %5.1f%%    %s / %s",
		tokenIndicator, tokenBar, tokenUsage,
		f.formatNumberWithCommas(metrics.CurrentTokens),
		f.formatNumberWithCommas(f.tokenLimit)))
	lines = append(lines, "")

	// Messages Usage
	messagesIndicator := f.getColorIndicator(messagesUsage)
	messagesBar := f.renderWideProgressBar(messagesUsage, "")
	lines = append(lines, fmt.Sprintf("📨 Messages Usage:       %s %s %5.1f%%    %d / %s",
		messagesIndicator, messagesBar, messagesUsage, messageCount,
		f.formatNumberWithCommas(f.messagesLimitP90)))
	lines = append(lines, strings.Repeat("─", 60))

	// Time to Reset
	timeIndicator := f.getColorIndicator(timePercentage)
	timeBar := f.renderWideProgressBar(timePercentage, "")
	hours := int(timeRemaining / 60)
	mins := int(timeRemaining) % 60
	lines = append(lines, fmt.Sprintf("⏱️  Time to Reset:       %s %s %dh %dm",
		timeIndicator, timeBar, hours, mins))
	lines = append(lines, "")

	// Model Distribution
	modelBar := f.renderModelDistributionSimple(metrics)
	lines = append(lines, fmt.Sprintf("🤖 Model Distribution:   🤖 %s", modelBar))
	lines = append(lines, strings.Repeat("─", 60))

	// Burn Rate with appropriate emoji
	emoji := "🐌"
	if burnRate > 100 {
		emoji = "🚀"
	} else if burnRate > 50 {
		emoji = "🏃"
	}
	lines = append(lines, fmt.Sprintf("🔥 Burn Rate:              %.1f tokens/min %s", burnRate, emoji))

	// Cost Rate
	costRate := f.calculateCostRate(metrics)
	lines = append(lines, fmt.Sprintf("💲 Cost Rate:              $%.4f $/min", costRate))

	lines = append(lines, "")
	lines = append(lines, "🔮 Predictions:")

	// Calculate when tokens will run out
	if burnRate > 0 {
		minutesUntilOut := float64(f.tokenLimit-metrics.CurrentTokens) / burnRate
		runOutTime := time.Now().Add(time.Duration(minutesUntilOut) * time.Minute)
		lines = append(lines, fmt.Sprintf("   Tokens will run out: %s", f.formatTimeShort(runOutTime)))
	} else {
		lines = append(lines, "   Tokens will run out: --:--")
	}

	// Reset time
	resetTime := sessionStart.Add(5 * time.Hour)
	lines = append(lines, fmt.Sprintf("   Limit resets at:     %s", f.formatTimeShort(resetTime)))
	lines = append(lines, "")

	return lines
}

// renderFooter renders the footer
func (f *ConsoleFormatter) renderFooter(hasActiveSession bool) string {
	currentTime := f.formatTime(time.Now())
	
	statusText := "No active session"
	if hasActiveSession {
		statusText = "Active session"
	}

	return fmt.Sprintf("⏰ %s 📝 %s", currentTime, statusText)
}

// renderWideProgressBar renders a 50-character wide progress bar
func (f *ConsoleFormatter) renderWideProgressBar(percentage float64, colorIndicator string) string {
	width := 50
	filled := int(percentage * float64(width) / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	// Use filled blocks and empty blocks
	filledBar := strings.Repeat("█", filled)
	emptyBar := strings.Repeat("░", width-filled)
	bar := filledBar + emptyBar

	if colorIndicator == "" {
		return fmt.Sprintf("[%s]", bar)
	}
	return fmt.Sprintf("%s [%s]", colorIndicator, bar)
}

// renderModelDistributionSimple renders a simplified model distribution
func (f *ConsoleFormatter) renderModelDistributionSimple(metrics *calculations.RealtimeMetrics) string {
	if metrics == nil || len(metrics.ModelDistribution) == 0 {
		return "[No model data]"
	}

	// Find the dominant model
	maxModel := ""
	maxPercentage := 0.0

	for model, modelMetrics := range metrics.ModelDistribution {
		percentage := 0.0
		if metrics.CurrentTokens > 0 {
			percentage = float64(modelMetrics.TokenCount) / float64(metrics.CurrentTokens) * 100
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

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	return fmt.Sprintf("[%s] %s %.1f%%", bar, displayName, maxPercentage)
}

// getColorIndicator returns the appropriate color indicator based on percentage
func (f *ConsoleFormatter) getColorIndicator(percentage float64) string {
	if percentage < 50 {
		return "🟢"
	} else if percentage < 80 {
		return "🟡"
	} else {
		return "🔴"
	}
}

// calculateBurnRate calculates the current burn rate in tokens/min
func (f *ConsoleFormatter) calculateBurnRate(blocks []models.SessionBlock) float64 {
	if blocks == nil || len(blocks) == 0 {
		return 0.0
	}

	// Use the burn rate calculator to get hourly burn rate
	calculator := calculations.NewBurnRateCalculator()
	return calculator.CalculateHourlyBurnRate(blocks, time.Now())
}

// calculateCostRate calculates the cost rate in $/min
func (f *ConsoleFormatter) calculateCostRate(metrics *calculations.RealtimeMetrics) float64 {
	if metrics == nil || metrics.SessionStart.IsZero() {
		return 0.0
	}

	elapsed := time.Since(metrics.SessionStart).Minutes()
	if elapsed <= 0 {
		return 0.0
	}

	return metrics.CurrentCost / elapsed
}

// formatNumber formats large numbers with K/M suffixes
func (f *ConsoleFormatter) formatNumber(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.0fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.0fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// formatNumberWithCommas formats numbers with commas for thousands
func (f *ConsoleFormatter) formatNumberWithCommas(n int) string {
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
func (f *ConsoleFormatter) formatTime(t time.Time) string {
	// Convert to configured timezone
	loc, err := time.LoadLocation(f.timezone)
	if err != nil {
		loc = time.UTC
	}
	t = t.In(loc)

	if f.timeFormat == "24h" {
		return t.Format("15:04:05")
	}
	return t.Format("3:04:05 PM")
}

// formatTimeShort formats time in short format (HH:MM)
func (f *ConsoleFormatter) formatTimeShort(t time.Time) string {
	// Convert to configured timezone
	loc, err := time.LoadLocation(f.timezone)
	if err != nil {
		loc = time.UTC
	}
	t = t.In(loc)

	if f.timeFormat == "24h" {
		return t.Format("15:04")
	}
	return t.Format("3:04 PM")
}

// updateLimits updates the limits based on plan or P90 calculations
func (f *ConsoleFormatter) updateLimits(blocks []models.SessionBlock) {
	// Calculate P90 limits if on custom plan
	if f.plan == "custom" && f.p90Calculator != nil {
		f.tokenLimit = f.p90Calculator.CalculateP90Limit(blocks, true)
		f.costLimitP90 = f.p90Calculator.GetCostP90(blocks)
		f.messagesLimitP90 = f.p90Calculator.GetMessagesP90(blocks)
	} else {
		// Set fixed limits based on plan
		switch f.plan {
		case "pro":
			f.tokenLimit = 1000000
			f.costLimitP90 = 18.0
			f.messagesLimitP90 = 1500
		case "max5":
			f.tokenLimit = 88000
			f.costLimitP90 = 35.0
			f.messagesLimitP90 = 1000
		case "max20":
			f.tokenLimit = 8000000
			f.costLimitP90 = 140.0
			f.messagesLimitP90 = 12000
		default:
			f.tokenLimit = 1000000
			f.costLimitP90 = 18.0
			f.messagesLimitP90 = 1500
		}
	}
}