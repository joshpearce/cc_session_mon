package tui

import (
	"cc_session_mon/internal/config"

	catppuccin "github.com/catppuccin/go"
	"github.com/charmbracelet/lipgloss"
)

// Theme holds the current color palette
type Theme struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Warning   lipgloss.Color
	Danger    lipgloss.Color
	Muted     lipgloss.Color
	Surface   lipgloss.Color
	Base      lipgloss.Color
	Text      lipgloss.Color
}

// currentTheme is the active theme
var currentTheme *Theme

// GetTheme returns the current theme, initializing if needed
func GetTheme() *Theme {
	if currentTheme == nil {
		currentTheme = loadTheme(config.Global().Theme)
	}
	return currentTheme
}

// loadTheme creates a Theme from a catppuccin flavor name
func loadTheme(name string) *Theme {
	var flavor catppuccin.Flavor

	switch name {
	case "latte":
		flavor = catppuccin.Latte
	case "frappe":
		flavor = catppuccin.Frappe
	case "macchiato":
		flavor = catppuccin.Macchiato
	case "mocha":
		flavor = catppuccin.Mocha
	default:
		flavor = catppuccin.Mocha
	}

	return &Theme{
		Primary:   lipgloss.Color(flavor.Mauve().Hex),
		Secondary: lipgloss.Color(flavor.Green().Hex),
		Warning:   lipgloss.Color(flavor.Yellow().Hex),
		Danger:    lipgloss.Color(flavor.Red().Hex),
		Muted:     lipgloss.Color(flavor.Overlay0().Hex),
		Surface:   lipgloss.Color(flavor.Surface0().Hex),
		Base:      lipgloss.Color(flavor.Base().Hex),
		Text:      lipgloss.Color(flavor.Text().Hex),
	}
}

// Style accessors - these create styles dynamically based on current theme

func TitleStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary)
}

func StatusStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Muted)
}

func ActiveIndicatorStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Secondary).
		Bold(true)
}

func InactiveIndicatorStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Muted)
}

func ErrorStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Danger).
		Bold(true).
		Padding(1)
}

func ActiveTabStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Bold(true).
		Background(t.Primary).
		Foreground(t.Base).
		Padding(0, 2)
}

func InactiveTabStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Muted).
		Padding(0, 2)
}

func TabGapStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Muted)
}

func SelectedItemStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Background(t.Surface).
		Foreground(t.Text).
		Bold(true)
}

func NormalItemStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Text)
}

func ActiveSessionStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Secondary).
		Bold(true)
}

func InactiveSessionStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Muted)
}

func BashStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Warning)
}

func EditStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Secondary)
}

func WriteStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Primary)
}

func DangerousStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Danger).
		Bold(true)
}

func CountBadgeStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(t.Base).
		Padding(0, 1)
}

func ExampleStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Muted).
		Italic(true)
}

func HelpStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Muted)
}

func TimestampStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Muted).
		Width(8)
}

// StyleForTool returns appropriate style based on tool and pattern
func StyleForTool(toolName, pattern string) lipgloss.Style {
	if IsDangerousPattern(pattern) {
		return DangerousStyle()
	}

	switch toolName {
	case "Bash":
		return BashStyle()
	case "Edit":
		return EditStyle()
	case "Write":
		return WriteStyle()
	default:
		return NormalItemStyle()
	}
}

// IsDangerousPattern checks if a pattern warrants extra attention
func IsDangerousPattern(pattern string) bool {
	return config.Global().IsDangerousPattern(pattern)
}
