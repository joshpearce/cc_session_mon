package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cc_session_mon/internal/config"
	"cc_session_mon/internal/session"
)

// actionCfg returns a config suitable for action-gate tests. EnableCorrectiveActions
// and ActionDryRun are always true here so no real process is ever signalled; tests
// that need the flag off pass their own config.
func actionCfg(sustained int) *config.Config {
	return &config.Config{
		EnableCorrectiveActions: true,
		ActionDryRun:            true,
		Alerts: []config.AlertRule{
			{
				Metric:               "active_subagents",
				AlertThreshold:       20,
				ActionThreshold:      40,
				ActionSustainedTicks: sustained,
			},
		},
	}
}

// trustedSession returns a Session whose FilePath lives under /trusted, with the
// given number of active subagents at now.
func trustedSession(id string, n int, now time.Time) *session.Session {
	s := sessionWithActiveAgents("/trusted/p/"+id+".jsonl", n, now)
	s.Origin = "local"
	s.ProjectPath = "/trusted/p"
	return s
}

// countActionEntries counts audit entries whose Action matches any of the given
// action strings (dry-run, pkill, skipped, failed).
func countActionEntries(entries []AuditEntry, actions ...string) int {
	set := make(map[string]bool, len(actions))
	for _, a := range actions {
		set[a] = true
	}
	count := 0
	for i := range entries {
		if set[entries[i].Action] {
			count++
		}
	}
	return count
}

// TestCorrectiveAction_OptInRespected verifies that when EnableCorrectiveActions
// is false no corrective action entry appears in the audit log, even when the
// session is well over the action threshold.
func TestCorrectiveAction_OptInRespected(t *testing.T) {
	// No t.Parallel(): mutates process-wide config global.
	prev := config.Global()
	cfg := actionCfg(1)
	cfg.EnableCorrectiveActions = false // override: opt-in is OFF
	config.SetGlobal(cfg)
	t.Cleanup(func() { config.SetGlobal(prev) })

	bells := 0
	m := makeActionModel(&bells, []string{"/trusted"})
	now := time.Now()
	sess := trustedSession("s1", 45, now)

	m = m.evaluateAlerts([]*session.Session{sess}, now)

	// May have an alert entry but must have zero action entries.
	if got := countActionEntries(m.audit.entries, "dry-run", "pkill", "skipped", "failed"); got != 0 {
		t.Fatalf("expected 0 action entries when opt-in is off, got %d; entries: %+v",
			got, m.audit.entries)
	}
	if m.actionLatch[sess.FilePath] {
		t.Fatal("session should not be action-latched when opt-in is off")
	}
}

// TestCorrectiveAction_SustainedTicksGate verifies that the action does not fire
// until actionStreak reaches ActionSustainedTicks.
//
// With ActionSustainedTicks==2:
//   - Tick 1: streak becomes 1 → no action entry, not latched.
//   - Tick 2: streak becomes 2 → exactly one "dry-run" entry, session latched.
func TestCorrectiveAction_SustainedTicksGate(t *testing.T) {
	// No t.Parallel(): mutates process-wide config global.
	prev := config.Global()
	config.SetGlobal(actionCfg(2)) // sustained=2
	t.Cleanup(func() { config.SetGlobal(prev) })

	bells := 0
	m := makeActionModel(&bells, []string{"/trusted"})
	now := time.Now()
	sess := trustedSession("s2", 45, now)

	// Tick 1: streak reaches 1 — not yet at ActionSustainedTicks.
	m = m.evaluateAlerts([]*session.Session{sess}, now)
	if got := countActionEntries(m.audit.entries, "dry-run", "pkill", "skipped", "failed"); got != 0 {
		t.Fatalf("tick 1: expected 0 action entries, got %d; entries: %+v",
			got, m.audit.entries)
	}
	if m.actionLatch[sess.FilePath] {
		t.Fatal("tick 1: session must not be latched yet")
	}
	key := alertKey{filePath: sess.FilePath, metric: "active_subagents"}
	if m.actionStreak[key] != 1 {
		t.Fatalf("tick 1: expected actionStreak=1, got %d", m.actionStreak[key])
	}

	// Tick 2: streak reaches 2 — action fires.
	m = m.evaluateAlerts([]*session.Session{sess}, now)
	if got := countActionEntries(m.audit.entries, "dry-run"); got != 1 {
		t.Fatalf("tick 2: expected exactly 1 dry-run entry, got %d; entries: %+v",
			got, m.audit.entries)
	}
	if !m.actionLatch[sess.FilePath] {
		t.Fatal("tick 2: session must be latched after action fires")
	}
}

// TestCorrectiveAction_FiresOnFirstTickWhenSustainedOne verifies that with
// ActionSustainedTicks==1 the very first over-threshold evaluate produces a
// dry-run audit entry.
func TestCorrectiveAction_FiresOnFirstTickWhenSustainedOne(t *testing.T) {
	// No t.Parallel(): mutates process-wide config global.
	prev := config.Global()
	config.SetGlobal(actionCfg(1))
	t.Cleanup(func() { config.SetGlobal(prev) })

	bells := 0
	m := makeActionModel(&bells, []string{"/trusted"})
	now := time.Now()
	sess := trustedSession("s3", 45, now)

	m = m.evaluateAlerts([]*session.Session{sess}, now)

	if got := countActionEntries(m.audit.entries, "dry-run"); got != 1 {
		t.Fatalf("expected exactly 1 dry-run action entry on first tick, got %d; entries: %+v",
			got, m.audit.entries)
	}
}

// TestCorrectiveAction_PerSessionLatch verifies that the action latch is per
// session: after the first action fires for a session, subsequent evaluateAlerts
// calls never append another action entry for that same FilePath, even if another
// rule would also trip.
func TestCorrectiveAction_PerSessionLatch(t *testing.T) {
	// No t.Parallel(): mutates process-wide config global.
	prev := config.Global()
	// Two rules, both will trip at 45 active subagents.
	config.SetGlobal(&config.Config{
		EnableCorrectiveActions: true,
		ActionDryRun:            true,
		Alerts: []config.AlertRule{
			{
				Metric:               "active_subagents",
				AlertThreshold:       20,
				ActionThreshold:      40,
				ActionSustainedTicks: 1,
			},
			{
				// A second rule on the same metric with a lower threshold so it
				// also trips on the first tick.
				Metric:               "active_subagents",
				AlertThreshold:       10,
				ActionThreshold:      30,
				ActionSustainedTicks: 1,
			},
		},
	})
	t.Cleanup(func() { config.SetGlobal(prev) })

	bells := 0
	m := makeActionModel(&bells, []string{"/trusted"})
	now := time.Now()
	sess := trustedSession("s4", 45, now)

	// First evaluate: both rules trip but the per-session latch must ensure at
	// most one action entry for this FilePath.
	m = m.evaluateAlerts([]*session.Session{sess}, now)
	firstCount := countActionEntries(m.audit.entries, "dry-run", "pkill", "skipped", "failed")
	if firstCount != 1 {
		t.Fatalf("expected exactly 1 action entry after first evaluate, got %d; entries: %+v",
			firstCount, m.audit.entries)
	}
	if !m.actionLatch[sess.FilePath] {
		t.Fatal("session must be latched after first action fires")
	}

	// Second evaluate: session is latched — no additional action entries.
	m = m.evaluateAlerts([]*session.Session{sess}, now)
	secondCount := countActionEntries(m.audit.entries, "dry-run", "pkill", "skipped", "failed")
	if secondCount != firstCount {
		t.Fatalf("expected action entry count to remain %d after second evaluate (latched), got %d; entries: %+v",
			firstCount, secondCount, m.audit.entries)
	}
}

// TestZeroThresholdRuleNeverActsOrAlerts verifies that a rule whose
// AlertThreshold and ActionThreshold are both zero (non-positive) is entirely
// inert: it never rings the bell, never appends an alert or action audit entry,
// and never sets actionLatch — regardless of the observed metric value.
//
// Note: no t.Parallel() because this test mutates the process-wide config global.
func TestZeroThresholdRuleNeverActsOrAlerts(t *testing.T) {
	prev := config.Global()
	config.SetGlobal(&config.Config{
		EnableCorrectiveActions: true,
		ActionDryRun:            true,
		Alerts: []config.AlertRule{
			{
				Metric:               "active_subagents",
				AlertThreshold:       0,
				ActionThreshold:      0,
				ActionSustainedTicks: 1,
			},
		},
	})
	t.Cleanup(func() { config.SetGlobal(prev) })

	bells := 0
	m := makeActionModel(&bells, []string{"/trusted"})
	now := time.Now()

	// Active session — 5 subagents; v >= 0 would be true for any metric value,
	// so without the guard this would ding and act on every session.
	active := trustedSession("zero-active", 5, now)

	// Idle session — 0 subagents; should also be completely inert.
	idle := trustedSession("zero-idle", 0, now)

	m = m.evaluateAlerts([]*session.Session{active, idle}, now)

	if bells != 0 {
		t.Fatalf("expected 0 bells for zero-threshold rule, got %d", bells)
	}

	for _, e := range m.audit.entries {
		if e.Action == "alert" {
			t.Errorf("unexpected alert audit entry: %+v", e)
		}
		if e.Action == "dry-run" || e.Action == "pkill" || e.Action == "skipped" || e.Action == "failed" {
			t.Errorf("unexpected action audit entry: %+v", e)
		}
	}

	if len(m.audit.entries) != 0 {
		t.Fatalf("expected 0 audit entries for zero-threshold rule, got %d: %+v",
			len(m.audit.entries), m.audit.entries)
	}

	if m.actionLatch[active.FilePath] {
		t.Errorf("active session must not be action-latched by a zero-threshold rule")
	}
	if m.actionLatch[idle.FilePath] {
		t.Errorf("idle session must not be action-latched by a zero-threshold rule")
	}
}

// TestCorrectiveAction_UntrustedPathGate verifies that when a session's FilePath
// is not under any trusted root the action entry has Action=="skipped" and an
// Outcome containing "untrusted path", and that NeutralizeSession/pkill is never
// invoked (the skipped branch fires before reaching it).
func TestCorrectiveAction_UntrustedPathGate(t *testing.T) {
	// No t.Parallel(): mutates process-wide config global.
	prev := config.Global()
	config.SetGlobal(actionCfg(1))
	t.Cleanup(func() { config.SetGlobal(prev) })

	bells := 0
	// trustedRoots is /trusted; the session is under /untrusted.
	m := makeActionModel(&bells, []string{"/trusted"})
	now := time.Now()

	// Build a session over the action threshold with an untrusted path.
	untrustedSess := sessionWithActiveAgents("/untrusted/p/x.jsonl", 45, now)
	untrustedSess.Origin = "local"
	untrustedSess.ProjectPath = "/untrusted/p"

	m = m.evaluateAlerts([]*session.Session{untrustedSess}, now)

	// Should have at least one action entry.
	actionEntries := make([]AuditEntry, 0)
	for _, e := range m.audit.entries {
		if e.Action == "skipped" || e.Action == "dry-run" || e.Action == "pkill" || e.Action == "failed" {
			actionEntries = append(actionEntries, e)
		}
	}
	if len(actionEntries) != 1 {
		t.Fatalf("expected exactly 1 action entry for untrusted path, got %d; entries: %+v",
			len(actionEntries), m.audit.entries)
	}

	entry := actionEntries[0]
	if entry.Action != "skipped" {
		t.Fatalf("expected Action==%q for untrusted path, got %q", "skipped", entry.Action)
	}
	if !strings.Contains(entry.Outcome, "untrusted path") {
		t.Fatalf("expected Outcome to mention %q, got %q", "untrusted path", entry.Outcome)
	}

	// Session must be latched so it cannot re-fire.
	if !m.actionLatch[untrustedSess.FilePath] {
		t.Fatal("untrusted-path session must be latched after skipped action to prevent re-fire")
	}
}

// filterPyContent is the representative proxy allow-list used in Part C.
const filterPyContent = `# proxy allow list
ALLOW = [
    "api.anthropic.com",
    "github.com",
]
`

// buildDevcontainerIntegrationFixture creates the devcontainer layout under
// t.TempDir() and returns a *session.Session and the absolute path to filter.py.
//
// Layout mirrors what the monitor discovers at runtime:
//
//	<tmp>/repo/.devcontainer/proxy/filter.py          ← filter file
//	<tmp>/repo/.devcontainer/containers/app/.claude/projects/enc/uuid.jsonl ← session
//
// The session FilePath is intentionally placed under the "enc" projects dir so
// the trusted-path gate passes when trustedRoots includes that dir.
func buildDevcontainerIntegrationFixture(t *testing.T) (sess *session.Session, filterPath, projectsDir string) {
	t.Helper()
	tmp := t.TempDir()

	anchor := filepath.Join(tmp, "repo", ".devcontainer")
	projectsDir = filepath.Join(anchor, "containers", "app", ".claude", "projects", "enc")
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll projectsDir: %v", err)
	}

	filterDir := filepath.Join(anchor, "proxy")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatalf("MkdirAll filterDir: %v", err)
	}
	filterPath = filepath.Join(filterDir, "filter.py")
	if err := os.WriteFile(filterPath, []byte(filterPyContent), 0o644); err != nil {
		t.Fatalf("WriteFile filter.py: %v", err)
	}

	sessionFilePath := filepath.Join(projectsDir, "abc123.jsonl")
	now := time.Now()
	// Build a session over the action threshold (45 agents) with non-local origin.
	lastSeen := make([]time.Time, 45)
	for i := range lastSeen {
		lastSeen[i] = now
	}
	sess = &session.Session{
		ID:       "integration-devcontainer-session",
		FilePath: sessionFilePath,
		Origin:   "repo/app",
		AgentStats: session.AgentMetrics{
			TotalAgents:   45,
			MaxConcurrent: 45,
			LastSeen:      lastSeen,
		},
	}
	return
}

// TestCorrectiveAction_DevcontainerFilterCutRecorded is an end-to-end integration
// test: it verifies that when a devcontainer session trips the action threshold
// the TUI alert engine (evaluateAlerts) calls NeutralizeSession, which comments
// the allow-rule line in the fixture filter.py, and records an audit entry with
// Action=="filter-cut".
//
// This exercises the Step-5 "Done when" criterion: a devcontainer session
// tripping an action threshold has its Anthropic allow-rule commented and the
// result is recorded in the audit panel.
//
// No t.Parallel(): mutates the process-wide config global.
func TestCorrectiveAction_DevcontainerFilterCutRecorded(t *testing.T) {
	// Restore process-wide config on exit.
	prev := config.Global()
	t.Cleanup(func() { config.SetGlobal(prev) })

	sess, filterPath, projectsDir := buildDevcontainerIntegrationFixture(t)

	cfg := &config.Config{
		EnableCorrectiveActions:   true,
		ActionDryRun:              false,
		DevcontainerFilterRelPath: "proxy/filter.py",
		AnthropicAllowPattern:     `api\.anthropic\.com`,
		Alerts: []config.AlertRule{
			{
				Metric:               "active_subagents",
				AlertThreshold:       20,
				ActionThreshold:      40,
				ActionSustainedTicks: 1,
			},
		},
	}
	config.SetGlobal(cfg)

	bells := 0
	// trustedRoots must include a prefix of sess.FilePath so the trusted-path gate passes.
	m := makeActionModel(&bells, []string{projectsDir})
	now := time.Now()

	m = m.evaluateAlerts([]*session.Session{sess}, now)

	// Assert: an audit entry with Action=="filter-cut" exists.
	var filterCutEntry *AuditEntry
	for i := range m.audit.entries {
		if m.audit.entries[i].Action == "filter-cut" {
			e := m.audit.entries[i]
			filterCutEntry = &e
			break
		}
	}
	if filterCutEntry == nil {
		t.Fatalf("expected an audit entry with Action==%q, but none found; entries: %+v",
			"filter-cut", m.audit.entries)
	}
	if filterCutEntry.FilePath != sess.FilePath {
		t.Errorf("audit entry FilePath: want %q, got %q", sess.FilePath, filterCutEntry.FilePath)
	}
	if filterCutEntry.Origin != sess.Origin {
		t.Errorf("audit entry Origin: want %q, got %q", sess.Origin, filterCutEntry.Origin)
	}
	if filterCutEntry.Metric != "active_subagents" {
		t.Errorf("audit entry Metric: want %q, got %q", "active_subagents", filterCutEntry.Metric)
	}

	// Assert: the allow-rule line in filter.py is now commented.
	got, err := os.ReadFile(filterPath) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("ReadFile filter.py after evaluateAlerts: %v", err)
	}
	gotStr := string(got)

	// The allow line must now start with "# ".
	foundCommented := false
	for _, line := range strings.Split(gotStr, "\n") {
		if strings.HasPrefix(line, "# ") && strings.Contains(line, "api.anthropic.com") {
			foundCommented = true
			break
		}
	}
	if !foundCommented {
		t.Errorf("expected allow-rule line to be commented in filter.py after filter-cut;\ngot content:\n%s", gotStr)
	}

	// The plain (uncommented) allow line must no longer appear.
	for _, line := range strings.Split(gotStr, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.Contains(trimmed, "api.anthropic.com") && !strings.HasPrefix(trimmed, "#") {
			t.Errorf("uncommented allow-rule line still present after filter-cut: %q", line)
		}
	}

	// Session must be action-latched after the cut.
	if !m.actionLatch[sess.FilePath] {
		t.Fatal("session must be action-latched after filter-cut")
	}
}
