package devagent

import (
	"testing"
)

func TestStripHostMntPrefix(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "with /host_mnt prefix",
			path:     "/host_mnt/Users/josh/.local/share/devagent/claude-configs/abc123/.claude",
			expected: "/Users/josh/.local/share/devagent/claude-configs/abc123/.claude",
		},
		{
			name:     "without /host_mnt prefix (Linux path)",
			path:     "/home/user/.local/share/devagent/claude-configs/abc123/.claude",
			expected: "/home/user/.local/share/devagent/claude-configs/abc123/.claude",
		},
		{
			name:     "empty string",
			path:     "",
			expected: "",
		},
		{
			name:     "only /host_mnt",
			path:     "/host_mnt",
			expected: "/host_mnt",
		},
		{
			name:     "path starting with /host_mnt but continues",
			path:     "/host_mnt/absolute/path",
			expected: "/absolute/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripHostMntPrefix(tt.path)
			if result != tt.expected {
				t.Errorf("stripHostMntPrefix(%q) = %q, want %q",
					tt.path, result, tt.expected)
			}
		})
	}
}

func TestParseOutput(t *testing.T) {
	tests := []struct {
		name      string
		jsonData  string
		wantLen   int
		wantErr   bool
		validate  func([]Environment)
	}{
		{
			name: "valid single environment",
			jsonData: `[
  {
    "project_path": "/Users/josh/code/my-project",
    "devcontainer": {
      "mounts": [
        {
          "type": "bind",
          "source": "/Users/josh/code/my-project/.devcontainer/home/vscode/.claude",
          "destination": "/home/vscode/.claude",
          "read_only": false
        }
      ]
    },
    "proxy_sidecar": {
      "container_name": "devagent-abc123-proxy",
      "state": "running"
    }
  }
]`,
			wantLen: 1,
			wantErr: false,
			validate: func(envs []Environment) {
				if len(envs) != 1 {
					t.Errorf("expected 1 environment, got %d", len(envs))
				}
				env := envs[0]
				if env.ContainerName != "devagent-abc123-proxy" {
					t.Errorf("expected container name 'devagent-abc123-proxy', got %q", env.ContainerName)
				}
				if env.ProjectPath != "/Users/josh/code/my-project" {
					t.Errorf("expected project path '/Users/josh/code/my-project', got %q", env.ProjectPath)
				}
				if env.ProjectsDir != "/Users/josh/code/my-project/.devcontainer/home/vscode/.claude/projects" {
					t.Errorf("expected projects dir '/Users/josh/code/my-project/.devcontainer/home/vscode/.claude/projects', got %q", env.ProjectsDir)
				}
				if env.State != "running" {
					t.Errorf("expected state 'running', got %q", env.State)
				}
			},
		},
		{
			name: "multiple environments with different states",
			jsonData: `[
  {
    "project_path": "/Users/josh/code/project1",
    "devcontainer": {
      "mounts": [
        {
          "type": "bind",
          "source": "/Users/josh/code/project1/.devcontainer/home/vscode/.claude",
          "destination": "/home/vscode/.claude",
          "read_only": false
        }
      ]
    },
    "proxy_sidecar": {
      "container_name": "devagent-abc123-proxy",
      "state": "running"
    }
  },
  {
    "project_path": "/Users/josh/code/project2",
    "devcontainer": {
      "mounts": [
        {
          "type": "bind",
          "source": "/Users/josh/code/project2/.devcontainer/home/vscode/.claude",
          "destination": "/home/vscode/.claude",
          "read_only": false
        }
      ]
    },
    "proxy_sidecar": {
      "container_name": "devagent-def456-proxy",
      "state": "stopped"
    }
  }
]`,
			wantLen: 2,
			wantErr: false,
			validate: func(envs []Environment) {
				if envs[0].State != "running" {
					t.Errorf("expected first env state 'running', got %q", envs[0].State)
				}
				if envs[1].State != "stopped" {
					t.Errorf("expected second env state 'stopped', got %q", envs[1].State)
				}
			},
		},
		{
			name: "Linux path without /host_mnt prefix",
			jsonData: `[
  {
    "project_path": "/home/user/my-project",
    "devcontainer": {
      "mounts": [
        {
          "type": "bind",
          "source": "/home/user/my-project/.devcontainer/home/vscode/.claude",
          "destination": "/home/vscode/.claude",
          "read_only": false
        }
      ]
    },
    "proxy_sidecar": {
      "container_name": "devagent-xyz789-proxy",
      "state": "running"
    }
  }
]`,
			wantLen: 1,
			wantErr: false,
			validate: func(envs []Environment) {
				env := envs[0]
				if env.ProjectsDir != "/home/user/my-project/.devcontainer/home/vscode/.claude/projects" {
					t.Errorf("expected projects dir '/home/user/my-project/.devcontainer/home/vscode/.claude/projects', got %q", env.ProjectsDir)
				}
			},
		},
		{
			name:     "empty array",
			jsonData: "[]",
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "container with no matching .claude mount (should be skipped)",
			jsonData: `[
  {
    "project_path": "/Users/josh/code/project",
    "devcontainer": {
      "mounts": [
        {
          "type": "bind",
          "source": "/host_mnt/some/other/mount",
          "destination": "/home/vscode/other",
          "read_only": false
        }
      ]
    },
    "proxy_sidecar": {
      "container_name": "devagent-xyz-proxy",
      "state": "running"
    }
  }
]`,
			wantLen: 0,
			wantErr: false,
		},
		{
			name:     "invalid JSON",
			jsonData: "not json",
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseOutput([]byte(tt.jsonData))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOutput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(result) != tt.wantLen {
				t.Errorf("ParseOutput() returned %d environments, want %d", len(result), tt.wantLen)
			}
			if tt.validate != nil && !tt.wantErr {
				tt.validate(result)
			}
		})
	}
}
