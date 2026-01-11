package tui

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"cc_session_mon/internal/session"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// Session Item
// ============================================================================

// sessionItem wraps a Session for the list component
type sessionItem struct {
	session *session.Session
}

func (i sessionItem) FilterValue() string { return i.session.ProjectPath }
func (i sessionItem) Title() string       { return filepath.Base(i.session.ProjectPath) }
func (i sessionItem) Description() string {
	status := "inactive"
	if i.session.IsActive {
		status = "active"
	}
	return fmt.Sprintf("%s | %d commands | %s",
		status,
		len(i.session.Commands),
		formatTimeAgo(i.session.LastActivity),
	)
}

// sessionDelegate renders session items
type sessionDelegate struct {
	width int
}

func newSessionDelegate() *sessionDelegate {
	return &sessionDelegate{width: 80}
}

func (d *sessionDelegate) SetWidth(w int) {
	d.width = w
}

func (d *sessionDelegate) Height() int                             { return 1 }
func (d *sessionDelegate) Spacing() int                            { return 0 }
func (d *sessionDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d *sessionDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(sessionItem)
	if !ok {
		return
	}

	// Build the row content
	var indicator string
	if i.session.IsActive {
		indicator = "● "
	} else {
		indicator = "  "
	}

	name := i.session.ProjectPath
	info := fmt.Sprintf(" %d cmds | %s",
		len(i.session.Commands),
		formatTimeAgo(i.session.LastActivity),
	)

	// Calculate available space for name
	availableWidth := d.width - len(indicator) - len(info) - 2
	if availableWidth < 10 {
		availableWidth = 10
	}

	// Truncate or pad name
	if len(name) > availableWidth {
		name = name[:availableWidth-3] + "..."
	}

	row := indicator + name + strings.Repeat(" ", max(0, availableWidth-len(name))) + info

	// Apply styling
	var style lipgloss.Style
	if index == m.Index() {
		style = lipgloss.NewStyle().
			Background(GetTheme().Surface).
			Foreground(GetTheme().Text).
			Bold(true).
			Width(d.width)
	} else if i.session.IsActive {
		style = lipgloss.NewStyle().
			Foreground(GetTheme().Secondary).
			Width(d.width)
	} else {
		style = lipgloss.NewStyle().
			Foreground(GetTheme().Muted).
			Width(d.width)
	}

	fmt.Fprint(w, style.Render(row))
}

// ============================================================================
// Command Item
// ============================================================================

// commandItem wraps a CommandEntry for the list component
type commandItem struct {
	command session.CommandEntry
}

func (i commandItem) FilterValue() string { return i.command.RawCommand }
func (i commandItem) Title() string       { return i.command.Pattern }
func (i commandItem) Description() string { return i.command.RawCommand }

// commandDelegate renders command items
type commandDelegate struct {
	width int
}

func newCommandDelegate() *commandDelegate {
	return &commandDelegate{width: 80}
}

func (d *commandDelegate) SetWidth(w int) {
	d.width = w
}

func (d *commandDelegate) Height() int                             { return 1 }
func (d *commandDelegate) Spacing() int                            { return 0 }
func (d *commandDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d *commandDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(commandItem)
	if !ok {
		return
	}

	// Format: "Jan 02 15:04  Pattern  command..."
	timestamp := i.command.Timestamp.Format("Jan 02 15:04")
	pattern := i.command.Pattern

	// Fixed widths for timestamp and pattern
	timestampWidth := 12
	patternWidth := 20

	// Pad/truncate pattern to fixed width
	if len(pattern) > patternWidth {
		pattern = pattern[:patternWidth-1] + "…"
	} else {
		pattern = pattern + strings.Repeat(" ", patternWidth-len(pattern))
	}

	// Calculate space for raw command
	// Format: "timestamp  pattern  command"
	fixedWidth := timestampWidth + 2 + patternWidth + 2
	commandWidth := d.width - fixedWidth
	if commandWidth < 10 {
		commandWidth = 10
	}

	// Replace newlines with visible marker to keep single-line display
	rawCmd := strings.ReplaceAll(i.command.RawCommand, "\n", "↵")
	if len(rawCmd) > commandWidth {
		rawCmd = rawCmd[:commandWidth-1] + "…"
	}

	row := fmt.Sprintf("%s  %s  %s", timestamp, pattern, rawCmd)

	// Pad to full width
	if len(row) < d.width {
		row = row + strings.Repeat(" ", d.width-len(row))
	}

	// Apply styling based on selection and tool type
	var style lipgloss.Style
	baseStyle := StyleForTool(i.command.ToolName, i.command.Pattern)

	if index == m.Index() {
		style = baseStyle.
			Background(GetTheme().Surface).
			Bold(true).
			Width(d.width)
	} else {
		style = baseStyle.Width(d.width)
	}

	fmt.Fprint(w, style.Render(row))
}

// ============================================================================
// Pattern Item
// ============================================================================

// patternItem wraps a CommandPattern for the list component
type patternItem struct {
	pattern *session.CommandPattern
}

func (i patternItem) FilterValue() string { return i.pattern.Pattern }
func (i patternItem) Title() string       { return i.pattern.Pattern }
func (i patternItem) Description() string {
	return fmt.Sprintf("%d occurrences", i.pattern.Count)
}

// patternDelegate renders pattern items
type patternDelegate struct {
	width int
}

func newPatternDelegate() *patternDelegate {
	return &patternDelegate{width: 80}
}

func (d *patternDelegate) SetWidth(w int) {
	d.width = w
}

func (d *patternDelegate) Height() int                             { return 1 }
func (d *patternDelegate) Spacing() int                            { return 0 }
func (d *patternDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d *patternDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(patternItem)
	if !ok {
		return
	}

	// Format: "Pattern  [count]  example..."
	pattern := i.pattern.Pattern
	countStr := fmt.Sprintf("[%d]", i.pattern.Count)

	// Fixed widths
	patternWidth := 25
	countWidth := 8

	// Pad/truncate pattern
	if len(pattern) > patternWidth {
		pattern = pattern[:patternWidth-1] + "…"
	} else {
		pattern = pattern + strings.Repeat(" ", patternWidth-len(pattern))
	}

	// Pad count
	countStr = strings.Repeat(" ", countWidth-len(countStr)) + countStr

	// Calculate space for example
	fixedWidth := patternWidth + 2 + countWidth + 2
	exampleWidth := d.width - fixedWidth
	if exampleWidth < 10 {
		exampleWidth = 10
	}

	example := ""
	if len(i.pattern.Examples) > 0 {
		example = i.pattern.Examples[0]
		if len(example) > exampleWidth {
			example = example[:exampleWidth-1] + "…"
		}
	}

	row := fmt.Sprintf("%s  %s  %s", pattern, countStr, example)

	// Pad to full width
	if len(row) < d.width {
		row = row + strings.Repeat(" ", d.width-len(row))
	}

	// Apply styling
	var style lipgloss.Style
	baseStyle := StyleForTool(i.pattern.ToolName, i.pattern.Pattern)

	if index == m.Index() {
		style = baseStyle.
			Background(GetTheme().Surface).
			Bold(true).
			Width(d.width)
	} else {
		style = baseStyle.Width(d.width)
	}

	fmt.Fprint(w, style.Render(row))
}

// ============================================================================
// Helper Functions
// ============================================================================

// formatTimeAgo returns a human-readable relative time string
func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", hours)
	default:
		return t.Format("Jan 2")
	}
}

// MutedStyle returns a style for description text
func MutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(GetTheme().Muted)
}
