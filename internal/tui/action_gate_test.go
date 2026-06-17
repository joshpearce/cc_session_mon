package tui

import (
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
