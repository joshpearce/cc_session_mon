package tui

import "fmt"

// View renders the UI based on the model state
func (m Model) View() string {
	return fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		titleStyle.Render("Counter"),
		counterStyle.Render(fmt.Sprintf("%d", m.counter)),
		helpStyle.Render("↑/k: increment • ↓/j: decrement • q: quit"),
	)
}
