package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI based on the model state
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.err != nil {
		return ErrorStyle().Render(fmt.Sprintf("Error: %v", m.err))
	}

	var b strings.Builder

	// Header with title and session count
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// View mode tabs
	b.WriteString(m.renderViewTabs())
	b.WriteString("\n")

	// Main content area based on view mode
	switch m.viewMode {
	case ViewSessions:
		b.WriteString(m.renderSessionHeaders())
		b.WriteString("\n")
		b.WriteString(m.sessionList.View())
	case ViewCommands:
		b.WriteString(m.renderCommandHeaders())
		b.WriteString("\n")
		b.WriteString(m.commandList.View())
	case ViewPatterns:
		b.WriteString(m.renderPatternHeaders())
		b.WriteString("\n")
		b.WriteString(m.patternList.View())
	}

	// Help footer
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

// renderHeader renders the top header bar
func (m Model) renderHeader() string {
	title := TitleStyle().Render("Claude Code Session Monitor")

	// Session status
	activeCount := 0
	for _, s := range m.sessions {
		if s.IsActive {
			activeCount++
		}
	}

	var status string
	if len(m.sessions) == 0 {
		status = StatusStyle().Render("No sessions found")
	} else {
		status = StatusStyle().Render(fmt.Sprintf(
			"%d sessions (%d active)",
			len(m.sessions),
			activeCount,
		))
	}

	// Add active session indicator
	activeSession := ""
	if sess := m.ActiveSession(); sess != nil {
		name := filepath.Base(sess.ProjectPath)
		if sess.IsActive {
			activeSession = ActiveIndicatorStyle().Render(" [" + name + "]")
		} else {
			activeSession = InactiveIndicatorStyle().Render(" [" + name + "]")
		}
	}

	// Calculate spacing
	leftPart := lipgloss.Width(title)
	rightPart := lipgloss.Width(status) + lipgloss.Width(activeSession)
	spacing := m.width - leftPart - rightPart - 4
	if spacing < 1 {
		spacing = 1
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		title,
		strings.Repeat(" ", spacing),
		status,
		activeSession,
	)
}

// renderViewTabs renders the tab bar for view modes
func (m Model) renderViewTabs() string {
	tabs := []struct {
		name string
		mode ViewMode
		key  string
	}{
		{"Sessions", ViewSessions, "1"},
		{"Commands", ViewCommands, "2"},
		{"Patterns", ViewPatterns, "3"},
	}

	rendered := make([]string, len(tabs))
	for i, t := range tabs {
		label := fmt.Sprintf("%s %s", t.key, t.name)
		if t.mode == m.viewMode {
			rendered[i] = ActiveTabStyle().Render(label)
		} else {
			rendered[i] = InactiveTabStyle().Render(label)
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	gap := strings.Repeat("─", max(0, m.width-lipgloss.Width(row)-2))

	return row + TabGapStyle().Render(gap)
}

// renderHelp renders the help footer
func (m Model) renderHelp() string {
	var help []string

	switch m.viewMode {
	case ViewSessions:
		help = []string{
			"j/k:navigate",
			"enter:select",
			"tab:next session",
			"h/l:switch view",
			"r:refresh",
			"q:quit",
		}
	case ViewCommands:
		help = []string{
			"j/k:navigate",
			"tab:next session",
			"h/l:switch view",
			"esc:back",
			"q:quit",
		}
	case ViewPatterns:
		help = []string{
			"j/k:navigate",
			"h/l:switch view",
			"esc:back",
			"q:quit",
		}
	}

	return HelpStyle().Render(strings.Join(help, " | "))
}

// renderSessionHeaders renders column headers for the session list
func (m Model) renderSessionHeaders() string {
	// Session list doesn't have fixed columns, just a simple indicator
	header := "  Session Path"
	return ColumnHeaderStyle(m.width - 4).Render(header)
}

// renderCommandHeaders renders column headers for the command list
func (m Model) renderCommandHeaders() string {
	// Build header with same widths as delegate
	date := padRight("Date", CommandTimestampWidth)
	group := padRight("Group", CommandGroupWidth)
	pattern := padRight("Pattern", CommandPatternWidth)
	command := "Command"

	header := fmt.Sprintf("%s  %s  %s  %s", date, group, pattern, command)
	return ColumnHeaderStyle(m.width - 4).Render(header)
}

// renderPatternHeaders renders column headers for the pattern list
func (m Model) renderPatternHeaders() string {
	// Build header with same widths as delegate
	pattern := padRight("Pattern", PatternPatternWidth)
	group := padRight("Group", PatternGroupWidth)
	count := padLeft("Count", PatternCountWidth)
	example := "Example"

	header := fmt.Sprintf("%s  %s  %s  %s", pattern, group, count, example)
	return ColumnHeaderStyle(m.width - 4).Render(header)
}

// padRight pads a string with spaces on the right to reach target width
func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// padLeft pads a string with spaces on the left to reach target width
func padLeft(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// max returns the larger of two ints
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
