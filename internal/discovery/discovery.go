// Package discovery locates Claude Code session "projects" directories: the
// user's local directory and any nested ones found under configured search
// roots (for example .claude folders mounted inside devcontainers).
package discovery

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// maxSearchDepth bounds how deep FindProjectsDirs recurses below a search root,
// so a misconfigured root (e.g. "/" or "$HOME") can't trigger an unbounded walk.
const maxSearchDepth = 10

// ProjectsDir is a discovered Claude "projects" directory together with a short
// origin label used to tag its sessions in the UI.
type ProjectsDir struct {
	Path  string
	Label string
}

// LocalProjectsDir returns the user's local Claude projects directory, honoring
// CLAUDE_CONFIG_DIR (the same override Claude Code itself uses) and falling back
// to $HOME/.claude.
func LocalProjectsDir() string {
	return filepath.Join(claudeConfigDir(), "projects")
}

// claudeConfigDir resolves the base Claude config directory. Claude Code reads
// CLAUDE_CONFIG_DIR first, so we honor it for users who relocate the folder.
func claudeConfigDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(os.Getenv("HOME"), ".claude")
}

// FindProjectsDirs walks each search root looking for ".claude/projects"
// directories and returns them with derived labels. Roots that don't exist or
// can't be read are skipped rather than failing the whole scan. Results are
// deduplicated by path and recursion is capped at maxSearchDepth.
func FindProjectsDirs(roots []string) []ProjectsDir {
	var found []ProjectsDir
	seen := make(map[string]bool)

	for _, root := range roots {
		root = expandHome(root)
		if root == "" {
			continue
		}

		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil // skip unreadable entry, keep scanning siblings
			}
			if !d.IsDir() {
				return nil
			}
			if relDepth(root, path) > maxSearchDepth {
				return fs.SkipDir
			}
			if d.Name() != ".claude" {
				return nil
			}

			// Found a .claude directory; record its projects dir if present and
			// stop descending into it either way.
			projects := filepath.Join(path, "projects")
			if info, err := os.Stat(projects); err == nil && info.IsDir() && !seen[projects] {
				seen[projects] = true
				found = append(found, ProjectsDir{
					Path:  projects,
					Label: deriveLabel(path),
				})
			}
			return fs.SkipDir
		})
	}

	return found
}

// deriveLabel produces a short origin label for a discovered .claude directory.
// For devcontainer layouts (<repo>/.devcontainer/.../.claude) it uses the repo
// directory name plus the container name when present, e.g. "quartermaster/app".
// Otherwise it falls back to the name of the directory containing .claude.
func deriveLabel(claudeDir string) string {
	segments := strings.Split(filepath.ToSlash(filepath.Clean(claudeDir)), "/")
	for i, seg := range segments {
		if seg != ".devcontainer" || i == 0 {
			continue
		}
		repo := segments[i-1]
		// Layout: <repo>/.devcontainer/containers/<name>/...
		if i+2 < len(segments) && segments[i+1] == "containers" {
			return repo + "/" + segments[i+2]
		}
		return repo
	}

	// Fallback: the directory that contains .claude.
	if base := filepath.Base(filepath.Dir(claudeDir)); base != "." && base != string(filepath.Separator) {
		return base
	}
	return claudeDir
}

// relDepth returns how many path segments deep path is below root (0 == root).
func relDepth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator)) + 1
}

// expandHome expands a leading ~ in a configured path to $HOME.
func expandHome(path string) string {
	if path == "~" {
		return os.Getenv("HOME")
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(os.Getenv("HOME"), path[2:])
	}
	return path
}
