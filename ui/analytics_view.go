package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/claudecat/calculations"
	"github.com/penwyp/claudecat/models"
	"github.com/penwyp/claudecat/sessions"
)

// AnalyticsView represents the analytics view (combined monitor + analytics)
type AnalyticsView struct {
	sessions []*sessions.Session
	entries  []models.UsageEntry
	blocks   []models.SessionBlock
	monitor  *MonitorView // Embedded monitor view
	width    int
	height   int
	config   Config
	styles   Styles
	metrics  *calculations.RealtimeMetrics
}

// NewAnalyticsView creates a new analytics view
func NewAnalyticsView() *AnalyticsView {
	return &AnalyticsView{
		styles:  NewStyles(DefaultTheme()),
		monitor: NewMonitorView(Config{}), // Create embedded monitor view
	}
}

// Init initializes the analytics view
func (a *AnalyticsView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the analytics view
func (a *AnalyticsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle analytics-specific messages
	return a, nil
}

// View renders the analytics view (combined monitor + analytics)
func (a *AnalyticsView) View() string {
	if a.width == 0 || a.height == 0 {
		return "Analytics loading..."
	}

	// Calculate height distribution (60% monitor, 40% analytics)
	monitorHeight := int(float64(a.height) * 0.6)
	analyticsHeight := a.height - monitorHeight - 2 // -2 for separator

	// Get monitor view content
	monitorContent := ""
	if a.monitor != nil {
		a.monitor.Resize(a.width, monitorHeight)
		monitorContent = a.monitor.View()
	}

	// Create separator
	separator := strings.Repeat("─", a.width)

	// Get analytics content
	analyticsContent := a.renderAnalyticsSection(analyticsHeight)

	// Combine both views
	return strings.Join([]string{
		monitorContent,
		separator,
		analyticsContent,
	}, "\n")
}

// UpdateData updates the analytics with new data
func (a *AnalyticsView) UpdateData(sessions []*sessions.Session, entries []models.UsageEntry) {
	a.sessions = sessions
	a.entries = entries

	// Also update monitor view if we have blocks
	if a.monitor != nil && len(a.blocks) > 0 {
		a.monitor.UpdateBlocks(a.blocks)
	}
}

// UpdateBlocks updates session blocks for the embedded monitor view
func (a *AnalyticsView) UpdateBlocks(blocks []models.SessionBlock) {
	a.blocks = blocks
	if a.monitor != nil {
		a.monitor.UpdateBlocks(blocks)
	}
}

// UpdateMetrics updates realtime metrics for the embedded monitor view
func (a *AnalyticsView) UpdateMetrics(metrics *calculations.RealtimeMetrics) {
	a.metrics = metrics
	if a.monitor != nil {
		a.monitor.UpdateMetrics(metrics)
	}
}

// UpdateStats updates statistics for the embedded monitor view
func (a *AnalyticsView) UpdateStats(stats Statistics) {
	if a.monitor != nil {
		a.monitor.UpdateStats(stats)
	}
}

// Resize updates the view dimensions
func (a *AnalyticsView) Resize(width, height int) {
	a.width = width
	a.height = height
}

// UpdateConfig updates the view configuration
func (a *AnalyticsView) UpdateConfig(config Config) {
	a.config = config
	a.styles = NewStyles(GetThemeByName(config.Theme))

	// Also update monitor view config
	if a.monitor != nil {
		a.monitor.UpdateConfig(config)
	}
}

// renderAnalyticsSection renders the analytics portion of the combined view
func (a *AnalyticsView) renderAnalyticsSection(height int) string {
	header := a.renderHeader()
	charts := a.renderCharts()
	stats := a.renderStats()

	content := strings.Join([]string{
		header,
		charts,
		stats,
	}, "\n\n")

	// Limit content to available height
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// renderHeader renders the analytics header
func (a *AnalyticsView) renderHeader() string {
	title := a.styles.Title.Render("Analytics Section")
	subtitle := a.styles.Subtitle.Render(
		fmt.Sprintf("Analyzing %d sessions and %d entries",
			len(a.sessions), len(a.entries)),
	)
	return strings.Join([]string{title, subtitle}, "\n")
}

// renderCharts renders analytics charts
func (a *AnalyticsView) renderCharts() string {
	// Simple text-based charts for now
	modelDist := a.renderModelDistribution()
	costTrend := a.renderCostTrend()

	return strings.Join([]string{
		a.styles.Subtitle.Render("Model Distribution:"),
		modelDist,
		a.styles.Subtitle.Render("Cost Trend:"),
		costTrend,
	}, "\n")
}

// renderStats renders summary statistics
func (a *AnalyticsView) renderStats() string {
	if len(a.entries) == 0 {
		return a.styles.Muted.Render("No data available for analysis")
	}

	totalTokens := int64(0)
	totalCost := 0.0
	modelCounts := make(map[string]int)

	for _, entry := range a.entries {
		totalTokens += int64(entry.TotalTokens)
		totalCost += entry.CostUSD
		modelCounts[entry.Model]++
	}

	avgCost := totalCost / float64(len(a.entries))
	avgTokens := float64(totalTokens) / float64(len(a.entries))

	stats := []string{
		fmt.Sprintf("Total Entries: %d", len(a.entries)),
		fmt.Sprintf("Total Tokens: %s", a.formatNumber(totalTokens)),
		fmt.Sprintf("Total Cost: $%.2f", totalCost),
		fmt.Sprintf("Avg Cost/Entry: $%.4f", avgCost),
		fmt.Sprintf("Avg Tokens/Entry: %.0f", avgTokens),
		fmt.Sprintf("Models Used: %d", len(modelCounts)),
	}

	return strings.Join(stats, "\n")
}

// renderModelDistribution renders model usage distribution
func (a *AnalyticsView) renderModelDistribution() string {
	if len(a.entries) == 0 {
		return a.styles.Muted.Render("No data")
	}

	modelCounts := make(map[string]int)
	for _, entry := range a.entries {
		modelCounts[entry.Model]++
	}

	var lines []string
	for model, count := range modelCounts {
		percentage := float64(count) / float64(len(a.entries)) * 100
		bar := a.renderSimpleBar(percentage, 20)
		lines = append(lines, fmt.Sprintf("%-15s %s %.1f%%",
			model, bar, percentage))
	}

	return strings.Join(lines, "\n")
}

// renderCostTrend renders a simple cost trend
func (a *AnalyticsView) renderCostTrend() string {
	if len(a.sessions) == 0 {
		return a.styles.Muted.Render("No data")
	}

	// Simple trend showing recent sessions
	var lines []string
	recentCount := 10
	if len(a.sessions) < recentCount {
		recentCount = len(a.sessions)
	}

	for i := len(a.sessions) - recentCount; i < len(a.sessions); i++ {
		session := a.sessions[i]
		// Calculate cost from entries
		cost := 0.0
		for _, entry := range session.Entries {
			cost += entry.CostUSD
		}
		bar := a.renderSimpleBar(cost*10, 15) // Scale for visualization
		lines = append(lines, fmt.Sprintf("%s %s $%.2f",
			session.StartTime.Format("15:04"), bar, cost))
	}

	return strings.Join(lines, "\n")
}

// renderSimpleBar renders a simple text-based bar
func (a *AnalyticsView) renderSimpleBar(value float64, maxWidth int) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}

	filled := int(value * float64(maxWidth) / 100)
	empty := maxWidth - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return a.styles.ChartBar.Render(bar)
}

// formatNumber formats large numbers with appropriate units
func (a *AnalyticsView) formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
