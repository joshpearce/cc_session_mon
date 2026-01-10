package tui

import (
	"fmt"
	"io"
	"path/filepath"
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
type sessionDelegate struct{}

func newSessionDelegate() sessionDelegate {
	return sessionDelegate{}
}

func (d sessionDelegate) Height() int                             { return 2 }
func (d sessionDelegate) Spacing() int                            { return 1 }
func (d sessionDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d sessionDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(sessionItem)
	if !ok {
		return
	}

	// Activity indicator
	var indicator string
	var nameStyle lipgloss.Style
	if i.session.IsActive {
		indicator = activeSessionStyle.Render("● ")
		nameStyle = activeSessionStyle
	} else {
		indicator = inactiveSessionStyle.Render("  ")
		nameStyle = inactiveSessionStyle
	}

	// Selection highlight
	if index == m.Index() {
		nameStyle = selectedItemStyle
	}

	name := nameStyle.Render(filepath.Base(i.session.ProjectPath))
	desc := mutedStyle.Render(fmt.Sprintf("  %d commands | %s",
		len(i.session.Commands),
		formatTimeAgo(i.session.LastActivity),
	))

	fmt.Fprintf(w, "%s%s\n%s", indicator, name, desc)
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
func (i commandItem) Description() string { return truncate(i.command.RawCommand, 60) }

// commandDelegate renders command items
type commandDelegate struct{}

func newCommandDelegate() commandDelegate {
	return commandDelegate{}
}

func (d commandDelegate) Height() int                             { return 2 }
func (d commandDelegate) Spacing() int                            { return 0 }
func (d commandDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d commandDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(commandItem)
	if !ok {
		return
	}

	// Get style based on tool/pattern
	style := StyleForTool(i.command.ToolName, i.command.Pattern)
	if index == m.Index() {
		style = style.Background(lipgloss.Color("#374151"))
	}

	timestamp := timestampStyle.Render(i.command.Timestamp.Format("15:04:05"))
	pattern := style.Render(i.command.Pattern)
	raw := truncate(i.command.RawCommand, 50)

	fmt.Fprintf(w, "%s %s\n   %s", timestamp, pattern, mutedStyle.Render(raw))
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
type patternDelegate struct{}

func newPatternDelegate() patternDelegate {
	return patternDelegate{}
}

func (d patternDelegate) Height() int                             { return 2 }
func (d patternDelegate) Spacing() int                            { return 0 }
func (d patternDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d patternDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(patternItem)
	if !ok {
		return
	}

	// Get style based on tool/pattern
	style := StyleForTool(i.pattern.ToolName, i.pattern.Pattern)
	if index == m.Index() {
		style = style.Background(lipgloss.Color("#374151"))
	}

	pattern := style.Render(i.pattern.Pattern)
	count := countBadgeStyle.Render(fmt.Sprintf("%d", i.pattern.Count))

	// Show first example if available
	example := ""
	if len(i.pattern.Examples) > 0 {
		example = truncate(i.pattern.Examples[0], 40)
	}

	fmt.Fprintf(w, "%s %s\n   %s", pattern, count, exampleStyle.Render(example))
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

// truncate shortens a string to max length with ellipsis
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// mutedStyle for description text
var mutedStyle = lipgloss.NewStyle().Foreground(mutedColor)
