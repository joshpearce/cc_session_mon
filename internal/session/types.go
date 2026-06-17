package session

import "time"

// Session represents a Claude Code session being monitored
type Session struct {
	ID           string         // UUID from filename
	ProjectPath  string         // Decoded path (e.g., /Users/josh/code/project)
	FilePath     string         // Full path to .jsonl file
	GitBranch    string         // Current git branch
	LastActivity time.Time      // Timestamp of last command
	Commands     []CommandEntry // All write operation commands
	Usages       []UsageEntry   // Timestamped token usage samples (for burn rate)
	IsActive     bool           // True if file modified recently (within 5 minutes)
	Origin       string         // "local" or a derived label for a search-path dir

	// Burn, BurnRecent and AgentStats are precomputed when the session's
	// transcripts are (re)parsed, under the watcher lock, so the UI render path
	// only reads finished values instead of re-scanning Usages every frame. Burn
	// uses the configurable window; BurnRecent uses the fixed RecentWindow (1m).
	// See Watcher.refreshSessionMetrics.
	Burn       BurnRateResult
	BurnRecent BurnRateResult
	AgentStats AgentMetrics

	// metricsModTime is the subtree modification time the precomputed metrics
	// were last derived from, used to skip redundant re-reads on the tick.
	metricsModTime time.Time
}

// CommandEntry represents a single tool invocation
type CommandEntry struct {
	Timestamp  time.Time // When the command was executed
	ToolName   string    // "Bash", "Edit", "Write", "NotebookEdit"
	Pattern    string    // e.g., "Bash(git:*)", "Edit", "Write"
	RawCommand string    // Full command for Bash, file_path for others
	SessionID  string    // Session UUID
	UUID       string    // Message UUID for deduplication
	LineNumber int       // Line number in JSONL file (1-indexed) for lazy loading
	FilePath   string    // Path to session JSONL file
}

// CommandPattern represents a unique command pattern for aggregation
type CommandPattern struct {
	Pattern  string    // e.g., "Bash(rm:*)", "Write"
	ToolName string    // Tool name without pattern
	Count    int       // Number of occurrences
	LastSeen time.Time // Most recent occurrence
	Examples []string  // Sample raw commands (limit to 5)
}

// ProjectSummary provides an overview for the session list view
type ProjectSummary struct {
	ProjectPath    string
	SessionCount   int
	ActiveSessions int
	TotalCommands  int
	LastActivity   time.Time
}
