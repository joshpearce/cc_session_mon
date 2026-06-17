package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// subagentFiles returns the subagent transcript paths for a session's main
// file. Subagent files for every depth live flat in one subagents/ directory,
// so a single glob captures the whole subtree. The layout
// (<session>/<id>/subagents/agent-*.jsonl) is derived in this one place.
func subagentFiles(mainPath string) []string {
	sessionID := strings.TrimSuffix(filepath.Base(mainPath), ".jsonl")
	subDir := filepath.Join(filepath.Dir(mainPath), sessionID, "subagents")
	subs, err := filepath.Glob(filepath.Join(subDir, "*.jsonl"))
	if err != nil {
		return nil
	}
	return subs
}

// ScanSubtree reads a session's whole transcript subtree — the main file plus
// every subagent file — in a single pass per file, and returns the merged
// token-usage samples (for burn rate) and one activity record per subagent file
// (for agent counts). This is the burn-rate / agent-count input, recomputed in
// full on refresh. It is the only place that reads the subtree, so the files are
// globbed and scanned exactly once each.
func ScanSubtree(mainPath string) (usages []UsageEntry, agents []AgentNode) {
	usages, _ = scanTranscript(mainPath)

	for _, sp := range subagentFiles(mainPath) {
		subUsages, lastSeen := scanTranscript(sp)
		usages = append(usages, subUsages...)
		agents = append(agents, AgentNode{
			ID:       agentIDFromPath(sp),
			LastSeen: lastSeen,
		})
	}
	return usages, agents
}

// agentIDFromPath extracts the agentId from a subagent file path
// (agent-<id>.jsonl -> <id>).
func agentIDFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	return strings.TrimPrefix(base, "agent-")
}

// scanTranscript reads one JSONL transcript in a single pass, returning the
// timestamped token-usage samples from its assistant messages and the timestamp
// of its most recent record (its activity span end). A read error yields an
// empty result rather than failing the whole subtree.
func scanTranscript(path string) (usages []UsageEntry, lastSeen time.Time) {
	file, err := os.Open(path) //nolint:gosec // path from discovered projects dirs
	if err != nil {
		return nil, lastSeen
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	for scanner.Scan() {
		var record JSONLRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}

		if t, err := time.Parse(time.RFC3339, record.Timestamp); err == nil && t.After(lastSeen) {
			lastSeen = t
		}

		if record.Type != "assistant" || record.Message == nil || record.Message.Usage == nil {
			continue
		}
		u := record.Message.Usage
		entry := UsageEntry{
			InputTokens:         u.InputTokens,
			OutputTokens:        u.OutputTokens,
			CacheCreationTokens: u.CacheCreationTokens,
			CacheReadTokens:     u.CacheReadTokens,
		}
		if t, err := time.Parse(time.RFC3339, record.Timestamp); err == nil {
			entry.Timestamp = t
		}
		usages = append(usages, entry)
	}

	return usages, lastSeen
}
