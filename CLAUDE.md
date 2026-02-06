# cc_session_mon

Last verified: 2026-02-06

A Go TUI application for monitoring Claude Code sessions using Bubbletea, Bubbles, and Lipgloss. Supports monitoring both local sessions and sessions running inside devagent containers.

## Architecture

This app follows the Elm Architecture (Model-Update-View):

- `internal/tui/model.go` - Application state (`Model`, `ModelOptions`), session management, pattern aggregation
- `internal/tui/update.go` - Event handling (keyboard input, file events, timers)
- `internal/tui/view.go` - UI rendering with tabs for sessions/commands/patterns
- `internal/tui/styles.go` - Lipgloss style definitions, Catppuccin theming
- `internal/tui/delegates.go` - List item rendering delegates

## Key Packages

### internal/config

Configuration system with pattern-based tool grouping:

- `ToolGroup` - Defines styling (color, bold) and patterns for a group of tools
- `matchPattern()` - Wildcard pattern matching (`*` anywhere in pattern)
- `GetToolGroup()` - Returns first matching group for a pattern
- `ShouldExclude()` - Checks if a pattern should be hidden

### internal/devagent

Discovers devagent container environments for remote session monitoring:

- `Discover()` - Runs `devagent list` CLI and returns `[]Environment`
- `ParseOutput(data []byte)` - Parses JSON output from devagent list
- `Environment` - Container name, project path, host-side projects dir, state
- Finds the `/home/vscode/.claude` mount, strips `/host_mnt` prefix (macOS Docker), appends `/projects`
- Containers without a `.claude` mount are skipped

### internal/session

Session parsing and monitoring:

- `Session` - Represents a Claude Code session with commands; has `Origin` field (`"local"` or `"devagent:<container-name>"`)
- `CommandEntry` - A single tool call with timestamp, tool name, and pattern
- `CommandPattern` - Aggregated pattern with count and examples
- `ParseSessionFile()` - Parses JSONL session files
- `GenericInput` - Extracts display strings from any tool's JSON input
- `Watcher` - fsnotify-based file watcher for live updates; monitors multiple project directories
- `NewWatcher(projectsDirs []string)` - Creates watcher for one or more project directories
- `AddProjectsDir(dir string) bool` - Dynamically adds a directory to monitor
- `SetOrigin(dir, label string)` - Associates an origin label with a projects directory

## Commands

### Nix (preferred)

- `nix build` - Build with Nix
- `nix run` - Run directly
- `nix develop` - Enter dev shell with Go, gopls, golangci-lint
- `regenSRI` - Regenerate SRI hash after go.mod/go.sum changes (in dev shell)

### Make

- `make deps` - Install/update dependencies
- `make build` - Build binary to `bin/cc_session_mon`
- `make run` - Run the application
- `make test` - Run tests
- `make lint` - Run golangci-lint

### CLI Flags

- `--follow-devagent` - Monitor sessions in devagent containers (discovers environments via `devagent list`)

## Development Workflow

Uses direnv with Nix flakes. The `.envrc` activates the dev shell automatically.

Pre-commit hooks (lefthook) run golangci-lint automatically on staged files. The `.golangci.yml` (v2 format) enables strict linting with relaxed rules for test files.

### CI

GitHub Actions CI runs on PRs:
- **ci_go.yml**: lint (golangci-lint v2), test, build
- **ci_nix.yml**: flake check, nix build
- **dependabot_regenerate_sri.yml**: auto-regenerates SRI hash when go.mod/go.sum change

### Nix Flake Structure

The flake uses flake-parts with partitions:
- Main flake: packages only (lightweight for consumers)
- Dev partition (`dev/`): devshell, generate-go-sri for `regenSRI` command
- `cc-session-mon.sri`: vendorHash read from file for reproducible builds

## Key Libraries

- [Bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - Pre-built TUI components (list)
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Styling
- [Catppuccin](https://github.com/catppuccin/go) - Color palette

## Configuration

Config file at `~/.config/cc_session_mon/config.yaml`:

```yaml
theme: mocha  # mocha, macchiato, frappe, latte

tool_groups:
  - name: dangerous
    color: red
    bold: true
    patterns: ["Bash(rm:*)", "Bash(sudo:*)"]
  - name: write
    color: peach
    patterns: [Write, NotebookEdit]
  - name: edit
    color: yellow
    patterns: [Edit]
  - name: bash
    color: mauve
    patterns: ["Bash(*)"]
  - name: read-only
    color: green
    patterns: [Read, Glob, Grep]
  - name: unmatched
    color: overlay1
    patterns: ["*"]
```

## Adding Features

1. Add new state fields to `Model` in `model.go`; add new options to `ModelOptions` if configurable at startup
2. Handle new key bindings or messages in `update.go` (app-level in `handleAppMsg`, keyboard in `handleKeyPress`)
3. Update the `View()` function to render new state
4. Add styles in `styles.go` as needed

## Design Decisions

### Pattern Matching
Tool groups use pattern matching with wildcards. Patterns like `Bash(rm:*)` match any rm command. Groups are checked in order; first match wins.

### Generic Input Parsing
`GenericInput` in parser.go extracts display strings from tool inputs by trying common field names (file_path, path, command, pattern, query, etc.). This handles unknown tools gracefully.

### Scroll Position Preservation
Lists preserve scroll position during updates unless: session changes, initial load, or user was already at top. View switching (h/l keys) returns early to avoid passing keys to list components.

### Per-Session Patterns
The patterns view shows aggregated command patterns for the currently selected session only, not across all sessions.

### Multi-Directory Watching
The `Watcher` monitors multiple project directories simultaneously. Each directory has an origin label (e.g., `"local"`, `"devagent:container-name"`). Sessions inherit the origin of the directory they were discovered in. When `--follow-devagent` is enabled, devagent environments are re-discovered on each tick and new directories are added dynamically via `AddProjectsDir`.

### Devagent Integration
Devagent environments are discovered by running `devagent list` and parsing its JSON output. The host-side session path is derived from the container's `.claude` mount point. Sessions from devagent containers display a `[da]` tag in the session list. If devagent discovery fails, the app falls back to local-only monitoring.
