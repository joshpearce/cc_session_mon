package tui

import (
	"strings"
	"testing"
	"time"

	"cc_session_mon/internal/session"
)

func TestFormatTokPerMin(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "—"},
		{-5, "—"},
		{42, "42"},
		{999, "999"},
		{1234, "1.2k"},
		{9999, "10.0k"},
		{41131, "41k"},
		{2_500_000, "2.5M"},
	}
	for _, c := range cases {
		if got := formatTokPerMin(c.in); got != c.want {
			t.Errorf("formatTokPerMin(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRenderSessionHeadersShowsWindow(t *testing.T) {
	m := NewModel(ModelOptions{})
	m.width = 120
	// Default config window is 10m.
	header := m.renderSessionHeaders()
	if !strings.Contains(header, "tok/m(10m)") {
		t.Fatalf("expected header to label the burn window, got: %q", header)
	}
}

func TestSessionRowRendersRate(t *testing.T) {
	m := NewModel(ModelOptions{})
	m.width = 120
	m.height = 40
	m.viewMode = ViewSessions

	now := time.Now()
	// Metrics are precomputed on the session (the render path reads finished
	// values). 1000 billable tokens / 10-min window = 100 tok/min.
	usages := []session.UsageEntry{
		{Timestamp: now, InputTokens: 600, OutputTokens: 400},
	}
	m.sessions = []*session.Session{
		{
			ID:           "s1",
			ProjectPath:  "/projects/alpha",
			Usages:       usages,
			Burn:         session.ComputeBurnRate(usages, 10*time.Minute),
			LastActivity: now,
		},
	}
	m = m.updateSessionList()

	out := m.sessionList.View()
	if !strings.Contains(out, "100") {
		t.Fatalf("expected session row to show 100 tok/min, got:\n%s", out)
	}
}
