package session

import (
	"cc_session_mon/internal/config"
	"strings"
)

// IsWriteOperation returns true for operations that are NOT read-only
// Uses an exclude list: anything not in the read-only list is considered a write operation
func IsWriteOperation(toolName string) bool {
	return !config.Global().IsReadOnlyTool(toolName)
}

// IsDangerousPattern returns true for patterns that warrant extra attention
func IsDangerousPattern(pattern string) bool {
	return config.Global().IsDangerousPattern(pattern)
}

// ExtractPattern converts a tool call into Claude permission pattern format
func ExtractPattern(toolName, input string) string {
	switch toolName {
	case "Bash":
		return extractBashPattern(input)
	case "Edit", "Write", "NotebookEdit":
		return toolName
	default:
		return toolName
	}
}

// extractBashPattern extracts the command pattern from a bash command
func extractBashPattern(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return "Bash"
	}

	words := strings.Fields(command)
	if len(words) == 0 {
		return "Bash"
	}

	firstWord := words[0]

	// Skip environment variable assignments (FOO=bar command)
	for strings.Contains(firstWord, "=") && !strings.HasPrefix(firstWord, "-") {
		words = words[1:]
		if len(words) == 0 {
			return "Bash"
		}
		firstWord = words[0]
	}

	// Handle sudo - get the actual command
	if firstWord == "sudo" && len(words) > 1 {
		// Skip sudo flags like -u, -E, etc.
		for i := 1; i < len(words); i++ {
			if !strings.HasPrefix(words[i], "-") {
				firstWord = words[i]
				break
			}
			// Skip flag arguments (e.g., -u root)
			if words[i] == "-u" || words[i] == "-g" {
				i++ // skip the next argument too
			}
		}
	}

	// Handle common command wrappers
	switch firstWord {
	case "env":
		// env VAR=val command or env -i command
		for i := 1; i < len(words); i++ {
			if strings.Contains(words[i], "=") || strings.HasPrefix(words[i], "-") {
				continue
			}
			firstWord = words[i]
			break
		}
	case "time", "nohup", "strace", "ltrace":
		// Simple wrappers that take a command directly
		if len(words) > 1 {
			firstWord = words[1]
		}
	case "nice":
		// nice can have -n VALUE, skip flags and their arguments
		for i := 1; i < len(words); i++ {
			if words[i] == "-n" && i+1 < len(words) {
				i++ // skip the priority value
				continue
			}
			if strings.HasPrefix(words[i], "-") {
				continue
			}
			firstWord = words[i]
			break
		}
	case "xargs":
		// Find the command after xargs flags
		for i := 1; i < len(words); i++ {
			if !strings.HasPrefix(words[i], "-") {
				firstWord = words[i]
				break
			}
		}
	}

	// Handle shell built-ins that wrap commands
	if firstWord == "bash" || firstWord == "sh" || firstWord == "zsh" {
		// Check for -c flag
		for i := 1; i < len(words); i++ {
			if words[i] == "-c" && i+1 < len(words) {
				// Parse the command string
				subCmd := strings.TrimSpace(words[i+1])
				subWords := strings.Fields(subCmd)
				if len(subWords) > 0 {
					firstWord = subWords[0]
				}
				break
			}
		}
	}

	return "Bash(" + firstWord + ":*)"
}
