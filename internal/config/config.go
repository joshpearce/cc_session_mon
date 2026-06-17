package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultBurnWindow is the trailing window used for burn-rate calculation when
// none is configured.
const DefaultBurnWindow = 10 * time.Minute

// ToolGroup defines a group of patterns with styling
type ToolGroup struct {
	// Name is the display name of this group
	Name string `yaml:"name"`

	// Color is the catppuccin color name (e.g., "red", "yellow", "green", "mauve")
	Color string `yaml:"color"`

	// Bold makes the text bold
	Bold bool `yaml:"bold"`

	// Patterns is a list of command patterns that belong to this group (supports wildcards)
	Patterns []string `yaml:"patterns"`

	// Exclude if true, commands matching this group are excluded from display entirely
	Exclude bool `yaml:"exclude"`
}

// Config holds the application configuration
type Config struct {
	// Theme is the color theme to use (mocha, macchiato, frappe, latte)
	Theme string `yaml:"theme"`

	// ToolGroups defines styling groups for commands (checked in order, first match wins)
	ToolGroups []ToolGroup `yaml:"tool_groups"`

	// SearchPaths are roots scanned recursively at startup for nested
	// .claude/projects directories (e.g. devcontainer mounts). The user's
	// local Claude projects directory is always watched in addition to these.
	SearchPaths []string `yaml:"search_paths"`

	// BurnWindowMinutes is the trailing window, in minutes, over which the token
	// burn rate is averaged (measured back from a session's last action). Zero
	// or negative falls back to the default window.
	BurnWindowMinutes int `yaml:"burn_window_minutes"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Theme: "mocha",
		ToolGroups: []ToolGroup{
			{
				Name:  "dangerous",
				Color: "red",
				Bold:  true,
				Patterns: []string{
					"Bash(rm:*)",
					"Bash(sudo:*)",
					"Bash(chmod:*)",
					"Bash(chown:*)",
					"Bash(dd:*)",
					"Bash(mkfs:*)",
					"Bash(kill:*)",
					"Bash(pkill:*)",
					"Bash(killall:*)",
				},
			},
			{
				Name:     "write",
				Color:    "peach",
				Patterns: []string{"Write", "NotebookEdit"},
			},
			{
				Name:     "edit",
				Color:    "yellow",
				Patterns: []string{"Edit"},
			},
			{
				Name:     "bash",
				Color:    "mauve",
				Patterns: []string{"Bash(*)"},
			},
			{
				Name:     "task",
				Color:    "lavender",
				Patterns: []string{"Task", "TaskOutput"},
			},
			{
				Name:  "read-only",
				Color: "green",
				Patterns: []string{
					"Read",
					"Glob",
					"Grep",
					"WebFetch",
					"WebSearch",
					"TodoRead",
					"AskUserQuestion",
					"mcp__*",
				},
			},
			{
				Name:     "unmatched",
				Color:    "overlay1",
				Patterns: []string{"*"},
			},
		},
	}
}

// BurnWindow returns the configured burn-rate window as a duration, falling
// back to DefaultBurnWindow when unset or non-positive.
func (c *Config) BurnWindow() time.Duration {
	if c.BurnWindowMinutes <= 0 {
		return DefaultBurnWindow
	}
	return time.Duration(c.BurnWindowMinutes) * time.Minute
}

// Load reads the config from a YAML file, falling back to defaults
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath) //nolint:gosec // config path from known locations
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Use defaults if no config file
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadFromDefaultPath attempts to load config from standard locations
func LoadFromDefaultPath() (*Config, error) {
	// Check in order: current dir, ~/.config/cc_session_mon/, XDG_CONFIG_HOME
	paths := []string{
		"config.yaml",
		filepath.Join(os.Getenv("HOME"), ".config", "cc_session_mon", "config.yaml"),
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "cc_session_mon", "config.yaml"))
	}

	for _, path := range paths {
		cleanPath := filepath.Clean(path)
		if _, err := os.Stat(cleanPath); err == nil { //nolint:gosec // config path from known locations
			return Load(cleanPath)
		}
	}

	return DefaultConfig(), nil
}

// GetToolGroup returns the first matching tool group for a pattern, or nil
func (c *Config) GetToolGroup(pattern string) *ToolGroup {
	for i := range c.ToolGroups {
		group := &c.ToolGroups[i]
		if group.Matches(pattern) {
			return group
		}
	}
	return nil
}

// Matches returns true if the pattern matches this group
func (g *ToolGroup) Matches(pattern string) bool {
	for _, p := range g.Patterns {
		if matchPattern(p, pattern) {
			return true
		}
	}
	return false
}

// ShouldExclude returns true if the pattern should be excluded from display
func (c *Config) ShouldExclude(pattern string) bool {
	group := c.GetToolGroup(pattern)
	return group != nil && group.Exclude
}

// matchPattern checks if a pattern matches (supports * wildcards)
func matchPattern(pattern, value string) bool {
	// Exact match
	if pattern == value {
		return true
	}

	// Wildcard match - supports single * anywhere in pattern
	// e.g., "Bash(rm:*)" matches "Bash(rm:rf)" and "Bash(rm:file.txt)"
	if strings.Contains(pattern, "*") {
		parts := strings.SplitN(pattern, "*", 2)
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			return strings.HasPrefix(value, prefix) && strings.HasSuffix(value, suffix)
		}
	}

	return false
}

// global config instance
var globalConfig *Config

// Global returns the global config instance, loading it if necessary
func Global() *Config {
	if globalConfig == nil {
		cfg, err := LoadFromDefaultPath()
		if err != nil {
			cfg = DefaultConfig()
		}
		globalConfig = cfg
	}
	return globalConfig
}

// SetGlobal sets the global config instance (useful for testing)
func SetGlobal(cfg *Config) {
	globalConfig = cfg
}
