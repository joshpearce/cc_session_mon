package session

import "testing"

func TestExtractPattern(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    string
		expected string
	}{
		// Basic Bash commands
		{"simple git", "Bash", "git status", "Bash(git:*)"},
		{"simple ls", "Bash", "ls -la", "Bash(ls:*)"},
		{"npm install", "Bash", "npm install express", "Bash(npm:*)"},

		// Sudo handling
		{"sudo rm", "Bash", "sudo rm -rf /tmp/foo", "Bash(rm:*)"},
		{"sudo with flags", "Bash", "sudo -u root apt update", "Bash(apt:*)"},

		// Env var prefixes
		{"env var prefix", "Bash", "FOO=bar npm run build", "Bash(npm:*)"},
		{"multiple env vars", "Bash", "FOO=1 BAR=2 node server.js", "Bash(node:*)"},

		// Command wrappers
		{"time wrapper", "Bash", "time make build", "Bash(make:*)"},
		{"nice wrapper", "Bash", "nice -n 10 cargo build", "Bash(cargo:*)"},

		// Empty/edge cases
		{"empty command", "Bash", "", "Bash"},
		{"whitespace only", "Bash", "   ", "Bash"},

		// Non-Bash tools
		{"Edit tool", "Edit", "/path/to/file.go", "Edit"},
		{"Write tool", "Write", "/path/to/file.go", "Write"},
		{"NotebookEdit tool", "NotebookEdit", "/path/to/notebook.ipynb", "NotebookEdit"},

		// Unknown tool
		{"unknown tool", "Unknown", "something", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPattern(tt.toolName, tt.input)
			if result != tt.expected {
				t.Errorf("ExtractPattern(%q, %q) = %q, want %q",
					tt.toolName, tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsWriteOperation(t *testing.T) {
	tests := []struct {
		toolName string
		expected bool
	}{
		{"Bash", true},
		{"Edit", true},
		{"Write", true},
		{"NotebookEdit", true},
		{"Read", false},
		{"Grep", false},
		{"Glob", false},
		{"WebFetch", false},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := IsWriteOperation(tt.toolName)
			if result != tt.expected {
				t.Errorf("IsWriteOperation(%q) = %v, want %v",
					tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestIsDangerousPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		expected bool
	}{
		{"Bash(rm:*)", true},
		{"Bash(sudo:*)", true},
		{"Bash(chmod:*)", true},
		{"Bash(git:*)", false},
		{"Bash(ls:*)", false},
		{"Edit", false},
		{"Write", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := IsDangerousPattern(tt.pattern)
			if result != tt.expected {
				t.Errorf("IsDangerousPattern(%q) = %v, want %v",
					tt.pattern, result, tt.expected)
			}
		})
	}
}
