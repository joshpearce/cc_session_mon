package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"cc_session_mon/internal/config"
	"cc_session_mon/internal/tui"
)

func main() {
	opts := tui.ModelOptions{
		SearchPaths: config.Global().SearchPaths,
	}
	p := tea.NewProgram(tui.NewModel(opts), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
