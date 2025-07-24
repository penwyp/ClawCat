package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/sessions"
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

// Update handles incoming messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Handle window resize
		m.Resize(msg.Width, msg.Height)
		m.SetReady(true)
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
			m.dashboard = updatedView.(*DashboardView)
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
			m.dashboard = updatedView.(*DashboardView)
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

	// Render current view
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

	default:
		return m.renderEmptyView("Unknown")
	}
}

// renderLoading renders the loading screen
func (m Model) renderLoading() string {
	content := m.styles.Normal.Render("Initializing ClawCat...")
	
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