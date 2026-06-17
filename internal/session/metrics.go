package session

import "time"

// MetricFunc computes one live per-session metric value at time now. Each metric
// is responsible for its own wall-clock liveness so an idle session decays to a
// non-tripping value rather than latching a threshold forever.
type MetricFunc func(s *Session, now time.Time) float64

// metricRegistry maps a config metric name to its evaluator. Adding a new alert
// metric is a registry entry here plus a config rule — no control-flow changes.
var metricRegistry = map[string]MetricFunc{
	// active_subagents: subagents whose last activity is within RecentWindow of
	// now. ActiveWithin already decays against the wall clock.
	"active_subagents": func(s *Session, now time.Time) float64 {
		return float64(s.AgentStats.ActiveWithin(now, RecentWindow))
	},
	// tokens_per_min_1m: the fixed 1-minute burn rate, but gated to 0 once the
	// session has been idle longer than RecentWindow. BurnRecent is anchored on
	// the last action, so without this gate a session that burned hard then went
	// quiet would trip forever.
	"tokens_per_min_1m": func(s *Session, now time.Time) float64 {
		if now.Sub(s.BurnRecent.WindowEnd) > RecentWindow {
			return 0
		}
		return s.BurnRecent.TokensPerMinute
	},
}

// Metric returns the evaluator for a metric name and whether it is registered.
func Metric(name string) (MetricFunc, bool) {
	f, ok := metricRegistry[name]
	return f, ok
}
