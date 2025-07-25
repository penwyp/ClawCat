package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/sessions"
)

// tickCmd returns a command that sends a tick message after a brief delay
func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// refreshDataCmd returns a command that refreshes session data
func refreshDataCmd(manager *sessions.Manager) tea.Cmd {
	return func() tea.Msg {
		// Get sessions from manager
		activeSessions := manager.GetAllActiveSessions()
		return DataUpdateMsg{
			Sessions: activeSessions,
			Entries:  []models.UsageEntry{}, // Could be populated with recent entries
		}
	}
}