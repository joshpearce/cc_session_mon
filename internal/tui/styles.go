package tui

import (
	"cc_session_mon/internal/config"

	catppuccin "github.com/catppuccin/go"
	"github.com/charmbracelet/lipgloss"
)

// Theme holds the current color palette
type Theme struct {
	flavor    catppuccin.Flavor
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Warning   lipgloss.Color
	Danger    lipgloss.Color
	Muted     lipgloss.Color
	Surface   lipgloss.Color // Surface0 - used for selected row
	Surface1  lipgloss.Color // Surface1 - used for header row
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
		flavor:    flavor,
		Primary:   lipgloss.Color(flavor.Mauve().Hex),
		Secondary: lipgloss.Color(flavor.Green().Hex),
		Warning:   lipgloss.Color(flavor.Yellow().Hex),
		Danger:    lipgloss.Color(flavor.Red().Hex),
		Muted:     lipgloss.Color(flavor.Overlay0().Hex),
		Surface:   lipgloss.Color(flavor.Surface0().Hex),
		Surface1:  lipgloss.Color(flavor.Surface1().Hex),
		Base:      lipgloss.Color(flavor.Base().Hex),
		Text:      lipgloss.Color(flavor.Text().Hex),
	}
}

// ColorByName returns a lipgloss.Color for a catppuccin color name
func (t *Theme) ColorByName(name string) lipgloss.Color {
	switch name {
	case "rosewater":
		return lipgloss.Color(t.flavor.Rosewater().Hex)
	case "flamingo":
		return lipgloss.Color(t.flavor.Flamingo().Hex)
	case "pink":
		return lipgloss.Color(t.flavor.Pink().Hex)
	case "mauve":
		return lipgloss.Color(t.flavor.Mauve().Hex)
	case "red":
		return lipgloss.Color(t.flavor.Red().Hex)
	case "maroon":
		return lipgloss.Color(t.flavor.Maroon().Hex)
	case "peach":
		return lipgloss.Color(t.flavor.Peach().Hex)
	case "yellow":
		return lipgloss.Color(t.flavor.Yellow().Hex)
	case "green":
		return lipgloss.Color(t.flavor.Green().Hex)
	case "teal":
		return lipgloss.Color(t.flavor.Teal().Hex)
	case "sky":
		return lipgloss.Color(t.flavor.Sky().Hex)
	case "sapphire":
		return lipgloss.Color(t.flavor.Sapphire().Hex)
	case "blue":
		return lipgloss.Color(t.flavor.Blue().Hex)
	case "lavender":
		return lipgloss.Color(t.flavor.Lavender().Hex)
	case "text":
		return lipgloss.Color(t.flavor.Text().Hex)
	case "subtext1":
		return lipgloss.Color(t.flavor.Subtext1().Hex)
	case "subtext0":
		return lipgloss.Color(t.flavor.Subtext0().Hex)
	case "overlay2":
		return lipgloss.Color(t.flavor.Overlay2().Hex)
	case "overlay1":
		return lipgloss.Color(t.flavor.Overlay1().Hex)
	case "overlay0":
		return lipgloss.Color(t.flavor.Overlay0().Hex)
	case "surface2":
		return lipgloss.Color(t.flavor.Surface2().Hex)
	case "surface1":
		return lipgloss.Color(t.flavor.Surface1().Hex)
	case "surface0":
		return lipgloss.Color(t.flavor.Surface0().Hex)
	case "base":
		return lipgloss.Color(t.flavor.Base().Hex)
	case "mantle":
		return lipgloss.Color(t.flavor.Mantle().Hex)
	case "crust":
		return lipgloss.Color(t.flavor.Crust().Hex)
	default:
		return lipgloss.Color(t.flavor.Text().Hex)
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

func ColumnHeaderStyle(width int) lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Surface1).
		Bold(true).
		Width(width)
}

func TimestampStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Muted).
		Width(8)
}

// StyleForPattern returns appropriate style based on pattern
func StyleForPattern(pattern string) lipgloss.Style {
	t := GetTheme()

	group := config.Global().GetToolGroup(pattern)
	if group == nil {
		return NormalItemStyle()
	}

	style := lipgloss.NewStyle().Foreground(t.ColorByName(group.Color))
	if group.Bold {
		style = style.Bold(true)
	}
	return style
}
