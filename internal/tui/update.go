package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles incoming messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.updateListSizes()

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case sessionsDiscoveredMsg:
		m.sessions = msg
		m = m.updateSessionList()
		m = m.updateCommandList()
		m = m.aggregatePatterns()

		// Start watching for updates
		if m.watcher != nil {
			m.watcher.Start()
			cmds = append(cmds, m.watchSessionsCmd())
		}

	case sessionEventMsg:
		m = m.handleSessionEvent(msg)
		// Continue watching
		cmds = append(cmds, m.watchSessionsCmd())

	case tickMsg:
		// Refresh activity status periodically
		if m.watcher != nil {
			m.watcher.RefreshActivityStatus()
			m = m.updateSessionList()
		}
		cmds = append(cmds, m.tickCmd())

	case errMsg:
		m.err = msg
	}

	// Update the active list component
	var cmd tea.Cmd
	switch m.viewMode {
	case ViewSessions:
		m.sessionList, cmd = m.sessionList.Update(msg)
	case ViewCommands:
		m.commandList, cmd = m.commandList.Update(msg)
	case ViewPatterns:
		m.patternList, cmd = m.patternList.Update(msg)
	}
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "tab":
		// Next session
		if len(m.sessions) > 0 {
			m.activeIdx = (m.activeIdx + 1) % len(m.sessions)
			m = m.updateCommandList()
			m = m.aggregatePatterns()
		}

	case "shift+tab":
		// Previous session
		if len(m.sessions) > 0 {
			m.activeIdx = (m.activeIdx - 1 + len(m.sessions)) % len(m.sessions)
			m = m.updateCommandList()
			m = m.aggregatePatterns()
		}

	case "l", "right":
		// Next view mode - return early to avoid passing key to list
		switch m.viewMode {
		case ViewSessions:
			m.viewMode = ViewCommands
		case ViewCommands:
			m.viewMode = ViewPatterns
			// Ensure patterns are for the current session
			m = m.aggregatePatterns()
		case ViewPatterns:
			m.viewMode = ViewSessions
		}
		return m, nil

	case "h", "left":
		// Previous view mode - return early to avoid passing key to list
		switch m.viewMode {
		case ViewSessions:
			m.viewMode = ViewPatterns
			// Ensure patterns are for the current session
			m = m.aggregatePatterns()
		case ViewPatterns:
			m.viewMode = ViewCommands
		case ViewCommands:
			m.viewMode = ViewSessions
		}
		return m, nil

	case "enter":
		// Drill down from sessions to commands
		if m.viewMode == ViewSessions {
			// Set active session to currently selected in list
			if i := m.sessionList.Index(); i >= 0 && i < len(m.sessions) {
				m.activeIdx = i
				m = m.updateCommandList()
				m = m.aggregatePatterns()
			}
			m.viewMode = ViewCommands
		}

	case "esc":
		// Go back to sessions view (don't pass to list component)
		if m.viewMode != ViewSessions {
			m.viewMode = ViewSessions
			return m, nil
		}
		return m, nil

	case "backspace":
		// Go back to sessions view
		if m.viewMode != ViewSessions {
			m.viewMode = ViewSessions
		}

	case "r":
		// Refresh sessions
		return m, m.discoverSessionsCmd()

	case "1":
		m.viewMode = ViewSessions
	case "2":
		m.viewMode = ViewCommands
	case "3":
		m.viewMode = ViewPatterns
	}

	// Pass through to active list for j/k navigation
	var cmd tea.Cmd
	switch m.viewMode {
	case ViewSessions:
		m.sessionList, cmd = m.sessionList.Update(msg)
	case ViewCommands:
		m.commandList, cmd = m.commandList.Update(msg)
	case ViewPatterns:
		m.patternList, cmd = m.patternList.Update(msg)
	}

	return m, cmd
}

// handleSessionEvent processes watcher events
func (m Model) handleSessionEvent(event sessionEventMsg) Model {
	switch event.Type {
	case "discovered", "new_session":
		// Add new session if not already tracked
		found := false
		for _, s := range m.sessions {
			if s.FilePath == event.Session.FilePath {
				found = true
				break
			}
		}
		if !found {
			m.sessions = append(m.sessions, event.Session)
			// Re-sort by activity
			sort.Slice(m.sessions, func(i, j int) bool {
				return m.sessions[i].LastActivity.After(m.sessions[j].LastActivity)
			})
			m = m.updateSessionList()
			m = m.aggregatePatterns()
		}

	case "new_commands":
		// Update the session and refresh views
		for i, s := range m.sessions {
			if s.FilePath == event.Session.FilePath {
				m.sessions[i] = event.Session
				break
			}
		}
		// Re-sort by activity
		sort.Slice(m.sessions, func(i, j int) bool {
			return m.sessions[i].LastActivity.After(m.sessions[j].LastActivity)
		})
		m = m.updateSessionList()
		m = m.updateCommandList()
		m = m.aggregatePatterns()
	}

	return m
}
