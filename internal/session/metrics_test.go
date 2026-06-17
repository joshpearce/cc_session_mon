package session

import (
	"testing"
	"time"
)

// TestMetricRegistry verifies that the registered metric names resolve and that
// unknown names do not.
func TestMetricRegistry(t *testing.T) {
	t.Parallel()

	type tc struct {
		name   string
		metric string
		wantOK bool
	}

	tests := []tc{
		{name: "active_subagents_registered", metric: "active_subagents", wantOK: true},
		{name: "tokens_per_min_1m_registered", metric: "tokens_per_min_1m", wantOK: true},
		{name: "unknown_not_registered", metric: "nope", wantOK: false},
		{name: "empty_string_not_registered", metric: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, ok := Metric(tt.metric)
			if ok != tt.wantOK {
				t.Fatalf("Metric(%q): got ok=%v, want %v", tt.metric, ok, tt.wantOK)
			}
		})
	}
}

// TestMetricActiveSubagents_IdleSessionReturnsZero verifies that a session
// whose agent last-seen timestamps are all older than RecentWindow reports 0.
func TestMetricActiveSubagents_IdleSessionReturnsZero(t *testing.T) {
	t.Parallel()

	mf, ok := Metric("active_subagents")
	if !ok {
		t.Fatal("active_subagents not registered")
	}

	now := time.Now()
	stale := now.Add(-RecentWindow - time.Second)

	sess := &Session{
		AgentStats: AgentMetrics{
			TotalAgents: 3,
			LastSeen:    []time.Time{stale, stale, stale},
		},
	}

	got := mf(sess, now)
	if got != 0 {
		t.Fatalf("expected 0 for idle session, got %v", got)
	}
}

// TestMetricTokensPerMin1m_IdleSessionReturnsZero verifies that a session
// whose BurnRecent.WindowEnd is older than RecentWindow reports 0 — even when
// TokensPerMinute itself is large.
func TestMetricTokensPerMin1m_IdleSessionReturnsZero(t *testing.T) {
	t.Parallel()

	mf, ok := Metric("tokens_per_min_1m")
	if !ok {
		t.Fatal("tokens_per_min_1m not registered")
	}

	now := time.Now()
	stale := now.Add(-RecentWindow - time.Second)

	sess := &Session{
		BurnRecent: BurnRateResult{
			WindowEnd:       stale,
			TokensPerMinute: 99999,
		},
	}

	got := mf(sess, now)
	if got != 0 {
		t.Fatalf("expected 0 for session idle > RecentWindow, got %v", got)
	}
}

// TestMetricActiveSubagents_LiveValuePassesThrough verifies that agents
// last-seen at or after now-RecentWindow are counted.
func TestMetricActiveSubagents_LiveValuePassesThrough(t *testing.T) {
	t.Parallel()

	mf, ok := Metric("active_subagents")
	if !ok {
		t.Fatal("active_subagents not registered")
	}

	now := time.Now()

	sess := &Session{
		AgentStats: AgentMetrics{
			TotalAgents: 3,
			LastSeen:    []time.Time{now, now, now},
		},
	}

	got := mf(sess, now)
	if got != 3 {
		t.Fatalf("expected 3 active subagents, got %v", got)
	}
}

// TestMetricTokensPerMin1m_LiveValuePassesThrough verifies that a recent burn
// rate passes through unchanged.
func TestMetricTokensPerMin1m_LiveValuePassesThrough(t *testing.T) {
	t.Parallel()

	mf, ok := Metric("tokens_per_min_1m")
	if !ok {
		t.Fatal("tokens_per_min_1m not registered")
	}

	now := time.Now()

	sess := &Session{
		BurnRecent: BurnRateResult{
			WindowEnd:       now,
			TokensPerMinute: 5000,
		},
	}

	got := mf(sess, now)
	if got != 5000 {
		t.Fatalf("expected 5000 tok/m, got %v", got)
	}
}

// TestMetricBoundaries checks the precise boundary semantics for both metrics.
//
// active_subagents: a subagent last-seen exactly at now-RecentWindow is COUNTED
// because ActiveWithin uses !ts.Before(cutoff), i.e. >= rather than >.
//
// tokens_per_min_1m: a WindowEnd exactly at now-RecentWindow is NOT gated to 0
// because the gate is "now.Sub(WindowEnd) > RecentWindow" (strictly greater).
func TestMetricBoundaries(t *testing.T) {
	t.Parallel()

	now := time.Now()
	exactCutoff := now.Add(-RecentWindow) // == now - 1m

	t.Run("active_subagents_at_exact_cutoff_is_counted", func(t *testing.T) {
		t.Parallel()

		mf, _ := Metric("active_subagents")
		sess := &Session{
			AgentStats: AgentMetrics{
				LastSeen: []time.Time{exactCutoff},
			},
		}
		got := mf(sess, now)
		if got != 1 {
			t.Fatalf("expected subagent at exact cutoff to be counted (got %v)", got)
		}
	})

	t.Run("active_subagents_just_before_cutoff_is_not_counted", func(t *testing.T) {
		t.Parallel()

		mf, _ := Metric("active_subagents")
		sess := &Session{
			AgentStats: AgentMetrics{
				LastSeen: []time.Time{exactCutoff.Add(-time.Nanosecond)},
			},
		}
		got := mf(sess, now)
		if got != 0 {
			t.Fatalf("expected subagent 1ns before cutoff to be excluded (got %v)", got)
		}
	})

	t.Run("tokens_per_min_1m_window_end_at_exact_cutoff_passes_through", func(t *testing.T) {
		t.Parallel()

		// now.Sub(exactCutoff) == RecentWindow, which is NOT > RecentWindow.
		// So the value should pass through.
		mf, _ := Metric("tokens_per_min_1m")
		sess := &Session{
			BurnRecent: BurnRateResult{
				WindowEnd:       exactCutoff,
				TokensPerMinute: 1234,
			},
		}
		got := mf(sess, now)
		if got != 1234 {
			t.Fatalf("expected 1234 tok/m at exact boundary (gate is >, not >=); got %v", got)
		}
	})

	t.Run("tokens_per_min_1m_window_end_one_ns_past_cutoff_is_zero", func(t *testing.T) {
		t.Parallel()

		// now.Sub(exactCutoff - 1ns) > RecentWindow → gated to 0.
		mf, _ := Metric("tokens_per_min_1m")
		sess := &Session{
			BurnRecent: BurnRateResult{
				WindowEnd:       exactCutoff.Add(-time.Nanosecond),
				TokensPerMinute: 1234,
			},
		}
		got := mf(sess, now)
		if got != 0 {
			t.Fatalf("expected 0 when WindowEnd is 1ns past cutoff (got %v)", got)
		}
	})
}
