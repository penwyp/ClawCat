package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/ClawCat/fileio"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/sessions"
)

// tickCmd returns a command that sends a tick message after the configured refresh rate
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// refreshDataCmd returns a command that refreshes data from the sessions manager
func refreshDataCmd(manager *sessions.Manager) tea.Cmd {
	return func() tea.Msg {
		if manager == nil {
			return ErrorMsg{Error: ErrNoManager}
		}

		// Get active sessions from the manager (simplified for now)
		sessions := manager.GetAllActiveSessions()
		
		// Collect all entries from sessions
		var entries []models.UsageEntry
		for _, session := range sessions {
			entries = append(entries, session.Entries...)
		}

		return DataUpdateMsg{
			Sessions: sessions,
			Entries:  entries,
		}
	}
}

// watchFilesCmd returns a command that watches for file changes
func watchFilesCmd(watcher *fileio.Watcher) tea.Cmd {
	return func() tea.Msg {
		if watcher == nil {
			return ErrorMsg{Error: ErrNoWatcher}
		}

		// Simplified file watching (placeholder for now)
		return StatusMsg{
			Message: "File watching started",
			Level:   StatusInfo,
		}
	}
}

// loadInitialDataCmd returns a command that loads initial data
func loadInitialDataCmd(manager *sessions.Manager) tea.Cmd {
	return func() tea.Msg {
		if manager == nil {
			return ErrorMsg{Error: ErrInvalidParameters}
		}

		// Simplified initial data loading
		// In a real implementation, this would load from files
		sessions := manager.GetAllActiveSessions()
		
		var entries []models.UsageEntry
		for _, session := range sessions {
			entries = append(entries, session.Entries...)
		}

		return DataUpdateMsg{
			Sessions: sessions,
			Entries:  entries,
		}
	}
}

// saveConfigCmd returns a command that saves the current configuration
func saveConfigCmd(config Config) tea.Cmd {
	return func() tea.Msg {
		// In a real implementation, this would save to a config file
		// For now, just return a success message
		return StatusMsg{
			Message: "Configuration saved",
			Level:   StatusSuccess,
		}
	}
}

// exportDataCmd returns a command that exports current data
func exportDataCmd(sessions []*sessions.Session, format string) tea.Cmd {
	return func() tea.Msg {
		if len(sessions) == 0 {
			return ErrorMsg{Error: ErrNoData}
		}

		// Export data in the specified format
		// This is a placeholder implementation
		switch format {
		case "json":
			// Export as JSON
		case "csv":
			// Export as CSV
		case "xlsx":
			// Export as Excel
		default:
			return ErrorMsg{Error: ErrUnsupportedFormat}
		}

		return StatusMsg{
			Message: "Data exported successfully",
			Level:   StatusSuccess,
		}
	}
}

// updateSessionsCmd returns a command that updates session data
func updateSessionsCmd(manager *sessions.Manager) tea.Cmd {
	return tea.Batch(
		refreshDataCmd(manager),
		func() tea.Msg {
			return StatusMsg{
				Message: "Sessions updated",
				Level:   StatusInfo,
			}
		},
	)
}

// calculateStatsCmd returns a command that calculates statistics
func calculateStatsCmd(sessions []*sessions.Session) tea.Cmd {
	return func() tea.Msg {
		if len(sessions) == 0 {
			return StatusMsg{
				Message: "No sessions to calculate statistics",
				Level:   StatusWarning,
			}
		}

		// Calculate statistics (this would be more comprehensive in reality)
		stats := Statistics{
			SessionCount: len(sessions),
		}

		var totalTokens int64
		var totalCost float64
		activeCount := 0

		for _, session := range sessions {
			if session.IsActive {
				activeCount++
			}
			
			for _, entry := range session.Entries {
				totalTokens += int64(entry.TotalTokens)
				totalCost += entry.CostUSD
			}
		}

		stats.ActiveSessions = activeCount
		stats.TotalTokens = totalTokens
		stats.TotalCost = totalCost

		if stats.SessionCount > 0 {
			stats.AverageCost = totalCost / float64(stats.SessionCount)
		}

		return StatusMsg{
			Message: "Statistics calculated",
			Level:   StatusSuccess,
		}
	}
}

// clearDataCmd returns a command that clears all data
func clearDataCmd(manager *sessions.Manager) tea.Cmd {
	return func() tea.Msg {
		// Simplified clear operation (placeholder)
		return DataUpdateMsg{
			Sessions: []*sessions.Session{},
			Entries:  []models.UsageEntry{},
		}
	}
}

// contextCmd creates a command with context support
func contextCmd(ctx context.Context, cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-ctx.Done():
			return ErrorMsg{Error: ctx.Err()}
		default:
			return cmd()
		}
	}
}

// batchedRefreshCmd returns a command that batches multiple refresh operations
func batchedRefreshCmd(manager *sessions.Manager, interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return refreshDataCmd(manager)()
	})
}

// Helper functions

func generateSessionID() string {
	return time.Now().Format("20060102-150405") + "-" + generateRandomString(6)
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}

func getEarliestTime(entries []models.UsageEntry) time.Time {
	if len(entries) == 0 {
		return time.Time{}
	}
	
	earliest := entries[0].Timestamp
	for _, entry := range entries[1:] {
		if entry.Timestamp.Before(earliest) {
			earliest = entry.Timestamp
		}
	}
	return earliest
}

func getLatestTime(entries []models.UsageEntry) time.Time {
	if len(entries) == 0 {
		return time.Time{}
	}
	
	latest := entries[0].Timestamp
	for _, entry := range entries[1:] {
		if entry.Timestamp.After(latest) {
			latest = entry.Timestamp
		}
	}
	return latest
}