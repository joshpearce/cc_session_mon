package tui

import (
	"fmt"
	"strings"

	"cc_session_mon/internal/session"

	"github.com/charmbracelet/lipgloss"
)

// renderDetailPanel renders the command detail side panel
func (m Model) renderDetailPanel(width, height int) string {
	var b strings.Builder

	// Panel header
	header := DetailHeaderStyle(width).Render("Command Details")
	b.WriteString(header)
	b.WriteString("\n")

	if m.loadingDetail {
		b.WriteString(MutedStyle().Render("Loading..."))
		return lipgloss.NewStyle().Width(width).Height(height).Render(b.String())
	}

	if m.detailError != nil {
		b.WriteString(ErrorStyle().Render(fmt.Sprintf("Error: %v", m.detailError)))
		return lipgloss.NewStyle().Width(width).Height(height).Render(b.String())
	}

	if m.loadedInput == nil || m.selectedCommand == nil {
		b.WriteString(MutedStyle().Render("Select a command and press Enter"))
		return lipgloss.NewStyle().Width(width).Height(height).Render(b.String())
	}

	// Tool-specific formatting
	content := formatToolInput(m.selectedCommand.ToolName, m.loadedInput, width-2)
	b.WriteString(content)

	return lipgloss.NewStyle().Width(width).Height(height).Render(b.String())
}

// formatToolInput dispatches to tool-specific formatters
func formatToolInput(toolName string, input *session.ToolInput, width int) string {
	switch toolName {
	case "Bash":
		return formatBashDetail(input, width)
	case "Edit":
		return formatEditDetail(input, width)
	case "Write":
		return formatWriteDetail(input, width)
	case "Read":
		return formatReadDetail(input, width)
	case "Glob":
		return formatGlobDetail(input, width)
	case "Grep":
		return formatGrepDetail(input, width)
	case "Task":
		return formatTaskDetail(input, width)
	case "WebFetch", "WebSearch":
		return formatWebDetail(input, width)
	default:
		return formatGenericDetail(input, width)
	}
}

// formatBashDetail renders Bash command details with security warnings
func formatBashDetail(input *session.ToolInput, width int) string {
	var b strings.Builder

	command := getString(input.Parsed, "command")
	description := getString(input.Parsed, "description")
	timeout := getFloat(input.Parsed, "timeout")
	runInBg := getBool(input.Parsed, "run_in_background")

	// Security analysis
	warnings := analyzeBashSecurity(command)
	if len(warnings) > 0 {
		b.WriteString(DangerHeaderStyle().Render("! Security Warnings"))
		b.WriteString("\n")
		for _, w := range warnings {
			b.WriteString(DangerStyle().Render("  - " + w))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Command field
	b.WriteString(LabelStyle().Render("Command:"))
	b.WriteString("\n")
	b.WriteString(CodeBlockStyle(width).Render(wrapText(command, width-4)))
	b.WriteString("\n\n")

	// Description if present
	if description != "" {
		b.WriteString(LabelStyle().Render("Description:"))
		b.WriteString("\n")
		b.WriteString(MutedStyle().Render(wrapText(description, width-2)))
		b.WriteString("\n\n")
	}

	// Metadata
	if timeout > 0 {
		b.WriteString(LabelStyle().Render("Timeout: "))
		fmt.Fprintf(&b, "%.0fms", timeout)
		b.WriteString("\n")
	}

	if runInBg {
		b.WriteString(WarningStyle().Render("* Runs in background"))
		b.WriteString("\n")
	}

	// Context info
	if input.CWD != "" {
		b.WriteString("\n")
		b.WriteString(MutedStyle().Render("CWD: " + input.CWD))
		b.WriteString("\n")
	}

	// Tool result/output
	b.WriteString(formatResultSection(input, width))

	return b.String()
}

// securityCheck defines a check function and its warning message
type securityCheck struct {
	check   func(cmd string) bool
	warning string
}

// securityChecks contains all bash security checks
var securityChecks = []securityCheck{
	{checkRecursiveRm, "Recursive file deletion"},
	{checkSimpleRm, "File deletion"},
	{checkSudo, "Runs with elevated privileges"},
	{checkChmod, "Changes file permissions"},
	{checkChown, "Changes file ownership"},
	{checkCurlPipeShell, "Downloads and pipes to shell"},
	{checkDd, "Direct disk/device operation"},
	{checkMkfs, "Filesystem creation"},
	{checkKill, "Process termination"},
	{checkGitForcePush, "Force push to remote"},
	{checkGitHardReset, "Hard reset (discards changes)"},
}

// analyzeBashSecurity returns security warnings for a bash command
func analyzeBashSecurity(command string) []string {
	var warnings []string
	cmd := strings.ToLower(command)

	for _, sc := range securityChecks {
		if sc.check(cmd) {
			warnings = append(warnings, sc.warning)
		}
	}
	return warnings
}

// hasCommand checks if cmd contains "name " or starts with "name\t"
func hasCommand(cmd, name string) bool {
	return strings.Contains(cmd, name+" ") || strings.HasPrefix(cmd, name+"\t")
}

func checkRecursiveRm(cmd string) bool {
	if !hasCommand(cmd, "rm") && !strings.HasPrefix(cmd, "rm\n") {
		return false
	}
	return strings.Contains(cmd, "-rf") || strings.Contains(cmd, "-r ") || strings.Contains(cmd, " -fr")
}

func checkSimpleRm(cmd string) bool {
	if !hasCommand(cmd, "rm") && !strings.HasPrefix(cmd, "rm\n") {
		return false
	}
	// Only flag if not already caught by recursive check
	return !checkRecursiveRm(cmd)
}

func checkSudo(cmd string) bool {
	return strings.Contains(cmd, "sudo ") || strings.HasPrefix(cmd, "sudo\t")
}

func checkChmod(cmd string) bool {
	return strings.Contains(cmd, "chmod ")
}

func checkChown(cmd string) bool {
	return strings.Contains(cmd, "chown ")
}

func checkCurlPipeShell(cmd string) bool {
	if !strings.Contains(cmd, "|") {
		return false
	}
	hasCurl := strings.Contains(cmd, "curl") || strings.Contains(cmd, "wget")
	hasShell := strings.Contains(cmd, "bash") || strings.Contains(cmd, "sh")
	return hasCurl && hasShell
}

func checkDd(cmd string) bool {
	return hasCommand(cmd, "dd")
}

func checkMkfs(cmd string) bool {
	return strings.Contains(cmd, "mkfs")
}

func checkKill(cmd string) bool {
	return strings.Contains(cmd, "kill ") || strings.Contains(cmd, "pkill ") || strings.Contains(cmd, "killall ")
}

func checkGitForcePush(cmd string) bool {
	return strings.Contains(cmd, "git push") && strings.Contains(cmd, "--force")
}

func checkGitHardReset(cmd string) bool {
	return strings.Contains(cmd, "git reset --hard")
}

// formatEditDetail renders Edit tool details
func formatEditDetail(input *session.ToolInput, width int) string {
	var b strings.Builder

	filePath := getString(input.Parsed, "file_path")
	oldString := getString(input.Parsed, "old_string")
	newString := getString(input.Parsed, "new_string")
	replaceAll := getBool(input.Parsed, "replace_all")

	// File path with security check
	b.WriteString(LabelStyle().Render("File:"))
	b.WriteString("\n")
	if isSensitivePath(filePath) {
		b.WriteString(DangerStyle().Render("! " + filePath))
	} else {
		b.WriteString(PathStyle().Render(filePath))
	}
	b.WriteString("\n\n")

	// Show diff-like view
	b.WriteString(LabelStyle().Render("Change:"))
	b.WriteString("\n")

	// Old string (red/deletion style)
	if oldString != "" {
		b.WriteString(DeletionStyle().Render("- " + truncateMultiline(oldString, width-4, 5)))
		b.WriteString("\n")
	}

	// New string (green/addition style)
	if newString != "" {
		b.WriteString(AdditionStyle().Render("+ " + truncateMultiline(newString, width-4, 5)))
		b.WriteString("\n")
	}

	if replaceAll {
		b.WriteString("\n")
		b.WriteString(WarningStyle().Render("* Replaces ALL occurrences"))
		b.WriteString("\n")
	}

	// Tool result/output
	b.WriteString(formatResultSection(input, width))

	return b.String()
}

// formatWriteDetail renders Write tool details
func formatWriteDetail(input *session.ToolInput, width int) string {
	var b strings.Builder

	filePath := getString(input.Parsed, "file_path")
	content := getString(input.Parsed, "content")

	// Security warnings
	if isSensitivePath(filePath) {
		b.WriteString(DangerHeaderStyle().Render("! Writing to sensitive path"))
		b.WriteString("\n\n")
	}

	b.WriteString(LabelStyle().Render("File:"))
	b.WriteString("\n")
	b.WriteString(PathStyle().Render(filePath))
	b.WriteString("\n\n")

	b.WriteString(LabelStyle().Render("Content:"))
	fmt.Fprintf(&b, " (%d bytes)", len(content))
	b.WriteString("\n")
	b.WriteString(CodeBlockStyle(width).Render(truncateMultiline(content, width-4, 10)))
	b.WriteString("\n")

	// Tool result/output
	b.WriteString(formatResultSection(input, width))

	return b.String()
}

// formatReadDetail renders Read tool details
func formatReadDetail(input *session.ToolInput, width int) string {
	var b strings.Builder

	filePath := getString(input.Parsed, "file_path")
	offset := getFloat(input.Parsed, "offset")
	limit := getFloat(input.Parsed, "limit")

	// Security check
	if isSensitivePath(filePath) {
		b.WriteString(DangerHeaderStyle().Render("! Reading sensitive path"))
		b.WriteString("\n\n")
	}

	b.WriteString(LabelStyle().Render("File:"))
	b.WriteString("\n")
	b.WriteString(PathStyle().Render(filePath))
	b.WriteString("\n\n")

	if offset > 0 || limit > 0 {
		b.WriteString(LabelStyle().Render("Range:"))
		b.WriteString("\n")
		if offset > 0 {
			fmt.Fprintf(&b, "  Offset: %.0f\n", offset)
		}
		if limit > 0 {
			fmt.Fprintf(&b, "  Limit: %.0f lines\n", limit)
		}
	}

	// Tool result/output
	b.WriteString(formatResultSection(input, width))

	return b.String()
}

// formatGlobDetail renders Glob tool details
func formatGlobDetail(input *session.ToolInput, width int) string {
	var b strings.Builder

	pattern := getString(input.Parsed, "pattern")
	path := getString(input.Parsed, "path")

	b.WriteString(LabelStyle().Render("Pattern:"))
	b.WriteString("\n")
	b.WriteString(CodeBlockStyle(width).Render(pattern))
	b.WriteString("\n\n")

	if path != "" {
		b.WriteString(LabelStyle().Render("Path:"))
		b.WriteString("\n")
		b.WriteString(PathStyle().Render(path))
		b.WriteString("\n")
	}

	// Tool result/output
	b.WriteString(formatResultSection(input, width))

	return b.String()
}

// formatGrepDetail renders Grep tool details
func formatGrepDetail(input *session.ToolInput, width int) string {
	var b strings.Builder

	pattern := getString(input.Parsed, "pattern")
	path := getString(input.Parsed, "path")
	glob := getString(input.Parsed, "glob")
	fileType := getString(input.Parsed, "type")
	outputMode := getString(input.Parsed, "output_mode")

	b.WriteString(LabelStyle().Render("Pattern:"))
	b.WriteString("\n")
	b.WriteString(CodeBlockStyle(width).Render(pattern))
	b.WriteString("\n\n")

	if path != "" {
		b.WriteString(LabelStyle().Render("Path:"))
		b.WriteString("\n")
		b.WriteString(PathStyle().Render(path))
		b.WriteString("\n\n")
	}

	// Options
	var opts []string
	if glob != "" {
		opts = append(opts, "glob: "+glob)
	}
	if fileType != "" {
		opts = append(opts, "type: "+fileType)
	}
	if outputMode != "" {
		opts = append(opts, "mode: "+outputMode)
	}

	if len(opts) > 0 {
		b.WriteString(LabelStyle().Render("Options:"))
		b.WriteString("\n")
		b.WriteString(MutedStyle().Render(strings.Join(opts, ", ")))
		b.WriteString("\n")
	}

	// Tool result/output
	b.WriteString(formatResultSection(input, width))

	return b.String()
}

// formatTaskDetail renders Task tool details (subagent spawning)
func formatTaskDetail(input *session.ToolInput, width int) string {
	var b strings.Builder

	description := getString(input.Parsed, "description")
	prompt := getString(input.Parsed, "prompt")
	subagentType := getString(input.Parsed, "subagent_type")
	model := getString(input.Parsed, "model")

	// Security note for subagents
	if subagentType != "" {
		b.WriteString(WarningStyle().Render("* Spawns subagent: " + subagentType))
		b.WriteString("\n\n")
	}

	if description != "" {
		b.WriteString(LabelStyle().Render("Task:"))
		b.WriteString("\n")
		b.WriteString(wrapText(description, width-2))
		b.WriteString("\n\n")
	}

	if prompt != "" {
		b.WriteString(LabelStyle().Render("Prompt:"))
		b.WriteString("\n")
		b.WriteString(MutedStyle().Render(truncateMultiline(prompt, width-2, 8)))
		b.WriteString("\n\n")
	}

	if model != "" {
		b.WriteString(LabelStyle().Render("Model: "))
		b.WriteString(model)
		b.WriteString("\n")
	}

	// Tool result/output
	b.WriteString(formatResultSection(input, width))

	return b.String()
}

// formatWebDetail renders WebFetch/WebSearch tool details
func formatWebDetail(input *session.ToolInput, width int) string {
	var b strings.Builder

	url := getString(input.Parsed, "url")
	query := getString(input.Parsed, "query")
	prompt := getString(input.Parsed, "prompt")

	if url != "" {
		b.WriteString(LabelStyle().Render("URL:"))
		b.WriteString("\n")
		b.WriteString(PathStyle().Render(url))
		b.WriteString("\n\n")
	}

	if query != "" {
		b.WriteString(LabelStyle().Render("Query:"))
		b.WriteString("\n")
		b.WriteString(wrapText(query, width-2))
		b.WriteString("\n\n")
	}

	if prompt != "" {
		b.WriteString(LabelStyle().Render("Prompt:"))
		b.WriteString("\n")
		b.WriteString(MutedStyle().Render(truncateMultiline(prompt, width-2, 5)))
		b.WriteString("\n")
	}

	// Tool result/output
	b.WriteString(formatResultSection(input, width))

	return b.String()
}

// formatGenericDetail renders a generic tool detail view
func formatGenericDetail(input *session.ToolInput, width int) string {
	var b strings.Builder

	b.WriteString(LabelStyle().Render("Tool: "))
	b.WriteString(input.ToolName)
	b.WriteString("\n\n")

	// Show all parsed fields
	if len(input.Parsed) > 0 {
		b.WriteString(LabelStyle().Render("Parameters:"))
		b.WriteString("\n")
		for key, value := range input.Parsed {
			valueStr := fmt.Sprintf("%v", value)
			if len(valueStr) > width-4 {
				valueStr = valueStr[:width-7] + "..."
			}
			fmt.Fprintf(&b, "  %s: %s\n", key, valueStr)
		}
	}

	// Tool result/output
	b.WriteString(formatResultSection(input, width))

	return b.String()
}

// sensitivePatterns contains path patterns that indicate security-sensitive files.
// Defined at package level to avoid allocation on each isSensitivePath call.
var sensitivePatterns = []string{
	"/etc/", "/usr/", "/bin/", "/sbin/",
	".ssh/", ".gnupg/", ".aws/",
	".env", "credentials", "secrets",
	"/root/", "sudoers", "passwd", "shadow",
}

// isSensitivePath checks if a path is security-sensitive
func isSensitivePath(path string) bool {
	pathLower := strings.ToLower(path)
	for _, s := range sensitivePatterns {
		if strings.Contains(pathLower, s) {
			return true
		}
	}
	return false
}

// Helper functions for parsing input

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// wrapText wraps text at word boundaries to fit within width
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		words := strings.Fields(line)
		lineLen := 0

		for j, word := range words {
			wordLen := len(word)
			if lineLen+wordLen+1 > width && lineLen > 0 {
				result.WriteString("\n")
				lineLen = 0
			}
			if lineLen > 0 {
				result.WriteString(" ")
				lineLen++
			}
			// Truncate very long words
			if wordLen > width {
				word = word[:width-3] + "..."
				wordLen = width
			}
			result.WriteString(word)
			lineLen += wordLen
			_ = j // satisfy linter
		}
	}

	return result.String()
}

// truncateMultiline truncates text to maxLines and width
func truncateMultiline(text string, width, maxLines int) string {
	lines := strings.Split(text, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, "...")
	}
	for i, line := range lines {
		// Replace tabs with spaces for consistent display
		line = strings.ReplaceAll(line, "\t", "  ")
		if len(line) > width {
			lines[i] = line[:width-3] + "..."
		} else {
			lines[i] = line
		}
	}
	return strings.Join(lines, "\n")
}

// formatResultSection renders the tool result/output section if available
func formatResultSection(input *session.ToolInput, width int) string {
	if input.Result == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")

	if input.IsError {
		b.WriteString(DangerHeaderStyle().Render("Output (Error):"))
	} else {
		b.WriteString(LabelStyle().Render("Output:"))
	}
	b.WriteString("\n")

	// Truncate long results
	result := truncateMultiline(input.Result, width-4, 8)
	if input.IsError {
		b.WriteString(DangerStyle().Render(result))
	} else {
		b.WriteString(CodeBlockStyle(width).Render(result))
	}
	b.WriteString("\n")

	return b.String()
}
