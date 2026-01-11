package session

import (
	"testing"

	"cc_session_mon/internal/config"
)

func TestExtractPattern(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    string
		expected string
	}{
		// === Basic commands without subcommands ===
		{"simple ls", "Bash", "ls -la", "Bash(ls:*)"},
		{"cat file", "Bash", "cat /etc/passwd", "Bash(cat:*)"},
		{"rm file", "Bash", "rm -rf /tmp/foo", "Bash(rm:*)"},

		// === Sudo preservation ===
		{"sudo rm", "Bash", "sudo rm -rf /tmp/foo", "Bash(sudo:rm:*)"},
		{"sudo with -u flag", "Bash", "sudo -u root apt update", "Bash(sudo:apt:*)"},
		{"sudo with -E flag", "Bash", "sudo -E npm install", "Bash(sudo:npm:install:*)"},
		{"sudo only", "Bash", "sudo", "Bash(sudo:*)"},
		{"sudo with flags only", "Bash", "sudo -u root", "Bash(sudo:*)"},

		// === Git subcommands ===
		{"git status", "Bash", "git status", "Bash(git:status:*)"},
		{"git push", "Bash", "git push origin main", "Bash(git:push:*)"},
		{"git push --force", "Bash", "git push --force origin main", "Bash(git:push:*)"},
		{"git reset --hard", "Bash", "git reset --hard HEAD~1", "Bash(git:reset:*)"},
		{"git log", "Bash", "git log --oneline -10", "Bash(git:log:*)"},
		{"git commit", "Bash", "git commit -m 'message'", "Bash(git:commit:*)"},
		{"git diff", "Bash", "git diff HEAD~1", "Bash(git:diff:*)"},
		{"git add", "Bash", "git add .", "Bash(git:add:*)"},
		{"git clone", "Bash", "git clone https://github.com/user/repo", "Bash(git:clone:*)"},
		{"git with -C flag", "Bash", "git -C /path status", "Bash(git:/path:*)"}, // -C takes an arg, so /path is captured

		// === ZFS/ZPool ===
		{"zfs destroy", "Bash", "zfs destroy tank/data", "Bash(zfs:destroy:*)"},
		{"zfs list", "Bash", "zfs list -t snapshot", "Bash(zfs:list:*)"},
		{"zpool status", "Bash", "zpool status", "Bash(zpool:status:*)"},
		{"zpool destroy", "Bash", "zpool destroy tank", "Bash(zpool:destroy:*)"},
		{"sudo zfs destroy", "Bash", "sudo zfs destroy pool/data", "Bash(sudo:zfs:destroy:*)"},
		{"sudo zfs list", "Bash", "sudo zfs list", "Bash(sudo:zfs:list:*)"},

		// === Container/VM tools ===
		{"incus exec", "Bash", "incus exec container -- bash", "Bash(incus:exec:*)"},
		{"incus list", "Bash", "incus list", "Bash(incus:list:*)"},
		{"sudo incus exec", "Bash", "sudo incus exec container -- bash", "Bash(sudo:incus:exec:*)"},
		{"docker run", "Bash", "docker run -it ubuntu bash", "Bash(docker:run:*)"},
		{"docker rm", "Bash", "docker rm -f container", "Bash(docker:rm:*)"},
		{"podman rm", "Bash", "podman rm -f container", "Bash(podman:rm:*)"},
		{"kubectl get", "Bash", "kubectl get pods -n kube-system", "Bash(kubectl:get:*)"},
		{"kubectl delete", "Bash", "kubectl delete pod nginx", "Bash(kubectl:delete:*)"},
		{"helm install", "Bash", "helm install myapp ./chart", "Bash(helm:install:*)"},

		// === System services ===
		{"systemctl status", "Bash", "systemctl status nginx", "Bash(systemctl:status:*)"},
		{"systemctl restart", "Bash", "systemctl restart nginx", "Bash(systemctl:restart:*)"},
		{"sudo systemctl stop", "Bash", "sudo systemctl stop nginx", "Bash(sudo:systemctl:stop:*)"},

		// === Build tools with subcommands ===
		{"go build", "Bash", "go build ./...", "Bash(go:build:*)"},
		{"go test", "Bash", "go test -v ./...", "Bash(go:test:*)"},
		{"go mod tidy", "Bash", "go mod tidy", "Bash(go:mod:*)"},
		{"cargo build", "Bash", "cargo build --release", "Bash(cargo:build:*)"},
		{"cargo test", "Bash", "cargo test", "Bash(cargo:test:*)"},
		{"npm install", "Bash", "npm install express", "Bash(npm:install:*)"},
		{"npm run", "Bash", "npm run build", "Bash(npm:run:*)"},
		{"make build", "Bash", "make build", "Bash(make:build:*)"},
		{"make test", "Bash", "make test", "Bash(make:test:*)"},
		{"pip install", "Bash", "pip install requests", "Bash(pip:install:*)"},

		// === GitHub CLI ===
		{"gh pr create", "Bash", "gh pr create --title 'PR'", "Bash(gh:pr:*)"},
		{"gh issue list", "Bash", "gh issue list", "Bash(gh:issue:*)"},

		// === Tmux ===
		{"tmux new-session", "Bash", "tmux new-session -s main", "Bash(tmux:new-session:*)"},
		{"tmux kill-session", "Bash", "tmux kill-session -t old", "Bash(tmux:kill-session:*)"},
		{"tmux send-keys", "Bash", "tmux send-keys -t session 'cmd' Enter", "Bash(tmux:send-keys:*)"},

		// === Nix ===
		{"nix build", "Bash", "nix build .#package", "Bash(nix:build:*)"},
		{"nix develop", "Bash", "nix develop", "Bash(nix:develop:*)"},
		{"nix flake", "Bash", "nix flake update", "Bash(nix:flake:*)"},
		{"nixos-rebuild switch", "Bash", "nixos-rebuild switch", "Bash(nixos-rebuild:switch:*)"},

		// === macOS defaults ===
		{"defaults read", "Bash", "defaults read com.apple.dock", "Bash(defaults:read:*)"},
		{"defaults write", "Bash", "defaults write com.apple.dock autohide -bool true", "Bash(defaults:write:*)"},

		// === Env var prefixes ===
		{"env var prefix", "Bash", "FOO=bar npm run build", "Bash(npm:run:*)"},
		{"multiple env vars", "Bash", "FOO=1 BAR=2 go test ./...", "Bash(go:test:*)"},

		// === Command wrappers ===
		{"time wrapper", "Bash", "time make build", "Bash(make:build:*)"},
		{"nice wrapper", "Bash", "nice -n 10 cargo build --release", "Bash(cargo:build:*)"},
		{"env wrapper", "Bash", "env FOO=bar npm run test", "Bash(npm:run:*)"},

		// === Shell -c ===
		// Note: In real JSON-decoded commands, the -c argument is a single string
		// strings.Fields("bash -c 'git status'") produces ["bash", "-c", "'git", "status'"]
		// which is not how real commands come through. This tests the realistic case:
		{"bash -c no quotes", "Bash", "bash -c git", "Bash(git:*)"},
		{"sh -c no quotes", "Bash", "sh -c ls", "Bash(ls:*)"},

		// === Empty/edge cases ===
		{"empty command", "Bash", "", "Bash"},
		{"whitespace only", "Bash", "   ", "Bash"},

		// === Non-Bash tools (unchanged) ===
		{"Edit tool", "Edit", "/path/to/file.go", "Edit"},
		{"Write tool", "Write", "/path/to/file.go", "Write"},
		{"NotebookEdit tool", "NotebookEdit", "/path/to/notebook.ipynb", "NotebookEdit"},

		// === Unknown tool ===
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

func TestShouldInclude(t *testing.T) {
	// Set up a known config for testing
	config.SetGlobal(&config.Config{
		ToolGroups: []config.ToolGroup{
			{
				Name:     "excluded",
				Exclude:  true,
				Patterns: []string{"Read", "Glob", "Grep", "WebFetch"},
			},
			{
				Name:     "bash",
				Color:    "yellow",
				Patterns: []string{"Bash(*)"},
			},
		},
	})

	tests := []struct {
		pattern  string
		expected bool
	}{
		// Excluded patterns
		{"Read", false},
		{"Grep", false},
		{"Glob", false},
		{"WebFetch", false},

		// Included patterns
		{"Bash(ls:la)", true},
		{"Bash(git:status:*)", true},
		{"Bash(sudo:rm:*)", true},
		{"Edit", true},
		{"Write", true},
		{"NotebookEdit", true},
		{"SomeNewTool", true}, // Unknown tools are included
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := ShouldInclude(tt.pattern)
			if result != tt.expected {
				t.Errorf("ShouldInclude(%q) = %v, want %v",
					tt.pattern, result, tt.expected)
			}
		})
	}
}
