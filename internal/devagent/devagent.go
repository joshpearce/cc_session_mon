package devagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Container represents a devcontainer from devagent list output
type Container struct {
	ProjectPath  string        `json:"project_path"`
	DevContainer DevContainer  `json:"devcontainer"`
	ProxySidecar ProxySidecar  `json:"proxy_sidecar"`
}

// DevContainer contains mount information
type DevContainer struct {
	Mounts []Mount `json:"mounts"`
}

// Mount represents a devcontainer mount
type Mount struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ReadOnly    bool   `json:"read_only"`
}

// ProxySidecar contains proxy sidecar information
type ProxySidecar struct {
	ContainerName string `json:"container_name"`
	State         string `json:"state"`
}

// Environment represents an extracted devagent environment with host-side paths
type Environment struct {
	ContainerName string
	ProjectPath   string
	ProjectsDir   string
	State         string
}

// Discover runs devagent list and returns available environments.
// If devagent is not installed or errors, returns the error.
func Discover() ([]Environment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "devagent", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run devagent list: %w", err)
	}
	return ParseOutput(output)
}

// ParseOutput parses JSON output from devagent list.
// Returns all environments regardless of container state.
// For each container, finds mount with destination "/home/vscode/.claude",
// strips /host_mnt prefix from source, and appends /projects to get ProjectsDir.
func ParseOutput(data []byte) ([]Environment, error) {
	var containers []Container
	if err := json.Unmarshal(data, &containers); err != nil {
		return nil, fmt.Errorf("failed to parse devagent output: %w", err)
	}

	var envs []Environment

	for _, container := range containers {
		// Find mount with destination "/home/vscode/.claude"
		var claudeMount *Mount
		for i := range container.DevContainer.Mounts {
			if container.DevContainer.Mounts[i].Destination == "/home/vscode/.claude" {
				claudeMount = &container.DevContainer.Mounts[i]
				break
			}
		}

		// Skip containers without the .claude mount
		if claudeMount == nil {
			continue
		}

		// Derive host-side projects dir from mount source
		basePath := stripHostMntPrefix(claudeMount.Source)
		projectsDir := basePath + "/projects"

		envs = append(envs, Environment{
			ContainerName: container.ProxySidecar.ContainerName,
			ProjectPath:   container.ProjectPath,
			ProjectsDir:   projectsDir,
			State:         container.ProxySidecar.State,
		})
	}

	return envs, nil
}

// stripHostMntPrefix removes the /host_mnt prefix if present.
// This is Docker's macOS mount prefix. On Linux paths without the prefix
// pass through unchanged.
func stripHostMntPrefix(path string) string {
	if remainder, ok := strings.CutPrefix(path, "/host_mnt"); ok {
		// Only strip if there's something after the prefix
		if remainder != "" && remainder != "/" {
			return remainder
		}
	}
	return path
}
