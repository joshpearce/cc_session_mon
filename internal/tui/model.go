package tui

import tea "github.com/charmbracelet/bubbletea"

// Model represents the application state
type Model struct {
	counter int
}

// NewModel creates a new Model with default values
func NewModel() Model {
	return Model{
		counter: 0,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Counter returns the current counter value
func (m Model) Counter() int {
	return m.counter
}
