package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/claudecat/models"
	"github.com/penwyp/claudecat/sessions"
)

// App represents the main application structure
type App struct {
	model   Model
	program *tea.Program
	config  Config
	ctx     context.Context
	cancel  context.CancelFunc
}

// Config holds UI configuration
type Config struct {
	RefreshRate      time.Duration
	Theme            string
	ShowSpinner      bool
	CompactMode      bool
	ChartHeight      int
	TablePageSize    int
	SubscriptionPlan string
}

// DefaultConfig returns the default UI configuration
var DefaultConfig = Config{
	RefreshRate:   time.Second,
	Theme:         "dark",
	ShowSpinner:   true,
	CompactMode:   false,
	ChartHeight:   10,
	TablePageSize: 20,
}

// NewApp creates a new application instance
func NewApp(cfg Config) *App {
	ctx, cancel := context.WithCancel(context.Background())

	model := NewModel(cfg)

	app := &App{
		model:  model,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	// Create Bubble Tea program without alt screen for inline terminal display
	app.program = tea.NewProgram(
		model,
		tea.WithMouseCellMotion(),
		tea.WithContext(ctx),
	)

	return app
}

// Start begins the application
func (a *App) Start() error {
	// Start the program
	_, err := a.program.Run()
	return err
}

// Stop gracefully shuts down the application
func (a *App) Stop() error {
	if a.program != nil {
		a.program.Kill()
	}
	if a.cancel != nil {
		a.cancel()
	}
	return nil
}

// SetDataSource sets the data source for the application
func (a *App) SetDataSource(manager *sessions.Manager) {
	a.model.SetDataSource(manager)
}

// SendMessage sends a message to the application
func (a *App) SendMessage(msg tea.Msg) {
	if a.program != nil {
		a.program.Send(msg)
	}
}

// Resize handles terminal resize events
func (a *App) Resize(width, height int) {
	if a.program != nil {
		a.program.Send(tea.WindowSizeMsg{
			Width:  width,
			Height: height,
		})
	}
}

// UpdateData sends new data to the UI
func (a *App) UpdateData(sessions []*sessions.Session, entries []models.UsageEntry) {
	if a.program != nil {
		a.program.Send(DataUpdateMsg{
			Sessions: sessions,
			Entries:  entries,
		})
	}
}

// IsRunning returns true if the application is currently running
func (a *App) IsRunning() bool {
	return a.ctx.Err() == nil
}

// GetConfig returns the current configuration
func (a *App) GetConfig() Config {
	return a.config
}

// UpdateConfig updates the application configuration
func (a *App) UpdateConfig(cfg Config) {
	a.config = cfg
	if a.program != nil {
		a.program.Send(ConfigUpdateMsg{Config: cfg})
	}
}
