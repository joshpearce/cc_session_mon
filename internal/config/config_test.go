package config

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"gopkg.in/yaml.v3"
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

func TestDefaultConfigAlertRules(t *testing.T) {
	cfg := DefaultConfig()

	// Exactly one alert rule ships by default.
	if len(cfg.Alerts) != 1 {
		t.Fatalf("DefaultConfig() should have exactly 1 alert rule, got %d", len(cfg.Alerts))
	}

	rule := cfg.Alerts[0]
	if rule.Metric != "active_subagents" {
		t.Errorf("default alert rule Metric = %q, want %q", rule.Metric, "active_subagents")
	}
	if rule.AlertThreshold != 20 {
		t.Errorf("default alert rule AlertThreshold = %v, want 20", rule.AlertThreshold)
	}
	if rule.ActionThreshold != 40 {
		t.Errorf("default alert rule ActionThreshold = %v, want 40", rule.ActionThreshold)
	}
	if rule.ActionSustainedTicks != 1 {
		t.Errorf("default alert rule ActionSustainedTicks = %v, want 1", rule.ActionSustainedTicks)
	}

	// Corrective actions are opt-in and must be off by default.
	if cfg.EnableCorrectiveActions {
		t.Error("DefaultConfig() EnableCorrectiveActions should be false")
	}
	if cfg.ActionDryRun {
		t.Error("DefaultConfig() ActionDryRun should be false")
	}

	// Action-path defaults must be non-empty.
	if cfg.DevcontainerFilterRelPath != "proxy/filter.py" {
		t.Errorf("DevcontainerFilterRelPath = %q, want %q", cfg.DevcontainerFilterRelPath, "proxy/filter.py")
	}
	if cfg.AnthropicAllowPattern == "" {
		t.Error("AnthropicAllowPattern should be non-empty")
	}

	// The default AnthropicAllowPattern must compile as a valid Go regex.
	if _, err := regexp.Compile(cfg.AnthropicAllowPattern); err != nil {
		t.Errorf("default AnthropicAllowPattern must compile: %v", err)
	}
}

func TestLoadOmittedAlertsKeepsDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write a config file that sets only theme — no alerts key at all.
	content := "theme: mocha\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// The default active_subagents rule must survive.
	if len(cfg.Alerts) != 1 {
		t.Fatalf("Load() with no alerts key should keep 1 default rule, got %d", len(cfg.Alerts))
	}
	rule := cfg.Alerts[0]
	if rule.Metric != "active_subagents" {
		t.Errorf("surviving rule Metric = %q, want %q", rule.Metric, "active_subagents")
	}
	if rule.AlertThreshold != 20 {
		t.Errorf("surviving rule AlertThreshold = %v, want 20", rule.AlertThreshold)
	}
	if rule.ActionThreshold != 40 {
		t.Errorf("surviving rule ActionThreshold = %v, want 40", rule.ActionThreshold)
	}
	if rule.ActionSustainedTicks != 1 {
		t.Errorf("surviving rule ActionSustainedTicks = %v, want 1", rule.ActionSustainedTicks)
	}

	// Action flags still off.
	if cfg.EnableCorrectiveActions {
		t.Error("EnableCorrectiveActions should remain false when omitted from YAML")
	}
	if cfg.ActionDryRun {
		t.Error("ActionDryRun should remain false when omitted from YAML")
	}
}

func TestLoadMultiRuleYAMLRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `theme: mocha
enable_corrective_actions: true
action_dry_run: true
alerts:
  - metric: active_subagents
    alert_threshold: 20
    action_threshold: 40
    action_sustained_ticks: 1
  - metric: tokens_per_min_1m
    alert_threshold: 50000
    action_threshold: 120000
    action_sustained_ticks: 3
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// The YAML alerts slice replaces the default single-rule slice.
	if len(cfg.Alerts) != 2 {
		t.Fatalf("Load() with 2 alert rules should have 2 rules, got %d", len(cfg.Alerts))
	}

	type wantRule struct {
		metric               string
		alertThreshold       float64
		actionThreshold      float64
		actionSustainedTicks int
	}
	wants := []wantRule{
		{"active_subagents", 20, 40, 1},
		{"tokens_per_min_1m", 50000, 120000, 3},
	}

	for i, w := range wants {
		r := cfg.Alerts[i]
		if r.Metric != w.metric {
			t.Errorf("Alerts[%d].Metric = %q, want %q", i, r.Metric, w.metric)
		}
		if r.AlertThreshold != w.alertThreshold {
			t.Errorf("Alerts[%d].AlertThreshold = %v, want %v", i, r.AlertThreshold, w.alertThreshold)
		}
		if r.ActionThreshold != w.actionThreshold {
			t.Errorf("Alerts[%d].ActionThreshold = %v, want %v", i, r.ActionThreshold, w.actionThreshold)
		}
		if r.ActionSustainedTicks != w.actionSustainedTicks {
			t.Errorf("Alerts[%d].ActionSustainedTicks = %v, want %v", i, r.ActionSustainedTicks, w.actionSustainedTicks)
		}
	}

	if !cfg.EnableCorrectiveActions {
		t.Error("EnableCorrectiveActions should be true when set in YAML")
	}
	if !cfg.ActionDryRun {
		t.Error("ActionDryRun should be true when set in YAML")
	}

	// Round-trip: marshal the loaded config back to YAML, then unmarshal into a
	// fresh Config and verify that Alerts and the two action booleans survive
	// re-encoding unchanged.
	encoded, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	var roundTripped Config
	if err := yaml.Unmarshal(encoded, &roundTripped); err != nil {
		t.Fatalf("yaml.Unmarshal() after re-encode error = %v", err)
	}
	if !reflect.DeepEqual(cfg.Alerts, roundTripped.Alerts) {
		t.Errorf("Alerts did not survive round-trip:\n  before: %+v\n  after:  %+v", cfg.Alerts, roundTripped.Alerts)
	}
	if cfg.EnableCorrectiveActions != roundTripped.EnableCorrectiveActions {
		t.Errorf("EnableCorrectiveActions round-trip: before=%v, after=%v",
			cfg.EnableCorrectiveActions, roundTripped.EnableCorrectiveActions)
	}
	if cfg.ActionDryRun != roundTripped.ActionDryRun {
		t.Errorf("ActionDryRun round-trip: before=%v, after=%v",
			cfg.ActionDryRun, roundTripped.ActionDryRun)
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
