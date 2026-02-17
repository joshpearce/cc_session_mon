package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cc_session_mon/internal/devagent"
	"cc_session_mon/internal/session"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ViewMode represents the current view
type ViewMode int

const (
	ViewSessions ViewMode = iota // Session list
	ViewCommands                 // Command log for selected session
	ViewPatterns                 // Unique patterns aggregation
)

// ModelOptions configures Model creation
type ModelOptions struct {
	FollowDevagent bool
}

// Model represents the application state
type Model struct {
	// Core state
	watcher   *session.Watcher
	sessions  []*session.Session
	activeIdx int // Currently selected session index
	viewMode  ViewMode

	// UI components
	sessionList list.Model
	commandList list.Model
	patternList list.Model

	// Delegates (stored to update width)
	sessionDelegate *sessionDelegate
	commandDelegate *commandDelegate
	patternDelegate *patternDelegate

	// Aggregated patterns for active session
	patterns           []*session.CommandPattern
	patternListSession string // Session ID for which patterns are displayed

	// Detail panel state
	detailPanelOpen bool                  // Whether the detail panel is visible
	selectedCommand *session.CommandEntry // Currently selected command for details
	loadedInput     *session.ToolInput    // Lazily loaded input data
	loadingDetail   bool                  // Loading state indicator
	detailError     error                 // Error from loading details

	// Path dialog state
	showPathDialog bool // Whether the session path dialog is visible

	// Search state
	searchActive    bool            // Whether search bar is visible
	searchFocused   bool            // Whether search input has keyboard focus
	searchInput     textinput.Model // Text input component
	allCommandItems []list.Item     // Unfiltered command items for active session

	// UI dimensions
	width  int
	height int

	// Error state
	err error

	// Devagent support
	followDevagent bool
}

// NewModel creates a new Model with initialized state
func NewModel(opts ModelOptions) Model {
	var projectsDirs []string
	var watcher *session.Watcher
	var err error

	// Create delegates
	sessionDel := newSessionDelegate()
	commandDel := newCommandDelegate()
	patternDel := newPatternDelegate()

	// Initialize based on devagent flag
	if opts.FollowDevagent {
		// Discover devagent environments and build projects dirs
		envs, discoverErr := devagent.Discover()
		if discoverErr != nil {
			// Fall back to local if discovery fails
			projectsDir := filepath.Join(os.Getenv("HOME"), ".claude", "projects")
			projectsDirs = []string{projectsDir}
			watcher, err = session.NewWatcher(projectsDirs)
			if err == nil {
				watcher.SetOrigin(projectsDir, "local")
			}
		} else {
			// Build projects dirs from environments
			for _, env := range envs {
				projectsDirs = append(projectsDirs, env.ProjectsDir)
			}
			watcher, err = session.NewWatcher(projectsDirs)
			if err == nil {
				// Set origin labels for each environment
				for _, env := range envs {
					watcher.SetOrigin(env.ProjectsDir, "devagent:"+env.ContainerName)
				}
			}
		}
	} else {
		// Local mode: use ~/.claude/projects
		projectsDir := filepath.Join(os.Getenv("HOME"), ".claude", "projects")
		projectsDirs = []string{projectsDir}
		watcher, err = session.NewWatcher(projectsDirs)
		if err == nil {
			watcher.SetOrigin(projectsDir, "local")
		}
	}

	m := Model{
		watcher:         watcher,
		viewMode:        ViewSessions,
		activeIdx:       0,
		err:             err,
		sessionDelegate: sessionDel,
		commandDelegate: commandDel,
		patternDelegate: patternDel,
		followDevagent:  opts.FollowDevagent,
	}

	// Initialize search input
	m.searchInput = textinput.New()
	m.searchInput.Placeholder = "search commands..."
	m.searchInput.Prompt = "/ "
	m.searchInput.CharLimit = 200

	// Initialize list components with delegates
	m.sessionList = list.New([]list.Item{}, sessionDel, 0, 0)
	m.sessionList.SetShowTitle(false)
	m.sessionList.SetShowHelp(false)
	m.sessionList.SetShowStatusBar(false)
	m.sessionList.SetFilteringEnabled(false)
	m.sessionList.DisableQuitKeybindings()

	m.commandList = list.New([]list.Item{}, commandDel, 0, 0)
	m.commandList.SetShowTitle(false)
	m.commandList.SetShowHelp(false)
	m.commandList.SetShowStatusBar(false)
	m.commandList.SetFilteringEnabled(false)
	m.commandList.DisableQuitKeybindings()

	m.patternList = list.New([]list.Item{}, patternDel, 0, 0)
	m.patternList.SetShowTitle(false)
	m.patternList.SetShowHelp(false)
	m.patternList.SetShowStatusBar(false)
	m.patternList.SetFilteringEnabled(false)
	m.patternList.DisableQuitKeybindings()

	return m
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.discoverSessionsCmd(),
		m.tickCmd(),
	)
}

// Message types
type (
	sessionsDiscoveredMsg []*session.Session
	sessionEventMsg       session.WatchEvent
	tickMsg               time.Time
	errMsg                struct{ error }    // General error
	detailLoadedMsg       *session.ToolInput // Tool input loaded successfully
	detailErrorMsg        struct{ error }    // Error loading tool input
	devagentRefreshMsg    struct {
		envs []devagent.Environment
	}
)

// discoverSessionsCmd discovers existing sessions
func (m Model) discoverSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.watcher == nil {
			return errMsg{m.err}
		}
		sessions, err := m.watcher.DiscoverSessions()
		if err != nil {
			return errMsg{err}
		}
		return sessionsDiscoveredMsg(sessions)
	}
}

// watchSessionsCmd returns a command that waits for session events
func (m Model) watchSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.watcher == nil {
			return nil
		}
		select {
		case event := <-m.watcher.Events:
			return sessionEventMsg(event)
		case err := <-m.watcher.Errors:
			return errMsg{err}
		}
	}
}

// tickCmd returns a command that ticks every 30 seconds to refresh timestamps
func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// devagentRefreshCmd discovers devagent environments and returns a refresh message
func (m Model) devagentRefreshCmd() tea.Cmd {
	return func() tea.Msg {
		envs, err := devagent.Discover()
		if err != nil {
			return errMsg{err}
		}
		return devagentRefreshMsg{envs: envs}
	}
}

// loadDetailCmd asynchronously loads tool input for a command
func (m Model) loadDetailCmd(cmd session.CommandEntry) tea.Cmd {
	return func() tea.Msg {
		input, err := session.FetchToolInput(cmd.FilePath, cmd.LineNumber, cmd.ToolName, cmd.UUID)
		if err != nil {
			return detailErrorMsg{err}
		}
		return detailLoadedMsg(input)
	}
}

// updateSessionList rebuilds the session list items
func (m Model) updateSessionList() Model {
	items := make([]list.Item, len(m.sessions))
	for i, s := range m.sessions {
		items[i] = sessionItem{session: s}
	}
	m.sessionList.SetItems(items)
	return m
}

// updateCommandList rebuilds the command list for the active session
func (m Model) updateCommandList() Model {
	if m.activeIdx >= len(m.sessions) || len(m.sessions) == 0 {
		m.allCommandItems = nil
		m.commandList.SetItems([]list.Item{})
		return m
	}

	sess := m.sessions[m.activeIdx]

	// Remember if user was at the top (following tail)
	wasAtTop := m.commandList.Index() == 0
	previousCount := len(m.commandList.Items())

	// Create sorted indices instead of copying the full slice
	indices := make([]int, len(sess.Commands))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return sess.Commands[indices[i]].Timestamp.After(sess.Commands[indices[j]].Timestamp)
	})

	// Build items using sorted indices, avoiding struct copy in range
	items := make([]list.Item, len(indices))
	for i, idx := range indices {
		items[i] = commandItem{command: sess.Commands[idx]}
	}

	// Store unfiltered items and apply search filter
	m.allCommandItems = items
	m = m.applySearchFilter()

	m.commandList.Title = "Commands - " + filepath.Base(sess.ProjectPath)

	// Only auto-scroll to top if user was already at top, or this is initial load
	if wasAtTop || previousCount == 0 {
		m.commandList.Select(0)
	}

	return m
}

// applySearchFilter filters allCommandItems by search text and sets commandList items.
func (m Model) applySearchFilter() Model {
	if !m.searchActive || m.searchInput.Value() == "" {
		m.commandList.SetItems(m.allCommandItems)
		return m
	}

	text := strings.ToLower(m.searchInput.Value())
	filtered := make([]list.Item, 0, len(m.allCommandItems))
	for _, item := range m.allCommandItems {
		if ci, ok := item.(commandItem); ok {
			if strings.Contains(strings.ToLower(ci.command.RawCommand), text) {
				filtered = append(filtered, item)
			}
		}
	}
	m.commandList.SetItems(filtered)
	return m
}

// aggregatePatterns builds the unique patterns for the active session
func (m Model) aggregatePatterns() Model {
	patternMap := make(map[string]*session.CommandPattern)

	sess := m.ActiveSession()
	if sess == nil {
		m.patterns = nil
		m.patternList.SetItems([]list.Item{})
		m.patternListSession = ""
		return m
	}

	// Check if we switched to a different session
	sessionChanged := m.patternListSession != sess.ID
	m.patternListSession = sess.ID

	// Remember scroll position for preserving during updates (only if same session)
	wasAtTop := m.patternList.Index() == 0
	previousCount := len(m.patternList.Items())

	// Use a map per pattern to track unique examples (O(1) lookup instead of O(n))
	exampleSets := make(map[string]map[string]struct{})

	for i := range sess.Commands {
		cmd := &sess.Commands[i] // Use pointer to avoid copying 128-byte struct

		if p, exists := patternMap[cmd.Pattern]; exists {
			p.Count++
			if cmd.Timestamp.After(p.LastSeen) {
				p.LastSeen = cmd.Timestamp
			}
			// Use set for O(1) duplicate check
			if len(p.Examples) < 5 {
				exSet := exampleSets[cmd.Pattern]
				if _, seen := exSet[cmd.RawCommand]; !seen {
					exSet[cmd.RawCommand] = struct{}{}
					p.Examples = append(p.Examples, cmd.RawCommand)
				}
			}
		} else {
			patternMap[cmd.Pattern] = &session.CommandPattern{
				Pattern:  cmd.Pattern,
				ToolName: cmd.ToolName,
				Count:    1,
				LastSeen: cmd.Timestamp,
				Examples: []string{cmd.RawCommand},
			}
			// Initialize example set for this pattern
			exampleSets[cmd.Pattern] = map[string]struct{}{cmd.RawCommand: {}}
		}
	}

	// Convert to slice and sort by count
	m.patterns = make([]*session.CommandPattern, 0, len(patternMap))
	for _, p := range patternMap {
		m.patterns = append(m.patterns, p)
	}
	sort.Slice(m.patterns, func(i, j int) bool {
		return m.patterns[i].Count > m.patterns[j].Count
	})

	// Update pattern list
	items := make([]list.Item, len(m.patterns))
	for i, p := range m.patterns {
		items[i] = patternItem{pattern: p}
	}
	m.patternList.SetItems(items)
	m.patternList.Title = "Patterns - " + filepath.Base(sess.ProjectPath)

	// Reset to top if session changed, initial load, or user was already at top
	if sessionChanged || previousCount == 0 || wasAtTop {
		m.patternList.Select(0)
	}

	return m
}

// updateListSizes updates list dimensions based on terminal size
func (m Model) updateListSizes() Model {
	// Reserve space for header (2), tabs (2), column headers (1), help (2), margins (2)
	listHeight := m.height - 9
	if listHeight < 5 {
		listHeight = 5
	}
	listWidth := m.width - 4
	if listWidth < 20 {
		listWidth = 20
	}

	// Command list height is reduced when search bar is active
	commandListHeight := listHeight
	if m.searchActive {
		commandListHeight -= 2 // 1 for search input, 1 for spacing
		if commandListHeight < 3 {
			commandListHeight = 3
		}
	}

	// Command list width is reduced when detail panel is open
	commandListWidth := listWidth
	if m.viewMode == ViewCommands && m.detailPanelOpen {
		commandListWidth = int(float64(listWidth) * 0.58)
	}

	// Update delegate widths
	m.sessionDelegate.SetWidth(listWidth)
	m.commandDelegate.SetWidth(commandListWidth)
	m.patternDelegate.SetWidth(listWidth)

	m.sessionList.SetSize(listWidth, listHeight)
	m.commandList.SetSize(commandListWidth, commandListHeight)
	m.patternList.SetSize(listWidth, listHeight)

	return m
}

// ActiveSession returns the currently selected session or nil
func (m Model) ActiveSession() *session.Session {
	if m.activeIdx >= 0 && m.activeIdx < len(m.sessions) {
		return m.sessions[m.activeIdx]
	}
	return nil
}
