package session

import (
	"cc_session_mon/internal/config"
	"strings"
)

// subcommandDepth defines how many subcommand levels to capture for each command.
// Commands not in this map get depth 0 (command only, no subcommands).
var subcommandDepth = map[string]int{
	// Version control
	"git": 1,

	// Storage
	"zfs":   1,
	"zpool": 1,

	// Containers/VMs
	"incus":   1,
	"lxc":     1,
	"podman":  1,
	"docker":  1,
	"kubectl": 1,
	"helm":    1,

	// System services
	"systemctl": 1,
	"launchctl": 1,

	// Nix ecosystem
	"nix":           1,
	"nixos-rebuild": 1,
	"home-manager":  1,

	// Build tools
	"go":    1,
	"cargo": 1,
	"npm":   1,
	"yarn":  1,
	"pnpm":  1,
	"pip":   1,
	"uv":    1,
	"make":  1,

	// GitHub CLI
	"gh": 1,

	// Terminal multiplexer
	"tmux": 1,

	// macOS defaults
	"defaults": 1,

	// Database tools
	"alembic": 1,
}

// ShouldInclude returns true if the pattern should be included in the display
func ShouldInclude(pattern string) bool {
	return !config.Global().ShouldExclude(pattern)
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
// Pattern format: Bash([sudo:]<command>[:<subcommand>]:*)
func extractBashPattern(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return "Bash"
	}

	words := strings.Fields(command)
	if len(words) == 0 {
		return "Bash"
	}

	// Skip environment variable assignments (FOO=bar command)
	words = skipEnvVars(words)
	if len(words) == 0 {
		return "Bash"
	}

	// Check for sudo prefix and preserve it
	hasSudo := false
	if words[0] == "sudo" {
		hasSudo = true
		words = skipSudoFlags(words[1:])
		if len(words) == 0 {
			return "Bash(sudo:*)"
		}
	}

	// Handle command wrappers (env, time, nice, etc.)
	words = unwrapCommand(words)
	if len(words) == 0 {
		if hasSudo {
			return "Bash(sudo:*)"
		}
		return "Bash"
	}

	// Handle shell -c "subcommand"
	if isShell(words[0]) {
		words = extractShellCommand(words)
		if len(words) == 0 {
			if hasSudo {
				return "Bash(sudo:*)"
			}
			return "Bash"
		}
	}

	// Build pattern parts
	var parts []string
	if hasSudo {
		parts = append(parts, "sudo")
	}

	cmd := words[0]
	parts = append(parts, cmd)

	// Check if command has subcommands to capture
	depth := subcommandDepth[cmd]
	if depth > 0 && len(words) > 1 {
		subArgs := words[1:]
		for i := 0; i < depth && len(subArgs) > 0; i++ {
			// Skip flags to find the subcommand
			for len(subArgs) > 0 && strings.HasPrefix(subArgs[0], "-") {
				subArgs = subArgs[1:]
			}
			if len(subArgs) == 0 {
				break
			}
			parts = append(parts, subArgs[0])
			subArgs = subArgs[1:]
		}
	}

	return "Bash(" + strings.Join(parts, ":") + ":*)"
}

// skipEnvVars skips environment variable assignments at the start of a command
func skipEnvVars(words []string) []string {
	for len(words) > 0 && strings.Contains(words[0], "=") && !strings.HasPrefix(words[0], "-") {
		words = words[1:]
	}
	return words
}

// skipSudoFlags advances past sudo flags and returns remaining words
func skipSudoFlags(words []string) []string {
	for len(words) > 0 {
		w := words[0]
		if !strings.HasPrefix(w, "-") {
			return words
		}
		// Flags that take an argument
		if w == "-u" || w == "-g" || w == "-C" || w == "-D" || w == "-h" || w == "-p" {
			if len(words) > 1 {
				words = words[2:]
			} else {
				words = words[1:]
			}
		} else {
			words = words[1:]
		}
	}
	return words
}

// unwrapCommand handles command wrappers like env, time, nice, etc.
func unwrapCommand(words []string) []string {
	if len(words) == 0 {
		return words
	}

	switch words[0] {
	case "env":
		// env VAR=val command or env -i command
		for i := 1; i < len(words); i++ {
			if strings.Contains(words[i], "=") || strings.HasPrefix(words[i], "-") {
				continue
			}
			return words[i:]
		}
		return nil
	case "time", "nohup", "strace", "ltrace":
		// Simple wrappers that take a command directly
		if len(words) > 1 {
			return words[1:]
		}
		return nil
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
			return words[i:]
		}
		return nil
	case "xargs":
		// Find the command after xargs flags
		for i := 1; i < len(words); i++ {
			if !strings.HasPrefix(words[i], "-") {
				return words[i:]
			}
		}
		return nil
	}

	return words
}

// isShell returns true if the command is a shell
func isShell(cmd string) bool {
	return cmd == "bash" || cmd == "sh" || cmd == "zsh"
}

// extractShellCommand extracts the command from "sh -c 'command'"
func extractShellCommand(words []string) []string {
	for i := 1; i < len(words); i++ {
		if words[i] == "-c" && i+1 < len(words) {
			subCmd := strings.TrimSpace(words[i+1])
			// Strip surrounding quotes if present
			subCmd = strings.Trim(subCmd, "'\"")
			return strings.Fields(subCmd)
		}
	}
	return words // Return original if no -c found
}
