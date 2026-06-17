package session

import "time"

// AgentNode is one spawned subagent: its identity and the last time it was seen
// active. There is no parent link — the spawning Agent/Task tool call is not
// persisted in the transcripts, so nesting depth cannot be reconstructed from
// the on-disk format. The main session is the implicit root and is not a node.
type AgentNode struct {
	ID       string // agentId (matches the agent-<ID>.jsonl filename)
	LastSeen time.Time
}

// AgentMetrics summarizes subagent activity for a session.
type AgentMetrics struct {
	// TotalAgents is every distinct subagent spawned over the session's life.
	TotalAgents int
	// CurrentAgents is the number of subagents active within the trailing
	// window (a subagent counts as active if it emitted a record in the window
	// ending at the last agent action).
	CurrentAgents int
	// Window is the window used for CurrentAgents.
	Window time.Duration
}

// ComputeAgentMetrics derives subagent counts from the per-agent nodes. A
// non-positive window falls back to DefaultBurnWindow so the "current agents"
// window matches the burn-rate window by default.
func ComputeAgentMetrics(nodes []AgentNode, window time.Duration) AgentMetrics {
	if window <= 0 {
		window = DefaultBurnWindow
	}

	res := AgentMetrics{Window: window, TotalAgents: len(nodes)}
	if len(nodes) == 0 {
		return res
	}

	// Anchor "current" on the most recent agent activity, mirroring the
	// burn-rate window, so an idle session still reports who was active right
	// before it went quiet.
	var anchor time.Time
	for _, n := range nodes {
		if n.LastSeen.After(anchor) {
			anchor = n.LastSeen
		}
	}

	start := anchor.Add(-window)
	for _, n := range nodes {
		if !n.LastSeen.Before(start) {
			res.CurrentAgents++
		}
	}
	return res
}
