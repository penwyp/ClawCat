package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/ClawCat/calculations"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/sessions"
	"github.com/penwyp/ClawCat/ui/components"
)

// ViewType represents different views in the application
type ViewType int

const (
	ViewDashboard ViewType = iota
	ViewSessions
	ViewAnalytics
	ViewHelp
	ViewCount // Keep track of total views
)

// Model represents the application state
type Model struct {
	// Data
	sessions        []*sessions.Session
	entries         []models.UsageEntry
	stats           Statistics
	manager         *sessions.Manager
	realtimeMetrics *calculations.RealtimeMetrics

	// UI State
	view          ViewType
	width         int
	height        int
	ready         bool
	loading       bool
	lastUpdate    time.Time
	streamingMode bool // New: enables non-fullscreen streaming mode

	// Components
	dashboard     *DashboardView
	sessionList   *SessionListView
	analytics     *AnalyticsView
	help          *HelpView
	streamDisplay *components.StreamingDisplay

	// Utilities
	keys    KeyMap
	styles  Styles
	spinner spinner.Model
	config  Config
}

// Statistics holds current usage statistics
type Statistics struct {
	ActiveSessions  int
	TotalTokens     int64
	TotalCost       float64
	TimeToReset     time.Duration
	CurrentBurnRate float64
	PlanUsage       float64
	LastSession     *sessions.Session
	TopModel        string
	SessionCount    int
	AverageCost     float64
}

// NewModel creates a new application model
func NewModel(cfg Config) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = NewStyles(DefaultTheme()).Normal

	m := Model{
		config:        cfg,
		view:          ViewDashboard,
		ready:         true, // Set ready to true initially to avoid stuck loading
		loading:       true,
		keys:          DefaultKeyMap(),
		styles:        NewStyles(DefaultTheme()),
		spinner:       s,
		lastUpdate:    time.Now(),
		streamingMode: !cfg.CompactMode, // Enable streaming mode unless in compact mode
	}

	// Initialize views
	m.dashboard = NewDashboardView()
	m.sessionList = NewSessionListView()
	m.analytics = NewAnalyticsView()
	m.help = NewHelpView()
	m.streamDisplay = components.NewStreamingDisplay()

	return m
}

// Init returns initial commands for the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
		tea.WindowSize(),
		// Add a timeout to ensure we don't stay in loading state forever
		tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return ForceReadyMsg{}
		}),
	)
}

// SetDataSource sets the sessions manager
func (m *Model) SetDataSource(manager *sessions.Manager) {
	m.manager = manager
}

// UpdateSessions updates the session data
func (m *Model) UpdateSessions(sessions []*sessions.Session) {
	m.sessions = sessions
	m.updateStatistics()

	// Update individual views
	if m.dashboard != nil {
		m.dashboard.UpdateStats(m.stats)
	}
	if m.sessionList != nil {
		m.sessionList.UpdateSessions(sessions)
	}
	if m.analytics != nil {
		m.analytics.UpdateData(sessions, m.entries)
	}

	m.lastUpdate = time.Now()
	m.loading = false
}

// UpdateEntries updates the usage entries
func (m *Model) UpdateEntries(entries []models.UsageEntry) {
	m.entries = entries
	m.updateStatistics()

	// Update analytics view
	if m.analytics != nil {
		m.analytics.UpdateData(m.sessions, entries)
	}

	m.lastUpdate = time.Now()
}

// UpdateRealtimeMetrics updates the realtime metrics
func (m *Model) UpdateRealtimeMetrics(metrics *calculations.RealtimeMetrics) {
	m.realtimeMetrics = metrics

	// Update streaming display
	if m.streamDisplay != nil {
		m.streamDisplay.SetMetrics(metrics)
		m.streamDisplay.SetSessions(m.sessions)
		m.streamDisplay.SetWidth(m.width)
	}

	m.lastUpdate = time.Now()
}

// SetStreamingMode enables/disables streaming mode
func (m *Model) SetStreamingMode(enabled bool) {
	m.streamingMode = enabled
}

// updateStatistics calculates current statistics
func (m *Model) updateStatistics() {
	stats := Statistics{
		SessionCount: len(m.sessions),
	}

	// Count active sessions and calculate totals
	var totalTokens int64
	var totalCost float64
	modelCounts := make(map[string]int)

	for _, session := range m.sessions {
		if session.IsActive {
			stats.ActiveSessions++
		}

		// Sum up session metrics
		for _, entry := range session.Entries {
			totalTokens += int64(entry.TotalTokens)
			totalCost += entry.CostUSD
			modelCounts[entry.Model]++
		}

		// Track most recent session
		if stats.LastSession == nil || session.StartTime.After(stats.LastSession.StartTime) {
			stats.LastSession = session
		}
	}

	stats.TotalTokens = totalTokens
	stats.TotalCost = totalCost

	// Calculate average cost
	if stats.SessionCount > 0 {
		stats.AverageCost = totalCost / float64(stats.SessionCount)
	}

	// Find most used model
	maxCount := 0
	for model, count := range modelCounts {
		if count > maxCount {
			maxCount = count
			stats.TopModel = model
		}
	}

	// Calculate burn rate (tokens per hour)
	if len(m.sessions) > 1 {
		oldestTime := m.sessions[0].StartTime
		newestTime := m.sessions[len(m.sessions)-1].EndTime
		if newestTime.IsZero() {
			newestTime = time.Now()
		}

		hours := newestTime.Sub(oldestTime).Hours()
		if hours > 0 {
			stats.CurrentBurnRate = float64(totalTokens) / hours
		}
	}

	// Time to reset (assuming monthly billing cycle)
	now := time.Now()
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	stats.TimeToReset = nextMonth.Sub(now)

	// Plan usage (mock calculation - would need actual plan limits)
	stats.PlanUsage = (totalCost / 100.0) * 100 // Assume $100 plan
	if stats.PlanUsage > 100 {
		stats.PlanUsage = 100
	}

	m.stats = stats
}

// SwitchView changes the current view
func (m *Model) SwitchView(view ViewType) {
	if view >= 0 && view < ViewCount {
		m.view = view
	}
}

// NextView switches to the next view
func (m *Model) NextView() {
	m.view = (m.view + 1) % (ViewCount - 1) // Exclude ViewCount itself
}

// PrevView switches to the previous view
func (m *Model) PrevView() {
	if m.view == 0 {
		m.view = ViewCount - 2 // Last valid view
	} else {
		m.view--
	}
}

// GetCurrentView returns the current view name
func (m Model) GetCurrentView() string {
	switch m.view {
	case ViewDashboard:
		return "Dashboard"
	case ViewSessions:
		return "Sessions"
	case ViewAnalytics:
		return "Analytics"
	case ViewHelp:
		return "Help"
	default:
		return "Unknown"
	}
}

// IsReady returns whether the model is ready to display
func (m Model) IsReady() bool {
	return m.ready
}

// IsLoading returns whether the model is currently loading
func (m Model) IsLoading() bool {
	return m.loading
}

// SetReady marks the model as ready
func (m *Model) SetReady(ready bool) {
	m.ready = ready
}

// SetLoading sets the loading state
func (m *Model) SetLoading(loading bool) {
	m.loading = loading
}

// Resize updates the model dimensions
func (m *Model) Resize(width, height int) {
	m.width = width
	m.height = height

	// Update all views with new dimensions
	if m.dashboard != nil {
		m.dashboard.Resize(width, height)
	}
	if m.sessionList != nil {
		m.sessionList.Resize(width, height)
	}
	if m.analytics != nil {
		m.analytics.Resize(width, height)
	}
	if m.help != nil {
		m.help.Resize(width, height)
	}
}

// GetStats returns current statistics
func (m Model) GetStats() Statistics {
	return m.stats
}

// GetSessions returns current sessions
func (m Model) GetSessions() []*sessions.Session {
	return m.sessions
}

// GetEntries returns current entries
func (m Model) GetEntries() []models.UsageEntry {
	return m.entries
}
