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
	if getter, ok := t.colorGetters()[name]; ok {
		return lipgloss.Color(getter().Hex)
	}
	return lipgloss.Color(t.flavor.Text().Hex)
}

// colorGetters returns a map of color name to getter function.
// This replaces the switch statement for O(1) lookup.
func (t *Theme) colorGetters() map[string]func() catppuccin.Color {
	return map[string]func() catppuccin.Color{
		"rosewater": t.flavor.Rosewater,
		"flamingo":  t.flavor.Flamingo,
		"pink":      t.flavor.Pink,
		"mauve":     t.flavor.Mauve,
		"red":       t.flavor.Red,
		"maroon":    t.flavor.Maroon,
		"peach":     t.flavor.Peach,
		"yellow":    t.flavor.Yellow,
		"green":     t.flavor.Green,
		"teal":      t.flavor.Teal,
		"sky":       t.flavor.Sky,
		"sapphire":  t.flavor.Sapphire,
		"blue":      t.flavor.Blue,
		"lavender":  t.flavor.Lavender,
		"text":      t.flavor.Text,
		"subtext1":  t.flavor.Subtext1,
		"subtext0":  t.flavor.Subtext0,
		"overlay2":  t.flavor.Overlay2,
		"overlay1":  t.flavor.Overlay1,
		"overlay0":  t.flavor.Overlay0,
		"surface2":  t.flavor.Surface2,
		"surface1":  t.flavor.Surface1,
		"surface0":  t.flavor.Surface0,
		"base":      t.flavor.Base,
		"mantle":    t.flavor.Mantle,
		"crust":     t.flavor.Crust,
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

// Detail panel styles

// DetailHeaderStyle returns style for detail panel header
func DetailHeaderStyle(width int) lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Text).
		Background(t.Surface1).
		Width(width).
		Padding(0, 1)
}

// LabelStyle returns style for field labels in detail panel
func LabelStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary)
}

// PathStyle returns style for file paths
func PathStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Secondary)
}

// CodeBlockStyle returns style for code blocks
func CodeBlockStyle(width int) lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Background(t.Surface).
		Foreground(t.Text).
		Width(width).
		Padding(0, 1)
}

// DangerHeaderStyle returns style for security warning headers
func DangerHeaderStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Danger)
}

// DangerStyle returns style for security warning text
func DangerStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Danger)
}

// WarningStyle returns style for warning/caution text
func WarningStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Warning)
}

// DeletionStyle returns style for deleted/old content in diffs
func DeletionStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Danger)
}

// AdditionStyle returns style for added/new content in diffs
func AdditionStyle() lipgloss.Style {
	t := GetTheme()
	return lipgloss.NewStyle().
		Foreground(t.Secondary)
}
