package main

import (
	"flag"
	"fmt"
	"os"

	"cc_session_mon/internal/config"
	"cc_session_mon/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// version is the release version, injected at build time via
// -ldflags "-X main.version=...". It is "dev" for local builds.
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	opts := tui.ModelOptions{
		SearchPaths: config.Global().SearchPaths,
	}
	p := tea.NewProgram(tui.NewModel(opts), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
