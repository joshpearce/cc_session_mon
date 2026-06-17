package session

import (
	"testing"
	"time"
)

func TestComputeAgentMetrics_Empty(t *testing.T) {
	res := ComputeAgentMetrics(nil, DefaultBurnWindow)
	if res.TotalAgents != 0 || res.CurrentAgents != 0 {
		t.Fatalf("expected zero metrics, got %+v", res)
	}
	if res.Window != DefaultBurnWindow {
		t.Fatalf("expected window %v, got %v", DefaultBurnWindow, res.Window)
	}
}

func TestComputeAgentMetrics_NonPositiveWindowFallsBack(t *testing.T) {
	res := ComputeAgentMetrics(nil, 0)
	if res.Window != DefaultBurnWindow {
		t.Fatalf("expected fallback window %v, got %v", DefaultBurnWindow, res.Window)
	}
}

func TestComputeAgentMetrics_TotalCountsEveryAgent(t *testing.T) {
	now := time.Now()
	nodes := []AgentNode{
		{ID: "a", LastSeen: now},
		{ID: "b", LastSeen: now},
		{ID: "c", LastSeen: now},
		{ID: "d", LastSeen: now},
	}
	res := ComputeAgentMetrics(nodes, 10*time.Minute)
	if res.TotalAgents != 4 {
		t.Fatalf("expected 4 agents, got %d", res.TotalAgents)
	}
}

func TestComputeAgentMetrics_CurrentAgentsWindow(t *testing.T) {
	now := time.Now()
	// Two agents active recently, one that went quiet well before the window.
	nodes := []AgentNode{
		{ID: "recent1", LastSeen: now},
		{ID: "recent2", LastSeen: now.Add(-3 * time.Minute)},
		{ID: "stale", LastSeen: now.Add(-40 * time.Minute)},
	}
	res := ComputeAgentMetrics(nodes, 10*time.Minute)

	if res.TotalAgents != 3 {
		t.Fatalf("expected 3 total agents, got %d", res.TotalAgents)
	}
	if res.CurrentAgents != 2 {
		t.Fatalf("expected 2 current agents in the 10m window, got %d", res.CurrentAgents)
	}
}
