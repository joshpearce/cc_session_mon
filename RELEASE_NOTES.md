# cc_session_mon v0.1.1

First usable release of the Claude Code session-monitoring TUI. Watches your
local sessions plus any nested `.claude/projects` directories (e.g. devcontainer
mounts) and surfaces live activity, token burn, and subagent fan-out. Highlights
below; the full list of merged pull requests is appended automatically.

> **Supersedes v0.1.0.** That tag's binaries reported `0.1.0-dev` from
> `--version`: the release workflow wrote the tag to a gitignored `.version`
> file, which a plain `.#` flakeref can't see, so the build silently fell back
> to a hardcoded literal. Fixed in #26 by building via `path:.`. The v0.1.0
> binaries are otherwise identical in behavior, but use these instead.

## ✨ Features

- **Live token burn-rate columns.** Two billable-tokens/min figures per session:
  `tok/m(Nm)` over a configurable trailing window (`burn_window_minutes`) and a
  fixed `tok/m(1m)`. Both are anchored on the session's last action — an idle
  session still reports how hard it was burning before going quiet — and cache
  reads are excluded so cache-heavy sessions don't read as runaways (#19).

- **Subagent fan-out metrics.** A `peak/active` `agents` column showing the
  lifetime peak of overlapping subagent spans and the count still active in the
  trailing minute, computed across the whole transcript subtree off the render
  lock (#19).

- **Metric-threshold alert engine.** Configurable `alerts` rules over named live
  metrics (`active_subagents`, `tokens_per_min_1m`) with a two-tier design: an
  alert tier that rings the console bell once per crossing, and a gated
  corrective-action tier (kill local / cut devcontainer proxy allow-rule) that
  is read-only until you opt in via `enable_corrective_actions`, with dry-run,
  trusted-path, and sustained-streak safety rails. Every alert, action, and skip
  lands in an in-app Audit view (#20).

- **Recursive search-path discovery.** Replaces the old `--follow-devagent`
  flag: configured `search_paths` are scanned at startup for nested
  `.claude/projects` directories, each watched with a derived origin label (#18).

- **Command search/filter.** Filter the Commands tab by tool or pattern (#15).

## 🛠 Build & CI

- Reproducible Nix builds across linux-amd64 and darwin-arm64, with the version
  injected from the release tag into the binary (`cc_session_mon --version`),
  verified end-to-end (#26).
- golangci-lint v2, Go 1.25, and automatic SRI-hash regeneration for Dependabot
  bumps.
