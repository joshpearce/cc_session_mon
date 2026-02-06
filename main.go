package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"cc_session_mon/internal/tui"
)

func main() {
	followDevagent := flag.Bool("follow-devagent", false, "Monitor sessions in devagent containers")
	flag.Parse()

	opts := tui.ModelOptions{
		FollowDevagent: *followDevagent,
	}
	p := tea.NewProgram(tui.NewModel(opts), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
