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
		if m.detailPanelOpen {
			b.WriteString(m.renderSplitCommandView())
		} else {
			b.WriteString(m.renderCommandHeaders())
			b.WriteString("\n")
			b.WriteString(m.commandList.View())
		}
		if m.searchActive {
			b.WriteString("\n")
			b.WriteString(m.renderSearchBar())
		}
	case ViewPatterns:
		b.WriteString(m.renderPatternHeaders())
		b.WriteString("\n")
		b.WriteString(m.patternList.View())
	}

	// Help footer
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	// Overlay path dialog if active
	if m.showPathDialog {
		return m.overlayPathDialog(b.String())
	}

	return b.String()
}

// renderHeader renders the top header bar
func (m Model) renderHeader() string {
	titleText := "Claude Code Session Monitor"
	if m.followDevagent {
		titleText += " [devagent]"
	}
	title := TitleStyle().Render(titleText)

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
			"p:path",
			"r:refresh",
			"q:quit",
		}
	case ViewCommands:
		switch {
		case m.searchActive && m.searchFocused:
			help = []string{
				"type to filter",
				"esc:unfocus",
				"tab:next session",
				"ctrl+f:close",
				"ctrl+c:quit",
			}
		case m.detailPanelOpen:
			help = []string{
				"j/k:navigate",
				"enter:close panel",
				"esc:close panel",
				"tab:next session",
				"ctrl+f:search",
				"p:path",
				"q:quit",
			}
		default:
			help = []string{
				"j/k:navigate",
				"enter:show details",
				"tab:next session",
				"h/l:switch view",
				"ctrl+f:search",
				"p:path",
				"esc:back",
				"q:quit",
			}
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

// renderSearchBar renders the search input at the bottom of the Commands tab
func (m Model) renderSearchBar() string {
	return SearchBarStyle().Render(m.searchInput.View())
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

// overlayPathDialog renders the path dialog centered over the existing view
func (m Model) overlayPathDialog(background string) string {
	sess := m.ActiveSession()
	if sess == nil {
		return background
	}

	sessionDir := filepath.Dir(sess.FilePath)

	// Build dialog content
	t := GetTheme()

	pathLabel := LabelStyle().Render("Session data path:")
	pathValue := lipgloss.NewStyle().Foreground(t.Secondary).Render(sessionDir)

	grepLabel := LabelStyle().Render("Search example:")
	grepCmd := lipgloss.NewStyle().Foreground(t.Text).
		Background(t.Surface).
		Padding(0, 1).
		Render(fmt.Sprintf("grep -ri 'search_term' %s", sessionDir))

	dismiss := lipgloss.NewStyle().Foreground(t.Muted).Italic(true).Render("Press any key to dismiss")

	content := lipgloss.JoinVertical(lipgloss.Left,
		pathLabel,
		pathValue,
		"",
		grepLabel,
		grepCmd,
		"",
		dismiss,
	)

	// Build bordered dialog box
	dialogWidth := min(m.width-8, lipgloss.Width(grepCmd)+6)
	if dialogWidth < 40 {
		dialogWidth = 40
	}

	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2).
		Width(dialogWidth).
		Render(content)

	// Center the dialog on screen
	dialogHeight := lipgloss.Height(dialog)
	dialogW := lipgloss.Width(dialog)

	// Split background into lines and overlay
	bgLines := strings.Split(background, "\n")
	startRow := (m.height - dialogHeight) / 2
	startCol := (m.width - dialogW) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	dialogLines := strings.Split(dialog, "\n")
	for i, dLine := range dialogLines {
		row := startRow + i
		if row >= len(bgLines) {
			break
		}
		bgLine := bgLines[row]
		// Pad background line if needed
		bgW := lipgloss.Width(bgLine)
		if bgW < startCol+lipgloss.Width(dLine) {
			bgLine += strings.Repeat(" ", startCol+lipgloss.Width(dLine)-bgW)
		}
		// Replace the portion of the background line with dialog line
		bgLines[row] = placeover(bgLine, dLine, startCol)
	}

	return strings.Join(bgLines, "\n")
}

// placeover places overlay text at a given column position in a line
func placeover(bg, overlay string, col int) string {
	// Use lipgloss.PlaceHorizontal for ANSI-aware placement
	bgWidth := lipgloss.Width(bg)
	overlayWidth := lipgloss.Width(overlay)
	totalWidth := col + overlayWidth
	if totalWidth < bgWidth {
		totalWidth = bgWidth
	}

	// Build: left padding + overlay + right portion
	left := ""
	if col > 0 {
		// Take first col characters from background
		left = truncateAnsi(bg, col)
	}

	return left + overlay + strings.Repeat(" ", max(0, totalWidth-col-overlayWidth))
}

// truncateAnsi truncates a string to a display width, preserving ANSI sequences
func truncateAnsi(s string, width int) string {
	// Use lipgloss.PlaceHorizontal to get a fixed-width string, then take what we need
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(
		lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(s),
	)
}

// renderSplitCommandView renders the commands list with detail panel side-by-side
func (m Model) renderSplitCommandView() string {
	// Calculate widths: 60% for list, 40% for detail (minus separator)
	totalWidth := m.width - 4
	listWidth := int(float64(totalWidth) * 0.58)
	detailWidth := totalWidth - listWidth - 1 // -1 for separator

	// Calculate available height for content (same as list height calculation)
	contentHeight := m.height - 9
	if contentHeight < 5 {
		contentHeight = 5
	}
	// Reduce height when search bar is active
	if m.searchActive {
		contentHeight -= 2
		if contentHeight < 3 {
			contentHeight = 3
		}
	}

	// Build the list side with headers
	listHeader := m.renderCommandHeadersWithWidth(listWidth)

	// Get list view - need to ensure it's rendered at the right width
	// The list component should already be sized correctly from updateListSizes
	listView := m.commandList.View()

	// Build left side (header + list)
	leftSide := lipgloss.NewStyle().
		Width(listWidth).
		Height(contentHeight + 1). // +1 for header
		Render(listHeader + "\n" + listView)

	// Build the separator - a vertical line
	separator := lipgloss.NewStyle().
		Foreground(GetTheme().Muted).
		Render(strings.Repeat("│\n", contentHeight+1))

	// Build the detail panel
	rightSide := m.renderDetailPanel(detailWidth, contentHeight+1)

	// Join horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, leftSide, separator, rightSide)
}

// renderCommandHeadersWithWidth renders column headers at a specific width
func (m Model) renderCommandHeadersWithWidth(width int) string {
	date := padRight("Date", CommandTimestampWidth)
	group := padRight("Group", CommandGroupWidth)
	pattern := padRight("Pattern", CommandPatternWidth)
	command := "Command"

	header := fmt.Sprintf("%s  %s  %s  %s", date, group, pattern, command)
	return ColumnHeaderStyle(width).Render(header)
}
