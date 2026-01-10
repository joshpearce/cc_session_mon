package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	warningColor   = lipgloss.Color("#F59E0B") // Amber
	dangerColor    = lipgloss.Color("#EF4444") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	bgColor        = lipgloss.Color("#1F2937") // Dark background
	fgColor        = lipgloss.Color("#F9FAFB") // Light foreground
)

// Header styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	statusStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	activeIndicatorStyle = lipgloss.NewStyle().
				Foreground(secondaryColor).
				Bold(true)

	inactiveIndicatorStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(dangerColor).
			Bold(true).
			Padding(1)
)

// Tab styles
var (
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Background(primaryColor).
			Foreground(fgColor).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Padding(0, 2)

	tabGapStyle = lipgloss.NewStyle().
			Foreground(mutedColor)
)

// List item styles
var (
	selectedItemStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#374151")).
				Foreground(fgColor).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(fgColor)

	// Active session indicator
	activeSessionStyle = lipgloss.NewStyle().
				Foreground(secondaryColor).
				Bold(true)

	inactiveSessionStyle = lipgloss.NewStyle().
				Foreground(mutedColor)
)

// Tool-specific styles
var (
	bashStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	editStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	writeStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	dangerousStyle = lipgloss.NewStyle().
			Foreground(dangerColor).
			Bold(true)
)

// Pattern styles
var (
	countBadgeStyle = lipgloss.NewStyle().
			Background(primaryColor).
			Foreground(fgColor).
			Padding(0, 1)

	exampleStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)
)

// Help style
var (
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor)
)

// Timestamp style
var (
	timestampStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Width(8)
)

// StyleForTool returns appropriate style based on tool and pattern
func StyleForTool(toolName, pattern string) lipgloss.Style {
	// Highlight dangerous patterns
	if IsDangerousPattern(pattern) {
		return dangerousStyle
	}

	switch toolName {
	case "Bash":
		return bashStyle
	case "Edit":
		return editStyle
	case "Write":
		return writeStyle
	default:
		return normalItemStyle
	}
}

// IsDangerousPattern checks if a pattern warrants extra attention
func IsDangerousPattern(pattern string) bool {
	dangerous := map[string]bool{
		"Bash(rm:*)":    true,
		"Bash(sudo:*)":  true,
		"Bash(chmod:*)": true,
		"Bash(chown:*)": true,
		"Bash(mv:*)":    true,
		"Bash(dd:*)":    true,
		"Bash(mkfs:*)":  true,
		"Bash(kill:*)":  true,
	}
	return dangerous[pattern]
}
