package tui

import "time"

// auditCapacity bounds the retained audit entries (newest kept).
const auditCapacity = 50

// AuditEntry records one alert or action attempt for the audit panel.
type AuditEntry struct {
	Time      time.Time
	SessionID string
	FilePath  string
	Origin    string
	Metric    string
	Value     float64
	Threshold float64
	Action    string // "alert", "config-error"; and later "pkill" / "filter-cut" / "dry-run" / "skipped"
	Outcome   string // human-readable result, e.g. "bell", "killed", "unknown metric"
}

// auditLog is a fixed-capacity ring of AuditEntry, oldest dropped first.
type auditLog struct {
	entries []AuditEntry // chronological (oldest first)
}

func (a *auditLog) append(e AuditEntry) {
	a.entries = append(a.entries, e)
	if len(a.entries) > auditCapacity {
		// Drop oldest; copy down so the backing array can't grow unbounded.
		a.entries = append(a.entries[:0:0], a.entries[len(a.entries)-auditCapacity:]...)
	}
}

// recent returns the entries newest-first (for rendering).
// Used by the audit panel (Step 3); declared here so the ring buffer is
// self-contained before the view wires it in.
//
//nolint:unused // consumed by the audit panel view added in Step 3
func (a *auditLog) recent() []AuditEntry {
	out := make([]AuditEntry, len(a.entries))
	for i := range a.entries {
		out[len(a.entries)-1-i] = a.entries[i]
	}
	return out
}
