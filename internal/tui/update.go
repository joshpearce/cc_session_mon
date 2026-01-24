package tui

import (
	"cc_session_mon/internal/session"

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
		m.err = msg.error

	case detailLoadedMsg:
		m.loadingDetail = false
		m.loadedInput = msg

	case detailErrorMsg:
		m.loadingDetail = false
		m.detailError = msg.error
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

	// Global keys (always handled)
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "r":
		return m, m.discoverSessionsCmd()
	}

	// Session navigation keys
	if newModel, handled := m.handleSessionNavigation(key); handled {
		return newModel, nil
	}

	// View switching keys
	if newModel, handled := m.handleViewSwitch(key); handled {
		return newModel, nil
	}

	// Action keys (enter, esc, backspace)
	if newModel, cmd, handled := m.handleActionKeys(key); handled {
		return newModel, cmd
	}

	// Number keys for direct view access
	if newModel, handled := m.handleNumberKeys(key); handled {
		return newModel, nil
	}

	// Pass through to active list and handle detail panel updates
	return m.handleListNavigation(msg)
}

// handleSessionNavigation handles tab/shift+tab for session switching
func (m Model) handleSessionNavigation(key string) (Model, bool) {
	if len(m.sessions) == 0 {
		return m, false
	}

	switch key {
	case "tab":
		m.activeIdx = (m.activeIdx + 1) % len(m.sessions)
		m = m.updateCommandList()
		m = m.aggregatePatterns()
		return m, true
	case "shift+tab":
		m.activeIdx = (m.activeIdx - 1 + len(m.sessions)) % len(m.sessions)
		m = m.updateCommandList()
		m = m.aggregatePatterns()
		return m, true
	}
	return m, false
}

// handleViewSwitch handles h/l and arrow keys for view cycling
func (m Model) handleViewSwitch(key string) (Model, bool) {
	switch key {
	case "l", "right":
		m = m.cycleViewForward()
		return m, true
	case "h", "left":
		m = m.cycleViewBackward()
		return m, true
	}
	return m, false
}

// cycleViewForward moves to the next view
func (m Model) cycleViewForward() Model {
	switch m.viewMode {
	case ViewSessions:
		m.viewMode = ViewCommands
	case ViewCommands:
		m.viewMode = ViewPatterns
		m = m.aggregatePatterns()
	case ViewPatterns:
		m.viewMode = ViewSessions
	}
	return m
}

// cycleViewBackward moves to the previous view
func (m Model) cycleViewBackward() Model {
	switch m.viewMode {
	case ViewSessions:
		m.viewMode = ViewPatterns
		m = m.aggregatePatterns()
	case ViewPatterns:
		m.viewMode = ViewCommands
	case ViewCommands:
		m.viewMode = ViewSessions
	}
	return m
}

// handleActionKeys handles enter, esc, backspace
func (m Model) handleActionKeys(key string) (Model, tea.Cmd, bool) {
	switch key {
	case "enter":
		return m.handleEnter()
	case "esc":
		return m.handleEsc()
	case "backspace":
		if m.viewMode != ViewSessions {
			m.viewMode = ViewSessions
		}
		return m, nil, true
	}
	return m, nil, false
}

// handleEnter processes enter key based on current view
func (m Model) handleEnter() (Model, tea.Cmd, bool) {
	switch m.viewMode {
	case ViewSessions:
		if i := m.sessionList.Index(); i >= 0 && i < len(m.sessions) {
			m.activeIdx = i
			m = m.updateCommandList()
			m = m.aggregatePatterns()
		}
		m.viewMode = ViewCommands
		return m, nil, true

	case ViewCommands:
		return m.toggleDetailPanel()

	case ViewPatterns:
		// No action on enter in patterns view
		return m, nil, false
	}
	return m, nil, false
}

// toggleDetailPanel opens/closes the detail panel for the selected command
func (m Model) toggleDetailPanel() (Model, tea.Cmd, bool) {
	item, ok := m.commandList.SelectedItem().(commandItem)
	if !ok {
		return m, nil, true
	}

	cmd := item.command

	// If panel is open and same command selected, close panel
	if m.detailPanelOpen && m.selectedCommand != nil &&
		m.selectedCommand.UUID == cmd.UUID &&
		m.selectedCommand.ToolName == cmd.ToolName {
		m = m.closeDetailPanel()
		return m, nil, true
	}

	// Open panel and start loading
	m = m.openDetailPanel(&cmd)
	return m, m.loadDetailCmd(cmd), true
}

// closeDetailPanel closes the detail panel and clears related state
func (m Model) closeDetailPanel() Model {
	m.detailPanelOpen = false
	m.selectedCommand = nil
	m.loadedInput = nil
	m.detailError = nil
	m = m.updateListSizes()
	return m
}

// openDetailPanel opens the detail panel for a command
func (m Model) openDetailPanel(cmd *session.CommandEntry) Model {
	m.detailPanelOpen = true
	m.selectedCommand = cmd
	m.loadedInput = nil
	m.loadingDetail = true
	m.detailError = nil
	m = m.updateListSizes()
	return m
}

// handleEsc processes escape key
func (m Model) handleEsc() (Model, tea.Cmd, bool) {
	// If detail panel is open, close it first
	if m.viewMode == ViewCommands && m.detailPanelOpen {
		m = m.closeDetailPanel()
		return m, nil, true
	}
	// Go back to sessions view
	if m.viewMode != ViewSessions {
		m.viewMode = ViewSessions
	}
	return m, nil, true
}

// handleNumberKeys handles 1/2/3 for direct view switching
func (m Model) handleNumberKeys(key string) (Model, bool) {
	switch key {
	case "1":
		m.viewMode = ViewSessions
		return m, true
	case "2":
		m.viewMode = ViewCommands
		return m, true
	case "3":
		m.viewMode = ViewPatterns
		return m, true
	}
	return m, false
}

// handleListNavigation passes keys to the active list component
func (m Model) handleListNavigation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.viewMode {
	case ViewSessions:
		m.sessionList, cmd = m.sessionList.Update(msg)
	case ViewCommands:
		m.commandList, cmd = m.commandList.Update(msg)
		// If detail panel is open and selection changed, reload details
		if m.detailPanelOpen {
			if item, ok := m.commandList.SelectedItem().(commandItem); ok {
				newCmd := item.command
				if m.selectedCommand == nil ||
					m.selectedCommand.UUID != newCmd.UUID ||
					m.selectedCommand.ToolName != newCmd.ToolName {
					m.selectedCommand = &newCmd
					m.loadedInput = nil
					m.loadingDetail = true
					m.detailError = nil
					return m, m.loadDetailCmd(newCmd)
				}
			}
		}
	case ViewPatterns:
		m.patternList, cmd = m.patternList.Update(msg)
	}

	return m, cmd
}

// handleSessionEvent processes watcher events
func (m Model) handleSessionEvent(event sessionEventMsg) Model {
	if m.watcher == nil {
		return m
	}

	// Remember currently selected session by file path
	var selectedFilePath string
	if m.activeIdx >= 0 && m.activeIdx < len(m.sessions) {
		selectedFilePath = m.sessions[m.activeIdx].FilePath
	}

	// Get fresh sorted list from watcher (already sorted, no re-sort needed)
	m.sessions = m.watcher.GetSessions()

	// Restore selection by finding the session with the same file path
	if selectedFilePath != "" {
		for i, s := range m.sessions {
			if s.FilePath == selectedFilePath {
				m.activeIdx = i
				break
			}
		}
	}

	// Clamp activeIdx to valid range
	if m.activeIdx >= len(m.sessions) {
		m.activeIdx = len(m.sessions) - 1
	}
	if m.activeIdx < 0 && len(m.sessions) > 0 {
		m.activeIdx = 0
	}

	m = m.updateSessionList()
	if event.Type == "new_commands" {
		m = m.updateCommandList()
	}
	m = m.aggregatePatterns()

	return m
}
