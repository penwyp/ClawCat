package orchestrator

import (
	"fmt"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/logging"
)

// SessionChangeType represents the type of session change
type SessionChangeType string

const (
	SessionStart SessionChangeType = "session_start"
	SessionEnd   SessionChangeType = "session_end"
	SessionUpdate SessionChangeType = "session_update"
)

// SessionMonitor monitors session changes and validates data
type SessionMonitor struct {
	currentSessionID string
	sessionCount     int
	lastUpdateTime   time.Time
	callbacks        []SessionChangeCallback
	mu               sync.RWMutex
}

// NewSessionMonitor creates a new session monitor
func NewSessionMonitor() *SessionMonitor {
	return &SessionMonitor{
		callbacks: make([]SessionChangeCallback, 0),
	}
}

// Update validates data and updates session tracking
func (sm *SessionMonitor) Update(data *AnalysisResult) (bool, []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	var errors []string
	
	// Basic data validation
	if data == nil {
		errors = append(errors, "data is nil")
		return false, errors
	}
	
	if len(data.Blocks) == 0 {
		errors = append(errors, "no blocks in data")
		return false, errors
	}
	
	// Find active sessions
	var activeBlocks []string
	for _, block := range data.Blocks {
		if block.IsActive && !block.IsGap {
			activeBlocks = append(activeBlocks, block.ID)
		}
	}
	
	// Update session tracking
	_ = sm.currentSessionID // previousSessionID was unused
	if len(activeBlocks) > 0 {
		// Use the first active block as the current session
		newSessionID := activeBlocks[0]
		if newSessionID != sm.currentSessionID {
			// Session changed
			if sm.currentSessionID != "" {
				// End previous session
				sm.notifySessionChange(SessionEnd, sm.currentSessionID, nil)
			}
			
			// Start new session
			sm.currentSessionID = newSessionID
			sm.sessionCount++
			sm.notifySessionChange(SessionStart, sm.currentSessionID, nil)
		} else {
			// Session updated
			sm.notifySessionChange(SessionUpdate, sm.currentSessionID, nil)
		}
	} else {
		// No active sessions
		if sm.currentSessionID != "" {
			// End current session
			sm.notifySessionChange(SessionEnd, sm.currentSessionID, nil)
			sm.currentSessionID = ""
		}
	}
	
	sm.lastUpdateTime = time.Now()
	
	// Additional validation
	if err := sm.validateBlockStructure(data.Blocks); err != nil {
		errors = append(errors, err.Error())
	}
	
	return len(errors) == 0, errors
}

// RegisterCallback registers a callback for session changes
func (sm *SessionMonitor) RegisterCallback(callback SessionChangeCallback) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.callbacks = append(sm.callbacks, callback)
}

// GetCurrentSessionID returns the current session ID
func (sm *SessionMonitor) GetCurrentSessionID() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentSessionID
}

// GetSessionCount returns the total session count
func (sm *SessionMonitor) GetSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessionCount
}

// GetLastUpdateTime returns the last update time
func (sm *SessionMonitor) GetLastUpdateTime() time.Time {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastUpdateTime
}

// notifySessionChange notifies all registered callbacks of session changes
func (sm *SessionMonitor) notifySessionChange(eventType SessionChangeType, sessionID string, sessionData interface{}) {
	for _, callback := range sm.callbacks {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logging.LogErrorf("Session callback panic: %v", r)
				}
			}()
			callback(string(eventType), sessionID, sessionData)
		}()
	}
}

// validateBlockStructure validates the structure of session blocks
func (sm *SessionMonitor) validateBlockStructure(blocks []models.SessionBlock) error {
	// This is a placeholder for block structure validation
	// In a real implementation, we might check:
	// - Block time ordering
	// - Block ID uniqueness
	// - Required fields presence
	// - Data consistency
	
	if len(blocks) == 0 {
		return fmt.Errorf("no blocks provided for validation")
	}
	
	// For now, just return success
	return nil
}

// Reset resets the session monitor state
func (sm *SessionMonitor) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.currentSessionID != "" {
		sm.notifySessionChange(SessionEnd, sm.currentSessionID, nil)
	}
	
	sm.currentSessionID = ""
	sm.sessionCount = 0
	sm.lastUpdateTime = time.Time{}
}

// GetStatistics returns session monitoring statistics
func (sm *SessionMonitor) GetStatistics() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	return map[string]interface{}{
		"current_session_id": sm.currentSessionID,
		"session_count":      sm.sessionCount,
		"last_update_time":   sm.lastUpdateTime,
		"callbacks_count":    len(sm.callbacks),
	}
}