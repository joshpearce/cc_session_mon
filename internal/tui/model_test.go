package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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
