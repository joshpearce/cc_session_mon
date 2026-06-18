package tui

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"cc_session_mon/internal/config"
	"cc_session_mon/internal/session"

	"github.com/charmbracelet/bubbles/list"
)

// TestAuditLog_AppendCapsAtCapacity verifies that the ring buffer never grows
// beyond auditCapacity and drops the oldest entries when it overflows.
func TestAuditLog_AppendCapsAtCapacity(t *testing.T) {
	t.Parallel() // safe: no global state

	a := &auditLog{}
	total := auditCapacity + 10

	for i := range total {
		a.append(AuditEntry{Value: float64(i)})
	}

	// Buffer must not exceed the cap.
	if len(a.entries) != auditCapacity {
		t.Fatalf("expected %d entries, got %d", auditCapacity, len(a.entries))
	}

	// The oldest 10 entries (Value 0..9) must have been dropped.
	// entries[0] should now be Value==10 (the 11th appended).
	wantFirst := float64(10)
	if a.entries[0].Value != wantFirst {
		t.Errorf("expected first retained entry Value=%v, got %v", wantFirst, a.entries[0].Value)
	}

	// The last retained entry must be the very last one appended.
	wantLast := float64(total - 1)
	if a.entries[len(a.entries)-1].Value != wantLast {
		t.Errorf("expected last retained entry Value=%v, got %v", wantLast, a.entries[len(a.entries)-1].Value)
	}

	// entries must remain in chronological (oldest-first) order.
	for i := 1; i < len(a.entries); i++ {
		if a.entries[i].Value <= a.entries[i-1].Value {
			t.Errorf("entries not in ascending order at index %d: %v then %v",
				i, a.entries[i-1].Value, a.entries[i].Value)
		}
	}

	// recent() must return them newest-first: first element == last appended.
	r := a.recent()
	if r[0].Value != wantLast {
		t.Errorf("recent()[0] should be newest (Value=%v), got %v", wantLast, r[0].Value)
	}
	if r[len(r)-1].Value != wantFirst {
		t.Errorf("recent()[last] should be oldest retained (Value=%v), got %v", wantFirst, r[len(r)-1].Value)
	}
}

// TestAuditLog_RecentOnEmpty verifies that recent() on a zero-entry log
// returns an empty slice without panicking.
func TestAuditLog_RecentOnEmpty(t *testing.T) {
	t.Parallel()

	a := &auditLog{}
	got := a.recent()

	if len(got) != 0 {
		t.Fatalf("expected empty slice from recent() on empty log, got %d entries", len(got))
	}
}

// TestAlertAuditEntry_ContainsMetricValueAndThreshold verifies the Step-3
// "Done when" criterion at the data level: when evaluateAlerts fires an alert,
// the resulting AuditEntry records the tripping metric, its observed value, and
// the configured threshold.
func TestAlertAuditEntry_ContainsMetricValueAndThreshold(t *testing.T) {
	// No t.Parallel(): mutates process-wide config global.
	const alertThreshold = 20.0
	prev := config.Global()
	config.SetGlobal(&config.Config{
		Alerts: []config.AlertRule{
			{
				Metric:               "active_subagents",
				AlertThreshold:       alertThreshold,
				ActionThreshold:      100,
				ActionSustainedTicks: 1,
			},
		},
	})
	t.Cleanup(func() { config.SetGlobal(prev) })

	const activeAgents = 25
	bells := 0
	m := makeAlertModel(&bells)
	now := time.Now()
	sess := sessionWithActiveAgents("/tmp/audit-content.jsonl", activeAgents, now)

	m = m.evaluateAlerts([]*session.Session{sess}, now)

	if len(m.audit.entries) == 0 {
		t.Fatal("expected at least one audit entry after alert, got none")
	}

	// Find the alert entry (there may also be non-alert entries in theory, so
	// search rather than assume index 0).
	var alertEntry *AuditEntry
	for i := range m.audit.entries {
		if m.audit.entries[i].Action == "alert" {
			alertEntry = &m.audit.entries[i]
			break
		}
	}
	if alertEntry == nil {
		t.Fatal("expected an entry with Action=='alert', found none")
	}

	if alertEntry.Metric != "active_subagents" {
		t.Errorf("Metric: want %q, got %q", "active_subagents", alertEntry.Metric)
	}
	if alertEntry.Value != activeAgents {
		t.Errorf("Value: want %v, got %v", float64(activeAgents), alertEntry.Value)
	}
	if alertEntry.Threshold != alertThreshold {
		t.Errorf("Threshold: want %v, got %v", alertThreshold, alertEntry.Threshold)
	}
}

// TestUpdateAuditList_ItemCountAndRenderSmoke verifies that updateAuditList
// populates auditList with the correct number of items and that calling the
// delegate's Render does not panic and produces output containing the metric
// name and formatted value.
func TestUpdateAuditList_ItemCountAndRenderSmoke(t *testing.T) {
	t.Parallel() // safe: no global state

	// Build a bare model with the audit list initialised but no watcher.
	auditDel := newAuditDelegate()
	auditDel.SetWidth(80)
	m := Model{
		audit:         &auditLog{},
		auditDelegate: auditDel,
		auditList:     list.New([]list.Item{}, auditDel, 80, 20),
	}
	m.auditList.SetShowTitle(false)
	m.auditList.SetShowHelp(false)
	m.auditList.SetShowStatusBar(false)
	m.auditList.SetFilteringEnabled(false)
	m.auditList.DisableQuitKeybindings()

	// Append two entries with distinct metrics.
	now := time.Now()
	m.audit.append(AuditEntry{
		Time:      now,
		Metric:    "active_subagents",
		Value:     25,
		Threshold: 20,
		Action:    "alert",
		Outcome:   "bell",
	})
	m.audit.append(AuditEntry{
		Time:      now.Add(time.Second),
		Metric:    "tokens_per_min_1m",
		Value:     5000,
		Threshold: 3000,
		Action:    "alert",
		Outcome:   "bell",
	})

	m = m.updateAuditList()

	// Item count must match entry count.
	items := m.auditList.Items()
	if len(items) != 2 {
		t.Fatalf("expected 2 items in auditList, got %d", len(items))
	}

	// auditItem.Title() and Description() must surface metric and outcome.
	ai, ok := items[0].(auditItem)
	if !ok {
		t.Fatalf("expected auditItem, got %T", items[0])
	}
	if ai.Title() != "tokens_per_min_1m" {
		// recent() returns newest-first, so index 0 is the second-appended entry.
		t.Errorf("Title(): want %q, got %q", "tokens_per_min_1m", ai.Title())
	}
	if ai.Description() != "bell" {
		t.Errorf("Description(): want %q, got %q", "bell", ai.Description())
	}

	// Delegate Render must not panic and must include the metric name.
	var buf bytes.Buffer
	lm := list.New(items, auditDel, 80, 20)
	auditDel.Render(&buf, lm, 0, items[0])
	rendered := buf.String()
	if rendered == "" {
		t.Fatal("Render produced empty output")
	}
	// The metric name is truncated to AuditMetricWidth by the delegate;
	// assert on the prefix that will always appear regardless of truncation.
	metricPrefix := "tokens_per_min"
	if !bytes.Contains(buf.Bytes(), []byte(metricPrefix)) {
		t.Errorf("rendered row does not contain metric prefix %q; got: %s", metricPrefix, rendered)
	}

	// The formatted value for 5000 should be "5.0k".
	wantVal := fmt.Sprintf("%s/%s", formatAuditValue(5000), formatAuditValue(3000))
	if !bytes.Contains(buf.Bytes(), []byte(wantVal)) {
		t.Errorf("rendered row does not contain %q; got: %s", wantVal, rendered)
	}
}
