# cc_session_mon

A terminal user interface for monitoring Claude Code sessions in real-time. Built with [Bubbletea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lipgloss](https://github.com/charmbracelet/lipgloss).

## Features

- **Live Session Monitoring**: Watches `~/.claude/projects/` for active Claude Code sessions
- **Command History**: View tool calls made by Claude in each session
- **Pattern Analysis**: See aggregated command patterns per session with counts
- **Configurable Styling**: Customize colors and visibility of different tool types
- **Catppuccin Themes**: Supports mocha, macchiato, frappe, and latte color schemes

## Prerequisites

- Go 1.25 or later
- Or [Nix](https://nixos.org/) with flakes enabled

## Quick Start

### With Nix (recommended)

```bash
# Build with Nix
nix build

# Run directly
nix run

# Or enter dev shell with all tools
nix develop
```

If you have [direnv](https://direnv.net/) installed, the dev shell activates automatically when you `cd` into the project.

### With Make

```bash
# Install dependencies
make deps

# Build the application
make build

# Run the application
make run
```

## Usage

### Navigation

- `j`/`k` or `↑`/`↓` - Navigate lists
- `h`/`l` or `←`/`→` - Switch between views (Sessions, Commands, Patterns)
- `Tab`/`Shift+Tab` - Switch active session
- `Enter` - Drill down from sessions to commands
- `Esc`/`Backspace` - Go back to sessions view
- `1`/`2`/`3` - Jump directly to Sessions/Commands/Patterns view
- `r` - Refresh sessions
- `q` or `Ctrl+C` - Quit

### Views

1. **Sessions**: List of discovered Claude Code sessions, sorted by activity
2. **Commands**: Tool calls for the selected session (newest first)
3. **Patterns**: Aggregated command patterns for the selected session with counts

## Configuration

Create a config file at `~/.config/cc_session_mon/config.yaml`:

```yaml
# Catppuccin theme: mocha, macchiato, frappe, latte
theme: mocha

# Tool groups (checked in order, first match wins)
tool_groups:
  # Dangerous commands - red and bold
  - name: dangerous
    color: red
    bold: true
    patterns:
      - "Bash(rm:*)"
      - "Bash(sudo:*)"
      - "Bash(chmod:*)"

  # File writes - peach
  - name: write
    color: peach
    patterns:
      - Write
      - NotebookEdit

  # Edits - yellow (less safe than read)
  - name: edit
    color: yellow
    patterns:
      - Edit

  # Other bash commands - mauve
  - name: bash
    color: mauve
    patterns:
      - "Bash(*)"

  # Subagent tasks - lavender
  - name: task
    color: lavender
    patterns:
      - Task
      - TaskOutput

  # Read-only operations - green (safe)
  - name: read-only
    color: green
    patterns:
      - Read
      - Glob
      - Grep
      - WebFetch
      - WebSearch
      - "mcp__*"

  # Exclude specific tools from display
  - name: hidden
    exclude: true
    patterns:
      - TodoRead

  # Catch-all for unmatched tools
  - name: unmatched
    color: overlay1
    patterns:
      - "*"
```

### Pattern Syntax

Patterns support wildcard matching with `*`:
- `Edit` - Exact match
- `Bash(*)` - Matches any Bash command
- `Bash(rm:*)` - Matches rm commands specifically
- `mcp__*` - Matches all MCP tool calls

### Available Colors

Any Catppuccin color name: `red`, `peach`, `yellow`, `green`, `teal`, `blue`, `mauve`, `lavender`, `pink`, `flamingo`, `rosewater`, `sky`, `sapphire`, `maroon`, `text`, `subtext0`, `subtext1`, `overlay0`, `overlay1`, `overlay2`, `surface0`, `surface1`, `surface2`, `base`, `mantle`, `crust`

## Development

```bash
# Run tests
make test

# Run linter (requires golangci-lint)
make lint

# Clean build artifacts
make clean

# Regenerate SRI hash after changing go.mod/go.sum (in nix develop shell)
regenSRI
```

### CI

GitHub Actions runs on every PR:
- **Go CI**: lint, test, build
- **Nix CI**: flake check, nix build
- **Dependabot**: weekly dependency updates with automatic SRI hash regeneration

## Project Structure

```
cc_session_mon/
├── main.go                    # Application entry point
├── config.yaml                # Local config (optional)
├── internal/
│   ├── config/
│   │   ├── config.go          # Configuration loading and tool groups
│   │   └── config_test.go     # Config tests
│   ├── session/
│   │   ├── parser.go          # JSONL session file parsing
│   │   ├── pattern.go         # Command pattern extraction
│   │   ├── pattern_test.go    # Pattern tests
│   │   ├── session.go         # Session data structures
│   │   └── watcher.go         # File system watcher
│   └── tui/
│       ├── model.go           # Application state (Model)
│       ├── update.go          # Event handling (Update)
│       ├── view.go            # UI rendering (View)
│       ├── styles.go          # Lipgloss styles and theming
│       └── delegates.go       # List item delegates
├── Makefile                   # Build commands
├── flake.nix                  # Nix flake definition
├── dev/                       # Nix dev partition (devshell, SRI tooling)
└── go.mod                     # Go module definition
```

## Architecture

This app follows the [Elm Architecture](https://guide.elm-lang.org/architecture/):

- **Model**: Application state including sessions, view mode, and UI components (`internal/tui/model.go`)
- **Update**: Handles keyboard input, file events, and timer ticks (`internal/tui/update.go`)
- **View**: Renders the active list and status information (`internal/tui/view.go`)

### Key Components

- **Session Parser**: Reads Claude Code's JSONL session files and extracts tool calls
- **Watcher**: Uses fsnotify to detect new sessions and file changes in real-time
- **Config System**: YAML-based configuration with pattern matching for tool styling
