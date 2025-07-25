package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DashboardView represents the main dashboard view
type DashboardView struct {
	stats  Statistics
	width  int
	height int
	config Config
	styles Styles
}

// NewDashboardView creates a new dashboard view
func NewDashboardView() *DashboardView {
	return &DashboardView{
		styles: NewStyles(DefaultTheme()),
	}
}

// Init initializes the dashboard view
func (d *DashboardView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the dashboard view
func (d *DashboardView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle dashboard-specific messages
	return d, nil
}

// View renders the dashboard view
func (d *DashboardView) View() string {
	if d.width == 0 || d.height == 0 {
		return "Dashboard loading..."
	}

	// Create dashboard layout
	header := d.renderHeader()
	metrics := d.renderMetrics()
	charts := d.renderCharts()
	footer := d.renderFooter()

	content := strings.Join([]string{
		header,
		metrics,
		charts,
		footer,
	}, "\n\n")

	return d.styles.Content.
		Width(d.width - 4).
		Height(d.height - 4).
		Render(content)
}

// UpdateStats updates the dashboard with new statistics
func (d *DashboardView) UpdateStats(stats Statistics) {
	d.stats = stats
}

// Resize updates the dashboard dimensions
func (d *DashboardView) Resize(width, height int) {
	d.width = width
	d.height = height
}

// UpdateConfig updates the dashboard configuration
func (d *DashboardView) UpdateConfig(config Config) {
	d.config = config
	d.styles = NewStyles(GetThemeByName(config.Theme))
}

// renderHeader renders the dashboard header
func (d *DashboardView) renderHeader() string {
	title := d.styles.Title.Render("ClawCat Dashboard")
	subtitle := d.styles.Subtitle.Render(
		fmt.Sprintf("Last updated: %s", time.Now().Format("15:04:05")),
	)

	return strings.Join([]string{title, subtitle}, "\n")
}

// renderMetrics renders the key metrics cards
func (d *DashboardView) renderMetrics() string {
	// Active sessions card
	activeCard := d.renderMetricCard(
		"Active Sessions",
		fmt.Sprintf("%d", d.stats.ActiveSessions),
		d.styles.Success,
	)

	// Total tokens card
	tokensCard := d.renderMetricCard(
		"Total Tokens",
		d.formatNumber(d.stats.TotalTokens),
		d.styles.Info,
	)

	// Total cost card
	costCard := d.renderMetricCard(
		"Total Cost",
		fmt.Sprintf("$%.2f", d.stats.TotalCost),
		d.styles.Warning,
	)

	// Burn rate card
	burnRateCard := d.renderMetricCard(
		"Burn Rate",
		fmt.Sprintf("%.1f tok/hr", d.stats.CurrentBurnRate),
		d.styles.Normal,
	)

	// Arrange cards in a row
	return d.arrangeInRow([]string{
		activeCard,
		tokensCard,
		costCard,
		burnRateCard,
	})
}

// renderCharts renders dashboard charts
func (d *DashboardView) renderCharts() string {
	// Usage progress bar
	usageBar := d.renderUsageProgress()

	// Time to reset
	resetInfo := d.renderResetInfo()

	return strings.Join([]string{usageBar, resetInfo}, "\n\n")
}

// renderFooter renders the dashboard footer
func (d *DashboardView) renderFooter() string {
	status := fmt.Sprintf(
		"Sessions: %d | Top Model: %s | Avg Cost: $%.2f",
		d.stats.SessionCount,
		d.stats.TopModel,
		d.stats.AverageCost,
	)

	return d.styles.Footer.Render(status)
}

// renderMetricCard renders a single metric card
func (d *DashboardView) renderMetricCard(title, value string, style lipgloss.Style) string {
	cardTitle := d.styles.DashboardLabel().Render(title)
	cardValue := style.Render(value)

	content := strings.Join([]string{cardTitle, cardValue}, "\n")

	return d.styles.DashboardCard().Render(content)
}

// arrangeInRow arranges multiple elements in a horizontal row
func (d *DashboardView) arrangeInRow(elements []string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, elements...)
}

// renderUsageProgress renders the usage progress bar
func (d *DashboardView) renderUsageProgress() string {
	title := d.styles.Subtitle.Render("Plan Usage")

	progressWidth := d.width - 20
	if progressWidth < 20 {
		progressWidth = 20
	}

	progress := d.styles.ProgressStyle(d.stats.PlanUsage, progressWidth)
	percentage := fmt.Sprintf("%.1f%%", d.stats.PlanUsage)

	return strings.Join([]string{
		title,
		progress,
		percentage,
	}, "\n")
}

// renderResetInfo renders time to reset information
func (d *DashboardView) renderResetInfo() string {
	title := d.styles.Subtitle.Render("Time to Reset")

	days := int(d.stats.TimeToReset.Hours() / 24)
	hours := int(d.stats.TimeToReset.Hours()) % 24

	timeText := fmt.Sprintf("%dd %dh", days, hours)

	return strings.Join([]string{
		title,
		d.styles.Normal.Render(timeText),
	}, "\n")
}

// formatNumber formats large numbers with appropriate units
func (d *DashboardView) formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
