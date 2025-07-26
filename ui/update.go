package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/claudecat/calculations"
	"github.com/penwyp/claudecat/models"
	"github.com/penwyp/claudecat/sessions"
)

// Message types for the application

// TickMsg is sent periodically to trigger updates
type TickMsg time.Time

// DataUpdateMsg carries updated data to the UI
type DataUpdateMsg struct {
	Sessions []*sessions.Session
	Entries  []models.UsageEntry
}

// ConfigUpdateMsg carries updated configuration
type ConfigUpdateMsg struct {
	Config Config
}

// ViewChangeMsg requests a view change
type ViewChangeMsg ViewType

// RealtimeMetricsMsg carries updated realtime metrics
type RealtimeMetricsMsg struct {
	Metrics *calculations.RealtimeMetrics
}

// SessionBlocksMsg carries session blocks for monitor view
type SessionBlocksMsg struct {
	Blocks []models.SessionBlock
}

// RefreshRequestMsg requests a data refresh
type RefreshRequestMsg struct{}

// ErrorMsg carries error information
type ErrorMsg struct {
	Error error
}

// StatusMsg carries status updates
type StatusMsg struct {
	Message string
	Level   StatusLevel
}

// StatusLevel represents the importance of a status message
type StatusLevel int

const (
	StatusInfo StatusLevel = iota
	StatusWarning
	StatusError
	StatusSuccess
)

// ForceReadyMsg is sent to force the UI into ready state
type ForceReadyMsg struct{}

// Update handles incoming messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Handle window resize
		m.Resize(msg.Width, msg.Height)
		// Always ensure ready state on window size
		if !m.ready {
			m.SetReady(true)
		}
		return m, nil

	case tea.KeyMsg:
		// Handle keyboard input
		return m.handleKeyPress(msg)

	case TickMsg:
		// Periodic update tick
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)

		// Request data refresh if we have a manager
		if m.manager != nil {
			cmds = append(cmds, refreshDataCmd(m.manager))
		}

		// Schedule next tick
		cmds = append(cmds, tickCmd())

		return m, tea.Batch(cmds...)

	case DataUpdateMsg:
		// Update data from manager
		m.UpdateSessions(msg.Sessions)
		m.UpdateEntries(msg.Entries)
		// Clear loading state when we receive data
		if m.loading && len(msg.Sessions) > 0 {
			m.SetLoading(false)
		}
		return m, nil

	case RealtimeMetricsMsg:
		// Update realtime metrics
		m.UpdateRealtimeMetrics(msg.Metrics)
		return m, nil

	case SessionBlocksMsg:
		// Update session blocks
		m.blocks = msg.Blocks
		if m.monitor != nil {
			m.monitor.UpdateBlocks(msg.Blocks)
		}
		return m, nil

	case ConfigUpdateMsg:
		// Update configuration
		m.config = msg.Config
		m.styles = NewStyles(getThemeByName(msg.Config.Theme))

		// Update all views with new config
		if m.dashboard != nil {
			m.dashboard.UpdateConfig(msg.Config)
		}
		if m.sessionList != nil {
			m.sessionList.UpdateConfig(msg.Config)
		}
		if m.analytics != nil {
			m.analytics.UpdateConfig(msg.Config)
		}
		if m.monitor != nil {
			m.monitor.UpdateConfig(msg.Config)
		}

		return m, nil

	case ViewChangeMsg:
		// Change view
		m.SwitchView(ViewType(msg))
		return m, nil

	case RefreshRequestMsg:
		// Manual refresh request
		m.SetLoading(true)
		if m.manager != nil {
			cmd = refreshDataCmd(m.manager)
		}
		return m, cmd

	case ErrorMsg:
		// Handle errors
		// Could display error in status bar or as a popup
		return m, nil

	case StatusMsg:
		// Handle status messages
		// Could display in status bar
		return m, nil

	case ForceReadyMsg:
		// Force ready state after timeout
		if !m.ready {
			m.SetReady(true)
		}
		return m, nil

	default:
		// Handle messages for current view
		return m.handleViewUpdate(msg)
	}
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Global key bindings
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "?", "h":
		m.SwitchView(ViewHelp)
		return m, nil

	case "tab":
		m.NextView()
		return m, nil

	case "1", "d":
		m.SwitchView(ViewDashboard)
		return m, nil

	case "2", "s":
		m.SwitchView(ViewSessions)
		return m, nil

	case "3", "a":
		m.SwitchView(ViewAnalytics)
		return m, nil

	case "4", "m":
		m.SwitchView(ViewMonitor)
		return m, nil

	case "r", "f5":
		m.SetLoading(true)
		if m.manager != nil {
			cmd = refreshDataCmd(m.manager)
		}
		return m, cmd
	}

	// View-specific key bindings
	switch m.view {
	case ViewDashboard:
		if m.dashboard != nil {
			updatedView, viewCmd := m.dashboard.Update(msg)
			m.dashboard = updatedView.(*EnhancedDashboardView)
			cmd = viewCmd
		}

	case ViewSessions:
		if m.sessionList != nil {
			updatedView, viewCmd := m.sessionList.Update(msg)
			m.sessionList = updatedView.(*SessionListView)
			cmd = viewCmd
		}

	case ViewAnalytics:
		if m.analytics != nil {
			updatedView, viewCmd := m.analytics.Update(msg)
			m.analytics = updatedView.(*AnalyticsView)
			cmd = viewCmd
		}

	case ViewHelp:
		if m.help != nil {
			updatedView, viewCmd := m.help.Update(msg)
			m.help = updatedView.(*HelpView)
			cmd = viewCmd
		}

	case ViewMonitor:
		if m.monitor != nil {
			updatedView, viewCmd := m.monitor.Update(msg)
			m.monitor = updatedView.(*MonitorView)
			cmd = viewCmd
		}
	}

	return m, cmd
}

// handleViewUpdate delegates updates to the appropriate view
func (m Model) handleViewUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Update spinner for all views
	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)

	// Update current view
	switch m.view {
	case ViewDashboard:
		if m.dashboard != nil {
			updatedView, viewCmd := m.dashboard.Update(msg)
			m.dashboard = updatedView.(*EnhancedDashboardView)
			cmd = tea.Batch(spinnerCmd, viewCmd)
		} else {
			cmd = spinnerCmd
		}

	case ViewSessions:
		if m.sessionList != nil {
			updatedView, viewCmd := m.sessionList.Update(msg)
			m.sessionList = updatedView.(*SessionListView)
			cmd = tea.Batch(spinnerCmd, viewCmd)
		} else {
			cmd = spinnerCmd
		}

	case ViewAnalytics:
		if m.analytics != nil {
			updatedView, viewCmd := m.analytics.Update(msg)
			m.analytics = updatedView.(*AnalyticsView)
			cmd = tea.Batch(spinnerCmd, viewCmd)
		} else {
			cmd = spinnerCmd
		}

	case ViewHelp:
		if m.help != nil {
			updatedView, viewCmd := m.help.Update(msg)
			m.help = updatedView.(*HelpView)
			cmd = tea.Batch(spinnerCmd, viewCmd)
		} else {
			cmd = spinnerCmd
		}

	case ViewMonitor:
		if m.monitor != nil {
			updatedView, viewCmd := m.monitor.Update(msg)
			m.monitor = updatedView.(*MonitorView)
			cmd = tea.Batch(spinnerCmd, viewCmd)
		} else {
			cmd = spinnerCmd
		}

	default:
		cmd = spinnerCmd
	}

	return m, cmd
}

// View renders the current view
func (m Model) View() string {
	if !m.ready {
		return m.renderLoading()
	}

	// Use streaming mode for non-fullscreen display
	if m.streamingMode {
		return m.renderStreamingView()
	}

	// Render current view (legacy fullscreen mode)
	switch m.view {
	case ViewDashboard:
		if m.dashboard != nil {
			return m.dashboard.View()
		}
		return m.renderEmptyView("Dashboard")

	case ViewSessions:
		if m.sessionList != nil {
			return m.sessionList.View()
		}
		return m.renderEmptyView("Sessions")

	case ViewAnalytics:
		if m.analytics != nil {
			return m.analytics.View()
		}
		return m.renderEmptyView("Analytics")

	case ViewHelp:
		if m.help != nil {
			return m.help.View()
		}
		return m.renderEmptyView("Help")

	case ViewMonitor:
		if m.monitor != nil {
			return m.monitor.View()
		}
		return m.renderEmptyView("Monitor")

	default:
		return m.renderEmptyView("Unknown")
	}
}

// renderStreamingView renders the non-fullscreen streaming view
func (m Model) renderStreamingView() string {
	if m.streamDisplay == nil {
		return "claudecat - Streaming mode loading..."
	}

	// Choose display format based on view
	switch m.view {
	case ViewDashboard:
		// Show header with inline summary
		header := m.streamDisplay.RenderHeader()
		return header

	case ViewAnalytics:
		// Show detailed report
		return m.streamDisplay.RenderDetailedReport()

	case ViewSessions:
		// Show inline summary with session info
		summary := m.streamDisplay.RenderInlineSummary()
		sessionInfo := m.renderSessionSummary()
		return summary + "\n" + sessionInfo

	case ViewHelp:
		return m.renderStreamingHelp()

	case ViewMonitor:
		if m.monitor != nil {
			return m.monitor.View()
		}
		return "Monitor view loading..."

	default:
		// Default to compact header
		return m.streamDisplay.RenderHeader()
	}
}

// renderSessionSummary renders a compact session summary
func (m Model) renderSessionSummary() string {
	if len(m.sessions) == 0 {
		return "No active sessions"
	}

	active := 0
	for _, session := range m.sessions {
		if session.IsActive {
			active++
		}
	}

	return fmt.Sprintf("Sessions: %d total, %d active", len(m.sessions), active)
}

// renderStreamingHelp renders help for streaming mode
func (m Model) renderStreamingHelp() string {
	return `claudecat Streaming Mode - Keyboard Shortcuts:
  q/Ctrl+C: Quit
  1/d: Dashboard view    2/s: Sessions view    3/a: Analytics view    4/m: Monitor view
  r/F5: Refresh data     Tab: Next view        h/?: Help
  
This is non-fullscreen mode - output streams inline with your terminal.`
}

// renderLoading renders the loading screen
func (m Model) renderLoading() string {
	if m.streamingMode {
		content := "📊 claudecat - Starting up..."
		if m.config.ShowSpinner {
			content = m.spinner.View() + " " + content
		}
		return content
	}

	content := m.styles.Normal.Render("Initializing claudecat...")

	if m.config.ShowSpinner {
		content = m.spinner.View() + " " + content
	}

	return m.styles.Border.
		Width(m.width - 4).
		Height(m.height - 4).
		Padding(1).
		Render(content)
}

// renderEmptyView renders an empty view placeholder
func (m Model) renderEmptyView(viewName string) string {
	content := m.styles.Normal.Render("No " + viewName + " available")

	return m.styles.Border.
		Width(m.width - 4).
		Height(m.height - 4).
		Padding(1).
		Render(content)
}

// getThemeByName returns a theme by name
func getThemeByName(name string) Theme {
	switch name {
	case "dark":
		return DarkTheme()
	case "light":
		return LightTheme()
	default:
		return DefaultTheme()
	}
}
