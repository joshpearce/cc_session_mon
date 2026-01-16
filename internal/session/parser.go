package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
	Type      string          `json:"type"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ID        string          `json:"id,omitempty"`         // tool_use ID
	ToolUseID string          `json:"tool_use_id,omitempty"` // References tool_use ID in tool_result
	Content   json.RawMessage `json:"content,omitempty"`     // tool_result content
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
	lineNumber := 0

	scanner := bufio.NewScanner(file)
	// Increase buffer for large lines (some tool results can be huge)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024) // 2MB max line size

	for scanner.Scan() {
		lineNumber++

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
				ToolName:   content.Name,
				SessionID:  record.SessionID,
				UUID:       record.UUID,
				LineNumber: lineNumber,
				FilePath:   path,
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
// Returns commands found, metadata, new offset, new line number, and any error
func ParseSessionFileFrom(path string, offset int64, startLine int) ([]CommandEntry, SessionMetadata, int64, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, SessionMetadata{}, offset, startLine, err
	}
	defer file.Close()

	// Seek to offset
	if offset > 0 {
		if _, err := file.Seek(offset, 0); err != nil {
			return nil, SessionMetadata{}, offset, startLine, err
		}
	}

	var commands []CommandEntry
	var meta SessionMetadata
	seen := make(map[string]bool)
	lineNumber := startLine

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		offset += int64(len(line)) + 1 // +1 for newline
		lineNumber++

		var record JSONLRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}

		// Capture metadata from records that have it
		if record.CWD != "" && meta.CWD == "" {
			meta.CWD = record.CWD
		}
		if record.GitBranch != "" && meta.GitBranch == "" {
			meta.GitBranch = record.GitBranch
		}

		if record.Type != "assistant" || record.Message == nil {
			continue
		}

		for _, content := range record.Message.Content {
			if content.Type != "tool_use" {
				continue
			}

			entry := CommandEntry{
				ToolName:   content.Name,
				SessionID:  record.SessionID,
				UUID:       record.UUID,
				LineNumber: lineNumber,
				FilePath:   path,
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

	return commands, meta, offset, lineNumber, scanner.Err()
}

// ToolInput holds the full parsed input for a tool call, loaded on demand
type ToolInput struct {
	Raw       json.RawMessage        // The raw JSON input
	Parsed    map[string]interface{} // Parsed as generic map for inspection
	ToolName  string                 // The tool name
	CWD       string                 // Working directory at time of call
	GitBranch string                 // Git branch at time of call
	ToolUseID string                 // The tool_use ID for linking to result
	Result    string                 // The tool result/output (if found)
	IsError   bool                   // Whether the result was an error
}

// FetchToolInput reads a tool call record and its result from a JSONL file.
// It first tries the line number (fast path), then falls back to UUID-based search.
// After finding the tool_use, it scans ahead to find the matching tool_result.
func FetchToolInput(filePath string, lineNumber int, toolName string, uuid string) (*ToolInput, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	var input *ToolInput
	var lines [][]byte // Buffer lines to search for result

	// First, try the fast path: read the specific line
	currentLine := 0
	for scanner.Scan() {
		currentLine++
		line := make([]byte, len(scanner.Bytes()))
		copy(line, scanner.Bytes())

		if currentLine == lineNumber {
			input = tryParseToolInput(line, toolName, uuid)
			if input != nil {
				// Continue reading to find the result
				lines = append(lines, line)
			}
		} else if input != nil {
			// After finding the tool_use, collect lines to search for result
			lines = append(lines, line)
			// Only look ahead a few lines for the result
			if len(lines) > 10 {
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// If fast path found the input, search for result in collected lines
	if input != nil {
		findToolResult(input, lines)
		return input, nil
	}

	// Fast path failed - fall back to UUID-based search
	file.Close()
	file, err = os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner = bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	lines = nil
	for scanner.Scan() {
		line := make([]byte, len(scanner.Bytes()))
		copy(line, scanner.Bytes())

		if input == nil {
			input = tryParseToolInput(line, toolName, uuid)
			if input != nil {
				lines = append(lines, line)
			}
		} else {
			lines = append(lines, line)
			if len(lines) > 10 {
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if input != nil {
		findToolResult(input, lines)
		return input, nil
	}

	return nil, fmt.Errorf("tool %s with UUID %s not found", toolName, uuid)
}

// tryParseToolInput attempts to parse a line and extract the tool input if it matches
func tryParseToolInput(line []byte, toolName string, uuid string) *ToolInput {
	var record JSONLRecord
	if err := json.Unmarshal(line, &record); err != nil {
		return nil
	}

	// Check UUID match if provided
	if uuid != "" && record.UUID != uuid {
		return nil
	}

	if record.Message == nil {
		return nil
	}

	// Find the matching tool_use content
	for _, content := range record.Message.Content {
		if content.Type == "tool_use" && content.Name == toolName {
			var parsed map[string]interface{}
			if err := json.Unmarshal(content.Input, &parsed); err != nil {
				parsed = make(map[string]interface{})
			}

			return &ToolInput{
				Raw:       content.Input,
				Parsed:    parsed,
				ToolName:  content.Name,
				ToolUseID: content.ID,
				CWD:       record.CWD,
				GitBranch: record.GitBranch,
			}
		}
	}

	return nil
}

// findToolResult searches lines for a tool_result matching the ToolUseID
func findToolResult(input *ToolInput, lines [][]byte) {
	if input.ToolUseID == "" {
		return
	}

	for _, line := range lines {
		var record JSONLRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}

		if record.Message == nil {
			continue
		}

		// Look for tool_result with matching tool_use_id
		for _, content := range record.Message.Content {
			if content.Type == "tool_result" && content.ToolUseID == input.ToolUseID {
				input.Result = extractResultText(content.Content)
				// Check if this is an error result (heuristic: look for error indicators)
				input.IsError = isErrorResult(input.Result)
				return
			}
		}
	}
}

// extractResultText extracts readable text from tool_result content
func extractResultText(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}

	// Try parsing as string first (simple case)
	var simpleStr string
	if err := json.Unmarshal(content, &simpleStr); err == nil {
		return simpleStr
	}

	// Try parsing as array of content items (common format)
	var items []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &items); err == nil {
		var result string
		for _, item := range items {
			if item.Type == "text" && item.Text != "" {
				if result != "" {
					result += "\n"
				}
				result += item.Text
			}
		}
		return result
	}

	// Fall back to raw string (truncated)
	s := string(content)
	if len(s) > 2000 {
		return s[:2000] + "..."
	}
	return s
}

// isErrorResult checks if the result text indicates an error
func isErrorResult(result string) bool {
	if len(result) < 5 {
		return false
	}
	prefix := strings.ToLower(result)
	if len(prefix) > 100 {
		prefix = prefix[:100]
	}
	return strings.HasPrefix(prefix, "error") || strings.HasPrefix(prefix, "failed")
}
