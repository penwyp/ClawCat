package sessions

import (
	"fmt"
	"sort"
	"time"

	"github.com/penwyp/claudecat/models"
)

// Detector intelligently detects session boundaries and gaps in usage data
type Detector struct {
	gapThreshold    time.Duration
	sessionDuration time.Duration
	lookbackWindow  time.Duration
}

// DetectionResult contains the results of session boundary detection
type DetectionResult struct {
	Sessions []SessionBoundary `json:"sessions"`
	Gaps     []GapPeriod       `json:"gaps"`
	Overlaps []OverlapPeriod   `json:"overlaps"`
	Warnings []string          `json:"warnings"`
}

// SessionBoundary represents a detected session with confidence level
type SessionBoundary struct {
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Confidence float64   `json:"confidence"` // 0.0 to 1.0
	Source     string    `json:"source"`     // "detected", "explicit", "inferred"
}

// GapPeriod represents a period of inactivity between sessions
type GapPeriod struct {
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
}

// OverlapPeriod represents overlapping sessions that need resolution
type OverlapPeriod struct {
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	SessionIDs []string  `json:"session_ids"`
}

// NewDetector creates a new session detector with default parameters
func NewDetector() *Detector {
	return &Detector{
		gapThreshold:    GapThreshold,    // 5 hours
		sessionDuration: SessionDuration, // 5 hours
		lookbackWindow:  24 * time.Hour,  // 24 hours for context
	}
}

// NewDetectorWithOptions creates a detector with custom parameters
func NewDetectorWithOptions(gapThreshold, sessionDuration, lookbackWindow time.Duration) *Detector {
	return &Detector{
		gapThreshold:    gapThreshold,
		sessionDuration: sessionDuration,
		lookbackWindow:  lookbackWindow,
	}
}

// DetectSessions analyzes usage entries and detects session boundaries
func (d *Detector) DetectSessions(entries []models.UsageEntry) DetectionResult {
	if len(entries) == 0 {
		return DetectionResult{
			Sessions: []SessionBoundary{},
			Gaps:     []GapPeriod{},
			Overlaps: []OverlapPeriod{},
			Warnings: []string{},
		}
	}

	// Sort entries by timestamp for chronological processing
	sortedEntries := make([]models.UsageEntry, len(entries))
	copy(sortedEntries, entries)
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].Timestamp.Before(sortedEntries[j].Timestamp)
	})

	result := DetectionResult{
		Sessions: []SessionBoundary{},
		Gaps:     []GapPeriod{},
		Overlaps: []OverlapPeriod{},
		Warnings: []string{},
	}

	// Detect session boundaries
	sessions := d.detectSessionBoundaries(sortedEntries)
	result.Sessions = sessions

	// Find gaps between sessions
	gaps := d.FindGaps(sessions)
	result.Gaps = gaps

	// Detect and resolve overlaps
	resolvedSessions := d.ResolveOverlaps(sessions)
	if len(resolvedSessions) != len(sessions) {
		// There were overlaps that needed resolution
		result.Sessions = resolvedSessions
		result.Overlaps = d.findOverlaps(sessions)
	}

	// Add warnings for edge cases
	result.Warnings = d.generateWarnings(sortedEntries, result.Sessions)

	return result
}

// detectSessionBoundaries identifies session start and end points from entries
func (d *Detector) detectSessionBoundaries(entries []models.UsageEntry) []SessionBoundary {
	if len(entries) == 0 {
		return []SessionBoundary{}
	}

	sessions := []SessionBoundary{}
	var currentSessionStart time.Time
	lastEntryTime := entries[0].Timestamp

	// Initialize first session
	currentSessionStart = RoundToSessionStart(entries[0].Timestamp)

	for i, entry := range entries {
		timeSinceLastEntry := entry.Timestamp.Sub(lastEntryTime)

		// Check if this entry indicates a new session
		if timeSinceLastEntry >= d.gapThreshold ||
			entry.Timestamp.Sub(currentSessionStart) >= d.sessionDuration {

			// End current session
			sessionEnd := lastEntryTime
			if currentSessionStart.Add(d.sessionDuration).Before(sessionEnd) {
				sessionEnd = currentSessionStart.Add(d.sessionDuration)
			}

			sessions = append(sessions, SessionBoundary{
				StartTime:  currentSessionStart,
				EndTime:    sessionEnd,
				Confidence: d.calculateConfidence(entries, i-1, currentSessionStart, sessionEnd),
				Source:     "detected",
			})

			// Start new session
			currentSessionStart = RoundToSessionStart(entry.Timestamp)
		}

		lastEntryTime = entry.Timestamp
	}

	// Add final session
	sessionEnd := lastEntryTime
	if currentSessionStart.Add(d.sessionDuration).Before(sessionEnd) {
		sessionEnd = currentSessionStart.Add(d.sessionDuration)
	}

	sessions = append(sessions, SessionBoundary{
		StartTime:  currentSessionStart,
		EndTime:    sessionEnd,
		Confidence: d.calculateConfidence(entries, len(entries)-1, currentSessionStart, sessionEnd),
		Source:     "detected",
	})

	return sessions
}

// FindGaps identifies gaps between session boundaries
func (d *Detector) FindGaps(sessions []SessionBoundary) []GapPeriod {
	if len(sessions) <= 1 {
		return []GapPeriod{}
	}

	// Sort sessions by start time
	sortedSessions := make([]SessionBoundary, len(sessions))
	copy(sortedSessions, sessions)
	sort.Slice(sortedSessions, func(i, j int) bool {
		return sortedSessions[i].StartTime.Before(sortedSessions[j].StartTime)
	})

	gaps := []GapPeriod{}

	for i := 0; i < len(sortedSessions)-1; i++ {
		currentEnd := sortedSessions[i].EndTime
		nextStart := sortedSessions[i+1].StartTime

		gapDuration := nextStart.Sub(currentEnd)

		// Only consider significant gaps
		if gapDuration >= d.gapThreshold {
			gaps = append(gaps, GapPeriod{
				StartTime: currentEnd,
				EndTime:   nextStart,
				Duration:  gapDuration,
			})
		}
	}

	return gaps
}

// ResolveOverlaps resolves overlapping sessions by merging or splitting
func (d *Detector) ResolveOverlaps(sessions []SessionBoundary) []SessionBoundary {
	if len(sessions) <= 1 {
		return sessions
	}

	// Sort sessions by start time
	sortedSessions := make([]SessionBoundary, len(sessions))
	copy(sortedSessions, sessions)
	sort.Slice(sortedSessions, func(i, j int) bool {
		return sortedSessions[i].StartTime.Before(sortedSessions[j].StartTime)
	})

	resolved := []SessionBoundary{}
	current := sortedSessions[0]

	for i := 1; i < len(sortedSessions); i++ {
		next := sortedSessions[i]

		// Check for overlap
		if current.EndTime.After(next.StartTime) {
			// Merge overlapping sessions
			if next.EndTime.After(current.EndTime) {
				current.EndTime = next.EndTime
			}
			// Take higher confidence
			if next.Confidence > current.Confidence {
				current.Confidence = next.Confidence
			}
			current.Source = "merged"
		} else {
			// No overlap, add current and move to next
			resolved = append(resolved, current)
			current = next
		}
	}

	// Add the last session
	resolved = append(resolved, current)

	return resolved
}

// findOverlaps identifies overlapping periods in the original sessions
func (d *Detector) findOverlaps(sessions []SessionBoundary) []OverlapPeriod {
	overlaps := []OverlapPeriod{}

	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			s1, s2 := sessions[i], sessions[j]

			// Check if sessions overlap
			if s1.StartTime.Before(s2.EndTime) && s2.StartTime.Before(s1.EndTime) {
				overlapStart := s1.StartTime
				if s2.StartTime.After(overlapStart) {
					overlapStart = s2.StartTime
				}

				overlapEnd := s1.EndTime
				if s2.EndTime.Before(overlapEnd) {
					overlapEnd = s2.EndTime
				}

				overlaps = append(overlaps, OverlapPeriod{
					StartTime:  overlapStart,
					EndTime:    overlapEnd,
					SessionIDs: []string{d.generateSessionID(s1), d.generateSessionID(s2)},
				})
			}
		}
	}

	return overlaps
}

// calculateConfidence calculates detection confidence based on entry patterns
func (d *Detector) calculateConfidence(entries []models.UsageEntry, lastIndex int, sessionStart, sessionEnd time.Time) float64 {
	if lastIndex < 0 || lastIndex >= len(entries) {
		return 0.5 // Medium confidence for edge cases
	}

	// Base confidence
	confidence := 0.7

	// Adjust based on session duration adherence
	sessionDuration := sessionEnd.Sub(sessionStart)
	durationRatio := float64(sessionDuration) / float64(d.sessionDuration)

	if durationRatio >= 0.9 && durationRatio <= 1.1 {
		confidence += 0.2 // Bonus for sessions close to expected duration
	} else if durationRatio < 0.5 || durationRatio > 1.5 {
		confidence -= 0.2 // Penalty for very short or long sessions
	}

	// Adjust based on entry distribution within session
	entriesInSession := 0
	for i := 0; i <= lastIndex; i++ {
		if entries[i].Timestamp.After(sessionStart) && entries[i].Timestamp.Before(sessionEnd) {
			entriesInSession++
		}
	}

	if entriesInSession >= 5 {
		confidence += 0.1 // Bonus for sessions with many entries
	} else if entriesInSession <= 1 {
		confidence -= 0.1 // Penalty for sessions with very few entries
	}

	// Ensure confidence stays within bounds
	if confidence > 1.0 {
		confidence = 1.0
	} else if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// generateWarnings creates warnings for potential detection issues
func (d *Detector) generateWarnings(entries []models.UsageEntry, sessions []SessionBoundary) []string {
	warnings := []string{}

	// Check for very short sessions
	for _, session := range sessions {
		duration := session.EndTime.Sub(session.StartTime)
		if duration < time.Hour {
			warnings = append(warnings,
				fmt.Sprintf("Very short session detected: duration %v", duration))
		}
	}

	// Check for very long gaps
	if len(sessions) > 1 {
		for i := 0; i < len(sessions)-1; i++ {
			gap := sessions[i+1].StartTime.Sub(sessions[i].EndTime)
			if gap > 24*time.Hour {
				warnings = append(warnings,
					fmt.Sprintf("Very long gap detected: %v between sessions", gap))
			}
		}
	}

	// Check for low confidence sessions
	for _, session := range sessions {
		if session.Confidence < 0.5 {
			warnings = append(warnings,
				fmt.Sprintf("Low confidence session detected: confidence %.2f", session.Confidence))
		}
	}

	return warnings
}

// generateSessionID creates a unique ID for a session boundary
func (d *Detector) generateSessionID(session SessionBoundary) string {
	return "session_" + session.StartTime.Format("20060102_150405")
}
