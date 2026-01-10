# cc_session_mon

A Go TUI application using Bubbletea, Bubbles, and Lipgloss.

## Architecture

This app follows the Elm Architecture (Model-Update-View):

- `internal/tui/model.go` - Application state and initialization
- `internal/tui/update.go` - Event handling (keyboard input, messages)
- `internal/tui/view.go` - UI rendering
- `internal/tui/styles.go` - Lipgloss style definitions

## Commands

- `make deps` - Install/update dependencies
- `make build` - Build binary to `bin/cc_session_mon`
- `make run` - Run the application
- `make test` - Run tests
- `make lint` - Run golangci-lint

## Key Libraries

- [Bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - Pre-built TUI components
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Styling

## Adding Features

1. Add new state fields to `Model` in `model.go`
2. Handle new key bindings or messages in `update.go`
3. Update the `View()` function to render new state
4. Add styles in `styles.go` as needed
