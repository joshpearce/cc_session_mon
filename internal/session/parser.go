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
		return g.formatGlob()
	case "Grep":
		return g.formatGrep()
	case "WebFetch", "WebSearch":
		return firstNonEmpty(g.URL, g.Query)
	case "Task":
		return g.Description
	case "Skill":
		return g.Skill
	}

	return g.fallbackDisplay()
}

// formatGlob returns a display string for Glob tool
func (g *GenericInput) formatGlob() string {
	if g.Pattern != "" && g.Path != "" {
		return g.Path + "/" + g.Pattern
	}
	return firstNonEmpty(g.Pattern, g.Path)
}

// formatGrep returns a display string for Grep tool
func (g *GenericInput) formatGrep() string {
	if g.Pattern != "" && g.Path != "" {
		return g.Pattern + " in " + g.Path
	}
	return firstNonEmpty(g.Pattern, g.Path)
}

// fallbackDisplay tries fields in priority order for unknown tools
func (g *GenericInput) fallbackDisplay() string {
	if s := firstNonEmpty(g.FilePath, g.Path, g.Command, g.Pattern, g.Query, g.URL, g.Description); s != "" {
		return s
	}
	if g.Prompt != "" {
		return truncate(g.Prompt, 100)
	}
	return g.Skill
}

// firstNonEmpty returns the first non-empty string from the arguments
func firstNonEmpty(strs ...string) string {
	for _, s := range strs {
		if s != "" {
			return s
		}
	}
	return ""
}

// truncate returns s truncated to maxLen with "..." suffix if needed
func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// SessionMetadata contains metadata extracted from a session file
type SessionMetadata struct {
	GitBranch string
	CWD       string
}

// parseState holds state for incremental JSONL parsing
type parseState struct {
	commands   []CommandEntry
	meta       SessionMetadata
	seen       map[string]bool
	lineNumber int
	offset     int64
	filePath   string
}

// newParseState creates a new parse state
func newParseState(filePath string, startLine int, startOffset int64) *parseState {
	return &parseState{
		seen:       make(map[string]bool),
		lineNumber: startLine,
		offset:     startOffset,
		filePath:   filePath,
	}
}

// processLine parses a single JSONL line and extracts commands.
// Returns the number of bytes consumed (for offset tracking).
func (ps *parseState) processLine(line []byte) int {
	lineLen := len(line) + 1 // +1 for newline
	ps.lineNumber++

	var record JSONLRecord
	if err := json.Unmarshal(line, &record); err != nil {
		return lineLen
	}

	ps.captureMetadata(&record)

	if record.Type != "assistant" || record.Message == nil {
		return lineLen
	}

	for _, content := range record.Message.Content {
		ps.processToolUse(&record, &content)
	}

	return lineLen
}

// captureMetadata extracts session metadata from a record
func (ps *parseState) captureMetadata(record *JSONLRecord) {
	if record.CWD != "" && ps.meta.CWD == "" {
		ps.meta.CWD = record.CWD
	}
	if record.GitBranch != "" && ps.meta.GitBranch == "" {
		ps.meta.GitBranch = record.GitBranch
	}
}

// processToolUse processes a single tool_use content item
func (ps *parseState) processToolUse(record *JSONLRecord, content *ContentItem) {
	if content.Type != "tool_use" {
		return
	}

	entry := CommandEntry{
		ToolName:   content.Name,
		SessionID:  record.SessionID,
		UUID:       record.UUID,
		LineNumber: ps.lineNumber,
		FilePath:   ps.filePath,
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
		return
	}

	// Create unique key for deduplication
	entryKey := record.UUID + content.Name
	if ps.seen[entryKey] {
		return
	}
	ps.seen[entryKey] = true

	// Parse timestamp
	if t, err := time.Parse(time.RFC3339, record.Timestamp); err == nil {
		entry.Timestamp = t
	} else {
		entry.Timestamp = time.Now()
	}

	// Only add if we got a valid command/path
	if entry.RawCommand != "" {
		ps.commands = append(ps.commands, entry)
	}
}

// ParseSessionFile reads a JSONL file and extracts command entries
func ParseSessionFile(path string) ([]CommandEntry, SessionMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, SessionMetadata{}, err
	}
	defer file.Close()

	ps := newParseState(path, 0, 0)

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024) // 2MB max line size

	for scanner.Scan() {
		ps.processLine(scanner.Bytes())
	}

	return ps.commands, ps.meta, scanner.Err()
}

// ParseSessionFileFrom reads a JSONL file starting from a byte offset
// Returns commands found, metadata, new offset, new line number, and any error
func ParseSessionFileFrom(path string, offset int64, startLine int) (commands []CommandEntry, meta SessionMetadata, newOffset int64, newLine int, err error) {
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

	ps := newParseState(path, startLine, offset)

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	for scanner.Scan() {
		ps.offset += int64(ps.processLine(scanner.Bytes()))
	}

	return ps.commands, ps.meta, ps.offset, ps.lineNumber, scanner.Err()
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
func FetchToolInput(filePath string, lineNumber int, toolName, uuid string) (*ToolInput, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	result := scanForToolInput(scanner, lineNumber, toolName, uuid)

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if result.input != nil {
		findToolResult(result.input, result.lines)
		return result.input, nil
	}

	// Fast path failed - search through collected lines by UUID
	if input := searchFallbackLines(result.allLines, toolName, uuid); input != nil {
		return input, nil
	}

	return nil, fmt.Errorf("tool %s with UUID %s not found", toolName, uuid)
}

// scanResult holds the result of scanning a file for tool input
type scanResult struct {
	input    *ToolInput
	lines    [][]byte // Lines for result search
	allLines [][]byte // All lines for fallback search
}

// scanForToolInput scans through the file trying to find the tool input
func scanForToolInput(scanner *bufio.Scanner, lineNumber int, toolName, uuid string) scanResult {
	var result scanResult
	needFallback := false
	currentLine := 0

	for scanner.Scan() {
		currentLine++
		line := make([]byte, len(scanner.Bytes()))
		copy(line, scanner.Bytes())

		// Try fast path: check the target line number
		if currentLine == lineNumber && result.input == nil {
			result.input = tryParseToolInput(line, toolName, uuid)
			if result.input != nil {
				result.lines = append(result.lines, line)
				continue
			}
			needFallback = true
		}

		switch {
		case result.input != nil:
			result.lines = append(result.lines, line)
			if len(result.lines) > 10 {
				return result
			}
		case needFallback:
			result.allLines = append(result.allLines, line)
		case currentLine < lineNumber:
			result.allLines = append(result.allLines, line)
		}
	}

	return result
}

// searchFallbackLines searches through collected lines by UUID
func searchFallbackLines(allLines [][]byte, toolName, uuid string) *ToolInput {
	for i, line := range allLines {
		input := tryParseToolInput(line, toolName, uuid)
		if input != nil {
			// Collect following lines for result search
			lines := [][]byte{line}
			for j := i + 1; j < len(allLines) && len(lines) <= 10; j++ {
				lines = append(lines, allLines[j])
			}
			findToolResult(input, lines)
			return input
		}
	}
	return nil
}

// tryParseToolInput attempts to parse a line and extract the tool input if it matches
func tryParseToolInput(line []byte, toolName, uuid string) *ToolInput {
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
