package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	m := NewModel()
	if m.Counter() != 0 {
		t.Errorf("expected counter to be 0, got %d", m.Counter())
	}
}

func TestUpdateIncrement(t *testing.T) {
	m := NewModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model := updated.(Model)
	if model.Counter() != 1 {
		t.Errorf("expected counter to be 1 after increment, got %d", model.Counter())
	}
}

func TestUpdateDecrement(t *testing.T) {
	m := NewModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.Counter() != -1 {
		t.Errorf("expected counter to be -1 after decrement, got %d", model.Counter())
	}
}
