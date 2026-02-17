package tui

import (
	"testing"
	"time"

	"cc_session_mon/internal/session"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// newTestModelWithSessions creates a Model with pre-populated sessions for testing.
// Each session has commands with known RawCommand values for predictable filtering.
func newTestModelWithSessions() Model {
	m := NewModel(ModelOptions{})
	m.width = 120
	m.height = 40
	m.viewMode = ViewCommands

	// Create two sessions with distinct commands
	now := time.Now()
	m.sessions = []*session.Session{
		{
			ID:          "session-1",
			FilePath:    "/tmp/test/session1.jsonl",
			ProjectPath: "/projects/alpha",
			Commands: []session.CommandEntry{
				{ToolName: "Bash", RawCommand: "git status", Pattern: "Bash(git:*)", Timestamp: now},
				{ToolName: "Read", RawCommand: "/path/to/file.go", Pattern: "Read", Timestamp: now.Add(-1 * time.Minute)},
				{ToolName: "Bash", RawCommand: "go test ./...", Pattern: "Bash(go:*)", Timestamp: now.Add(-2 * time.Minute)},
			},
		},
		{
			ID:          "session-2",
			FilePath:    "/tmp/test/session2.jsonl",
			ProjectPath: "/projects/beta",
			Commands: []session.CommandEntry{
				{ToolName: "Bash", RawCommand: "git diff", Pattern: "Bash(git:*)", Timestamp: now},
				{ToolName: "Write", RawCommand: "/path/to/new.go", Pattern: "Write", Timestamp: now.Add(-1 * time.Minute)},
				{ToolName: "Bash", RawCommand: "git commit -m fix", Pattern: "Bash(git:*)", Timestamp: now.Add(-2 * time.Minute)},
			},
		},
	}
	m.activeIdx = 0
	m = m.updateCommandList()
	return m
}

func TestNewModel(t *testing.T) {
	m := NewModel(ModelOptions{FollowDevagent: false})
	if m.viewMode != ViewSessions {
		t.Errorf("expected initial view mode to be ViewSessions, got %d", m.viewMode)
	}
	if m.activeIdx != 0 {
		t.Errorf("expected initial activeIdx to be 0, got %d", m.activeIdx)
	}
}

func TestViewModeCycleRight(t *testing.T) {
	m := NewModel(ModelOptions{FollowDevagent: false})
	// Set dimensions so view works
	m.width = 80
	m.height = 24

	// Initial state should be Sessions
	if m.viewMode != ViewSessions {
		t.Fatalf("expected initial view mode to be ViewSessions")
	}

	// Press 'l' to go to Commands
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model := updated.(Model)
	if model.viewMode != ViewCommands {
		t.Errorf("expected view mode to be ViewCommands after 'l', got %d", model.viewMode)
	}

	// Press 'l' again to go to Patterns
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model = updated.(Model)
	if model.viewMode != ViewPatterns {
		t.Errorf("expected view mode to be ViewPatterns after 'l', got %d", model.viewMode)
	}

	// Press 'l' again to wrap back to Sessions
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model = updated.(Model)
	if model.viewMode != ViewSessions {
		t.Errorf("expected view mode to wrap to ViewSessions after 'l', got %d", model.viewMode)
	}
}

func TestViewModeCycleLeft(t *testing.T) {
	m := NewModel(ModelOptions{FollowDevagent: false})
	m.width = 80
	m.height = 24

	// Initial state should be Sessions
	if m.viewMode != ViewSessions {
		t.Fatalf("expected initial view mode to be ViewSessions")
	}

	// Press 'h' to go to Patterns (wrapping backwards)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model := updated.(Model)
	if model.viewMode != ViewPatterns {
		t.Errorf("expected view mode to be ViewPatterns after 'h', got %d", model.viewMode)
	}
}

func TestViewModeNumbers(t *testing.T) {
	m := NewModel(ModelOptions{FollowDevagent: false})
	m.width = 80
	m.height = 24

	// Press '2' to go to Commands
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	model := updated.(Model)
	if model.viewMode != ViewCommands {
		t.Errorf("expected view mode to be ViewCommands after '2', got %d", model.viewMode)
	}

	// Press '3' to go to Patterns
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	model = updated.(Model)
	if model.viewMode != ViewPatterns {
		t.Errorf("expected view mode to be ViewPatterns after '3', got %d", model.viewMode)
	}

	// Press '1' to go back to Sessions
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	model = updated.(Model)
	if model.viewMode != ViewSessions {
		t.Errorf("expected view mode to be ViewSessions after '1', got %d", model.viewMode)
	}
}

func TestEscReturnsToSessions(t *testing.T) {
	m := NewModel(ModelOptions{FollowDevagent: false})
	m.width = 80
	m.height = 24
	m.viewMode = ViewCommands

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model := updated.(Model)
	if model.viewMode != ViewSessions {
		t.Errorf("expected view mode to be ViewSessions after ESC, got %d", model.viewMode)
	}
}

// testCommandItems creates a set of commandItem values for search filter tests.
func testCommandItems() []list.Item {
	return []list.Item{
		commandItem{command: session.CommandEntry{
			RawCommand: "git status",
			ToolName:   "Bash",
			Pattern:    "Bash(git:*)",
			Timestamp:  time.Now(),
		}},
		commandItem{command: session.CommandEntry{
			RawCommand: "git diff --staged",
			ToolName:   "Bash",
			Pattern:    "Bash(git:*)",
			Timestamp:  time.Now(),
		}},
		commandItem{command: session.CommandEntry{
			RawCommand: "/Users/josh/code/main.go",
			ToolName:   "Read",
			Pattern:    "Read",
			Timestamp:  time.Now(),
		}},
		commandItem{command: session.CommandEntry{
			RawCommand: "/Users/josh/code/utils.go",
			ToolName:   "Write",
			Pattern:    "Write",
			Timestamp:  time.Now(),
		}},
	}
}

func TestApplySearchFilter(t *testing.T) {
	tests := []struct {
		name        string
		searchText  string
		active      bool
		wantCount   int
		description string // AC being tested
	}{
		{
			name:        "AC2.1: filters matching items",
			searchText:  "git",
			active:      true,
			wantCount:   2,
			description: "GH-14.AC2.1: filtering with text returns only matching items",
		},
		{
			name:        "AC2.3: empty search returns all",
			searchText:  "",
			active:      true,
			wantCount:   4,
			description: "GH-14.AC2.3: empty search text returns all items",
		},
		{
			name:        "AC2.4: no matches returns empty",
			searchText:  "nonexistent-command",
			active:      true,
			wantCount:   0,
			description: "GH-14.AC2.4: when no commands match, the list is empty",
		},
		{
			name:        "AC2.2: case insensitive",
			searchText:  "GIT",
			active:      true,
			wantCount:   2,
			description: "GH-14.AC2.2: filtering is case-insensitive",
		},
		{
			name:        "inactive search returns all",
			searchText:  "git",
			active:      false,
			wantCount:   4,
			description: "search inactive returns all items regardless of text",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(ModelOptions{})
			m.width = 80
			m.height = 24
			m.viewMode = ViewCommands
			m.searchActive = tc.active
			m.allCommandItems = testCommandItems()
			m.searchInput.SetValue(tc.searchText)

			m = m.applySearchFilter()

			got := len(m.commandList.Items())
			if got != tc.wantCount {
				t.Errorf("%s: got %d items, want %d", tc.description, got, tc.wantCount)
			}
		})
	}
}

func TestApplySearchFilterMatchesRawCommandOnly(t *testing.T) {
	// AC2.2: Pattern contains "Bash" but RawCommand does not — should NOT match
	m := NewModel(ModelOptions{})
	m.width = 80
	m.height = 24
	m.viewMode = ViewCommands
	m.searchActive = true
	m.allCommandItems = []list.Item{
		commandItem{command: session.CommandEntry{
			RawCommand: "ls -la",
			ToolName:   "Bash",
			Pattern:    "Bash(ls:*)",
			Timestamp:  time.Now(),
		}},
		commandItem{command: session.CommandEntry{
			RawCommand: "/tmp/file.txt",
			ToolName:   "Read",
			Pattern:    "Read",
			Timestamp:  time.Now(),
		}},
	}

	// Search for "Bash" — only the first item has "Bash" in Pattern/ToolName,
	// but neither has "Bash" in RawCommand. Should return 0 items.
	m.searchInput.SetValue("Bash")
	m = m.applySearchFilter()

	got := len(m.commandList.Items())
	if got != 0 {
		t.Errorf("GH-14.AC2.2: expected 0 items when searching Pattern-only match, got %d", got)
	}
}

// --- Search toggle and focus management tests (Phase 2) ---

func TestCtrlFOpensSearch(t *testing.T) {
	// AC1.1: Ctrl+F when search is hidden opens the search bar and focuses the text input
	m := NewModel(ModelOptions{})
	m.width = 80
	m.height = 24
	m.viewMode = ViewCommands

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	model := updated.(Model)

	if !model.searchActive {
		t.Error("AC1.1: expected searchActive to be true after Ctrl+F")
	}
	if !model.searchFocused {
		t.Error("AC1.1: expected searchFocused to be true after Ctrl+F")
	}
}

func TestCtrlFClosesSearchWhenFocused(t *testing.T) {
	// AC1.2, AC5.1, AC5.2: Ctrl+F when focused closes search and clears filter text
	m := NewModel(ModelOptions{})
	m.width = 80
	m.height = 24
	m.viewMode = ViewCommands
	m.searchActive = true
	m.searchFocused = true
	m.searchInput.SetValue("git")
	m.allCommandItems = testCommandItems()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	model := updated.(Model)

	if model.searchActive {
		t.Error("AC1.2: expected searchActive to be false after Ctrl+F from focused state")
	}
	if model.searchFocused {
		t.Error("AC1.2: expected searchFocused to be false after Ctrl+F from focused state")
	}
	if model.searchInput.Value() != "" {
		t.Errorf("AC5.1: expected search text to be cleared, got %q", model.searchInput.Value())
	}
	// AC5.2: all items should be restored (filter cleared)
	got := len(model.commandList.Items())
	if got != len(testCommandItems()) {
		t.Errorf("AC5.2: expected all %d items restored, got %d", len(testCommandItems()), got)
	}
}

func TestCtrlFRefocusesWhenUnfocused(t *testing.T) {
	// AC1.3: Ctrl+F when visible but unfocused re-focuses the text input
	m := NewModel(ModelOptions{})
	m.width = 80
	m.height = 24
	m.viewMode = ViewCommands
	m.searchActive = true
	m.searchFocused = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	model := updated.(Model)

	if !model.searchFocused {
		t.Error("AC1.3: expected searchFocused to be true after Ctrl+F from unfocused state")
	}
	if !model.searchActive {
		t.Error("AC1.3: expected searchActive to remain true")
	}
}

func TestCtrlFOnlyOnCommandsTab(t *testing.T) {
	// AC1.4: Search bar only appears on the Commands tab
	for _, vm := range []ViewMode{ViewSessions, ViewPatterns} {
		m := NewModel(ModelOptions{})
		m.width = 80
		m.height = 24
		m.viewMode = vm

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
		model := updated.(Model)

		if model.searchActive {
			t.Errorf("AC1.4: expected searchActive to remain false on viewMode %d", vm)
		}
	}
}

func TestSearchFocusedKeysGoToInput(t *testing.T) {
	// AC4.1: When search input is focused, keyboard input goes to the text input
	m := NewModel(ModelOptions{})
	m.width = 80
	m.height = 24
	m.viewMode = ViewCommands
	m.searchActive = true
	m.searchFocused = true
	m.searchInput.Focus()
	m.allCommandItems = testCommandItems()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model := updated.(Model)

	if model.searchInput.Value() != "a" {
		t.Errorf("AC4.1: expected search input to contain 'a', got %q", model.searchInput.Value())
	}
	if model.viewMode != ViewCommands {
		t.Errorf("AC4.1: expected viewMode to remain ViewCommands, got %d", model.viewMode)
	}
}

func TestEscUnfocusesSearch(t *testing.T) {
	// AC4.2: Esc while search is focused unfocuses but keeps filter active
	m := NewModel(ModelOptions{})
	m.width = 80
	m.height = 24
	m.viewMode = ViewCommands
	m.searchActive = true
	m.searchFocused = true
	m.searchInput.Focus()
	m.searchInput.SetValue("git")
	m.allCommandItems = testCommandItems()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model := updated.(Model)

	if model.searchFocused {
		t.Error("AC4.2: expected searchFocused to be false after Esc")
	}
	if !model.searchActive {
		t.Error("AC4.2: expected searchActive to remain true after Esc")
	}
	if model.searchInput.Value() != "git" {
		t.Errorf("AC4.2: expected search text to remain 'git', got %q", model.searchInput.Value())
	}
}

func TestTabCyclesSessionAndUnfocuses(t *testing.T) {
	// AC4.3: Tab while search is focused cycles sessions and unfocuses
	m := NewModel(ModelOptions{})
	m.width = 80
	m.height = 24
	m.viewMode = ViewCommands
	m.searchActive = true
	m.searchFocused = true
	m.searchInput.Focus()
	m.sessions = []*session.Session{
		{ID: "session-1", ProjectPath: "/project1"},
		{ID: "session-2", ProjectPath: "/project2"},
	}
	m.activeIdx = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := updated.(Model)

	if model.searchFocused {
		t.Error("AC4.3: expected searchFocused to be false after Tab")
	}
	if model.activeIdx != 1 {
		t.Errorf("AC4.3: expected activeIdx to be 1 after Tab, got %d", model.activeIdx)
	}
}

func TestShiftTabCyclesSessionBackwardAndUnfocuses(t *testing.T) {
	// AC4.3: Shift+Tab while search is focused cycles sessions backward and unfocuses
	m := NewModel(ModelOptions{})
	m.width = 80
	m.height = 24
	m.viewMode = ViewCommands
	m.searchActive = true
	m.searchFocused = true
	m.searchInput.Focus()
	m.sessions = []*session.Session{
		{ID: "session-1", ProjectPath: "/project1"},
		{ID: "session-2", ProjectPath: "/project2"},
	}
	m.activeIdx = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := updated.(Model)

	if model.searchFocused {
		t.Error("AC4.3: expected searchFocused to be false after Shift+Tab")
	}
	if model.activeIdx != 0 {
		t.Errorf("AC4.3: expected activeIdx to be 0 after Shift+Tab, got %d", model.activeIdx)
	}
}

func TestQDoesNotQuitWhenSearchFocused(t *testing.T) {
	// AC4.1: q is routed to text input when search is focused, not to quit
	m := NewModel(ModelOptions{})
	m.width = 80
	m.height = 24
	m.viewMode = ViewCommands
	m.searchActive = true
	m.searchFocused = true
	m.searchInput.Focus()
	m.allCommandItems = testCommandItems()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	// The returned cmd should NOT be tea.Quit
	if cmd != nil {
		// Execute the cmd to check if it produces a quit message
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Error("AC4.1: expected q to NOT quit when search is focused, but got tea.QuitMsg")
		}
	}
}

func TestUnfocusedSearchAllowsListNavigation(t *testing.T) {
	// AC4.4: When search is visible but unfocused, j/k navigate the list normally
	m := NewModel(ModelOptions{})
	m.width = 80
	m.height = 24
	m.viewMode = ViewCommands
	m.searchActive = true
	m.searchFocused = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)

	// 'j' should NOT change search state — it should pass through to list navigation
	if model.searchFocused {
		t.Error("AC4.4: searchFocused should remain false")
	}
	if !model.searchActive {
		t.Error("AC4.4: searchActive should remain true")
	}
	if model.viewMode != ViewCommands {
		t.Errorf("AC4.4: viewMode should remain ViewCommands, got %d", model.viewMode)
	}
}

func TestUpdateCommandListAppliesFilter(t *testing.T) {
	// AC3.4: After calling updateCommandList (simulating new commands arriving),
	// the filter is still applied
	m := NewModel(ModelOptions{})
	m.width = 80
	m.height = 24
	m.viewMode = ViewCommands
	m.searchActive = true
	m.searchInput.SetValue("git")

	// Simulate a session with commands
	m.sessions = []*session.Session{
		{
			ID:          "test-session",
			ProjectPath: "/Users/josh/code/project",
			Commands: []session.CommandEntry{
				{
					RawCommand: "git status",
					ToolName:   "Bash",
					Pattern:    "Bash(git:*)",
					Timestamp:  time.Now(),
				},
				{
					RawCommand: "/tmp/file.txt",
					ToolName:   "Read",
					Pattern:    "Read",
					Timestamp:  time.Now(),
				},
				{
					RawCommand: "git log",
					ToolName:   "Bash",
					Pattern:    "Bash(git:*)",
					Timestamp:  time.Now(),
				},
			},
		},
	}
	m.activeIdx = 0

	m = m.updateCommandList()

	// Should have 2 items (git status, git log) — Read should be filtered out
	got := len(m.commandList.Items())
	if got != 2 {
		t.Errorf("GH-14.AC3.4: expected 2 filtered items after updateCommandList, got %d", got)
	}

	// allCommandItems should have all 3 unfiltered items
	allGot := len(m.allCommandItems)
	if allGot != 3 {
		t.Errorf("expected 3 unfiltered items in allCommandItems, got %d", allGot)
	}
}

func TestNewTestModelWithSessions(t *testing.T) {
	m := newTestModelWithSessions()
	if len(m.sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(m.sessions))
	}
	if len(m.sessions[0].Commands) != 3 {
		t.Errorf("expected 3 commands in session 1, got %d", len(m.sessions[0].Commands))
	}
	if len(m.sessions[1].Commands) != 3 {
		t.Errorf("expected 3 commands in session 2, got %d", len(m.sessions[1].Commands))
	}
	if m.viewMode != ViewCommands {
		t.Errorf("expected ViewCommands, got %d", m.viewMode)
	}
	// Command list should be populated with session 1's commands
	if len(m.commandList.Items()) != 3 {
		t.Errorf("expected 3 items in command list, got %d", len(m.commandList.Items()))
	}
}

// --- Filter persistence integration tests (Phase 4) ---

func TestFilterPersistsAcrossSessionSwitch(t *testing.T) {
	// AC3.1: Switching sessions with Tab re-applies the active search filter to the new session's commands
	m := newTestModelWithSessions()

	// Open search and set filter text "git"
	m.searchActive = true
	m.searchFocused = true
	m.searchInput.Focus()
	m.searchInput.SetValue("git")
	m = m.applySearchFilter()

	// Verify session 1 shows 1 filtered result ("git status" matches, "go test" and "Read" don't)
	got := len(m.commandList.Items())
	if got != 1 {
		t.Fatalf("AC3.1: expected 1 filtered item in session 1, got %d", got)
	}

	// Send Tab to switch to session 2 (Tab from focused search unfocuses AND switches)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := updated.(Model)

	// Verify session switched
	if model.activeIdx != 1 {
		t.Errorf("AC3.1: expected activeIdx to be 1 after Tab, got %d", model.activeIdx)
	}

	// Verify search state preserved (Tab unfocuses but keeps search active)
	if !model.searchActive {
		t.Error("AC3.1: expected searchActive to remain true after Tab")
	}
	if model.searchInput.Value() != "git" {
		t.Errorf("AC3.1: expected search text to remain 'git', got %q", model.searchInput.Value())
	}

	// Verify filtered command list contains 2 items ("git diff" and "git commit -m fix" from session 2)
	filteredCount := len(model.commandList.Items())
	if filteredCount != 2 {
		t.Errorf("AC3.1: expected 2 filtered items in session 2, got %d", filteredCount)
	}
}

func TestFilterPersistsAcrossViewSwitch(t *testing.T) {
	// AC3.2, AC3.3: Switching views with h/l preserves the search bar state and filter text
	m := newTestModelWithSessions()

	// Activate search, set filter text "git", apply filter
	m.searchActive = true
	m.searchFocused = false // Unfocused so 'l' goes to view switching, not text input
	m.searchInput.SetValue("git")
	m = m.applySearchFilter()

	// Verify filtered state on Commands tab
	initialCount := len(m.commandList.Items())
	if initialCount != 1 {
		t.Fatalf("AC3.2: expected 1 filtered item initially, got %d", initialCount)
	}

	// Send 'l' to switch to Patterns view
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model := updated.(Model)

	if model.viewMode != ViewPatterns {
		t.Errorf("AC3.2: expected ViewPatterns after 'l', got %d", model.viewMode)
	}
	if !model.searchActive {
		t.Error("AC3.2: expected searchActive to remain true after view switch")
	}
	if model.searchInput.Value() != "git" {
		t.Errorf("AC3.2: expected search text 'git' preserved, got %q", model.searchInput.Value())
	}

	// Send 'h' to return to Commands view
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model = updated.(Model)

	// AC3.3: Returning to Commands tab shows the search bar with previous text
	if model.viewMode != ViewCommands {
		t.Errorf("AC3.3: expected ViewCommands after 'h', got %d", model.viewMode)
	}
	if !model.searchActive {
		t.Error("AC3.3: expected searchActive to remain true after returning to Commands")
	}
	if model.searchInput.Value() != "git" {
		t.Errorf("AC3.3: expected search text 'git' still preserved, got %q", model.searchInput.Value())
	}

	// Verify the command list is still filtered (same count as before the view switch)
	afterCount := len(model.commandList.Items())
	if afterCount != initialCount {
		t.Errorf("AC3.3: expected %d filtered items after returning, got %d", initialCount, afterCount)
	}
}

func TestFilterReappliedOnNewCommands(t *testing.T) {
	// AC3.4: Live watcher events (new commands arriving) rebuild the list with the filter applied
	m := newTestModelWithSessions()

	// Activate search, set filter "git", apply filter
	m.searchActive = true
	m.searchFocused = false
	m.searchInput.SetValue("git")
	m = m.applySearchFilter()

	// Verify initial filtered count (1 result: "git status" in session 1)
	initialCount := len(m.commandList.Items())
	if initialCount != 1 {
		t.Fatalf("AC3.4: expected 1 filtered item initially, got %d", initialCount)
	}

	// Simulate a new matching command arriving
	now := time.Now()
	m.sessions[0].Commands = append(m.sessions[0].Commands, session.CommandEntry{
		ToolName:   "Bash",
		RawCommand: "git push",
		Pattern:    "Bash(git:*)",
		Timestamp:  now,
	})
	m = m.updateCommandList()

	// Verify search state preserved
	if !m.searchActive {
		t.Error("AC3.4: expected searchActive to remain true after new commands")
	}
	if m.searchInput.Value() != "git" {
		t.Errorf("AC3.4: expected search text 'git' preserved, got %q", m.searchInput.Value())
	}

	// Verify filtered count increased by 1 (now includes "git push")
	afterMatchCount := len(m.commandList.Items())
	if afterMatchCount != 2 {
		t.Errorf("AC3.4: expected 2 filtered items after adding 'git push', got %d", afterMatchCount)
	}

	// Append a non-matching command
	m.sessions[0].Commands = append(m.sessions[0].Commands, session.CommandEntry{
		ToolName:   "Bash",
		RawCommand: "ls -la",
		Pattern:    "Bash(ls:*)",
		Timestamp:  now,
	})
	m = m.updateCommandList()

	// Verify filtered count didn't increase for non-matching command
	afterNonMatchCount := len(m.commandList.Items())
	if afterNonMatchCount != 2 {
		t.Errorf("AC3.4: expected 2 filtered items after adding non-matching 'ls -la', got %d", afterNonMatchCount)
	}
}

