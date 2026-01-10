# cc_session_mon

A terminal user interface application built with [Bubbletea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lipgloss](https://github.com/charmbracelet/lipgloss).

## Prerequisites

- Go 1.22 or later
- (Optional) [golangci-lint](https://golangci-lint.run/) for linting

## Quick Start

```bash
# Install dependencies
make deps

# Build the application
make build

# Run the application
make run
```

## Usage

- `↑` or `k` - Increment counter
- `↓` or `j` - Decrement counter
- `q` or `Esc` - Quit

## Development

```bash
# Run tests
make test

# Run linter (requires golangci-lint)
make lint

# Clean build artifacts
make clean
```

## Project Structure

```
cc_session_mon/
├── main.go              # Application entry point
├── internal/tui/
│   ├── model.go         # Application state (Model)
│   ├── update.go        # Event handling (Update)
│   ├── view.go          # UI rendering (View)
│   ├── styles.go        # Lipgloss styles
│   └── model_test.go    # Tests
├── Makefile             # Build commands
└── go.mod               # Go module definition
```

## Architecture

This app follows the [Elm Architecture](https://guide.elm-lang.org/architecture/):

- **Model**: The application state (`internal/tui/model.go`)
- **Update**: Handles messages/events and updates state (`internal/tui/update.go`)
- **View**: Renders the UI based on current state (`internal/tui/view.go`)
