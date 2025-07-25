package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/ClawCat/sessions"
)

// SessionListView represents the session list view
type SessionListView struct {
	sessions []*sessions.Session
	selected int
	width    int
	height   int
	config   Config
	styles   Styles
}

// NewSessionListView creates a new session list view
func NewSessionListView() *SessionListView {
	return &SessionListView{
		styles: NewStyles(DefaultTheme()),
	}
}

// Init initializes the session list view
func (s *SessionListView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the session list view
func (s *SessionListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.selected > 0 {
				s.selected--
			}
		case "down", "j":
			if s.selected < len(s.sessions)-1 {
				s.selected++
			}
		case "home", "g":
			s.selected = 0
		case "end", "G":
			s.selected = len(s.sessions) - 1
		}
	}
	return s, nil
}

// View renders the session list view
func (s *SessionListView) View() string {
	if s.width == 0 || s.height == 0 {
		return "Sessions loading..."
	}

	header := s.renderHeader()
	table := s.renderTable()
	footer := s.renderFooter()

	content := strings.Join([]string{
		header,
		table,
		footer,
	}, "\n\n")

	return s.styles.Content.
		Width(s.width - 4).
		Height(s.height - 4).
		Render(content)
}

// UpdateSessions updates the session list
func (s *SessionListView) UpdateSessions(sessions []*sessions.Session) {
	s.sessions = sessions
	if s.selected >= len(sessions) {
		s.selected = len(sessions) - 1
	}
	if s.selected < 0 {
		s.selected = 0
	}
}

// Resize updates the view dimensions
func (s *SessionListView) Resize(width, height int) {
	s.width = width
	s.height = height
}

// UpdateConfig updates the view configuration
func (s *SessionListView) UpdateConfig(config Config) {
	s.config = config
	s.styles = NewStyles(GetThemeByName(config.Theme))
}

// renderHeader renders the session list header
func (s *SessionListView) renderHeader() string {
	title := s.styles.Title.Render("Sessions")
	count := s.styles.Subtitle.Render(fmt.Sprintf("Total: %d sessions", len(s.sessions)))
	return strings.Join([]string{title, count}, "\n")
}

// renderTable renders the session table
func (s *SessionListView) renderTable() string {
	if len(s.sessions) == 0 {
		return s.styles.Muted.Render("No sessions found")
	}

	// Table headers
	headers := []string{"ID", "Start Time", "Duration", "Entries", "Cost", "Status"}
	headerRow := s.renderTableRow(headers, true)

	// Table rows
	var rows []string
	rows = append(rows, headerRow)

	for i, session := range s.sessions {
		// Calculate session duration and cost
		var duration time.Duration
		var cost float64
		if !session.EndTime.IsZero() {
			duration = session.EndTime.Sub(session.StartTime)
		}
		for _, entry := range session.Entries {
			cost += entry.CostUSD
		}

		data := []string{
			session.ID[:8], // Show first 8 chars of ID
			session.StartTime.Format("15:04:05"),
			s.formatDuration(duration),
			fmt.Sprintf("%d", len(session.Entries)),
			fmt.Sprintf("$%.2f", cost),
			s.formatStatus(session.IsActive),
		}

		row := s.renderTableRow(data, false)
		if i == s.selected {
			row = s.styles.Selected.Render(row)
		}
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

// renderFooter renders the session list footer
func (s *SessionListView) renderFooter() string {
	if len(s.sessions) == 0 {
		return ""
	}

	selectedSession := s.sessions[s.selected]
	// Calculate total cost
	cost := 0.0
	for _, entry := range selectedSession.Entries {
		cost += entry.CostUSD
	}
	info := fmt.Sprintf(
		"Selected: %s | Entries: %d | Total Cost: $%.2f",
		selectedSession.ID[:8],
		len(selectedSession.Entries),
		cost,
	)

	return s.styles.Footer.Render(info)
}

// renderTableRow renders a single table row
func (s *SessionListView) renderTableRow(data []string, isHeader bool) string {
	// Define column widths
	widths := []int{10, 12, 10, 8, 10, 8}

	var cells []string
	for i, cell := range data {
		width := widths[i]
		if len(cell) > width {
			cell = cell[:width-3] + "..."
		} else {
			cell = fmt.Sprintf("%-*s", width, cell)
		}

		if isHeader {
			cell = s.styles.TableHeader.Render(cell)
		} else {
			cell = s.styles.TableRow.Render(cell)
		}

		cells = append(cells, cell)
	}

	return strings.Join(cells, " | ")
}

// formatDuration formats a duration for display
func (s *SessionListView) formatDuration(duration time.Duration) string {
	if duration == 0 {
		return "Active"
	}

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// formatStatus formats session status for display
func (s *SessionListView) formatStatus(isActive bool) string {
	if isActive {
		return s.styles.Success.Render("Active")
	}
	return s.styles.Muted.Render("Ended")
}
