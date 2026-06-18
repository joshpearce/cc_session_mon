# cc_session_mon

Last verified: 2026-06-17

A Go TUI application for monitoring Claude Code sessions using Bubbletea, Bubbles, and Lipgloss. Monitors the user's local sessions plus any nested `.claude/projects` directories found under configured search paths (e.g. devcontainer mounts).

## Architecture

This app follows the Elm Architecture (Model-Update-View):

- `internal/tui/model.go` - Application state (`Model`, `ModelOptions`), session management, pattern aggregation
- `internal/tui/update.go` - Event handling (keyboard input, file events, timers)
- `internal/tui/view.go` - UI rendering with tabs for sessions/commands/patterns/audit
- `internal/tui/styles.go` - Lipgloss style definitions, Catppuccin theming
- `internal/tui/delegates.go` - List item rendering delegates
- `internal/tui/alerts.go` - `evaluateAlerts` runs each configured rule against every session on the 30s tick; split into `applyAlertTier` (bell + alert latch) and `applyActionTier` (sustained-streak tracking + gated corrective-action dispatch)
- `internal/tui/audit.go` - `auditLog` fixed-capacity (50) ring buffer; `AuditEntry` records alert/action/skip/config-error events; `recent()` returns entries newest-first for the Audit view

## Key Packages

### internal/config

Configuration system with pattern-based tool grouping:

- `ToolGroup` - Defines styling (color, bold) and patterns for a group of tools
- `matchPattern()` - Wildcard pattern matching (`*` anywhere in pattern)
- `GetToolGroup()` - Returns first matching group for a pattern
- `ShouldExclude()` - Checks if a pattern should be hidden
- `BurnWindow()` - Returns the configured `burn_window_minutes` as a duration, falling back to `DefaultBurnWindow` (10m) when unset or non-positive; `DefaultBurnWindow` is the single shared source so `session.DefaultBurnWindow` aliases it
- `Global()` - Returns the process-wide loaded config (read by the session/render paths)
- `AlertRule` - A threshold-alert rule for one named live metric (`Metric`, `AlertThreshold`, `ActionThreshold`, `ActionSustainedTicks`). Thresholds are `float64` to cover counts and token rates.
- `Config.Alerts` - Slice of `AlertRule` evaluated on each UI tick; YAML key `alerts` replaces defaults when provided. Defaults ship one rule: `{active_subagents, 20, 40, 1}`.
- `Config.EnableCorrectiveActions` / `ActionDryRun` - Master opt-in and dry-run flag for side-effecting actions; both default false (app is read-only until opt-in).
- `Config.DevcontainerFilterRelPath` / `AnthropicAllowPattern` - Devcontainer proxy filter path (relative to `.devcontainer` anchor) and regex locating the `api.anthropic.com` allow-rule line.

### internal/discovery

Locates Claude `projects` directories to watch:

- `LocalProjectsDir()` - Resolves the local Claude projects dir, honoring `CLAUDE_CONFIG_DIR` (the same override Claude Code uses), falling back to `$HOME/.claude/projects`
- `FindProjectsDirs(roots []string)` - Recursively scans each search root for `.claude/projects` directories, returning `[]ProjectsDir{Path, Label}`; recursion is capped at `maxSearchDepth`, missing/unreadable roots are skipped, results are deduped by path
- `ProjectsDir` - A discovered projects dir plus a short origin label
- `deriveLabel()` - Pure helper that derives a label from a `.claude` path; for devcontainer layouts (`<repo>/.devcontainer/containers/<name>/...`) it yields `<repo>/<name>`, otherwise it falls back to the parent dir name
- `DevcontainerAnchor(p string) (string, bool)` - Walks ancestors of `p` to find the nearest `.devcontainer` directory; returns its path and true if found. Used as the trust boundary for `cutDevcontainer`: the filter file must resolve under this anchor.

### internal/session

Session parsing and monitoring:

- `Session` - Represents a Claude Code session with commands; has `Origin` field (`"local"` or a derived search-path label) and the precomputed metrics `Burn`, `BurnRecent`, and `AgentStats` (filled under the watcher lock so the render path only reads finished values)
- `CommandEntry` - A single tool call with timestamp, tool name, and pattern
- `CommandPattern` - Aggregated pattern with count and examples
- `ParseSessionFile()` - Parses JSONL session files
- `GenericInput` - Extracts display strings from any tool's JSON input
- `Watcher` - fsnotify-based file watcher for live updates; monitors multiple project directories
- `NewWatcher(projectsDirs []string)` - Creates watcher for one or more project directories
- `SetOrigin(dir, label string)` - Associates an origin label with a projects directory
- `RefreshActiveMetrics()` - Periodic-tick safety net that recomputes burn/agent metrics for active sessions; does subtree disk I/O *outside* the watcher lock and skips subtrees unchanged by mtime

#### Metrics (burnrate.go, agenttree.go, subtree.go)

- `UsageEntry` - One timestamped token-accounting sample from an assistant message; `BillableTokens()` = input + output + cache-creation (cache *reads* are deliberately excluded so cache-heavy sessions don't all look like they're burning)
- `BurnRateResult` / `ComputeBurnRate(usages, window)` - Billable tokens-per-minute over a trailing window *anchored on the session's last action* (not wall-clock), so idle sessions still report how hard they were burning before going quiet
- `RecentWindow` - Fixed 1-minute window behind the live `tok/m(1m)` and "active subagents" figures; intentionally not configurable ("what is happening right now")
- `AgentNode` - One spawned subagent's id and `[FirstSeen, LastSeen]` activity span (no parent link; nesting depth isn't recoverable from on-disk transcripts)
- `AgentMetrics` / `ComputeAgentMetrics(nodes)` - `TotalAgents` (lifetime), `MaxConcurrent` (peak overlapping spans via sweep line — "how parallel did it ever get"), and per-agent `LastSeen`; `ActiveWithin(now, window)` counts agents last active within `window` of the wall clock so the "active" figure decays to 0 when a session goes quiet
- `ScanSubtree(mainPath)` - Reads the main file plus every `subagents/agent-*.jsonl` once each, returning merged `[]UsageEntry` (burn rate) and one `AgentNode` per subagent file (agent counts); the sole reader of the subtree

#### Metric registry (metrics.go)

- `MetricFunc` - `func(s *Session, now time.Time) float64`; each metric is responsible for its own wall-clock liveness (an idle session should decay to a non-tripping value rather than latching a threshold forever)
- `metricRegistry` - String-keyed map of `MetricFunc`; `Metric(name)` is the public accessor. Adding a metric = one registry entry + a config rule; no control-flow changes required.
- `"active_subagents"` - Delegates to `AgentStats.ActiveWithin(now, RecentWindow)`; inherits that function's wall-clock decay.
- `"tokens_per_min_1m"` - Returns `BurnRecent.TokensPerMinute` but gates to 0 once `now - BurnRecent.WindowEnd > RecentWindow`, so a session that burned hard then went quiet doesn't trip forever.

#### Corrective actions (action.go)

- `NeutralizeSession(sess, cfg, dryRun)` - Dispatches by origin: local → `killLocal` (pkill -f), devcontainer → `cutDevcontainer`.
- `IsTrustedPath(filePath, roots)` - Reports whether `filePath` is equal to or under one of the watched project roots; used as a gate before any action fires.
- `cutDevcontainer` - Walks `sess.FilePath` up to its `.devcontainer` anchor via `discovery.DevcontainerAnchor`, joins `cfg.DevcontainerFilterRelPath`, verifies the result stays under the anchor both lexically and after `filepath.EvalSymlinks` (rejects `..` traversal and symlink escape), then prefix-comments (`# `) every uncommented line matching `cfg.AnthropicAllowPattern` in the proxy `filter.py`. Write is atomic (temp file + `os.Rename`), idempotent (already-commented matching lines are left untouched), and has no rollback — the cut persists until hand-edited.

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

`--version` prints the build version (injected via `-ldflags -X main.version`; `dev` for local builds) and exits. All other configuration is via `config.yaml` (see Configuration below).

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

# Trailing window, in minutes, the token burn rate is averaged over for the
# tok/m(Nm) column (anchored on each session's last action). Omit or set <= 0
# for the default (10 minutes). The tok/m(1m) column and the "active subagents"
# count use a fixed 1-minute window that is intentionally not configurable.
burn_window_minutes: 10

# Metric-threshold alert rules evaluated on each 30s UI tick.
# Omit this key to keep the default rule; an empty or null `alerts:` disables all alerting.
# alert_threshold  → console bell (BEL), once per crossing.
# action_threshold → corrective action when enable_corrective_actions is true.
# action_sustained_ticks → consecutive over-threshold ticks before action fires.
alerts:
  - metric: active_subagents
    alert_threshold: 20
    action_threshold: 40
    action_sustained_ticks: 1  # fires on the first over-threshold tick
  # Uncomment and calibrate to also alert on recent token burn rate:
  # - metric: tokens_per_min_1m
  #   alert_threshold: 5000
  #   action_threshold: 10000
  #   action_sustained_ticks: 2

# Corrective-action settings — all default to safe/off values.
enable_corrective_actions: false  # master opt-in; app is read-only by default
action_dry_run: false             # when true, log intended actions without executing
devcontainer_filter_rel_path: proxy/filter.py  # path relative to .devcontainer anchor
# Regex locating the allow-rule line. Default matches api.anthropic.com only as a
# complete host token (not as a substring of a longer domain). Override with a
# pattern scoped to your allow directive if filter.py also has deny/comment lines.
anthropic_allow_pattern: '(^|[^A-Za-z0-9.-])api\.anthropic\.com($|[^A-Za-z0-9.-])'

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

### Token Burn Rate
The session list shows two burn-rate columns, both billable tokens/min: `tok/m(Nm)` over the configurable `burn_window_minutes` window and `tok/m(1m)` over a fixed 1-minute window. Both windows are anchored on the session's *last action* rather than wall-clock time, so an idle session still reports how hard it was burning right before it went quiet. Billable tokens exclude cache reads (cheap, dominant in normal sessions) so cache-heavy sessions don't all read as runaways.

### Subagent Counts
The `agents` column reads `peak/active`. `peak` (`MaxConcurrent`) is the lifetime maximum of overlapping subagent activity spans (sweep line over each subagent file's first→last record) — "how parallel did this session ever get". `active` (`ActiveWithin`) counts subagents whose last record is within the trailing 1 minute, evaluated against the wall clock at render, so it decays to 0 within ~30s of a session going quiet. A session with no subagents shows `—`. There is no parent/child tree: the spawning Agent/Task call isn't persisted, so nesting depth can't be reconstructed. See `docs/testing-agent-metrics.md` for a way to drive this metric with a real subagent fan-out.

### Precomputed Metrics Off-Lock
Burn-rate and agent metrics are computed from the whole transcript subtree (main file + all `subagents/agent-*.jsonl`) and stored on the `Session` as finished values, so the TUI render path never re-scans files. They are recomputed on fsnotify writes and, as a safety net for missed writes or token growth that produced no new command, on the periodic tick via `RefreshActiveMetrics`. That tick does its disk I/O *outside* the watcher lock (so a large/slow subtree on a network mount can't stall `GetSessions`) and skips subtrees unchanged since the last refresh via `subtreeModTime`.

### Metric-Alert Engine and Corrective Actions

**Metric registry.** Each alertable metric is a `MetricFunc` in a string-keyed registry (`internal/session/metrics.go`). Every metric enforces its own wall-clock liveness: `active_subagents` delegates to `ActiveWithin` (which decays naturally), and `tokens_per_min_1m` gates to 0 once the session has been idle longer than `RecentWindow`. Adding a new alert metric requires only a registry entry and a config rule; no control-flow changes.

**Alert vs action tiers.** Each `AlertRule` has two independent thresholds:

- *Alert tier* (`AlertThreshold`): fires a stderr BEL once per threshold crossing, latched per `(FilePath, metric)` and cleared when the metric drops back below. A non-positive `AlertThreshold` disables this tier entirely.
- *Action tier* (`ActionThreshold`): maintains a consecutive over-threshold tick streak per `(FilePath, metric)`; when the streak reaches `ActionSustainedTicks`, the corrective action is considered. A non-positive `ActionThreshold` resets the streak to 0 and never acts.

**Safety rails (action tier only).** All of the following must hold before an action fires: (1) `EnableCorrectiveActions: true` in config (default false; app is read-only until opt-in); (2) the session has not been acted on yet — a per-session `actionLatch` keyed by `FilePath`; (3) `IsTrustedPath` verifies the session's `FilePath` lies within a watched projects root, so a crafted transcript path can't steer a kill or file-edit outside the monitored tree; (4) the streak has reached `ActionSustainedTicks`. `ActionDryRun: true` logs the intended action without executing it. The latch is consumed for every outcome **except** a transient `failed` one (e.g. `filter.py` momentarily unreadable, `pkill` errored): success, dry-run, and a deliberately `skipped` (untrusted-path — a stable condition) attempt all latch the session so it can't re-fire or spam the audit log, while a `failed` attempt is left un-latched so a later tick can retry.

**Local vs devcontainer dispatch.** `NeutralizeSession` dispatches by `sess.Origin`:

- Local (`origin == "" || "local"`): `killLocal` runs `pkill -f` with `regexp.QuoteMeta(sess.ProjectPath)` — no shell, pattern is matched literally. `ProjectPath` is untrusted transcript content (the session's reported cwd), so before it reaches `pkill` it must pass `isSpecificEnoughForPkill`: an absolute path at least two segments deep, which rejects the root and any single top-level directory (`/usr`, `/app`, …) that would over-match host processes. Exit code 1 (no match) is reported but not treated as an error.
- Devcontainer: `cutDevcontainer` walks `sess.FilePath` up to its `.devcontainer` anchor via `discovery.DevcontainerAnchor`, joins `cfg.DevcontainerFilterRelPath`, verifies the joined path remains under the anchor (rejects `..` traversal **and** symlink escape — the path is re-checked after `filepath.EvalSymlinks`), then prefix-comments (`# `) every uncommented line matching `cfg.AnthropicAllowPattern` in the proxy `filter.py`. The write is atomic (temp + `os.Rename`) and idempotent (already-commented lines are untouched). **There is no rollback** — the cut persists until hand-edited. `pkill -f` is similarly best-effort and leaves no undo path.

**Audit ring buffer.** Every alert, action, skip, and config-error is appended to a fixed-capacity (50-entry) `auditLog` ring buffer on the `Model`. Entries are displayed newest-first in the 4th TUI view ("Audit", key `4`). The buffer is in-memory only; it is not persisted across restarts.

**Adding a `tokens_per_min_1m` alert** requires only a config entry — no code changes.
