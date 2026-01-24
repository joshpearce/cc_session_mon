package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Should have some tool groups
	if len(cfg.ToolGroups) == 0 {
		t.Error("DefaultConfig should have some tool groups")
	}

	// First group should be dangerous
	if cfg.ToolGroups[0].Name != "dangerous" {
		t.Errorf("Expected first group to be 'dangerous', got %q", cfg.ToolGroups[0].Name)
	}

	// Last group should be unmatched wildcard
	last := cfg.ToolGroups[len(cfg.ToolGroups)-1]
	if last.Name != "unmatched" {
		t.Errorf("Expected last group to be 'unmatched', got %q", last.Name)
	}
	if len(last.Patterns) != 1 || last.Patterns[0] != "*" {
		t.Error("unmatched group should have single '*' pattern")
	}
}

func TestGetToolGroup(t *testing.T) {
	cfg := &Config{
		ToolGroups: []ToolGroup{
			{
				Name:     "excluded",
				Exclude:  true,
				Patterns: []string{"Read", "Glob"},
			},
			{
				Name:     "dangerous",
				Color:    "red",
				Bold:     true,
				Patterns: []string{"Bash(rm:*)", "Bash(sudo:*)"},
			},
			{
				Name:     "bash",
				Color:    "yellow",
				Patterns: []string{"Bash(*)"},
			},
			{
				Name:     "edit",
				Color:    "green",
				Patterns: []string{"Edit"},
			},
		},
	}

	tests := []struct {
		name        string
		pattern     string
		expectGroup string
		expectNil   bool
	}{
		{"excluded read", "Read", "excluded", false},
		{"excluded glob", "Glob", "excluded", false},
		{"dangerous rm", "Bash(rm:rf)", "dangerous", false},
		{"dangerous sudo", "Bash(sudo:apt)", "dangerous", false},
		{"normal bash", "Bash(ls:la)", "bash", false},
		{"edit tool", "Edit", "edit", false},
		{"unknown tool", "Unknown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := cfg.GetToolGroup(tt.pattern)
			if tt.expectNil {
				if group != nil {
					t.Errorf("GetToolGroup(%q) = %v, want nil", tt.pattern, group.Name)
				}
			} else {
				if group == nil {
					t.Errorf("GetToolGroup(%q) = nil, want %q", tt.pattern, tt.expectGroup)
				} else if group.Name != tt.expectGroup {
					t.Errorf("GetToolGroup(%q) = %q, want %q", tt.pattern, group.Name, tt.expectGroup)
				}
			}
		})
	}
}

func TestShouldExclude(t *testing.T) {
	cfg := &Config{
		ToolGroups: []ToolGroup{
			{
				Name:     "excluded",
				Exclude:  true,
				Patterns: []string{"Read", "Glob", "mcp__*"},
			},
			{
				Name:     "bash",
				Color:    "yellow",
				Patterns: []string{"Bash(*)"},
			},
		},
	}

	tests := []struct {
		pattern  string
		expected bool
	}{
		{"Read", true},
		{"Glob", true},
		{"mcp__ide__getDiagnostics", true},
		{"Bash(ls:la)", false},
		{"Edit", false},
		{"Unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := cfg.ShouldExclude(tt.pattern)
			if result != tt.expected {
				t.Errorf("ShouldExclude(%q) = %v, want %v", tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		value    string
		expected bool
	}{
		{"Bash(rm:*)", "Bash(rm:rf)", true},
		{"Bash(rm:*)", "Bash(rm:file.txt)", true},
		{"Bash(rm:*)", "Bash(sudo:rm)", false},
		{"Bash(git:*)", "Bash(git:commit)", true},
		{"Bash(*)", "Bash(ls:la)", true},
		{"mcp__*", "mcp__ide__getDiagnostics", true},
		{"exact", "exact", true},
		{"exact", "exactlynot", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.value, func(t *testing.T) {
			result := matchPattern(tt.pattern, tt.value)
			if result != tt.expected {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.value, result, tt.expected)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `theme: latte

tool_groups:
  - name: custom
    color: pink
    bold: true
    patterns:
      - "CustomPattern(*)"
  - name: hidden
    exclude: true
    patterns:
      - HiddenTool
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check theme
	if cfg.Theme != "latte" {
		t.Errorf("Expected theme 'latte', got %q", cfg.Theme)
	}

	// Check tool groups were loaded
	if len(cfg.ToolGroups) != 2 {
		t.Errorf("Expected 2 tool groups, got %d", len(cfg.ToolGroups))
	}
	if cfg.ToolGroups[0].Name != "custom" {
		t.Errorf("Expected group name 'custom', got %q", cfg.ToolGroups[0].Name)
	}
	if cfg.ToolGroups[0].Color != "pink" {
		t.Errorf("Expected group color 'pink', got %q", cfg.ToolGroups[0].Color)
	}
	if !cfg.ToolGroups[0].Bold {
		t.Error("Expected group bold to be true")
	}
	if cfg.ToolGroups[1].Name != "hidden" {
		t.Errorf("Expected second group name 'hidden', got %q", cfg.ToolGroups[1].Name)
	}
	if !cfg.ToolGroups[1].Exclude {
		t.Error("Expected second group exclude to be true")
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load() should not error for missing file, got: %v", err)
	}

	// Should return defaults
	if len(cfg.ToolGroups) == 0 {
		t.Error("Should return default config with tool groups")
	}
}

func TestSetGlobal(t *testing.T) {
	custom := &Config{
		Theme: "custom",
		ToolGroups: []ToolGroup{
			{Name: "test", Patterns: []string{"Test"}},
		},
	}

	SetGlobal(custom)
	got := Global()

	if got.Theme != "custom" {
		t.Error("SetGlobal did not set the global config correctly")
	}

	// Reset to nil so other tests use defaults
	SetGlobal(nil)
}
