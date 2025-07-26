package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/claudecat/models"
	"github.com/penwyp/claudecat/sessions"
)

// AnalyticsView represents the analytics view
type AnalyticsView struct {
	sessions []*sessions.Session
	entries  []models.UsageEntry
	width    int
	height   int
	config   Config
	styles   Styles
}

// NewAnalyticsView creates a new analytics view
func NewAnalyticsView() *AnalyticsView {
	return &AnalyticsView{
		styles: NewStyles(DefaultTheme()),
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

// View renders the analytics view
func (a *AnalyticsView) View() string {
	if a.width == 0 || a.height == 0 {
		return "Analytics loading..."
	}

	header := a.renderHeader()
	charts := a.renderCharts()
	stats := a.renderStats()

	content := strings.Join([]string{
		header,
		charts,
		stats,
	}, "\n\n")

	return a.styles.Content.
		Width(a.width - 4).
		Height(a.height - 4).
		Render(content)
}

// UpdateData updates the analytics with new data
func (a *AnalyticsView) UpdateData(sessions []*sessions.Session, entries []models.UsageEntry) {
	a.sessions = sessions
	a.entries = entries
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
}

// renderHeader renders the analytics header
func (a *AnalyticsView) renderHeader() string {
	title := a.styles.Title.Render("Analytics")
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
