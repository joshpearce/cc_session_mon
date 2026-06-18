# Plan: Metric-Alert Engine (Subagent Rate + extensible to tok/m)

**Date:** 2026-06-17

## Goal

A configurable engine that alerts (and optionally acts) when a per-session live
metric crosses thresholds, evaluated on the 30s UI tick. It ships with the
subagent-count metric and is shaped so the upcoming `tok/m(1m)` alert — and
future ones — are just another registry entry plus a config line, with no new
control-flow wiring.

- **Alert (lower threshold):** console bell ("ding"), once per crossing.
- **Action (upper threshold):** neutralize the session — `pkill -f` for local
  sessions; comment the Anthropic allow-rule in the devcontainer proxy's
  `filter.py` for devcontainer sessions. No auto-rollback.

## Core abstraction

**Metric registry.** `MetricFunc func(s *Session, now time.Time) float64`, keyed
by a string name. Each metric is responsible for its own **wall-clock liveness**
so idle sessions decay rather than perma-trip:

- `active_subagents` → `AgentStats.ActiveWithin(now, RecentWindow)` (already
  decays).
- `tokens_per_min_1m` → `BurnRecent.TokensPerMinute`, but returns 0 when
  `now.Sub(BurnRecent.WindowEnd) > RecentWindow` (burn rate is anchored on the
  last action, so without this gate an idle session that burned hard would
  trigger forever).

**Rule list.** Config holds `alerts: []AlertRule`, each `{metric, alert_threshold,
action_threshold, action_sustained_ticks}`. The tick evaluates every rule against
every session via `registry[rule.Metric](sess, now)`. Thresholds are `float64`
to cover both counts and token rates.

**Scope of latches.** Action settings are *global* (the action neutralizes the
session regardless of which metric tripped), so the **action latch is per-session**
(FilePath-keyed): once a session is acted on, no rule re-fires on it. The **alert
latch and the sustained-tick streak are per (session, rule)**.

## Non-negotiable safety rails (panel consensus)

The app is read-only today; this adds its first side effects. All steps honor:
opt-in (`enable_corrective_actions: false` default), **dry-run** logs-only mode,
per-session **one-shot action latch**, the tunable `action_sustained_ticks`
(**default 1 — fires on the first over-threshold tick** per your decision),
**trusted-path gate** (act only if `FilePath` has a known projects-dir prefix),
and an **audit ring buffer** surfaced in the TUI. Untrusted transcript strings
(`ProjectPath`/CWD) are `regexp.QuoteMeta`'d before reaching `pkill`; `filter.py`'s
path is derived from the monitor-discovered directory tree, never from transcript
content.

---

## Step 1 — Config: rule list + global action settings

Add `AlertRule` (`Metric string`, `AlertThreshold float64`, `ActionThreshold
float64`, `ActionSustainedTicks int`) and `Alerts []AlertRule` to `Config`, plus
global action fields: `EnableCorrectiveActions` (false), `ActionDryRun` (false),
`DevcontainerFilterRelPath` (default rel path from the `.devcontainer` anchor,
e.g. `proxy/filter.py`), `AnthropicAllowPattern` (default regex matching the
`api.anthropic.com` allow-rule line). `DefaultConfig()` ships one rule —
`{active_subagents, 20, 40, 1}` — with actions off. Document the `alerts` block
and a commented `tokens_per_min_1m` example in CLAUDE.md.

- **Files:** `internal/config/config.go`, `CLAUDE.md`
- **Tests:** omitted keys → default rule + 20/40/1; multi-rule YAML round-trips.
- **Done when:** defaults yield one alerting rule with corrective actions off.

## Step 2 — Metric registry + rule evaluation + alert

Add `MetricFunc` and a registry (`map[string]MetricFunc`) with the two built-in
metrics above, each enforcing wall-clock liveness. In `handleTick`
(`internal/tui/update.go`), after `RefreshActiveMetrics()`, for each session ×
each rule compute `v := registry[rule.Metric](sess, now)`; skip unknown metric
names (log once). Model state keyed by `(FilePath, Metric)`: `alertLatch` and
`actionStreak`. When `v >= AlertThreshold` and not alert-latched, emit `\a` (BEL
via stderr, survives alt-screen) and append an audit entry; clear the latch when
`v` drops below. Maintain `actionStreak` (++ while `v >= ActionThreshold`, else 0).

- **Files:** `internal/tui/update.go`, `internal/tui/model.go`, new
  `internal/session/metrics.go`
- **Tests:** registry returns 0 for an idle session on both metrics; alert dings
  once per crossing per rule; an unknown metric name is skipped, not fatal.
- **Done when:** crossing 20 active subagents rings the bell once, and a stale
  high-burn session does not alert.

## Step 3 — Audit ring buffer + panel

Add a fixed-capacity (~50) ring buffer to Model holding `{ts, sessionID,
filePath, origin, metric, value, threshold, action, outcome}`. Append on every
alert and every action attempt (including dry-run / skipped). Render as a bottom
strip or 4th view in `view.go`/`delegates.go`, newest first.

- **Files:** `internal/tui/model.go`, `internal/tui/view.go`,
  `internal/tui/delegates.go`
- **Tests:** appends ordered and capped; an alert entry names the tripping metric
  and its observed value.
- **Done when:** alerts (and later, actions) are visible with metric + value.

## Step 4 — Action dispatch + local `pkill` branch

Add dispatch in `handleTick`: when any rule has `actionStreak >=
ActionSustainedTicks` (default 1 → first over-threshold tick), `v >=
ActionThreshold`, `EnableCorrectiveActions`, the session is not action-latched,
and `FilePath` is under a trusted projects root → act, then latch the session.
`ActionDryRun` logs the intended action only. **Local** origin: run
`exec.Command("pkill", "-f", regexp.QuoteMeta(projectPath))` (no shell, SIGTERM);
record outcome. Devcontainer branch is a logged "would cut" placeholder here.

- **Files:** `internal/tui/update.go`, new `internal/session/action.go`
- **Tests:** gating respects opt-in/dry-run/per-session latch/sustained-ticks/
  trusted-path; metacharacter CWD is quoted; a latched session never re-fires for
  any rule.
- **Done when:** a session tripping any action threshold triggers one best-effort
  `pkill`; dry-run logs without executing.

## Step 5 — Devcontainer `filter.py` cut branch

Implement the devcontainer branch. From the session's monitor-discovered projects
dir, walk up to the `.devcontainer` anchor, join `DevcontainerFilterRelPath`, and
verify the file resolves under that anchor (reject otherwise). Read `filter.py`,
prefix-comment (`# `) each line matching `AnthropicAllowPattern`, write atomically
(temp + `os.Rename`). Record path and original line(s) in the audit entry. No
rollback — the cut persists until hand-edited.

- **Files:** `internal/session/action.go`, `internal/discovery` (anchor helper)
- **Tests (fixture tree):** the matching line is commented, rest byte-identical;
  missing/out-of-anchor path rejected and audited as failed; already-commented
  line is idempotent.
- **Done when:** a devcontainer session tripping an action threshold has its
  Anthropic allow-rule commented, recorded in the audit panel.

---

## Definition of Done

All five step criteria met; `make test` and `make lint` pass; no regression in
the read-only path (default config alerts only — corrective actions off until
enabled). Adding the `tok/m(1m)` alert requires only an `alerts` config entry; no
code change. With actions enabled and dry-run off, a tripping session fires
exactly one latched action of the correct branch, recorded with its tripping
metric in the audit panel.

## Explicitly out of scope

PID-accurate targeting (`pkill -f` is best-effort, your decision), config
hot-reload (restart-only), and `filter.py` auto-restore (no rollback — re-enable
is a manual hand-edit). The `tokens_per_min_1m` thresholds and rule are
deliberately *not* shipped on by default; they land as a config entry once you've
calibrated them.
