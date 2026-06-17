package tui

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cc_session_mon/internal/config"
	"cc_session_mon/internal/session"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Fixed column widths for the session list (shared by the header in view.go).
const (
	SessionAgentsWidth = 6 // subagents: "current/total", e.g. "3/29"
	SessionCmdsWidth   = 5 // command count
	SessionLastWidth   = 8 // relative time, e.g. "just now"
)

// formatAgents renders the subagent count column as "current/total", or a dash
// when the session spawned no subagents.
func formatAgents(current, total int) string {
	if total == 0 {
		return "—"
	}
	return fmt.Sprintf("%d/%d", current, total)
}

// SessionRateLabel is the burn-rate column header, parameterized by the
// configured window in minutes (e.g. "tok/m(10m)").
func SessionRateLabel(mins int) string {
	return fmt.Sprintf("tok/m(%dm)", mins)
}

// SessionRateWidth is the burn-rate column width: wide enough for the header
// label and for formatted values like "10.0k".
func SessionRateWidth(mins int) int {
	return max(len(SessionRateLabel(mins)), 7)
}

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

	// Tag sessions discovered outside the local projects dir with their origin
	// label (e.g. a devcontainer repo/container name).
	var originTag string
	if i.session.Origin != "" && i.session.Origin != "local" {
		originTag = " ▣ " + i.session.Origin
	}

	// Fixed-width right columns: rate | agents | cmds | last. Built with the same
	// format string as the header (renderSessionHeaders) so the columns line up.
	// Metrics are precomputed on the session (see Watcher.refreshSessionMetrics);
	// the render path only reads finished values.
	mins := int(config.Global().BurnWindow().Minutes())
	rate := formatTokPerMin(i.session.Burn.TokensPerMinute)
	am := i.session.AgentStats
	right := fmt.Sprintf("%*s  %*s  %*s  %*s",
		SessionRateWidth(mins), rate,
		SessionAgentsWidth, formatAgents(am.CurrentAgents, am.TotalAgents),
		SessionCmdsWidth, strconv.Itoa(len(i.session.Commands)),
		SessionLastWidth, formatTimeAgo(i.session.LastActivity),
	)

	// The remaining space is the flexible left region: path followed by the
	// origin tag. Reserve a two-space gutter before the right columns. The floor
	// is kept small so on a narrow terminal the path absorbs the shrink and the
	// (fixed, narrow) metric columns stay on screen rather than being clipped.
	leftWidth := d.width - lipgloss.Width(indicator) - lipgloss.Width(right) - 2
	if leftWidth < 4 {
		leftWidth = 4
	}

	// The origin tag is kept whole; the path takes whatever width remains and is
	// truncated by display width (not byte length) so multibyte paths never get
	// sliced mid-rune.
	nameSpace := leftWidth - lipgloss.Width(originTag)
	if nameSpace < 1 {
		nameSpace = 1
	}
	name := truncateToWidth(i.session.ProjectPath, nameSpace)

	left := name + originTag
	left += strings.Repeat(" ", max(0, leftWidth-lipgloss.Width(left)))

	row := indicator + left + "  " + right

	// Apply styling
	var style lipgloss.Style
	switch {
	case index == m.Index():
		style = lipgloss.NewStyle().
			Background(GetTheme().Surface).
			Foreground(GetTheme().Text).
			Bold(true).
			Width(d.width)
	case i.session.IsActive:
		style = lipgloss.NewStyle().
			Foreground(GetTheme().Secondary).
			Width(d.width)
	default:
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

// Column widths for command list (exported for header rendering)
const (
	CommandTimestampWidth = 12
	CommandGroupWidth     = 12
	CommandPatternWidth   = 20
)

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

	// Format: "Jan 02 15:04  group  Pattern  command..."
	timestamp := i.command.Timestamp.Format("Jan 02 15:04")
	pattern := i.command.Pattern

	// Get group name from config
	group := config.Global().GetToolGroup(pattern)
	groupName := ""
	if group != nil {
		groupName = group.Name
	}

	// Pad/truncate group to fixed width
	if len(groupName) > CommandGroupWidth {
		groupName = groupName[:CommandGroupWidth-1] + "…"
	} else {
		groupName += strings.Repeat(" ", CommandGroupWidth-len(groupName))
	}

	// Pad/truncate pattern to fixed width
	if len(pattern) > CommandPatternWidth {
		pattern = pattern[:CommandPatternWidth-1] + "…"
	} else {
		pattern += strings.Repeat(" ", CommandPatternWidth-len(pattern))
	}

	// Calculate space for raw command
	// Format: "timestamp  group  pattern  command"
	fixedWidth := CommandTimestampWidth + 2 + CommandGroupWidth + 2 + CommandPatternWidth + 2
	commandWidth := d.width - fixedWidth
	if commandWidth < 10 {
		commandWidth = 10
	}

	// Replace newlines with visible marker to keep single-line display
	rawCmd := strings.ReplaceAll(i.command.RawCommand, "\n", "↵")
	if len(rawCmd) > commandWidth {
		rawCmd = rawCmd[:commandWidth-1] + "…"
	}

	row := fmt.Sprintf("%s  %s  %s  %s", timestamp, groupName, pattern, rawCmd)

	// Pad to full width
	if len(row) < d.width {
		row += strings.Repeat(" ", d.width-len(row))
	}

	// Apply styling based on selection and tool type
	var style lipgloss.Style
	baseStyle := StyleForPattern(i.command.Pattern)

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

// Column widths for pattern list (exported for header rendering)
const (
	PatternPatternWidth = 25
	PatternGroupWidth   = 12
	PatternCountWidth   = 8
)

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

	// Format: "Pattern  Group  [count]  example..."
	pattern := i.pattern.Pattern
	countStr := fmt.Sprintf("[%d]", i.pattern.Count)

	// Get group name from config
	group := config.Global().GetToolGroup(pattern)
	groupName := ""
	if group != nil {
		groupName = group.Name
	}

	// Pad/truncate pattern
	if len(pattern) > PatternPatternWidth {
		pattern = pattern[:PatternPatternWidth-1] + "…"
	} else {
		pattern += strings.Repeat(" ", PatternPatternWidth-len(pattern))
	}

	// Pad/truncate group to fixed width
	if len(groupName) > PatternGroupWidth {
		groupName = groupName[:PatternGroupWidth-1] + "…"
	} else {
		groupName += strings.Repeat(" ", PatternGroupWidth-len(groupName))
	}

	// Pad count (right-aligned)
	countStr = strings.Repeat(" ", PatternCountWidth-len(countStr)) + countStr

	// Calculate space for example
	fixedWidth := PatternPatternWidth + 2 + PatternGroupWidth + 2 + PatternCountWidth + 2
	exampleWidth := d.width - fixedWidth
	if exampleWidth < 10 {
		exampleWidth = 10
	}

	example := ""
	if len(i.pattern.Examples) > 0 {
		// Replace newlines with visible marker to keep single-line display
		example = strings.ReplaceAll(i.pattern.Examples[0], "\n", "↵")
		if len(example) > exampleWidth {
			example = example[:exampleWidth-1] + "…"
		}
	}

	row := fmt.Sprintf("%s  %s  %s  %s", pattern, groupName, countStr, example)

	// Pad to full width
	if len(row) < d.width {
		row += strings.Repeat(" ", d.width-len(row))
	}

	// Apply styling
	var style lipgloss.Style
	baseStyle := StyleForPattern(i.pattern.Pattern)

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

// truncateToWidth shortens s to fit w display columns, appending an ellipsis
// when it has to cut. It measures by display width and cuts on rune boundaries
// so multibyte paths are never sliced mid-rune.
func truncateToWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	var b strings.Builder
	width := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if width+rw > w-1 { // reserve one column for the ellipsis
			break
		}
		b.WriteRune(r)
		width += rw
	}
	return b.String() + "…"
}

// formatTokPerMin renders a tokens-per-minute rate compactly for the session
// list column. Zero (idle / no usage) renders as a dash.
func formatTokPerMin(v float64) string {
	switch {
	case v <= 0:
		return "—"
	case v < 1000:
		return fmt.Sprintf("%.0f", v)
	case v < 10000:
		return fmt.Sprintf("%.1fk", v/1000)
	case v < 1_000_000:
		return fmt.Sprintf("%.0fk", v/1000)
	default:
		return fmt.Sprintf("%.1fM", v/1_000_000)
	}
}

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
