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
	IsActive     bool           // True if file modified recently (within 5 minutes)
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
