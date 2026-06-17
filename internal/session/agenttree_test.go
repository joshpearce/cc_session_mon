package session

import (
	"testing"
	"time"
)

func TestComputeAgentMetrics_Empty(t *testing.T) {
	res := ComputeAgentMetrics(nil)
	if res.TotalAgents != 0 || res.MaxConcurrent != 0 || len(res.LastSeen) != 0 {
		t.Fatalf("expected zero metrics, got %+v", res)
	}
}

func TestComputeAgentMetrics_TotalCountsEveryAgent(t *testing.T) {
	now := time.Now()
	nodes := []AgentNode{
		{ID: "a", FirstSeen: now, LastSeen: now},
		{ID: "b", FirstSeen: now, LastSeen: now},
		{ID: "c", FirstSeen: now, LastSeen: now},
		{ID: "d", FirstSeen: now, LastSeen: now},
	}
	res := ComputeAgentMetrics(nodes)
	if res.TotalAgents != 4 {
		t.Fatalf("expected 4 agents, got %d", res.TotalAgents)
	}
}

func TestComputeAgentMetrics_MaxConcurrent(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	at := func(n int) time.Time { return base.Add(time.Duration(n) * time.Minute) }

	// Spans on a 0-10 minute timeline:
	//   a: [0, 4]   b: [2, 6]   c: [5, 9]   d: [8, 10]
	// Peak overlap is 2: a&b overlap [2,4], b&c overlap [5,6], c&d overlap [8,9].
	nodes := []AgentNode{
		{ID: "a", FirstSeen: at(0), LastSeen: at(4)},
		{ID: "b", FirstSeen: at(2), LastSeen: at(6)},
		{ID: "c", FirstSeen: at(5), LastSeen: at(9)},
		{ID: "d", FirstSeen: at(8), LastSeen: at(10)},
	}
	res := ComputeAgentMetrics(nodes)
	if res.MaxConcurrent != 2 {
		t.Fatalf("expected peak concurrency 2, got %d", res.MaxConcurrent)
	}
}

func TestComputeAgentMetrics_MaxConcurrentAllOverlap(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	// Three agents all alive across an overlapping window -> peak 3.
	nodes := []AgentNode{
		{ID: "a", FirstSeen: base, LastSeen: base.Add(10 * time.Minute)},
		{ID: "b", FirstSeen: base.Add(time.Minute), LastSeen: base.Add(9 * time.Minute)},
		{ID: "c", FirstSeen: base.Add(2 * time.Minute), LastSeen: base.Add(8 * time.Minute)},
	}
	res := ComputeAgentMetrics(nodes)
	if res.MaxConcurrent != 3 {
		t.Fatalf("expected peak concurrency 3, got %d", res.MaxConcurrent)
	}
}

func TestComputeAgentMetrics_MaxConcurrentSkipsZeroSpan(t *testing.T) {
	// A degenerate node with no parseable timestamps must not inflate the peak.
	nodes := []AgentNode{
		{ID: "empty"},
		{ID: "empty2"},
	}
	res := ComputeAgentMetrics(nodes)
	if res.TotalAgents != 2 {
		t.Fatalf("expected 2 total agents, got %d", res.TotalAgents)
	}
	if res.MaxConcurrent != 0 {
		t.Fatalf("expected peak concurrency 0 for zero-span nodes, got %d", res.MaxConcurrent)
	}
}

func TestAgentMetrics_ActiveWithin(t *testing.T) {
	now := time.Now()
	// Two agents active within the last minute, one that went quiet well before.
	nodes := []AgentNode{
		{ID: "recent1", FirstSeen: now.Add(-2 * time.Minute), LastSeen: now.Add(-10 * time.Second)},
		{ID: "recent2", FirstSeen: now.Add(-5 * time.Minute), LastSeen: now.Add(-50 * time.Second)},
		{ID: "stale", FirstSeen: now.Add(-40 * time.Minute), LastSeen: now.Add(-30 * time.Minute)},
	}
	res := ComputeAgentMetrics(nodes)

	if got := res.ActiveWithin(now, time.Minute); got != 2 {
		t.Fatalf("expected 2 agents active in the last minute, got %d", got)
	}
	// A finished session evaluated far later reports nothing active.
	if got := res.ActiveWithin(now.Add(time.Hour), time.Minute); got != 0 {
		t.Fatalf("expected 0 active long after the session went quiet, got %d", got)
	}
}
