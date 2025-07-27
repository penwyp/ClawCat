package sessions

import (
	"fmt"
	"sync"
	"time"

	"github.com/penwyp/claudecat/calculations"
	"github.com/penwyp/claudecat/models"
)

// Manager coordinates all session operations with thread-safe access
type Manager struct {
	sessions       map[string]*Session          // All sessions by ID
	activeSessions []*Session                   // Currently active sessions
	mu             sync.RWMutex                 // Protects concurrent access
	detector       *Detector                    // Session boundary detection
	costCalc       *calculations.CostCalculator // Cost calculations
}

// Session represents a 5-hour usage session with statistics
type Session struct {
	ID         string              `json:"id"`
	StartTime  time.Time           `json:"start_time"`
	EndTime    time.Time           `json:"end_time"`
	IsActive   bool                `json:"is_active"`
	Entries    []models.UsageEntry `json:"entries"`
	Stats      SessionStats        `json:"stats"`
	LastUpdate time.Time           `json:"last_update"`
}

// SessionStats contains aggregated statistics for a session
type SessionStats struct {
	TotalTokens    int                                `json:"total_tokens"`
	TotalCost      float64                            `json:"total_cost"`
	ModelBreakdown map[string]calculations.ModelStats `json:"model_breakdown"`
	TimeRemaining  time.Duration                      `json:"time_remaining"`
	PercentageUsed float64                            `json:"percentage_used"`
}

const (
	SessionDuration = 5 * time.Hour
	GapThreshold    = 5 * time.Hour
)

// NewManager creates a new session manager with initialized components
func NewManager() *Manager {
	return &Manager{
		sessions:       make(map[string]*Session),
		activeSessions: make([]*Session, 0),
		detector:       NewDetector(),
		costCalc:       calculations.NewCostCalculator(),
	}
}

// AddEntry adds a usage entry to the appropriate session(s)
func (m *Manager) AddEntry(entry models.UsageEntry) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("invalid entry: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find or create appropriate session
	session := m.findOrCreateSession(entry.Timestamp)

	if err := session.AddEntry(entry); err != nil {
		return fmt.Errorf("failed to add entry to session: %w", err)
	}

	// Update session statistics
	if err := session.UpdateStats(m.costCalc); err != nil {
		return fmt.Errorf("failed to update session stats: %w", err)
	}

	// Update active sessions list
	m.updateActiveSessions()

	return nil
}

// GetActiveSession returns the most recent active session, or nil if none
func (m *Manager) GetActiveSession() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.activeSessions) == 0 {
		return nil
	}

	// Return the most recently updated active session
	var mostRecent *Session
	for _, session := range m.activeSessions {
		if mostRecent == nil || session.LastUpdate.After(mostRecent.LastUpdate) {
			mostRecent = session
		}
	}

	return mostRecent
}

// GetAllActiveSessions returns all currently active sessions
func (m *Manager) GetAllActiveSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]*Session, len(m.activeSessions))
	copy(result, m.activeSessions)
	return result
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	return session, nil
}

// RefreshStats recalculates statistics for all sessions
func (m *Manager) RefreshStats() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, session := range m.sessions {
		if err := session.UpdateStats(m.costCalc); err != nil {
			return fmt.Errorf("failed to refresh stats for session %s: %w", session.ID, err)
		}
	}

	m.updateActiveSessions()
	return nil
}

// GetSessionCount returns the total number of sessions
func (m *Manager) GetSessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// findOrCreateSession finds existing session or creates new one for timestamp
func (m *Manager) findOrCreateSession(timestamp time.Time) *Session {
	// Look for existing active session that can contain this timestamp
	for _, session := range m.activeSessions {
		if IsWithinSession(timestamp, session.StartTime) {
			return session
		}
	}

	// No existing session found, create new one
	session := CreateSession(timestamp)
	m.sessions[session.ID] = session
	return session
}

// updateActiveSessions refreshes the list of active sessions
func (m *Manager) updateActiveSessions() {
	m.activeSessions = m.activeSessions[:0] // Clear without reallocating

	for _, session := range m.sessions {
		if !session.IsExpired() {
			session.IsActive = true
			m.activeSessions = append(m.activeSessions, session)
		} else {
			session.IsActive = false
		}
	}
}

// CreateSession creates a new session starting at the given time
func CreateSession(timestamp time.Time) *Session {
	startTime := RoundToSessionStart(timestamp)
	sessionID := fmt.Sprintf("session_%d", startTime.Unix())

	return &Session{
		ID:         sessionID,
		StartTime:  startTime,
		EndTime:    startTime.Add(SessionDuration),
		IsActive:   true,
		Entries:    make([]models.UsageEntry, 0),
		Stats:      SessionStats{ModelBreakdown: make(map[string]calculations.ModelStats)},
		LastUpdate: timestamp,
	}
}

// AddEntry adds a usage entry to this session
func (s *Session) AddEntry(entry models.UsageEntry) error {
	if !IsWithinSession(entry.Timestamp, s.StartTime) {
		return fmt.Errorf("entry timestamp %v is outside session window %v-%v",
			entry.Timestamp, s.StartTime, s.EndTime)
	}

	s.Entries = append(s.Entries, entry)
	s.LastUpdate = time.Now()
	return nil
}

// UpdateStats recalculates session statistics
func (s *Session) UpdateStats(calc *calculations.CostCalculator) error {
	if len(s.Entries) == 0 {
		return nil
	}

	// Calculate total tokens and cost
	s.Stats.TotalTokens = 0
	s.Stats.TotalCost = 0
	s.Stats.ModelBreakdown = make(map[string]calculations.ModelStats)

	for _, entry := range s.Entries {
		s.Stats.TotalTokens += entry.CalculateTotalTokens()

		result, err := calc.Calculate(entry)
		if err != nil {
			return fmt.Errorf("failed to calculate cost for entry: %w", err)
		}
		s.Stats.TotalCost += result.TotalCost

		// Update model breakdown
		modelStats := s.Stats.ModelBreakdown[entry.Model]
		modelStats.TotalTokens += entry.CalculateTotalTokens()
		modelStats.TotalCost += result.TotalCost
		modelStats.EntryCount++
		s.Stats.ModelBreakdown[entry.Model] = modelStats
	}

	// Calculate time-based metrics
	s.Stats.TimeRemaining = s.TimeRemaining()
	s.Stats.PercentageUsed = s.PercentageComplete()

	return nil
}

// IsExpiring returns true if session expires within 30 minutes
func (s *Session) IsExpiring() bool {
	return s.TimeRemaining() <= 30*time.Minute && s.TimeRemaining() > 0
}

// IsExpired returns true if session has passed its end time
func (s *Session) IsExpired() bool {
	return time.Now().UTC().After(s.EndTime)
}

// TimeRemaining returns remaining time in the session
func (s *Session) TimeRemaining() time.Duration {
	remaining := s.EndTime.Sub(time.Now().UTC())
	if remaining < 0 {
		return 0
	}
	return remaining
}

// PercentageComplete returns how much of the session has elapsed (0-100)
func (s *Session) PercentageComplete() float64 {
	elapsed := time.Now().UTC().Sub(s.StartTime)
	if elapsed < 0 {
		return 0
	}
	if elapsed >= SessionDuration {
		return 100
	}
	return float64(elapsed) / float64(SessionDuration) * 100
}

// RoundToSessionStart rounds timestamp to the nearest hour start for session boundary
func RoundToSessionStart(t time.Time) time.Time {
	return t.Truncate(time.Hour)
}

// GetSessionWindow returns the start and end times for the session containing timestamp t
func GetSessionWindow(t time.Time) (start, end time.Time) {
	start = RoundToSessionStart(t)
	end = start.Add(SessionDuration)
	return start, end
}

// IsWithinSession checks if timestamp t falls within the session starting at sessionStart
func IsWithinSession(t, sessionStart time.Time) bool {
	sessionEnd := sessionStart.Add(SessionDuration)
	return !t.Before(sessionStart) && t.Before(sessionEnd)
}

// SessionsOverlap checks if two sessions have overlapping time periods
func SessionsOverlap(s1, s2 *Session) bool {
	return s1.StartTime.Before(s2.EndTime) && s2.StartTime.Before(s1.EndTime)
}
