package sessions

import (
	"testing"
	"time"

	"github.com/penwyp/ClawCat/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.sessions)
	assert.NotNil(t, manager.activeSessions)
	assert.NotNil(t, manager.detector)
	assert.NotNil(t, manager.costCalc)
	assert.Equal(t, 0, len(manager.sessions))
	assert.Equal(t, 0, len(manager.activeSessions))
}

func TestManager_AddEntry(t *testing.T) {
	manager := NewManager()
	now := time.Now()

	entry := models.UsageEntry{
		Timestamp:    now,
		Model:        "claude-3-sonnet-20240229",
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		CostUSD:      0.0045,
	}

	err := manager.AddEntry(entry)
	require.NoError(t, err)

	// Check that session was created
	assert.Equal(t, 1, manager.GetSessionCount())

	// Check that session is active
	activeSessions := manager.GetAllActiveSessions()
	assert.Equal(t, 1, len(activeSessions))

	// Check that entry was added to session
	activeSession := manager.GetActiveSession()
	require.NotNil(t, activeSession)
	assert.Equal(t, 1, len(activeSession.Entries))
	assert.Equal(t, entry.Timestamp, activeSession.Entries[0].Timestamp)
}

func TestManager_AddMultipleEntries(t *testing.T) {
	manager := NewManager()
	baseTime := time.Now().Add(-1 * time.Hour) // Use past time to avoid validation issues

	// Add entries within same session (1 hour apart)
	entries := []models.UsageEntry{
		{
			Timestamp:    baseTime,
			Model:        "claude-3-sonnet-20240229",
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
			CostUSD:      0.0045,
		},
		{
			Timestamp:    baseTime.Add(30 * time.Minute),
			Model:        "claude-3-sonnet-20240229",
			InputTokens:  200,
			OutputTokens: 100,
			TotalTokens:  300,
			CostUSD:      0.009,
		},
		{
			Timestamp:    baseTime.Add(1 * time.Hour),
			Model:        "claude-3-haiku-20240307",
			InputTokens:  50,
			OutputTokens: 25,
			TotalTokens:  75,
			CostUSD:      0.0003,
		},
	}

	for _, entry := range entries {
		err := manager.AddEntry(entry)
		require.NoError(t, err)
	}

	// Should have one session with all entries
	assert.Equal(t, 1, manager.GetSessionCount())

	activeSession := manager.GetActiveSession()
	require.NotNil(t, activeSession)
	assert.Equal(t, 3, len(activeSession.Entries))

	// Check session stats
	assert.Equal(t, 525, activeSession.Stats.TotalTokens) // 150 + 300 + 75
	assert.Greater(t, activeSession.Stats.TotalCost, 0.0)
	assert.Equal(t, 2, len(activeSession.Stats.ModelBreakdown)) // 2 different models
}

func TestManager_MultipleSessions(t *testing.T) {
	manager := NewManager()
	baseTime := time.Now().Add(-12 * time.Hour) // Use time well in the past to ensure sessions are expired

	// Add entry to first session
	entry1 := models.UsageEntry{
		Timestamp:    baseTime,
		Model:        "claude-3-sonnet-20240229",
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		CostUSD:      0.0045,
	}

	// Add entry to second session (6 hours later - creates new session)
	entry2 := models.UsageEntry{
		Timestamp:    baseTime.Add(6 * time.Hour),
		Model:        "claude-3-sonnet-20240229",
		InputTokens:  200,
		OutputTokens: 100,
		TotalTokens:  300,
		CostUSD:      0.009,
	}

	err := manager.AddEntry(entry1)
	require.NoError(t, err)

	err = manager.AddEntry(entry2)
	require.NoError(t, err)

	// Should have two sessions
	assert.Equal(t, 2, manager.GetSessionCount())

	activeSessions := manager.GetAllActiveSessions()
	// Sessions should be expired since we used past time
	assert.Equal(t, 0, len(activeSessions))
}

func TestManager_GetSession(t *testing.T) {
	manager := NewManager()
	now := time.Now()

	entry := models.UsageEntry{
		Timestamp:    now,
		Model:        "claude-3-sonnet-20240229",
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		CostUSD:      0.0045,
	}

	err := manager.AddEntry(entry)
	require.NoError(t, err)

	activeSession := manager.GetActiveSession()
	require.NotNil(t, activeSession)

	// Get session by ID
	retrievedSession, err := manager.GetSession(activeSession.ID)
	require.NoError(t, err)
	assert.Equal(t, activeSession.ID, retrievedSession.ID)
	assert.Equal(t, activeSession.StartTime, retrievedSession.StartTime)

	// Try to get non-existent session
	_, err = manager.GetSession("non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestManager_RefreshStats(t *testing.T) {
	manager := NewManager()
	now := time.Now()

	entry := models.UsageEntry{
		Timestamp:    now,
		Model:        "claude-3-sonnet-20240229",
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		CostUSD:      0.0045,
	}

	err := manager.AddEntry(entry)
	require.NoError(t, err)

	// Refresh stats
	err = manager.RefreshStats()
	require.NoError(t, err)

	activeSession := manager.GetActiveSession()
	require.NotNil(t, activeSession)

	// Verify stats were calculated
	assert.Equal(t, 150, activeSession.Stats.TotalTokens)
	assert.Greater(t, activeSession.Stats.TotalCost, 0.0)
	assert.NotEmpty(t, activeSession.Stats.ModelBreakdown)
}

func TestManager_ConcurrentAccess(t *testing.T) {
	manager := NewManager()
	now := time.Now().Add(-1 * time.Hour) // Use past time to avoid validation issues

	// Test concurrent access doesn't cause race conditions
	done := make(chan bool, 2)

	// Goroutine 1: Add entries
	go func() {
		for i := 0; i < 10; i++ {
			entry := models.UsageEntry{
				Timestamp:    now.Add(time.Duration(i) * time.Minute),
				Model:        "claude-3-sonnet-20240229",
				InputTokens:  100,
				OutputTokens: 50,
				TotalTokens:  150,
				CostUSD:      0.0045,
			}
			_ = manager.AddEntry(entry)
		}
		done <- true
	}()

	// Goroutine 2: Read sessions
	go func() {
		for i := 0; i < 10; i++ {
			_ = manager.GetAllActiveSessions()
			_ = manager.GetActiveSession()
			_ = manager.GetSessionCount()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify final state
	assert.Equal(t, 1, manager.GetSessionCount())
	activeSession := manager.GetActiveSession()
	assert.NotNil(t, activeSession)
	assert.Equal(t, 10, len(activeSession.Entries))
}

func TestManager_InvalidEntry(t *testing.T) {
	manager := NewManager()

	// Test with invalid entry (missing required fields)
	invalidEntry := models.UsageEntry{
		// Missing timestamp and other required fields
		Model: "",
	}

	err := manager.AddEntry(invalidEntry)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid entry")

	// Should not create any sessions
	assert.Equal(t, 0, manager.GetSessionCount())
}

func TestCreateSession(t *testing.T) {
	timestamp := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

	session := CreateSession(timestamp)

	assert.NotEmpty(t, session.ID)
	assert.True(t, session.IsActive)
	assert.Equal(t, 0, len(session.Entries))
	assert.NotNil(t, session.Stats.ModelBreakdown)

	// Start time should be rounded to hour
	expectedStart := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedStart, session.StartTime)

	// End time should be 5 hours later
	expectedEnd := expectedStart.Add(5 * time.Hour)
	assert.Equal(t, expectedEnd, session.EndTime)
}

func TestSession_AddEntry(t *testing.T) {
	session := CreateSession(time.Now())

	// Valid entry within session window
	validEntry := models.UsageEntry{
		Timestamp:    session.StartTime.Add(1 * time.Hour),
		Model:        "claude-3-sonnet-20240229",
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		CostUSD:      0.0045,
	}

	err := session.AddEntry(validEntry)
	require.NoError(t, err)
	assert.Equal(t, 1, len(session.Entries))

	// Invalid entry outside session window
	invalidEntry := models.UsageEntry{
		Timestamp:    session.StartTime.Add(6 * time.Hour), // Outside 5-hour window
		Model:        "claude-3-sonnet-20240229",
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		CostUSD:      0.0045,
	}

	err = session.AddEntry(invalidEntry)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside session window")
	assert.Equal(t, 1, len(session.Entries)) // Should not add invalid entry
}

func TestSession_TimeCalculations(t *testing.T) {
	// Create session that started 2 hours ago
	startTime := time.Now().Add(-2 * time.Hour)
	session := &Session{
		ID:        "test",
		StartTime: startTime,
		EndTime:   startTime.Add(SessionDuration),
		IsActive:  true,
	}

	// Test percentage complete (should be around 40% after 2 hours of 5-hour session)
	percentage := session.PercentageComplete()
	assert.Greater(t, percentage, 35.0)
	assert.Less(t, percentage, 45.0)

	// Test time remaining (should be around 3 hours)
	remaining := session.TimeRemaining()
	assert.Greater(t, remaining, 2*time.Hour+30*time.Minute)
	assert.Less(t, remaining, 3*time.Hour+30*time.Minute)

	// Test expiring status (should not be expiring yet)
	assert.False(t, session.IsExpiring())
	assert.False(t, session.IsExpired())
}

func TestSession_ExpirationStatus(t *testing.T) {
	// Create session that started 4.5 hours ago (expiring)
	expiringStart := time.Now().Add(-4*time.Hour - 45*time.Minute)
	expiringSession := &Session{
		ID:        "expiring",
		StartTime: expiringStart,
		EndTime:   expiringStart.Add(SessionDuration),
		IsActive:  true,
	}

	assert.True(t, expiringSession.IsExpiring())
	assert.False(t, expiringSession.IsExpired())

	// Create session that started 6 hours ago (expired)
	expiredStart := time.Now().Add(-6 * time.Hour)
	expiredSession := &Session{
		ID:        "expired",
		StartTime: expiredStart,
		EndTime:   expiredStart.Add(SessionDuration),
		IsActive:  true,
	}

	assert.False(t, expiredSession.IsExpiring())
	assert.True(t, expiredSession.IsExpired())
}

func TestUtilityFunctions(t *testing.T) {
	// Test RoundToSessionStart
	timestamp := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
	rounded := RoundToSessionStart(timestamp)
	expected := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, rounded)

	// Test GetSessionWindow
	start, end := GetSessionWindow(timestamp)
	assert.Equal(t, expected, start)
	assert.Equal(t, expected.Add(SessionDuration), end)

	// Test IsWithinSession
	sessionStart := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)

	// Within session
	withinTime := sessionStart.Add(2 * time.Hour)
	assert.True(t, IsWithinSession(withinTime, sessionStart))

	// Before session
	beforeTime := sessionStart.Add(-1 * time.Hour)
	assert.False(t, IsWithinSession(beforeTime, sessionStart))

	// After session
	afterTime := sessionStart.Add(6 * time.Hour)
	assert.False(t, IsWithinSession(afterTime, sessionStart))

	// Exactly at start (should be included)
	assert.True(t, IsWithinSession(sessionStart, sessionStart))

	// Exactly at end (should be excluded)
	sessionEnd := sessionStart.Add(SessionDuration)
	assert.False(t, IsWithinSession(sessionEnd, sessionStart))
}

func TestSessionsOverlap(t *testing.T) {
	baseTime := time.Now()

	session1 := &Session{
		ID:        "session1",
		StartTime: baseTime,
		EndTime:   baseTime.Add(5 * time.Hour),
	}

	session2 := &Session{
		ID:        "session2",
		StartTime: baseTime.Add(3 * time.Hour),
		EndTime:   baseTime.Add(8 * time.Hour),
	}

	session3 := &Session{
		ID:        "session3",
		StartTime: baseTime.Add(6 * time.Hour),
		EndTime:   baseTime.Add(11 * time.Hour),
	}

	// Test overlapping sessions
	assert.True(t, SessionsOverlap(session1, session2))
	assert.True(t, SessionsOverlap(session2, session1)) // Should be symmetric

	// Test non-overlapping sessions
	assert.False(t, SessionsOverlap(session1, session3))
	assert.False(t, SessionsOverlap(session3, session1)) // Should be symmetric

	// Test adjacent sessions (touching but not overlapping)
	adjacentSession := &Session{
		ID:        "adjacent",
		StartTime: session1.EndTime,
		EndTime:   session1.EndTime.Add(5 * time.Hour),
	}
	assert.False(t, SessionsOverlap(session1, adjacentSession))
}
