package session

import (
	"sort"
	"time"
)

// AgentNode is one spawned subagent: its identity and the span of wall-clock
// time over which it was seen active (first to last record). There is no parent
// link — the spawning Agent/Task tool call is not persisted in the transcripts,
// so nesting depth cannot be reconstructed from the on-disk format. The main
// session is the implicit root and is not a node.
type AgentNode struct {
	ID        string // agentId (matches the agent-<ID>.jsonl filename)
	FirstSeen time.Time
	LastSeen  time.Time
}

// AgentMetrics summarizes subagent activity for a session.
type AgentMetrics struct {
	// TotalAgents is every distinct subagent spawned over the session's life.
	TotalAgents int
	// MaxConcurrent is the peak number of subagents whose active spans overlapped
	// at the same instant — "how parallel did this session ever get". It is a
	// lifetime figure, independent of any trailing window.
	MaxConcurrent int
	// LastSeen holds each subagent's last-activity timestamp. ActiveWithin counts
	// these against the wall clock at render time so the "active now" figure
	// decays to zero once a session goes quiet, rather than freezing on the last
	// burst (see ActiveWithin).
	LastSeen []time.Time
}

// ActiveWithin returns the number of subagents whose last activity falls within
// window of now. It is evaluated against the wall clock (not the session's last
// action) so an idle or finished session reports 0 active agents.
func (m AgentMetrics) ActiveWithin(now time.Time, window time.Duration) int {
	cutoff := now.Add(-window)
	n := 0
	for _, ts := range m.LastSeen {
		if !ts.Before(cutoff) {
			n++
		}
	}
	return n
}

// ComputeAgentMetrics derives subagent counts from the per-agent nodes: the
// lifetime total, the peak concurrency, and the per-agent last-seen times used
// for the live "active in the last minute" figure.
func ComputeAgentMetrics(nodes []AgentNode) AgentMetrics {
	res := AgentMetrics{TotalAgents: len(nodes)}
	if len(nodes) == 0 {
		return res
	}

	res.LastSeen = make([]time.Time, 0, len(nodes))
	for _, n := range nodes {
		res.LastSeen = append(res.LastSeen, n.LastSeen)
	}
	res.MaxConcurrent = maxConcurrent(nodes)
	return res
}

// maxConcurrent returns the peak number of overlapping [FirstSeen, LastSeen]
// activity spans via a sweep line. Spans that touch only at an endpoint are
// treated as concurrent (a start is processed before an end at the same
// timestamp). Nodes with no parseable timestamps (zero span) are skipped so a
// degenerate empty transcript cannot inflate the count.
func maxConcurrent(nodes []AgentNode) int {
	type event struct {
		t     time.Time
		delta int
	}
	events := make([]event, 0, len(nodes)*2)
	for _, n := range nodes {
		if n.FirstSeen.IsZero() && n.LastSeen.IsZero() {
			continue
		}
		start, end := n.FirstSeen, n.LastSeen
		if end.Before(start) {
			start, end = end, start
		}
		events = append(events, event{start, +1}, event{end, -1})
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].t.Equal(events[j].t) {
			return events[i].delta > events[j].delta // open before close at a tie
		}
		return events[i].t.Before(events[j].t)
	})

	cur, peak := 0, 0
	for _, e := range events {
		cur += e.delta
		if cur > peak {
			peak = cur
		}
	}
	return peak
}
