package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/penwyp/ClawCat/calculations"
	"github.com/penwyp/ClawCat/sessions"
)

// StreamingDisplay provides non-fullscreen terminal display for metrics
type StreamingDisplay struct {
	metrics    *calculations.RealtimeMetrics
	sessions   []*sessions.Session
	width      int
	lastUpdate time.Time
}

// NewStreamingDisplay creates a new streaming display component
func NewStreamingDisplay() *StreamingDisplay {
	return &StreamingDisplay{
		width: 80, // Default terminal width
	}
}

// SetMetrics updates the metrics data
func (sd *StreamingDisplay) SetMetrics(metrics *calculations.RealtimeMetrics) {
	sd.metrics = metrics
	sd.lastUpdate = time.Now()
}

// SetSessions updates session data
func (sd *StreamingDisplay) SetSessions(sessions []*sessions.Session) {
	sd.sessions = sessions
}

// SetWidth sets the display width
func (sd *StreamingDisplay) SetWidth(width int) {
	sd.width = width
}

// RenderHeader renders a compact metrics header
func (sd *StreamingDisplay) RenderHeader() string {
	if sd.metrics == nil {
		return sd.renderLoadingHeader()
	}

	timestamp := time.Now().Format("15:04:05")

	// Key metrics in compact format
	tokens := formatNumber(sd.metrics.CurrentTokens)
	cost := fmt.Sprintf("$%.2f", sd.metrics.CurrentCost)
	burnRate := fmt.Sprintf("%.0f/min", sd.metrics.TokensPerMinute)
	progress := fmt.Sprintf("%.0f%%", sd.metrics.SessionProgress)

	// Color-coded status
	status := sd.getStatusIndicator()

	header := fmt.Sprintf(
		"[%s] %s | 📊 %s tokens | 💰 %s | ⚡ %s | 🎯 %s | %s",
		timestamp, status, tokens, cost, burnRate, progress, sd.getHealthStatus(),
	)

	// Truncate if too long
	if len(header) > sd.width {
		header = header[:sd.width-3] + "..."
	}

	return sd.styleHeader(header)
}

// RenderInlineSummary renders a one-line summary update
func (sd *StreamingDisplay) RenderInlineSummary() string {
	if sd.metrics == nil {
		return "📊 ClawCat - Waiting for data..."
	}

	// Compact inline format similar to docker stats
	return fmt.Sprintf(
		"📊 %s tok │ 💰 $%.2f │ ⚡ %.0f/min │ 🎯 %.0f%% │ %s",
		formatNumber(sd.metrics.CurrentTokens),
		sd.metrics.CurrentCost,
		sd.metrics.TokensPerMinute,
		sd.metrics.SessionProgress,
		sd.getCompactHealth(),
	)
}

// RenderDetailedReport renders a multi-line detailed report
func (sd *StreamingDisplay) RenderDetailedReport() string {
	if sd.metrics == nil {
		return "No metrics data available"
	}

	var report strings.Builder

	// Header
	report.WriteString("═══ ClawCat Metrics Report ═══\n")
	report.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	report.WriteString("\n")

	// Core metrics
	report.WriteString("📊 Usage Statistics:\n")
	report.WriteString(fmt.Sprintf("  Tokens:     %s (projected: %s)\n",
		formatNumber(sd.metrics.CurrentTokens),
		formatNumber(sd.metrics.ProjectedTokens)))
	report.WriteString(fmt.Sprintf("  Cost:       $%.2f (projected: $%.2f)\n",
		sd.metrics.CurrentCost, sd.metrics.ProjectedCost))
	report.WriteString(fmt.Sprintf("  Progress:   %.1f%% complete\n", sd.metrics.SessionProgress))
	report.WriteString("\n")

	// Performance metrics
	report.WriteString("⚡ Performance:\n")
	report.WriteString(fmt.Sprintf("  Burn rate:  %.1f tokens/min\n", sd.metrics.BurnRate))
	report.WriteString(fmt.Sprintf("  Cost rate:  $%.2f/hour\n", sd.metrics.CostPerHour))
	report.WriteString(fmt.Sprintf("  Efficiency: %.1f tokens/request\n", sd.getTokensPerRequest()))
	report.WriteString("\n")

	// Session info
	activeSessions := sd.getActiveSessions()
	report.WriteString(fmt.Sprintf("🔄 Sessions: %d active, %d total\n",
		activeSessions, len(sd.sessions)))

	// Model distribution (top 3)
	if len(sd.metrics.ModelDistribution) > 0 {
		report.WriteString("\n🤖 Top Models:\n")
		topModels := sd.getTopModels(3)
		for _, model := range topModels {
			report.WriteString(fmt.Sprintf("  %s: %.1f%% (%s tokens)\n",
				truncateString(model.Name, 15), model.Percentage,
				formatNumber(model.TokenCount)))
		}
	}

	// Time remaining and predictions
	if !sd.metrics.PredictedEndTime.IsZero() {
		report.WriteString(fmt.Sprintf("\n⏱️  Est. completion: %s (%.0f%% confidence)\n",
			sd.metrics.PredictedEndTime.Format("15:04"), sd.metrics.ConfidenceLevel))
	}

	report.WriteString("\n" + strings.Repeat("═", 50))

	return report.String()
}

// RenderProgressBar renders a terminal-friendly progress bar
func (sd *StreamingDisplay) RenderProgressBar() string {
	if sd.metrics == nil {
		return ""
	}

	barWidth := sd.width - 20 // Leave space for labels
	if barWidth < 10 {
		barWidth = 10
	}

	filled := int((sd.metrics.SessionProgress / 100.0) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	return fmt.Sprintf("Progress [%s] %.1f%%", bar, sd.metrics.SessionProgress)
}

// Helper methods

func (sd *StreamingDisplay) renderLoadingHeader() string {
	timestamp := time.Now().Format("15:04:05")
	return sd.styleHeader(fmt.Sprintf("[%s] 📊 ClawCat - Loading...", timestamp))
}

func (sd *StreamingDisplay) getStatusIndicator() string {
	if sd.metrics == nil {
		return "⏳"
	}

	if sd.metrics.BurnRate > 200 {
		return "🔴 HIGH"
	} else if sd.metrics.BurnRate > 100 {
		return "🟡 MED"
	}
	return "🟢 NORM"
}

func (sd *StreamingDisplay) getHealthStatus() string {
	activeSessions := sd.getActiveSessions()
	if activeSessions == 0 {
		return "💤 idle"
	}
	return fmt.Sprintf("🔄 %d active", activeSessions)
}

func (sd *StreamingDisplay) getCompactHealth() string {
	activeSessions := sd.getActiveSessions()
	if activeSessions == 0 {
		return "💤"
	}
	return fmt.Sprintf("🔄%d", activeSessions)
}

func (sd *StreamingDisplay) getActiveSessions() int {
	active := 0
	for _, session := range sd.sessions {
		if session.IsActive {
			active++
		}
	}
	return active
}

func (sd *StreamingDisplay) getTokensPerRequest() float64 {
	if sd.metrics == nil || len(sd.sessions) == 0 {
		return 0
	}

	totalRequests := 0
	for _, session := range sd.sessions {
		totalRequests += len(session.Entries)
	}

	if totalRequests == 0 {
		return 0
	}

	return float64(sd.metrics.CurrentTokens) / float64(totalRequests)
}

func (sd *StreamingDisplay) styleHeader(text string) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)
	return style.Render(text)
}

type ModelInfo struct {
	Name       string
	TokenCount int
	Percentage float64
}

func (sd *StreamingDisplay) getTopModels(limit int) []ModelInfo {
	if sd.metrics == nil {
		return nil
	}

	var models []ModelInfo
	for model, metrics := range sd.metrics.ModelDistribution {
		models = append(models, ModelInfo{
			Name:       model,
			TokenCount: metrics.TokenCount,
			Percentage: metrics.Percentage,
		})
	}

	// Sort by percentage (simple bubble sort for small lists)
	for i := 0; i < len(models)-1; i++ {
		for j := 0; j < len(models)-i-1; j++ {
			if models[j].Percentage < models[j+1].Percentage {
				models[j], models[j+1] = models[j+1], models[j]
			}
		}
	}

	if len(models) > limit {
		models = models[:limit]
	}

	return models
}

// End of StreamingDisplay implementation
