package session

import (
	"os"
	"path/filepath"
	"testing"
)

// usageLine builds a minimal assistant JSONL record carrying a usage block.
func usageLine(ts string, in, out int) string {
	return `{"type":"assistant","timestamp":"` + ts + `","message":{"role":"assistant","usage":{"input_tokens":` +
		itoa(in) + `,"output_tokens":` + itoa(out) + `}}}` + "\n"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestRefreshSessionMetrics_PicksUpAppendedUsage(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "sess.jsonl")
	if err := os.WriteFile(main, []byte(usageLine("2026-06-17T10:00:00Z", 500, 300)), 0o600); err != nil {
		t.Fatal(err)
	}

	sess := &Session{FilePath: main}
	refreshSessionMetrics(main, sess)
	if len(sess.Usages) != 1 {
		t.Fatalf("expected 1 usage sample after first refresh, got %d", len(sess.Usages))
	}

	// Simulate the session continuing to write (a new turn burning tokens).
	f, err := os.OpenFile(main, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(usageLine("2026-06-17T10:01:00Z", 1000, 700)); err != nil {
		t.Fatal(err)
	}
	f.Close()

	refreshSessionMetrics(main, sess)
	if len(sess.Usages) != 2 {
		t.Fatalf("expected refresh to re-read appended usage (2 samples), got %d", len(sess.Usages))
	}
}

func TestScanSubtree_IncludesSubagents(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "sess.jsonl")
	if err := os.WriteFile(main, []byte(usageLine("2026-06-17T10:00:00Z", 100, 100)), 0o600); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(dir, "sess", "subagents")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(subDir, "agent-abc123.jsonl")
	if err := os.WriteFile(sub, []byte(usageLine("2026-06-17T10:00:30Z", 200, 200)), 0o600); err != nil {
		t.Fatal(err)
	}

	usages, agents := ScanSubtree(main)
	if len(usages) != 2 {
		t.Fatalf("expected main + subagent usage (2 samples), got %d", len(usages))
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 subagent node, got %d", len(agents))
	}
	if agents[0].ID != "abc123" {
		t.Fatalf("expected agent id derived from filename, got %q", agents[0].ID)
	}
}
