package session

import (
	"testing"
	"time"
)

func ts(base time.Time, minutesAgo int) time.Time {
	return base.Add(-time.Duration(minutesAgo) * time.Minute)
}

func TestComputeBurnRate_Empty(t *testing.T) {
	res := ComputeBurnRate(nil, DefaultBurnWindow)
	if res.SampleCount != 0 || res.BillableTokens != 0 || res.TokensPerMinute != 0 {
		t.Fatalf("expected zero result, got %+v", res)
	}
	if res.Window != DefaultBurnWindow {
		t.Fatalf("expected window %v, got %v", DefaultBurnWindow, res.Window)
	}
}

func TestComputeBurnRate_NonPositiveWindowFallsBack(t *testing.T) {
	res := ComputeBurnRate(nil, 0)
	if res.Window != DefaultBurnWindow {
		t.Fatalf("expected fallback window %v, got %v", DefaultBurnWindow, res.Window)
	}
	res = ComputeBurnRate(nil, -5*time.Minute)
	if res.Window != DefaultBurnWindow {
		t.Fatalf("expected fallback window %v, got %v", DefaultBurnWindow, res.Window)
	}
}

func TestComputeBurnRate_SingleSampleAveragesOverWindow(t *testing.T) {
	now := time.Now()
	usages := []UsageEntry{
		{Timestamp: now, InputTokens: 600, OutputTokens: 300, CacheCreationTokens: 100, CacheReadTokens: 9999},
	}
	res := ComputeBurnRate(usages, 10*time.Minute)

	if res.SampleCount != 1 {
		t.Fatalf("expected 1 sample, got %d", res.SampleCount)
	}
	// Billable excludes the 9999 cache-read tokens: 600 + 300 + 100 = 1000.
	if res.BillableTokens != 1000 {
		t.Fatalf("expected 1000 billable tokens, got %d", res.BillableTokens)
	}
	// 1000 tokens averaged over a 10-minute window = 100 tok/min.
	if res.TokensPerMinute != 100 {
		t.Fatalf("expected 100 tok/min, got %v", res.TokensPerMinute)
	}
	if !res.WindowEnd.Equal(now) {
		t.Fatalf("expected anchor at last sample %v, got %v", now, res.WindowEnd)
	}
}

func TestComputeBurnRate_ExcludesSamplesOutsideWindow(t *testing.T) {
	now := time.Now()
	usages := []UsageEntry{
		{Timestamp: ts(now, 30), InputTokens: 100000}, // 30 min before anchor — excluded
		{Timestamp: ts(now, 5), InputTokens: 500},     // inside 10-min window
		{Timestamp: now, OutputTokens: 500},           // anchor
	}
	res := ComputeBurnRate(usages, 10*time.Minute)

	if res.SampleCount != 2 {
		t.Fatalf("expected 2 in-window samples, got %d", res.SampleCount)
	}
	if res.BillableTokens != 1000 {
		t.Fatalf("expected 1000 billable (old sample excluded), got %d", res.BillableTokens)
	}
}

func TestComputeBurnRate_AnchorsOnLastActionNotNow(t *testing.T) {
	// All activity happened in a burst an hour ago, then the session went idle.
	// The rate should reflect the burst, not be diluted by idle wall-clock time.
	base := time.Now().Add(-time.Hour)
	usages := []UsageEntry{
		{Timestamp: ts(base, 2), OutputTokens: 5000},
		{Timestamp: base, OutputTokens: 5000},
	}
	res := ComputeBurnRate(usages, 10*time.Minute)

	if res.SampleCount != 2 {
		t.Fatalf("expected 2 samples, got %d", res.SampleCount)
	}
	if res.BillableTokens != 10000 {
		t.Fatalf("expected 10000 billable, got %d", res.BillableTokens)
	}
	// 10000 / 10 = 1000 tok/min, regardless of the hour of idle time since.
	if res.TokensPerMinute != 1000 {
		t.Fatalf("expected 1000 tok/min, got %v", res.TokensPerMinute)
	}
	if !res.WindowEnd.Equal(base) {
		t.Fatalf("expected anchor at last action %v, got %v", base, res.WindowEnd)
	}
}

func TestComputeBurnRate_UnsortedInput(t *testing.T) {
	now := time.Now()
	// Deliberately out of order; anchor must still be the latest timestamp.
	usages := []UsageEntry{
		{Timestamp: now, OutputTokens: 200},
		{Timestamp: ts(now, 8), OutputTokens: 300},
		{Timestamp: ts(now, 3), OutputTokens: 100},
	}
	res := ComputeBurnRate(usages, 10*time.Minute)

	if !res.WindowEnd.Equal(now) {
		t.Fatalf("expected anchor %v, got %v", now, res.WindowEnd)
	}
	if res.SampleCount != 3 || res.BillableTokens != 600 {
		t.Fatalf("expected 3 samples / 600 billable, got %d / %d", res.SampleCount, res.BillableTokens)
	}
}
