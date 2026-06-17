# cc_session_mon

Last verified: 2026-06-17

A Go TUI application for monitoring Claude Code sessions using Bubbletea, Bubbles, and Lipgloss. Monitors the user's local sessions plus any nested `.claude/projects` directories found under configured search paths (e.g. devcontainer mounts).

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

### internal/discovery

Locates Claude `projects` directories to watch:

- `LocalProjectsDir()` - Resolves the local Claude projects dir, honoring `CLAUDE_CONFIG_DIR` (the same override Claude Code uses), falling back to `$HOME/.claude/projects`
- `FindProjectsDirs(roots []string)` - Recursively scans each search root for `.claude/projects` directories, returning `[]ProjectsDir{Path, Label}`; recursion is capped at `maxSearchDepth`, missing/unreadable roots are skipped, results are deduped by path
- `ProjectsDir` - A discovered projects dir plus a short origin label
- `deriveLabel()` - Pure helper that derives a label from a `.claude` path; for devcontainer layouts (`<repo>/.devcontainer/containers/<name>/...`) it yields `<repo>/<name>`, otherwise it falls back to the parent dir name

### internal/session

Session parsing and monitoring:

- `Session` - Represents a Claude Code session with commands; has `Origin` field (`"local"` or a derived search-path label)
- `CommandEntry` - A single tool call with timestamp, tool name, and pattern
- `CommandPattern` - Aggregated pattern with count and examples
- `ParseSessionFile()` - Parses JSONL session files
- `GenericInput` - Extracts display strings from any tool's JSON input
- `Watcher` - fsnotify-based file watcher for live updates; monitors multiple project directories
- `NewWatcher(projectsDirs []string)` - Creates watcher for one or more project directories
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

None. Configuration is via `config.yaml` (see Configuration below).

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

# Roots scanned recursively at startup for nested .claude/projects directories
# (e.g. devcontainer mounts). The local Claude projects dir is always watched
# in addition to these. Supports a leading ~ for $HOME.
search_paths:
  - ~/code

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
The `Watcher` monitors multiple project directories simultaneously. Each directory has an origin label (e.g., `"local"`, or a derived `<repo>/<container>` label). Sessions inherit the origin of the directory they were discovered in and non-local origins are shown as a `[label]` tag in the session list.

### Search-Path Discovery
At startup `internal/discovery` resolves the local Claude projects dir (`CLAUDE_CONFIG_DIR`-aware) and recursively scans each configured `search_paths` root for nested `.claude/projects` directories. Discovery happens once at startup; new session files inside already-discovered dirs still appear live via fsnotify, but brand-new `.claude` directories require a restart. Recursion is depth-capped and unreadable roots are skipped so a broad search root can't hang or crash the app.
