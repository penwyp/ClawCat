package ui

import (
	"strings"
	
	"github.com/charmbracelet/lipgloss"
)

// Theme defines color scheme for the application
type Theme struct {
	Primary     lipgloss.Color
	Secondary   lipgloss.Color
	Success     lipgloss.Color
	Warning     lipgloss.Color
	Error       lipgloss.Color
	Info        lipgloss.Color
	Background  lipgloss.Color
	Foreground  lipgloss.Color
	Muted       lipgloss.Color
	Border      lipgloss.Color
	Highlight   lipgloss.Color
	Accent      lipgloss.Color
}

// Styles contains all styled components
type Styles struct {
	// Basic text styles
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Normal      lipgloss.Style
	Bold        lipgloss.Style
	Italic      lipgloss.Style
	Muted       lipgloss.Style
	
	// Status styles
	Success     lipgloss.Style
	Warning     lipgloss.Style
	Error       lipgloss.Style
	Info        lipgloss.Style
	
	// Layout styles
	Border      lipgloss.Style
	Panel       lipgloss.Style
	Header      lipgloss.Style
	Footer      lipgloss.Style
	Sidebar     lipgloss.Style
	Content     lipgloss.Style
	
	// Component styles
	Button      lipgloss.Style
	ButtonFocus lipgloss.Style
	Input       lipgloss.Style
	InputFocus  lipgloss.Style
	Table       lipgloss.Style
	TableHeader lipgloss.Style
	TableRow    lipgloss.Style
	TableFocus  lipgloss.Style
	
	// Chart styles
	Chart       lipgloss.Style
	ChartAxis   lipgloss.Style
	ChartBar    lipgloss.Style
	ChartLine   lipgloss.Style
	
	// Progress styles
	Progress    lipgloss.Style
	ProgressBar lipgloss.Style
	ProgressFill lipgloss.Style
	
	// Special styles
	Highlight   lipgloss.Style
	Selected    lipgloss.Style
	Disabled    lipgloss.Style
	Loading     lipgloss.Style
}

// DefaultTheme returns the default color theme
func DefaultTheme() Theme {
	return DarkTheme()
}

// DarkTheme returns a dark color theme
func DarkTheme() Theme {
	return Theme{
		Primary:     lipgloss.Color("#7C3AED"), // Purple
		Secondary:   lipgloss.Color("#6366F1"), // Indigo
		Success:     lipgloss.Color("#10B981"), // Green
		Warning:     lipgloss.Color("#F59E0B"), // Amber
		Error:       lipgloss.Color("#EF4444"), // Red
		Info:        lipgloss.Color("#3B82F6"), // Blue
		Background:  lipgloss.Color("#1F2937"), // Gray-800
		Foreground:  lipgloss.Color("#F3F4F6"), // Gray-100
		Muted:       lipgloss.Color("#9CA3AF"), // Gray-400
		Border:      lipgloss.Color("#374151"), // Gray-700
		Highlight:   lipgloss.Color("#FCD34D"), // Yellow-300
		Accent:      lipgloss.Color("#EC4899"), // Pink
	}
}

// LightTheme returns a light color theme
func LightTheme() Theme {
	return Theme{
		Primary:     lipgloss.Color("#7C3AED"), // Purple
		Secondary:   lipgloss.Color("#6366F1"), // Indigo
		Success:     lipgloss.Color("#059669"), // Green-600
		Warning:     lipgloss.Color("#D97706"), // Amber-600
		Error:       lipgloss.Color("#DC2626"), // Red-600
		Info:        lipgloss.Color("#2563EB"), // Blue-600
		Background:  lipgloss.Color("#FFFFFF"), // White
		Foreground:  lipgloss.Color("#111827"), // Gray-900
		Muted:       lipgloss.Color("#6B7280"), // Gray-500
		Border:      lipgloss.Color("#D1D5DB"), // Gray-300
		Highlight:   lipgloss.Color("#FDE047"), // Yellow-300
		Accent:      lipgloss.Color("#EC4899"), // Pink
	}
}

// HighContrastTheme returns a high contrast theme for accessibility
func HighContrastTheme() Theme {
	return Theme{
		Primary:     lipgloss.Color("#FFFFFF"), // White
		Secondary:   lipgloss.Color("#CCCCCC"), // Light gray
		Success:     lipgloss.Color("#00FF00"), // Bright green
		Warning:     lipgloss.Color("#FFFF00"), // Bright yellow
		Error:       lipgloss.Color("#FF0000"), // Bright red
		Info:        lipgloss.Color("#00FFFF"), // Bright cyan
		Background:  lipgloss.Color("#000000"), // Black
		Foreground:  lipgloss.Color("#FFFFFF"), // White
		Muted:       lipgloss.Color("#808080"), // Gray
		Border:      lipgloss.Color("#FFFFFF"), // White
		Highlight:   lipgloss.Color("#FFFF00"), // Bright yellow
		Accent:      lipgloss.Color("#FF00FF"), // Bright magenta
	}
}

// NewStyles creates styles based on a theme
func NewStyles(theme Theme) Styles {
	return Styles{
		// Basic text styles
		Title: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true).
			MarginBottom(1),
		
		Subtitle: lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Bold(true),
		
		Normal: lipgloss.NewStyle().
			Foreground(theme.Foreground),
		
		Bold: lipgloss.NewStyle().
			Foreground(theme.Foreground).
			Bold(true),
		
		Italic: lipgloss.NewStyle().
			Foreground(theme.Foreground).
			Italic(true),
		
		Muted: lipgloss.NewStyle().
			Foreground(theme.Muted),
		
		// Status styles
		Success: lipgloss.NewStyle().
			Foreground(theme.Success).
			Bold(true),
		
		Warning: lipgloss.NewStyle().
			Foreground(theme.Warning).
			Bold(true),
		
		Error: lipgloss.NewStyle().
			Foreground(theme.Error).
			Bold(true),
		
		Info: lipgloss.NewStyle().
			Foreground(theme.Info).
			Bold(true),
		
		// Layout styles
		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1),
		
		Panel: lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1).
			Margin(1),
		
		Header: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Background(theme.Background).
			Bold(true).
			Padding(0, 1).
			Border(lipgloss.Border{Bottom: "─"}).
			BorderForeground(theme.Border),
		
		Footer: lipgloss.NewStyle().
			Foreground(theme.Muted).
			Background(theme.Background).
			Padding(0, 1).
			Border(lipgloss.Border{Top: "─"}).
			BorderForeground(theme.Border),
		
		Sidebar: lipgloss.NewStyle().
			Background(theme.Background).
			Border(lipgloss.Border{Right: "│"}).
			BorderForeground(theme.Border).
			Padding(1),
		
		Content: lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground).
			Padding(1),
		
		// Component styles
		Button: lipgloss.NewStyle().
			Foreground(theme.Background).
			Background(theme.Secondary).
			Padding(0, 2).
			Margin(0, 1),
		
		ButtonFocus: lipgloss.NewStyle().
			Foreground(theme.Background).
			Background(theme.Primary).
			Padding(0, 2).
			Margin(0, 1).
			Bold(true),
		
		Input: lipgloss.NewStyle().
			Foreground(theme.Foreground).
			Background(theme.Background).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),
		
		InputFocus: lipgloss.NewStyle().
			Foreground(theme.Foreground).
			Background(theme.Background).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Primary).
			Padding(0, 1),
		
		Table: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border),
		
		TableHeader: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true).
			Padding(0, 1).
			Border(lipgloss.Border{Bottom: "─"}).
			BorderForeground(theme.Border),
		
		TableRow: lipgloss.NewStyle().
			Foreground(theme.Foreground).
			Padding(0, 1),
		
		TableFocus: lipgloss.NewStyle().
			Foreground(theme.Background).
			Background(theme.Primary).
			Padding(0, 1).
			Bold(true),
		
		// Chart styles
		Chart: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1),
		
		ChartAxis: lipgloss.NewStyle().
			Foreground(theme.Muted),
		
		ChartBar: lipgloss.NewStyle().
			Foreground(theme.Primary),
		
		ChartLine: lipgloss.NewStyle().
			Foreground(theme.Secondary),
		
		// Progress styles
		Progress: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),
		
		ProgressBar: lipgloss.NewStyle().
			Background(theme.Background).
			Height(1),
		
		ProgressFill: lipgloss.NewStyle().
			Background(theme.Primary).
			Height(1),
		
		// Special styles
		Highlight: lipgloss.NewStyle().
			Foreground(theme.Background).
			Background(theme.Highlight).
			Bold(true),
		
		Selected: lipgloss.NewStyle().
			Foreground(theme.Background).
			Background(theme.Primary).
			Bold(true),
		
		Disabled: lipgloss.NewStyle().
			Foreground(theme.Muted).
			Strikethrough(true),
		
		Loading: lipgloss.NewStyle().
			Foreground(theme.Info).
			Bold(true),
	}
}

// Dashboard-specific styles
func (s Styles) DashboardCard() lipgloss.Style {
	return s.Panel.
		Width(30).
		Height(8)
}

func (s Styles) DashboardMetric() lipgloss.Style {
	return s.Bold.
		Align(lipgloss.Center)
}

func (s Styles) DashboardLabel() lipgloss.Style {
	return s.Muted.
		Align(lipgloss.Center)
}

// Session list specific styles
func (s Styles) SessionActive() lipgloss.Style {
	return s.Success
}

func (s Styles) SessionInactive() lipgloss.Style {
	return s.Muted
}

func (s Styles) SessionCost(cost float64) lipgloss.Style {
	if cost > 10.0 {
		return s.Error
	} else if cost > 1.0 {
		return s.Warning
	}
	return s.Success
}

// Analytics specific styles
func (s Styles) ChartTitle() lipgloss.Style {
	return s.Subtitle.
		Align(lipgloss.Center).
		MarginBottom(1)
}

func (s Styles) StatCard() lipgloss.Style {
	return s.Panel.
		Width(20).
		Height(5).
		Align(lipgloss.Center)
}

// Status bar styles
func (s Styles) StatusBar() lipgloss.Style {
	return s.Footer.
		Width(100).
		Align(lipgloss.Left)
}

func (s Styles) StatusInfo() lipgloss.Style {
	return s.Info
}

func (s Styles) StatusWarning() lipgloss.Style {
	return s.Warning
}

func (s Styles) StatusError() lipgloss.Style {
	return s.Error
}

// Help styles
func (s Styles) HelpTitle() lipgloss.Style {
	return s.Title.
		Align(lipgloss.Center).
		MarginBottom(2)
}

func (s Styles) HelpSection() lipgloss.Style {
	return s.Subtitle.
		MarginTop(1).
		MarginBottom(1)
}

func (s Styles) HelpKey() lipgloss.Style {
	return s.Bold
}

func (s Styles) HelpDescription() lipgloss.Style {
	return s.Normal
}

// Utility functions for dynamic styling

// ColorByValue returns a color based on a value range
func (s Styles) ColorByValue(value, min, max float64, theme Theme) lipgloss.Color {
	if value <= min {
		return theme.Success
	} else if value >= max {
		return theme.Error
	} else {
		return theme.Warning
	}
}

// StyleByStatus returns a style based on status
func (s Styles) StyleByStatus(status string) lipgloss.Style {
	switch status {
	case "active", "running", "online":
		return s.Success
	case "warning", "degraded":
		return s.Warning
	case "error", "failed", "offline":
		return s.Error
	case "info", "pending":
		return s.Info
	default:
		return s.Normal
	}
}

// ProgressStyle returns a styled progress bar
func (s Styles) ProgressStyle(percent float64, width int) string {
	filled := int(float64(width) * percent / 100.0)
	empty := width - filled
	
	bar := s.ProgressFill.Render(strings.Repeat("█", filled)) +
		s.ProgressBar.Render(strings.Repeat("░", empty))
	
	return s.Progress.Render(bar)
}


// GetThemeByName returns a theme by its name
func GetThemeByName(name string) Theme {
	switch name {
	case "dark":
		return DarkTheme()
	case "light":
		return LightTheme()
	case "high-contrast":
		return HighContrastTheme()
	default:
		return DefaultTheme()
	}
}

// GetAvailableThemes returns all available theme names
func GetAvailableThemes() []string {
	return []string{"dark", "light", "high-contrast"}
}

// Additional styles for progress components
func (s Styles) Box() lipgloss.Style {
	return s.Border
}

func (s Styles) SectionTitle() lipgloss.Style {
	return s.Subtitle.Bold(true)
}

func (s Styles) Card() lipgloss.Style {
	return s.Panel.
		Padding(1).
		Margin(0, 1)
}

func (s Styles) ButtonActive() lipgloss.Style {
	return s.ButtonFocus
}

func (s Styles) Faint() lipgloss.Style {
	return s.Muted
}

func (s Styles) Help() lipgloss.Style {
	return s.Footer.
		Foreground(s.Muted.GetForeground())
}