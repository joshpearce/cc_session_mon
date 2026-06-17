package tui

import (
	"testing"
	"time"

	"cc_session_mon/internal/config"
	"cc_session_mon/internal/session"
)

// makeAlertModel builds a bare Model with only the alert-engine fields
// initialized. It does NOT call NewModel so no filesystem watcher is created.
func makeAlertModel(bells *int) Model {
	return Model{
		alertLatch:    map[alertKey]bool{},
		actionStreak:  map[alertKey]int{},
		warnedMetrics: map[string]bool{},
		audit:         &auditLog{},
		bell:          func() { *bells++ },
	}
}

// sessionWithActiveAgents builds a Session whose AgentStats.LastSeen contains
// n timestamps at now, making ActiveWithin(now, RecentWindow) == n.
func sessionWithActiveAgents(filePath string, n int, now time.Time) *session.Session {
	lastSeen := make([]time.Time, n)
	for i := range lastSeen {
		lastSeen[i] = now
	}
	return &session.Session{
		ID:       "test-session-" + filePath,
		FilePath: filePath,
		AgentStats: session.AgentMetrics{
			TotalAgents:   n,
			MaxConcurrent: n,
			LastSeen:      lastSeen,
		},
	}
}

// sessionWithStaleActivity builds a Session with n subagents all last-seen
// well before RecentWindow (idle) and an also-stale BurnRecent.
func sessionWithStaleActivity(filePath string, n int, now time.Time) *session.Session {
	stale := now.Add(-session.RecentWindow - time.Minute)
	lastSeen := make([]time.Time, n)
	for i := range lastSeen {
		lastSeen[i] = stale
	}
	return &session.Session{
		ID:       "idle-session-" + filePath,
		FilePath: filePath,
		AgentStats: session.AgentMetrics{
			TotalAgents:   n,
			MaxConcurrent: n,
			LastSeen:      lastSeen,
		},
		BurnRecent: session.BurnRateResult{
			WindowEnd:       stale,
			TokensPerMinute: 99999, // high but stale → gated to 0
		},
	}
}

// withSubagentAlertConfig sets the global config to a single active_subagents
// rule with the given thresholds and registers a cleanup to restore the previous
// global config.
func withSubagentAlertConfig(t *testing.T, alert, action float64) {
	t.Helper()
	prev := config.Global()
	config.SetGlobal(&config.Config{
		Alerts: []config.AlertRule{
			{
				Metric:               "active_subagents",
				AlertThreshold:       alert,
				ActionThreshold:      action,
				ActionSustainedTicks: 1,
			},
		},
	})
	t.Cleanup(func() { config.SetGlobal(prev) })
}

// TestEvaluateAlerts_DingsOncePerCrossing verifies the ding-once (latch)
// semantics: bell fires on the first crossing, stays silent while the metric
// remains above the threshold, then re-fires after it dips below and comes back.
func TestEvaluateAlerts_DingsOncePerCrossing(t *testing.T) {
	// No t.Parallel(): this test mutates the process-wide config global.
	withSubagentAlertConfig(t, 20, 40)

	bells := 0
	m := makeAlertModel(&bells)
	now := time.Now()
	sess := sessionWithActiveAgents("/tmp/s1.jsonl", 25, now)

	// First evaluate: metric is 25 ≥ 20 → one bell.
	m = m.evaluateAlerts([]*session.Session{sess}, now)
	if bells != 1 {
		t.Fatalf("expected 1 bell after first crossing, got %d", bells)
	}

	// Audit entry should record the alert.
	if len(m.audit.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(m.audit.entries))
	}
	entry := m.audit.entries[0]
	if entry.Action != "alert" {
		t.Errorf("expected action 'alert', got %q", entry.Action)
	}
	if entry.Metric != "active_subagents" {
		t.Errorf("expected metric 'active_subagents', got %q", entry.Metric)
	}
	if entry.Value != 25 {
		t.Errorf("expected value 25, got %v", entry.Value)
	}

	// Second evaluate with same value: latched → no new bell.
	m = m.evaluateAlerts([]*session.Session{sess}, now)
	if bells != 1 {
		t.Fatalf("expected bell count to remain 1 (latched), got %d", bells)
	}
	if len(m.audit.entries) != 1 {
		t.Fatalf("expected audit entries to remain 1, got %d", len(m.audit.entries))
	}

	// Drop below threshold: latch clears.
	sessIdle := sessionWithActiveAgents("/tmp/s1.jsonl", 0, now)
	m = m.evaluateAlerts([]*session.Session{sessIdle}, now)
	if bells != 1 {
		t.Fatalf("expected no bell while below threshold, got %d total", bells)
	}

	// Re-cross: latch was cleared → bell fires again.
	m = m.evaluateAlerts([]*session.Session{sess}, now)
	if bells != 2 {
		t.Fatalf("expected 2 bells after second crossing, got %d", bells)
	}
	if len(m.audit.entries) != 2 {
		t.Fatalf("expected 2 audit entries after second crossing, got %d", len(m.audit.entries))
	}
}

// TestEvaluateAlerts_IdleSessionDoesNotAlert verifies that a session with
// all-stale subagents (decayed to 0 via RecentWindow) and a stale BurnRecent
// never triggers the bell, even when the raw values would be above threshold.
func TestEvaluateAlerts_IdleSessionDoesNotAlert(t *testing.T) {
	// No t.Parallel(): this test mutates the process-wide config global.
	prev := config.Global()
	// Two rules: subagents (threshold 20) and tokens_per_min_1m (low threshold 1)
	// so even a modest stale value would trip if the liveness gate were absent.
	config.SetGlobal(&config.Config{
		Alerts: []config.AlertRule{
			{
				Metric:               "active_subagents",
				AlertThreshold:       20,
				ActionThreshold:      40,
				ActionSustainedTicks: 1,
			},
			{
				Metric:               "tokens_per_min_1m",
				AlertThreshold:       1,
				ActionThreshold:      500,
				ActionSustainedTicks: 1,
			},
		},
	})
	t.Cleanup(func() { config.SetGlobal(prev) })

	bells := 0
	m := makeAlertModel(&bells)
	now := time.Now()
	// 30 stale subagents + 99999 stale tok/m → both metrics decay to 0.
	sess := sessionWithStaleActivity("/tmp/s2.jsonl", 30, now)

	m = m.evaluateAlerts([]*session.Session{sess}, now)

	if bells != 0 {
		t.Fatalf("expected 0 bells for idle session, got %d", bells)
	}
	for _, e := range m.audit.entries {
		if e.Action == "alert" {
			t.Fatalf("expected no alert audit entries for idle session, got: %+v", e)
		}
	}
}

// TestEvaluateAlerts_UnknownMetricSkipped verifies that an unknown metric name
// in the config does not panic, does not ring the bell, and records exactly one
// audit entry (log-once) with an outcome indicating the unknown metric.
func TestEvaluateAlerts_UnknownMetricSkipped(t *testing.T) {
	// No t.Parallel(): this test mutates the process-wide config global.
	prev := config.Global()
	config.SetGlobal(&config.Config{
		Alerts: []config.AlertRule{
			{
				Metric:               "bogus_metric",
				AlertThreshold:       1,
				ActionThreshold:      10,
				ActionSustainedTicks: 1,
			},
		},
	})
	t.Cleanup(func() { config.SetGlobal(prev) })

	bells := 0
	m := makeAlertModel(&bells)
	now := time.Now()
	sess := &session.Session{
		ID:       "sess-bogus",
		FilePath: "/tmp/s3.jsonl",
	}

	// First evaluate: should record one config-error audit entry and not ring.
	m = m.evaluateAlerts([]*session.Session{sess}, now)

	if bells != 0 {
		t.Fatalf("expected 0 bells for unknown metric, got %d", bells)
	}
	if len(m.audit.entries) != 1 {
		t.Fatalf("expected exactly 1 audit entry for unknown metric, got %d", len(m.audit.entries))
	}
	e := m.audit.entries[0]
	if e.Metric != "bogus_metric" {
		t.Errorf("audit entry metric: want 'bogus_metric', got %q", e.Metric)
	}
	if e.Outcome != "unknown metric" {
		t.Errorf("audit entry outcome: want 'unknown metric', got %q", e.Outcome)
	}

	// Second evaluate: log-once guard → no new audit entry.
	m = m.evaluateAlerts([]*session.Session{sess}, now)

	if len(m.audit.entries) != 1 {
		t.Fatalf("expected audit entries to remain 1 (log-once), got %d", len(m.audit.entries))
	}
	if bells != 0 {
		t.Fatalf("still expected 0 bells, got %d", bells)
	}
}

// TestEvaluateAlerts_ActionStreakMaintained verifies that actionStreak
// increments while the metric is ≥ ActionThreshold and resets to 0 when it
// drops below.
func TestEvaluateAlerts_ActionStreakMaintained(t *testing.T) {
	// No t.Parallel(): this test mutates the process-wide config global.
	withSubagentAlertConfig(t, 20, 40)

	bells := 0
	m := makeAlertModel(&bells)
	now := time.Now()
	key := alertKey{filePath: "/tmp/s4.jsonl", metric: "active_subagents"}

	// 45 agents ≥ ActionThreshold (40) → streak should increment.
	sessHigh := sessionWithActiveAgents("/tmp/s4.jsonl", 45, now)
	m = m.evaluateAlerts([]*session.Session{sessHigh}, now)
	if m.actionStreak[key] != 1 {
		t.Fatalf("expected actionStreak=1 after first over-action-threshold tick, got %d", m.actionStreak[key])
	}

	m = m.evaluateAlerts([]*session.Session{sessHigh}, now)
	if m.actionStreak[key] != 2 {
		t.Fatalf("expected actionStreak=2 after second tick, got %d", m.actionStreak[key])
	}

	// Drop to 25 (< ActionThreshold but ≥ AlertThreshold) → streak resets to 0.
	sessMid := sessionWithActiveAgents("/tmp/s4.jsonl", 25, now)
	m = m.evaluateAlerts([]*session.Session{sessMid}, now)
	if m.actionStreak[key] != 0 {
		t.Fatalf("expected actionStreak=0 after dropping below action threshold, got %d", m.actionStreak[key])
	}
}

// TestEvaluateAlerts_MultipleSessionsIndependent verifies that alert latches
// are per-(session, rule): two sessions each crossing the threshold both fire
// their own bell, independently of each other.
func TestEvaluateAlerts_MultipleSessionsIndependent(t *testing.T) {
	// No t.Parallel(): this test mutates the process-wide config global.
	withSubagentAlertConfig(t, 20, 40)

	bells := 0
	m := makeAlertModel(&bells)
	now := time.Now()
	s1 := sessionWithActiveAgents("/tmp/m1.jsonl", 25, now)
	s2 := sessionWithActiveAgents("/tmp/m2.jsonl", 30, now)

	m = m.evaluateAlerts([]*session.Session{s1, s2}, now)

	if bells != 2 {
		t.Fatalf("expected 2 bells (one per session), got %d", bells)
	}
	if len(m.audit.entries) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(m.audit.entries))
	}

	// Re-evaluate with same sessions → still latched, no new bells.
	m = m.evaluateAlerts([]*session.Session{s1, s2}, now)
	if bells != 2 {
		t.Fatalf("expected bells to remain 2 (latched), got %d", bells)
	}
}

// TestAuditLog_Recent verifies that recent() returns entries newest-first.
func TestAuditLog_Recent(t *testing.T) {
	t.Parallel() // safe: no global state

	a := &auditLog{}
	base := time.Now()
	a.append(AuditEntry{Time: base, Action: "first"})
	a.append(AuditEntry{Time: base.Add(time.Second), Action: "second"})
	a.append(AuditEntry{Time: base.Add(2 * time.Second), Action: "third"})

	got := a.recent()
	if len(got) != 3 {
		t.Fatalf("expected 3 entries from recent(), got %d", len(got))
	}
	if got[0].Action != "third" {
		t.Errorf("expected newest entry first; got action %q", got[0].Action)
	}
	if got[2].Action != "first" {
		t.Errorf("expected oldest entry last; got action %q", got[2].Action)
	}
}
