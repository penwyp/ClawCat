package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ProgressStyle defines different progress bar styles
type ProgressStyle int

const (
	ProgressStyleBar ProgressStyle = iota
	ProgressStyleDots
	ProgressStyleBlocks
	ProgressStyleSpinner
)

// ProgressIndicator provides terminal-native progress indicators
type ProgressIndicator struct {
	style     ProgressStyle
	width     int
	showLabel bool
	colors    bool
}

// NewProgressIndicator creates a new progress indicator
func NewProgressIndicator(style ProgressStyle, width int) *ProgressIndicator {
	return &ProgressIndicator{
		style:     style,
		width:     width,
		showLabel: true,
		colors:    true,
	}
}

// SetShowLabel enables/disables label display
func (pi *ProgressIndicator) SetShowLabel(show bool) {
	pi.showLabel = show
}

// SetColors enables/disables color output
func (pi *ProgressIndicator) SetColors(enabled bool) {
	pi.colors = enabled
}

// RenderProgress renders a progress bar with given percentage (0-100)
func (pi *ProgressIndicator) RenderProgress(percentage float64, label string) string {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}

	switch pi.style {
	case ProgressStyleBar:
		return pi.renderProgressBar(percentage, label)
	case ProgressStyleDots:
		return pi.renderProgressDots(percentage, label)
	case ProgressStyleBlocks:
		return pi.renderProgressBlocks(percentage, label)
	case ProgressStyleSpinner:
		return pi.renderSpinner(label)
	default:
		return pi.renderProgressBar(percentage, label)
	}
}

// renderProgressBar renders a traditional progress bar
func (pi *ProgressIndicator) renderProgressBar(percentage float64, label string) string {
	barWidth := pi.width
	if pi.showLabel && label != "" {
		// Reserve space for label and percentage
		barWidth = pi.width - len(label) - 10
		if barWidth < 10 {
			barWidth = 10
		}
	}

	filled := int((percentage / 100.0) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	var filledChar, emptyChar string
	if pi.colors {
		// Use color-coded progress
		color := pi.getProgressColor(percentage)
		filledChar = color.Render("█")
		emptyChar = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("░")
	} else {
		filledChar = "█"
		emptyChar = "░"
	}

	bar := strings.Repeat(filledChar, filled) + strings.Repeat(emptyChar, barWidth-filled)

	if pi.showLabel && label != "" {
		return fmt.Sprintf("%s [%s] %5.1f%%", label, bar, percentage)
	}
	return fmt.Sprintf("[%s] %5.1f%%", bar, percentage)
}

// renderProgressDots renders progress using dots
func (pi *ProgressIndicator) renderProgressDots(percentage float64, label string) string {
	dotCount := pi.width / 2 // Each dot takes 2 chars (dot + space)
	if dotCount < 5 {
		dotCount = 5
	}

	filled := int((percentage / 100.0) * float64(dotCount))
	if filled > dotCount {
		filled = dotCount
	}

	var dots []string
	for i := 0; i < dotCount; i++ {
		if i < filled {
			if pi.colors {
				color := pi.getProgressColor(percentage)
				dots = append(dots, color.Render("●"))
			} else {
				dots = append(dots, "●")
			}
		} else {
			if pi.colors {
				dots = append(dots, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○"))
			} else {
				dots = append(dots, "○")
			}
		}
	}

	dotsDisplay := strings.Join(dots, " ")

	if pi.showLabel && label != "" {
		return fmt.Sprintf("%s %s %.1f%%", label, dotsDisplay, percentage)
	}
	return fmt.Sprintf("%s %.1f%%", dotsDisplay, percentage)
}

// renderProgressBlocks renders progress using Unicode blocks
func (pi *ProgressIndicator) renderProgressBlocks(percentage float64, label string) string {
	blockWidth := pi.width
	if pi.showLabel && label != "" {
		blockWidth = pi.width - len(label) - 8
		if blockWidth < 8 {
			blockWidth = 8
		}
	}

	// Use different Unicode blocks for smoother progress
	blocks := []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}
	
	fullBlocks := int((percentage / 100.0) * float64(blockWidth))
	remainder := (percentage / 100.0) * float64(blockWidth) - float64(fullBlocks)
	partialBlock := int(remainder * 8)

	var progress strings.Builder
	
	// Add full blocks
	for i := 0; i < fullBlocks && i < blockWidth; i++ {
		if pi.colors {
			color := pi.getProgressColor(percentage)
			progress.WriteString(color.Render("█"))
		} else {
			progress.WriteString("█")
		}
	}
	
	// Add partial block
	if fullBlocks < blockWidth && partialBlock > 0 {
		if pi.colors {
			color := pi.getProgressColor(percentage)
			progress.WriteString(color.Render(blocks[partialBlock]))
		} else {
			progress.WriteString(blocks[partialBlock])
		}
		fullBlocks++
	}
	
	// Add empty space
	for i := fullBlocks; i < blockWidth; i++ {
		if pi.colors {
			progress.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("░"))
		} else {
			progress.WriteString("░")
		}
	}

	if pi.showLabel && label != "" {
		return fmt.Sprintf("%s %s %.1f%%", label, progress.String(), percentage)
	}
	return fmt.Sprintf("%s %.1f%%", progress.String(), percentage)
}

// renderSpinner renders an animated spinner
func (pi *ProgressIndicator) renderSpinner(label string) string {
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	// Use current time to animate
	frame := int(time.Now().UnixNano()/100000000) % len(spinners)
	
	spinner := spinners[frame]
	if pi.colors {
		spinner = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(spinner)
	}

	if pi.showLabel && label != "" {
		return fmt.Sprintf("%s %s", spinner, label)
	}
	return spinner
}

// getProgressColor returns appropriate color based on percentage
func (pi *ProgressIndicator) getProgressColor(percentage float64) lipgloss.Style {
	if percentage >= 80 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	} else if percentage >= 60 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // Yellow
	} else if percentage >= 40 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39"))  // Blue
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("46")) // Green
}

// MultiProgressIndicator handles multiple progress bars
type MultiProgressIndicator struct {
	indicators map[string]*ProgressIndicator
	width      int
}

// NewMultiProgressIndicator creates a new multi-progress indicator
func NewMultiProgressIndicator(width int) *MultiProgressIndicator {
	return &MultiProgressIndicator{
		indicators: make(map[string]*ProgressIndicator),
		width:      width,
	}
}

// AddProgress adds a new progress indicator
func (mpi *MultiProgressIndicator) AddProgress(id, label string, style ProgressStyle) {
	mpi.indicators[id] = NewProgressIndicator(style, mpi.width-len(label)-2)
}

// UpdateProgress updates a specific progress indicator
func (mpi *MultiProgressIndicator) UpdateProgress(id string, percentage float64) {
	if indicator, exists := mpi.indicators[id]; exists {
		// Store the percentage for later rendering
		// This is a simplified approach - in practice you'd store state
		_ = indicator
		_ = percentage
	}
}

// RenderAll renders all progress indicators
func (mpi *MultiProgressIndicator) RenderAll(progresses map[string]ProgressData) string {
	var lines []string
	
	for id, data := range progresses {
		if indicator, exists := mpi.indicators[id]; exists {
			line := indicator.RenderProgress(data.Percentage, data.Label)
			lines = append(lines, line)
		}
	}
	
	return strings.Join(lines, "\n")
}

// ProgressData holds data for a single progress item
type ProgressData struct {
	Label      string
	Percentage float64
}

// HealthIndicator provides health status indicators
type HealthIndicator struct {
	showDetails bool
	colors      bool
}

// NewHealthIndicator creates a new health indicator
func NewHealthIndicator() *HealthIndicator {
	return &HealthIndicator{
		showDetails: true,
		colors:      true,
	}
}

// RenderHealth renders health status with appropriate indicators
func (hi *HealthIndicator) RenderHealth(health int, label string, details map[string]interface{}) string {
	var indicator string
	var status string
	
	if health >= 90 {
		indicator = "🟢"
		status = "Excellent"
	} else if health >= 70 {
		indicator = "🟡"
		status = "Good"
	} else if health >= 50 {
		indicator = "🟠"
		status = "Warning"
	} else {
		indicator = "🔴"
		status = "Critical"
	}

	if hi.colors {
		switch {
		case health >= 90:
			status = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render(status)
		case health >= 70:
			status = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Render(status)
		case health >= 50:
			status = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render(status)
		default:
			status = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(status)
		}
	}

	result := fmt.Sprintf("%s %s: %s (%d%%)", indicator, label, status, health)

	if hi.showDetails && details != nil && len(details) > 0 {
		var detailStrings []string
		for key, value := range details {
			detailStrings = append(detailStrings, fmt.Sprintf("%s: %v", key, value))
		}
		result += " [" + strings.Join(detailStrings, ", ") + "]"
	}

	return result
}

// TrendIndicator shows trend arrows and patterns
type TrendIndicator struct {
	colors bool
}

// NewTrendIndicator creates a new trend indicator
func NewTrendIndicator() *TrendIndicator {
	return &TrendIndicator{colors: true}
}

// RenderTrend renders trend indicators
func (ti *TrendIndicator) RenderTrend(trend string, value float64, label string) string {
	var arrow string
	var color lipgloss.Style

	switch strings.ToLower(trend) {
	case "up", "increasing", "rising":
		arrow = "↗"
		color = lipgloss.NewStyle().Foreground(lipgloss.Color("46")) // Green
	case "down", "decreasing", "falling":
		arrow = "↘"
		color = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	case "stable", "steady":
		arrow = "→"
		color = lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // Blue
	case "volatile", "unstable":
		arrow = "↕"
		color = lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // Yellow
	default:
		arrow = "?"
		color = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray
	}

	if ti.colors {
		arrow = color.Render(arrow)
	}

	return fmt.Sprintf("%s %s: %.2f %s", arrow, label, value, trend)
}

// StatusBar renders a comprehensive status bar
type StatusBar struct {
	width      int
	sections   []StatusSection
	separator  string
	colors     bool
}

// StatusSection represents a section in the status bar
type StatusSection struct {
	Label string
	Value string
	Color lipgloss.Color
}

// NewStatusBar creates a new status bar
func NewStatusBar(width int) *StatusBar {
	return &StatusBar{
		width:     width,
		separator: " │ ",
		colors:    true,
	}
}

// AddSection adds a section to the status bar
func (sb *StatusBar) AddSection(label, value string, color lipgloss.Color) {
	sb.sections = append(sb.sections, StatusSection{
		Label: label,
		Value: value,
		Color: color,
	})
}

// Render renders the complete status bar
func (sb *StatusBar) Render() string {
	if len(sb.sections) == 0 {
		return ""
	}

	var parts []string
	for _, section := range sb.sections {
		part := fmt.Sprintf("%s: %s", section.Label, section.Value)
		if sb.colors && section.Color != "" {
			style := lipgloss.NewStyle().Foreground(section.Color)
			part = style.Render(part)
		}
		parts = append(parts, part)
	}

	result := strings.Join(parts, sb.separator)
	
	// Truncate if too long
	if len(result) > sb.width {
		result = result[:sb.width-3] + "..."
	}

	return result
}

// Clear clears all sections
func (sb *StatusBar) Clear() {
	sb.sections = []StatusSection{}
}