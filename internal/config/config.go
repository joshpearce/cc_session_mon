package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	// Theme is the color theme to use (mocha, macchiato, frappe, latte)
	Theme string `yaml:"theme"`

	// ReadOnlyTools is a list of tool names that are read-only and should be excluded
	ReadOnlyTools []string `yaml:"read_only_tools"`

	// DangerousPatterns is a list of patterns that warrant extra attention
	DangerousPatterns []string `yaml:"dangerous_patterns"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Theme: "mocha",
		ReadOnlyTools: []string{
			"Read",
			"Glob",
			"Grep",
			"WebFetch",
			"WebSearch",
			"Task",
			"TodoRead",
			"AskUserQuestion",
			"mcp__ide__getDiagnostics",
		},
		DangerousPatterns: []string{
			"Bash(rm:*)",
			"Bash(sudo:*)",
			"Bash(chmod:*)",
			"Bash(chown:*)",
			"Bash(mv:*)",
			"Bash(dd:*)",
			"Bash(mkfs:*)",
			"Bash(kill:*)",
		},
	}
}

// Load reads the config from a YAML file, falling back to defaults
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
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
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}

	return DefaultConfig(), nil
}

// IsReadOnlyTool returns true if the tool is in the read-only list
func (c *Config) IsReadOnlyTool(toolName string) bool {
	for _, t := range c.ReadOnlyTools {
		if t == toolName {
			return true
		}
	}
	return false
}

// IsDangerousPattern returns true if the pattern is in the dangerous list
func (c *Config) IsDangerousPattern(pattern string) bool {
	for _, p := range c.DangerousPatterns {
		if p == pattern {
			return true
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
