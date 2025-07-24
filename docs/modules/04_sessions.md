# Module: sessions

## Overview
The sessions package manages the lifecycle of 5-hour usage sessions, tracks multiple concurrent sessions, detects gaps, and provides real-time session state management. It handles the complex logic of session boundaries, overlaps, and aggregation.

## Package Structure
```
sessions/
├── manager.go      # Main session manager
├── tracker.go      # Real-time session tracking
├── detector.go     # Session boundary detection
├── aggregator.go   # Session data aggregation
├── state.go        # Session state persistence
└── *_test.go       # Unit and integration tests
```

## Core Components

### Session Manager
Central coordinator for all session operations.

```go
type Manager struct {
    sessions     map[string]*Session
    activeSessions []*Session
    mu           sync.RWMutex
    tracker      *Tracker
    aggregator   *Aggregator
    persistence  *StatePersistence
}

type Session struct {
    ID           string
    StartTime    time.Time
    EndTime      time.Time
    IsActive     bool
    Entries      []models.UsageEntry
    Stats        SessionStats
    LastUpdate   time.Time
}

type SessionStats struct {
    TotalTokens         int
    TotalCost           float64
    ModelBreakdown      map[string]models.ModelStat
    BurnRate            calculations.BurnRate
    TimeRemaining       time.Duration
    PercentageUsed      float64
}

func NewManager() *Manager
func (m *Manager) AddEntry(entry models.UsageEntry) error
func (m *Manager) GetActiveSession() *Session
func (m *Manager) GetAllActiveSessions() []*Session
func (m *Manager) GetSession(id string) (*Session, error)
func (m *Manager) RefreshStats() error
```

### Session Tracker
Real-time tracking of session states and transitions.

```go
type Tracker struct {
    sessions    []*TrackedSession
    events      chan SessionEvent
    stopCh      chan struct{}
}

type TrackedSession struct {
    Session     *Session
    State       SessionState
    Transitions []StateTransition
}

type SessionState int
const (
    StateNew SessionState = iota
    StateActive
    StateExpiring  // Last 30 minutes
    StateExpired
    StateGap
)

type SessionEvent struct {
    Type      EventType
    SessionID string
    Timestamp time.Time
    Data      interface{}
}

type EventType int
const (
    EventSessionStart EventType = iota
    EventSessionUpdate
    EventSessionExpiring
    EventSessionEnd
    EventGapDetected
)

func NewTracker() *Tracker
func (t *Tracker) Track(session *Session) error
func (t *Tracker) UpdateState(sessionID string, state SessionState) error
func (t *Tracker) Events() <-chan SessionEvent
func (t *Tracker) Start() error
func (t *Tracker) Stop() error
```

### Session Detector
Intelligent detection of session boundaries and gaps.

```go
type Detector struct {
    gapThreshold    time.Duration
    sessionDuration time.Duration
    lookbackWindow  time.Duration
}

type DetectionResult struct {
    Sessions      []SessionBoundary
    Gaps          []GapPeriod
    Overlaps      []OverlapPeriod
    Warnings      []string
}

type SessionBoundary struct {
    StartTime    time.Time
    EndTime      time.Time
    Confidence   float64
    Source       string
}

type GapPeriod struct {
    StartTime    time.Time
    EndTime      time.Time
    Duration     time.Duration
}

func NewDetector() *Detector
func (d *Detector) DetectSessions(entries []models.UsageEntry) DetectionResult
func (d *Detector) FindGaps(sessions []SessionBoundary) []GapPeriod
func (d *Detector) ResolveOverlaps(sessions []SessionBoundary) []SessionBoundary
```

### Session Aggregator
Aggregates entries into session blocks with statistics.

```go
type Aggregator struct {
    calculator  *calculations.CostCalculator
    burnRateCalc *calculations.BurnRateCalculator
}

type AggregationResult struct {
    Blocks       []models.SessionBlock
    Summary      SessionSummary
    Diagnostics  []string
}

type SessionSummary struct {
    TotalSessions      int
    ActiveSessions     int
    TotalTime          time.Duration
    TotalGapTime       time.Duration
    AverageSessionTime time.Duration
    TotalCost          float64
    TotalTokens        int
}

func NewAggregator() *Aggregator
func (a *Aggregator) Aggregate(entries []models.UsageEntry) AggregationResult
func (a *Aggregator) AggregateSession(session *Session) models.SessionBlock
func (a *Aggregator) MergeBlocks(blocks []models.SessionBlock) models.SessionBlock
```

### State Persistence
Persists session state for recovery and historical analysis.

```go
type StatePersistence struct {
    storePath string
    codec     Codec
}

type Codec interface {
    Encode(v interface{}) ([]byte, error)
    Decode(data []byte, v interface{}) error
}

type PersistedState struct {
    Version      int
    Sessions     []*Session
    LastUpdate   time.Time
    Checksum     string
}

func NewStatePersistence(path string) *StatePersistence
func (s *StatePersistence) Save(sessions []*Session) error
func (s *StatePersistence) Load() ([]*Session, error)
func (s *StatePersistence) SaveSnapshot(state PersistedState) error
func (s *StatePersistence) LoadSnapshot() (*PersistedState, error)
```

## Key Functions

### Session Lifecycle
```go
func CreateSession(startTime time.Time) *Session
func (s *Session) AddEntry(entry models.UsageEntry) error
func (s *Session) UpdateStats(calc *calculations.CostCalculator) error
func (s *Session) IsExpiring() bool
func (s *Session) IsExpired() bool
func (s *Session) TimeRemaining() time.Duration
func (s *Session) PercentageComplete() float64
```

### Multi-Session Support
```go
func FindConcurrentSessions(entries []models.UsageEntry) [][]*Session
func MergeConcurrentStats(sessions []*Session) SessionStats
func CalculateCombinedBurnRate(sessions []*Session) calculations.BurnRate
```

### Utilities
```go
func RoundToSessionStart(t time.Time) time.Time
func GetSessionWindow(t time.Time) (start, end time.Time)
func IsWithinSession(t, sessionStart time.Time) bool
func SessionsOverlap(s1, s2 *Session) bool
```

## Usage Example

```go
package main

import (
    "github.com/penwyp/ClawCat/sessions"
    "github.com/penwyp/ClawCat/models"
    "log"
)

func main() {
    // Create session manager
    manager := sessions.NewManager()
    
    // Start tracking
    tracker := manager.Tracker()
    go func() {
        for event := range tracker.Events() {
            switch event.Type {
            case sessions.EventSessionExpiring:
                log.Printf("Session %s expiring soon!", event.SessionID)
            case sessions.EventSessionEnd:
                log.Printf("Session %s ended", event.SessionID)
            }
        }
    }()
    
    // Add entries
    entries := loadEntries()
    for _, entry := range entries {
        if err := manager.AddEntry(entry); err != nil {
            log.Printf("Error adding entry: %v", err)
        }
    }
    
    // Get active sessions
    active := manager.GetAllActiveSessions()
    for _, session := range active {
        log.Printf("Active session: %s, Time remaining: %v", 
            session.ID, session.TimeRemaining())
    }
}
```

## Session Rules

1. **Duration**: Sessions are exactly 5 hours from start
2. **Boundaries**: Rounded to the nearest hour
3. **Gaps**: >= 5 hours of inactivity creates a gap
4. **Overlaps**: Multiple sessions can be active simultaneously
5. **Expiration**: Sessions expire at their 5-hour mark

## Testing Strategy

1. **Unit Tests**:
   - Session creation and lifecycle
   - Boundary detection accuracy
   - Gap detection logic
   - State transitions
   - Concurrent session handling

2. **Integration Tests**:
   - Full workflow with real data
   - State persistence and recovery
   - Event emission and handling
   - Multi-session scenarios

3. **Time-Based Tests**:
   - Mock time for deterministic tests
   - Edge cases around boundaries
   - Timezone handling
   - Daylight saving transitions

## Performance Considerations

1. Use read-write locks for concurrent access
2. Lazy load historical sessions
3. Efficient time-based indexing
4. Batch state updates
5. Compress persisted state

## Error Handling

```go
type SessionError struct {
    Type    ErrorType
    Session string
    Message string
}

type ErrorType int
const (
    ErrorInvalidEntry ErrorType = iota
    ErrorSessionNotFound
    ErrorStateCorrupted
    ErrorPersistenceFailed
)
```