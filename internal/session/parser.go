package session

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

// JSONLRecord represents a single line in the session file
type JSONLRecord struct {
	Type      string   `json:"type"`
	Timestamp string   `json:"timestamp"`
	UUID      string   `json:"uuid"`
	SessionID string   `json:"sessionId"`
	GitBranch string   `json:"gitBranch"`
	CWD       string   `json:"cwd"`
	Message   *Message `json:"message,omitempty"`
}

// Message represents the message field in a JSONL record
type Message struct {
	Role    string        `json:"role"`
	Content []ContentItem `json:"content"`
}

// ContentItem represents an item in the content array
type ContentItem struct {
	Type  string          `json:"type"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// GenericInput is used to extract common fields from any tool's input
// It tries multiple common field names to find relevant display info
type GenericInput struct {
	// Common file/path fields
	FilePath string `json:"file_path"`
	Path     string `json:"path"`
	URL      string `json:"url"`

	// Command/query fields
	Command     string `json:"command"`
	Pattern     string `json:"pattern"`
	Query       string `json:"query"`
	Prompt      string `json:"prompt"`
	Description string `json:"description"`

	// Task-specific
	Skill string `json:"skill"`
}

// ExtractDisplayString returns the most relevant string to display for this input
func (g *GenericInput) ExtractDisplayString(toolName string) string {
	// Priority order for what to display based on tool type
	switch toolName {
	case "Bash":
		return g.Command
	case "Edit", "Write", "NotebookEdit", "Read":
		return g.FilePath
	case "Glob":
		if g.Pattern != "" && g.Path != "" {
			return g.Path + "/" + g.Pattern
		}
		if g.Pattern != "" {
			return g.Pattern
		}
		return g.Path
	case "Grep":
		if g.Pattern != "" && g.Path != "" {
			return g.Pattern + " in " + g.Path
		}
		if g.Pattern != "" {
			return g.Pattern
		}
		return g.Path
	case "WebFetch", "WebSearch":
		if g.URL != "" {
			return g.URL
		}
		return g.Query
	case "Task":
		return g.Description
	case "Skill":
		return g.Skill
	}

	// Generic fallback: try fields in priority order
	if g.FilePath != "" {
		return g.FilePath
	}
	if g.Path != "" {
		return g.Path
	}
	if g.Command != "" {
		return g.Command
	}
	if g.Pattern != "" {
		return g.Pattern
	}
	if g.Query != "" {
		return g.Query
	}
	if g.URL != "" {
		return g.URL
	}
	if g.Description != "" {
		return g.Description
	}
	if g.Prompt != "" {
		// Truncate long prompts
		if len(g.Prompt) > 100 {
			return g.Prompt[:100] + "..."
		}
		return g.Prompt
	}
	if g.Skill != "" {
		return g.Skill
	}

	return ""
}

// SessionMetadata contains metadata extracted from a session file
type SessionMetadata struct {
	GitBranch string
	CWD       string
}

// ParseSessionFile reads a JSONL file and extracts command entries
func ParseSessionFile(path string) ([]CommandEntry, SessionMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, SessionMetadata{}, err
	}
	defer file.Close()

	var commands []CommandEntry
	var meta SessionMetadata
	seen := make(map[string]bool) // Track seen UUIDs to avoid duplicates

	scanner := bufio.NewScanner(file)
	// Increase buffer for large lines (some tool results can be huge)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024) // 2MB max line size

	for scanner.Scan() {
		var record JSONLRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue // Skip malformed lines
		}

		// Capture metadata from the first record that has each field
		if record.GitBranch != "" && meta.GitBranch == "" {
			meta.GitBranch = record.GitBranch
		}
		if record.CWD != "" && meta.CWD == "" {
			meta.CWD = record.CWD
		}

		// Only process assistant messages with tool calls
		if record.Type != "assistant" || record.Message == nil {
			continue
		}

		for _, content := range record.Message.Content {
			if content.Type != "tool_use" {
				continue
			}

			entry := CommandEntry{
				ToolName:  content.Name,
				SessionID: record.SessionID,
				UUID:      record.UUID,
			}

			// Parse input and extract display string
			var input GenericInput
			if err := json.Unmarshal(content.Input, &input); err == nil {
				entry.RawCommand = input.ExtractDisplayString(content.Name)
			}

			// Fall back to tool name if no display string extracted
			if entry.RawCommand == "" {
				entry.RawCommand = content.Name
			}

			// Extract pattern (Bash gets special treatment for command grouping)
			if content.Name == "Bash" {
				entry.Pattern = ExtractPattern("Bash", input.Command)
			} else {
				entry.Pattern = content.Name
			}

			// Skip if pattern should be excluded
			if !ShouldInclude(entry.Pattern) {
				continue
			}

			// Create unique key for deduplication
			entryKey := record.UUID + content.Name
			if seen[entryKey] {
				continue
			}
			seen[entryKey] = true

			// Parse timestamp
			if t, err := time.Parse(time.RFC3339, record.Timestamp); err == nil {
				entry.Timestamp = t
			} else {
				entry.Timestamp = time.Now()
			}

			// Only add if we got a valid command/path
			if entry.RawCommand != "" {
				commands = append(commands, entry)
			}
		}
	}

	return commands, meta, scanner.Err()
}

// ParseSessionFileFrom reads a JSONL file starting from a byte offset
// Returns commands found, new offset, and any error
func ParseSessionFileFrom(path string, offset int64) ([]CommandEntry, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer file.Close()

	// Seek to offset
	if offset > 0 {
		if _, err := file.Seek(offset, 0); err != nil {
			return nil, offset, err
		}
	}

	var commands []CommandEntry
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		offset += int64(len(line)) + 1 // +1 for newline

		var record JSONLRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}

		if record.Type != "assistant" || record.Message == nil {
			continue
		}

		for _, content := range record.Message.Content {
			if content.Type != "tool_use" {
				continue
			}

			entry := CommandEntry{
				ToolName:  content.Name,
				SessionID: record.SessionID,
				UUID:      record.UUID,
			}

			// Parse input and extract display string
			var input GenericInput
			if err := json.Unmarshal(content.Input, &input); err == nil {
				entry.RawCommand = input.ExtractDisplayString(content.Name)
			}

			// Fall back to tool name if no display string extracted
			if entry.RawCommand == "" {
				entry.RawCommand = content.Name
			}

			// Extract pattern (Bash gets special treatment for command grouping)
			if content.Name == "Bash" {
				entry.Pattern = ExtractPattern("Bash", input.Command)
			} else {
				entry.Pattern = content.Name
			}

			// Skip if pattern should be excluded
			if !ShouldInclude(entry.Pattern) {
				continue
			}

			entryKey := record.UUID + content.Name
			if seen[entryKey] {
				continue
			}
			seen[entryKey] = true

			if t, err := time.Parse(time.RFC3339, record.Timestamp); err == nil {
				entry.Timestamp = t
			}

			if entry.RawCommand != "" {
				commands = append(commands, entry)
			}
		}
	}

	return commands, offset, scanner.Err()
}
