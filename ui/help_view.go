package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
)

// HelpView represents the help view
type HelpView struct {
	keys   KeyMap
	width  int
	height int
	config Config
	styles Styles
}

// NewHelpView creates a new help view
func NewHelpView() *HelpView {
	return &HelpView{
		keys:   DefaultKeyMap(),
		styles: NewStyles(DefaultTheme()),
	}
}

// Init initializes the help view
func (h *HelpView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the help view
func (h *HelpView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Help view is mostly static
	return h, nil
}

// View renders the help view
func (h *HelpView) View() string {
	if h.width == 0 || h.height == 0 {
		return "Help loading..."
	}

	header := h.renderHeader()
	navigation := h.renderNavigationHelp()
	views := h.renderViewsHelp()
	actions := h.renderActionsHelp()
	system := h.renderSystemHelp()

	content := strings.Join([]string{
		header,
		navigation,
		views,
		actions,
		system,
	}, "\n\n")

	return h.styles.Content.
		Width(h.width - 4).
		Height(h.height - 4).
		Render(content)
}

// Resize updates the view dimensions
func (h *HelpView) Resize(width, height int) {
	h.width = width
	h.height = height
}

// UpdateConfig updates the view configuration
func (h *HelpView) UpdateConfig(config Config) {
	h.config = config
	h.styles = NewStyles(GetThemeByName(config.Theme))
}

// renderHeader renders the help header
func (h *HelpView) renderHeader() string {
	title := h.styles.HelpTitle().Render("ClawCat Help")
	subtitle := h.styles.Subtitle.Render("Keyboard shortcuts and navigation")
	return strings.Join([]string{title, subtitle}, "\n")
}

// renderNavigationHelp renders navigation help
func (h *HelpView) renderNavigationHelp() string {
	title := h.styles.HelpSection().Render("Navigation")
	
	bindings := []key.Binding{
		h.keys.Up, h.keys.Down, h.keys.Left, h.keys.Right,
		h.keys.Enter, h.keys.PageUp, h.keys.PageDown,
		h.keys.Home, h.keys.End,
	}
	
	help := h.renderKeyBindings(bindings)
	return strings.Join([]string{title, help}, "\n")
}

// renderViewsHelp renders view switching help
func (h *HelpView) renderViewsHelp() string {
	title := h.styles.HelpSection().Render("Views")
	
	bindings := []key.Binding{
		h.keys.Tab, h.keys.Dashboard, h.keys.Sessions,
		h.keys.Analytics, h.keys.Help,
	}
	
	help := h.renderKeyBindings(bindings)
	return strings.Join([]string{title, help}, "\n")
}

// renderActionsHelp renders action help
func (h *HelpView) renderActionsHelp() string {
	title := h.styles.HelpSection().Render("Actions")
	
	bindings := []key.Binding{
		h.keys.Refresh, h.keys.Search, h.keys.Filter,
		h.keys.Sort, h.keys.Export, h.keys.Save,
	}
	
	help := h.renderKeyBindings(bindings)
	return strings.Join([]string{title, help}, "\n")
}

// renderSystemHelp renders system control help
func (h *HelpView) renderSystemHelp() string {
	title := h.styles.HelpSection().Render("System")
	
	bindings := []key.Binding{
		h.keys.Quit, h.keys.Back, h.keys.Escape,
	}
	
	help := h.renderKeyBindings(bindings)
	return strings.Join([]string{title, help}, "\n")
}

// renderKeyBindings renders a list of key bindings
func (h *HelpView) renderKeyBindings(bindings []key.Binding) string {
	var lines []string
	
	for _, binding := range bindings {
		keyStr := h.styles.HelpKey().Render(binding.Help().Key)
		descStr := h.styles.HelpDescription().Render(binding.Help().Desc)
		line := keyStr + "  " + descStr
		lines = append(lines, line)
	}
	
	return strings.Join(lines, "\n")
}