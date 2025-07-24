package ui

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines all keyboard bindings for the application
type KeyMap struct {
	// Navigation
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
	Enter key.Binding
	
	// View switching
	Tab       key.Binding
	Dashboard key.Binding
	Sessions  key.Binding
	Analytics key.Binding
	Help      key.Binding
	
	// Actions
	Refresh key.Binding
	Export  key.Binding
	Search  key.Binding
	Filter  key.Binding
	Sort    key.Binding
	
	// Table navigation
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
	
	// Application control
	Quit    key.Binding
	Back    key.Binding
	Escape  key.Binding
	
	// View-specific keys
	ZoomIn     key.Binding
	ZoomOut    key.Binding
	Reset      key.Binding
	Details    key.Binding
	
	// Grouping and filtering
	GroupBy    key.Binding
	TimeRange  key.Binding
	
	// Export and save
	Save       key.Binding
	SaveAs     key.Binding
	
	// Debug and development
	Debug      key.Binding
	Reload     key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "move left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "move right"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select/activate"),
		),
		
		// View switching
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next view"),
		),
		Dashboard: key.NewBinding(
			key.WithKeys("1", "d"),
			key.WithHelp("1/d", "dashboard"),
		),
		Sessions: key.NewBinding(
			key.WithKeys("2", "s"),
			key.WithHelp("2/s", "sessions"),
		),
		Analytics: key.NewBinding(
			key.WithKeys("3", "a"),
			key.WithHelp("3/a", "analytics"),
		),
		Help: key.NewBinding(
			key.WithKeys("?", "h"),
			key.WithHelp("?/h", "help"),
		),
		
		// Actions
		Refresh: key.NewBinding(
			key.WithKeys("r", "f5"),
			key.WithHelp("r/F5", "refresh"),
		),
		Export: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "export"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		Sort: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "sort"),
		),
		
		// Table navigation
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("pgup/b", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", " "),
			key.WithHelp("pgdn/space", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to bottom"),
		),
		
		// Application control
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q/ctrl+c", "quit"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "escape/cancel"),
		),
		
		// View-specific keys
		ZoomIn: key.NewBinding(
			key.WithKeys("+", "="),
			key.WithHelp("+", "zoom in"),
		),
		ZoomOut: key.NewBinding(
			key.WithKeys("-"),
			key.WithHelp("-", "zoom out"),
		),
		Reset: key.NewBinding(
			key.WithKeys("0"),
			key.WithHelp("0", "reset zoom"),
		),
		Details: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "details"),
		),
		
		// Grouping and filtering
		GroupBy: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "group by"),
		),
		TimeRange: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "time range"),
		),
		
		// Export and save
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "save"),
		),
		SaveAs: key.NewBinding(
			key.WithKeys("ctrl+shift+s"),
			key.WithHelp("ctrl+shift+s", "save as"),
		),
		
		// Debug and development
		Debug: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "debug"),
		),
		Reload: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "reload"),
		),
	}
}

// ShortHelp returns the short help text for key bindings
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Help,
		k.Quit,
		k.Tab,
		k.Refresh,
	}
}

// FullHelp returns all key bindings organized by category
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigation
		{k.Up, k.Down, k.Left, k.Right, k.Enter},
		// Views
		{k.Dashboard, k.Sessions, k.Analytics, k.Help},
		// Actions
		{k.Refresh, k.Search, k.Filter, k.Sort, k.Export},
		// Table
		{k.PageUp, k.PageDown, k.Home, k.End},
		// Control
		{k.Quit, k.Back, k.Save},
	}
}

// GetNavigationHelp returns navigation-specific help
func (k KeyMap) GetNavigationHelp() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.Left, k.Right, k.Enter,
		k.PageUp, k.PageDown, k.Home, k.End,
	}
}

// GetViewHelp returns view-switching help
func (k KeyMap) GetViewHelp() []key.Binding {
	return []key.Binding{
		k.Tab, k.Dashboard, k.Sessions, k.Analytics, k.Help,
	}
}

// GetActionHelp returns action-specific help
func (k KeyMap) GetActionHelp() []key.Binding {
	return []key.Binding{
		k.Refresh, k.Search, k.Filter, k.Sort, k.Export,
		k.ZoomIn, k.ZoomOut, k.Reset, k.Details,
	}
}

// GetSystemHelp returns system control help
func (k KeyMap) GetSystemHelp() []key.Binding {
	return []key.Binding{
		k.Quit, k.Back, k.Escape, k.Save, k.SaveAs,
	}
}

// DashboardKeyMap returns dashboard-specific key bindings
func DashboardKeyMap() KeyMap {
	km := DefaultKeyMap()
	
	// Override or add dashboard-specific bindings
	km.Details = key.NewBinding(
		key.WithKeys("enter", "i"),
		key.WithHelp("enter/i", "view details"),
	)
	
	return km
}

// SessionsKeyMap returns sessions view-specific key bindings
func SessionsKeyMap() KeyMap {
	km := DefaultKeyMap()
	
	// Override or add sessions-specific bindings
	km.Details = key.NewBinding(
		key.WithKeys("enter", "i"),
		key.WithHelp("enter/i", "session details"),
	)
	
	km.Export = key.NewBinding(
		key.WithKeys("e", "x"),
		key.WithHelp("e/x", "export session"),
	)
	
	return km
}

// AnalyticsKeyMap returns analytics view-specific key bindings
func AnalyticsKeyMap() KeyMap {
	km := DefaultKeyMap()
	
	// Override or add analytics-specific bindings
	km.TimeRange = key.NewBinding(
		key.WithKeys("t", "T"),
		key.WithHelp("t/T", "change time range"),
	)
	
	km.GroupBy = key.NewBinding(
		key.WithKeys("g", "G"),
		key.WithHelp("g/G", "group by field"),
	)
	
	return km
}

// HelpKeyMap returns help view-specific key bindings
func HelpKeyMap() KeyMap {
	km := DefaultKeyMap()
	
	// Help view mainly uses navigation keys
	return km
}

// IsQuit checks if a key press is a quit command
func (k KeyMap) IsQuit(keyMsg string) bool {
	// Simplified check for common quit keys
	return keyMsg == "q" || keyMsg == "ctrl+c"
}

// IsNavigation checks if a key press is a navigation command
func (k KeyMap) IsNavigation(keyMsg string) bool {
	navKeys := []string{"up", "down", "left", "right", "k", "j", "h", "l", 
		"pgup", "pgdown", "home", "end", "g", "G", "b", " "}
	for _, navKey := range navKeys {
		if keyMsg == navKey {
			return true
		}
	}
	return false
}

// IsViewSwitch checks if a key press is a view switching command
func (k KeyMap) IsViewSwitch(keyMsg string) bool {
	viewKeys := []string{"tab", "1", "2", "3", "d", "s", "a", "?", "h"}
	for _, viewKey := range viewKeys {
		if keyMsg == viewKey {
			return true
		}
	}
	return false
}

// IsAction checks if a key press is an action command
func (k KeyMap) IsAction(keyMsg string) bool {
	actionKeys := []string{"r", "f5", "e", "/", "f", "S", "ctrl+s", "ctrl+shift+s"}
	for _, actionKey := range actionKeys {
		if keyMsg == actionKey {
			return true
		}
	}
	return false
}