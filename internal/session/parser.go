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

// BashInput represents the input for a Bash tool call
type BashInput struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	Timeout     int    `json:"timeout,omitempty"`
}

// FileInput represents the input for Edit/Write/NotebookEdit tool calls
type FileInput struct {
	FilePath  string `json:"file_path"`
	Content   string `json:"content,omitempty"`
	OldString string `json:"old_string,omitempty"`
	NewString string `json:"new_string,omitempty"`
}

// ParseSessionFile reads a JSONL file and extracts command entries
func ParseSessionFile(path string) ([]CommandEntry, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	var commands []CommandEntry
	var gitBranch string
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

		// Capture git branch from any record that has it
		if record.GitBranch != "" && gitBranch == "" {
			gitBranch = record.GitBranch
		}

		// Only process assistant messages with tool calls
		if record.Type != "assistant" || record.Message == nil {
			continue
		}

		for _, content := range record.Message.Content {
			if content.Type != "tool_use" {
				continue
			}

			// Skip non-write operations
			if !IsWriteOperation(content.Name) {
				continue
			}

			// Create unique key for deduplication
			entryKey := record.UUID + content.Name
			if seen[entryKey] {
				continue
			}
			seen[entryKey] = true

			entry := CommandEntry{
				ToolName:  content.Name,
				SessionID: record.SessionID,
				UUID:      record.UUID,
			}

			// Parse timestamp
			if t, err := time.Parse(time.RFC3339, record.Timestamp); err == nil {
				entry.Timestamp = t
			} else {
				entry.Timestamp = time.Now()
			}

			// Extract raw command/path and pattern based on tool type
			switch content.Name {
			case "Bash":
				var input BashInput
				if err := json.Unmarshal(content.Input, &input); err == nil {
					entry.RawCommand = input.Command
					entry.Pattern = ExtractPattern("Bash", input.Command)
				}
			case "Edit", "Write", "NotebookEdit":
				var input FileInput
				if err := json.Unmarshal(content.Input, &input); err == nil {
					entry.RawCommand = input.FilePath
					entry.Pattern = content.Name
				}
			default:
				// For other tools, use the tool name as both pattern and command
				entry.Pattern = content.Name
				entry.RawCommand = content.Name
			}

			// Only add if we got a valid command/path
			if entry.RawCommand != "" {
				commands = append(commands, entry)
			}
		}
	}

	return commands, gitBranch, scanner.Err()
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
			if content.Type != "tool_use" || !IsWriteOperation(content.Name) {
				continue
			}

			entryKey := record.UUID + content.Name
			if seen[entryKey] {
				continue
			}
			seen[entryKey] = true

			entry := CommandEntry{
				ToolName:  content.Name,
				SessionID: record.SessionID,
				UUID:      record.UUID,
			}

			if t, err := time.Parse(time.RFC3339, record.Timestamp); err == nil {
				entry.Timestamp = t
			}

			switch content.Name {
			case "Bash":
				var input BashInput
				if err := json.Unmarshal(content.Input, &input); err == nil {
					entry.RawCommand = input.Command
					entry.Pattern = ExtractPattern("Bash", input.Command)
				}
			case "Edit", "Write", "NotebookEdit":
				var input FileInput
				if err := json.Unmarshal(content.Input, &input); err == nil {
					entry.RawCommand = input.FilePath
					entry.Pattern = content.Name
				}
			default:
				// For other tools, use the tool name as both pattern and command
				entry.Pattern = content.Name
				entry.RawCommand = content.Name
			}

			if entry.RawCommand != "" {
				commands = append(commands, entry)
			}
		}
	}

	return commands, offset, scanner.Err()
}
