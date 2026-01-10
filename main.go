package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"cc_session_mon/internal/tui"
)

func main() {
	p := tea.NewProgram(tui.NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
