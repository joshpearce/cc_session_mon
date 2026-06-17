package session

import (
	"time"

	"cc_session_mon/internal/config"
)

// DefaultBurnWindow is the trailing window used for burn-rate calculation when
// none is configured. It is the single source shared with config.BurnWindow so
// the fallback window and the displayed window can never diverge.
const DefaultBurnWindow = config.DefaultBurnWindow

// RecentWindow is the fixed short window behind the "(1m)" live-activity
// metrics: the 1-minute burn rate and the count of subagents active in the last
// minute. Unlike the configurable burn window it is deliberately not tunable —
// it answers "what is happening right now".
const RecentWindow = time.Minute

// UsageEntry is a single timestamped token-accounting sample, extracted from an
// assistant message's usage block.
type UsageEntry struct {
	Timestamp           time.Time
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
}

// BillableTokens is the count of "fresh" tokens this sample processed: prompt
// input, generated output, and newly written cache. It deliberately excludes
// cache reads, which are cheap and dominate normal sessions — counting them
// would make every session look like it is burning.
func (u UsageEntry) BillableTokens() int {
	return u.InputTokens + u.OutputTokens + u.CacheCreationTokens
}

// BurnRateResult summarizes token consumption over a trailing window that ends
// at the session's last action.
type BurnRateResult struct {
	// WindowEnd is the anchor: the timestamp of the last usage sample (the last
	// token-producing action in the session).
	WindowEnd time.Time
	// WindowStart is WindowEnd minus the window duration.
	WindowStart time.Time
	// Window is the duration the rate is averaged over.
	Window time.Duration
	// SampleCount is the number of usage samples that fell inside the window.
	SampleCount int

	// BillableTokens is the windowed sum of UsageEntry.BillableTokens.
	BillableTokens int
	// TokensPerMinute is BillableTokens averaged over the full window. It is a
	// rate over the fixed window, not over the span between first and last
	// sample, so a single recent burst still reports as elevated.
	TokensPerMinute float64
}

// ComputeBurnRate sums token usage over a trailing window ending at the most
// recent sample, and returns the billable tokens-per-minute over that window.
//
// The window is anchored on the last action rather than "now" so the metric is
// meaningful for sessions that have gone idle: it answers "how hard was this
// session burning right before it last did anything?".
//
// A non-positive window falls back to DefaultBurnWindow. With no samples, a
// zero-valued result is returned.
func ComputeBurnRate(usages []UsageEntry, window time.Duration) BurnRateResult {
	if window <= 0 {
		window = DefaultBurnWindow
	}

	res := BurnRateResult{Window: window}
	if len(usages) == 0 {
		return res
	}

	// Anchor on the latest sample; do not assume input is sorted.
	anchor := usages[0].Timestamp
	for _, u := range usages[1:] {
		if u.Timestamp.After(anchor) {
			anchor = u.Timestamp
		}
	}

	start := anchor.Add(-window)
	res.WindowEnd = anchor
	res.WindowStart = start

	for _, u := range usages {
		// Include samples in (start, anchor]. Boundary at start is included so a
		// window exactly spanning the first sample still counts it.
		if u.Timestamp.Before(start) || u.Timestamp.After(anchor) {
			continue
		}
		res.SampleCount++
		res.BillableTokens += u.BillableTokens()
	}

	res.TokensPerMinute = float64(res.BillableTokens) / window.Minutes()
	return res
}
