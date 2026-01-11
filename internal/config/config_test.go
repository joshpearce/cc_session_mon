package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Should have some default read-only tools
	if len(cfg.ReadOnlyTools) == 0 {
		t.Error("DefaultConfig should have some read-only tools")
	}

	// Read should be in the list
	found := false
	for _, tool := range cfg.ReadOnlyTools {
		if tool == "Read" {
			found = true
			break
		}
	}
	if !found {
		t.Error("DefaultConfig should include 'Read' in read-only tools")
	}

	// Should have some dangerous patterns
	if len(cfg.DangerousPatterns) == 0 {
		t.Error("DefaultConfig should have some dangerous patterns")
	}
}

func TestIsReadOnlyTool(t *testing.T) {
	cfg := &Config{
		ReadOnlyTools: []string{"Read", "Grep", "Glob"},
	}

	tests := []struct {
		tool     string
		expected bool
	}{
		{"Read", true},
		{"Grep", true},
		{"Glob", true},
		{"Write", false},
		{"Bash", false},
		{"Edit", false},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			result := cfg.IsReadOnlyTool(tt.tool)
			if result != tt.expected {
				t.Errorf("IsReadOnlyTool(%q) = %v, want %v", tt.tool, result, tt.expected)
			}
		})
	}
}

func TestIsDangerousPattern(t *testing.T) {
	cfg := &Config{
		DangerousPatterns: []string{"Bash(rm:*)", "Bash(sudo:*)"},
	}

	tests := []struct {
		pattern  string
		expected bool
	}{
		{"Bash(rm:*)", true},
		{"Bash(sudo:*)", true},
		{"Bash(git:*)", false},
		{"Edit", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := cfg.IsDangerousPattern(tt.pattern)
			if result != tt.expected {
				t.Errorf("IsDangerousPattern(%q) = %v, want %v", tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `read_only_tools:
  - CustomRead
  - CustomGrep

dangerous_patterns:
  - "Bash(danger:*)"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check read-only tools were loaded
	if len(cfg.ReadOnlyTools) != 2 {
		t.Errorf("Expected 2 read-only tools, got %d", len(cfg.ReadOnlyTools))
	}
	if cfg.ReadOnlyTools[0] != "CustomRead" {
		t.Errorf("Expected first tool to be 'CustomRead', got %q", cfg.ReadOnlyTools[0])
	}

	// Check dangerous patterns were loaded
	if len(cfg.DangerousPatterns) != 1 {
		t.Errorf("Expected 1 dangerous pattern, got %d", len(cfg.DangerousPatterns))
	}
	if cfg.DangerousPatterns[0] != "Bash(danger:*)" {
		t.Errorf("Expected pattern 'Bash(danger:*)', got %q", cfg.DangerousPatterns[0])
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load() should not error for missing file, got: %v", err)
	}

	// Should return defaults
	if len(cfg.ReadOnlyTools) == 0 {
		t.Error("Should return default config with read-only tools")
	}
}

func TestSetGlobal(t *testing.T) {
	custom := &Config{
		ReadOnlyTools: []string{"TestTool"},
	}

	SetGlobal(custom)
	got := Global()

	if len(got.ReadOnlyTools) != 1 || got.ReadOnlyTools[0] != "TestTool" {
		t.Error("SetGlobal did not set the global config correctly")
	}

	// Reset to nil so other tests use defaults
	SetGlobal(nil)
}
