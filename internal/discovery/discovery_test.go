package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDevcontainerAnchor(t *testing.T) {
	t.Parallel()

	type tc struct {
		name      string
		path      string
		wantPath  string
		wantFound bool
	}

	tests := []tc{
		{
			name:      "absolute path with devcontainer",
			path:      "/repo/.devcontainer/containers/app/home/vscode/.claude/projects/abc/session.jsonl",
			wantPath:  "/repo/.devcontainer",
			wantFound: true,
		},
		{
			name:      "deeper devcontainer nesting",
			path:      "/Users/josh/code/myrepo/.devcontainer/proxy/filter.py",
			wantPath:  "/Users/josh/code/myrepo/.devcontainer",
			wantFound: true,
		},
		{
			name:      "devcontainer is the last segment",
			path:      "/repo/.devcontainer",
			wantPath:  "/repo/.devcontainer",
			wantFound: true,
		},
		{
			name:      "no devcontainer in path",
			path:      "/containers/repo/app/project/session.jsonl",
			wantPath:  "",
			wantFound: false,
		},
		{
			name:      "partial match does not count",
			path:      "/repo/.devcontainer-extra/file.txt",
			wantPath:  "",
			wantFound: false,
		},
		// Cases required by the Step-5 test specification:
		{
			// Exact path from the plan spec: .devcontainer is at depth 2.
			name:      "plan_spec_enc_session",
			path:      "/repo/.devcontainer/containers/app/.claude/projects/enc/x.jsonl",
			wantPath:  "/repo/.devcontainer",
			wantFound: true,
		},
		{
			// Two .devcontainer segments: the function must return the deepest one.
			name:      "nested_deepest_wins",
			path:      "/a/.devcontainer/b/.devcontainer/c/x",
			wantPath:  "/a/.devcontainer/b/.devcontainer",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := DevcontainerAnchor(filepath.FromSlash(tt.path))
			if ok != tt.wantFound {
				t.Fatalf("DevcontainerAnchor(%q): found=%v, want %v", tt.path, ok, tt.wantFound)
			}
			if tt.wantFound {
				want := filepath.FromSlash(tt.wantPath)
				if got != want {
					t.Fatalf("DevcontainerAnchor(%q): path=%q, want %q", tt.path, got, want)
				}
			}
		})
	}
}

func TestDeriveLabel(t *testing.T) {
	tests := []struct {
		name      string
		claudeDir string
		want      string
	}{
		{
			name:      "devcontainer with containers segment",
			claudeDir: "/Users/josh/code/quartermaster/.devcontainer/containers/app/home/vscode/.claude",
			want:      "quartermaster/app",
		},
		{
			name:      "devcontainer without containers segment",
			claudeDir: "/Users/josh/code/myrepo/.devcontainer/home/vscode/.claude",
			want:      "myrepo",
		},
		{
			name:      "no devcontainer falls back to parent dir",
			claudeDir: "/srv/work/project-x/.claude",
			want:      "project-x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveLabel(tt.claudeDir); got != tt.want {
				t.Errorf("deriveLabel(%q) = %q, want %q", tt.claudeDir, got, tt.want)
			}
		})
	}
}

func TestLocalProjectsDir_HonorsClaudeConfigDir(t *testing.T) {
	dir := "/custom/claude"
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	want := filepath.Join(dir, "projects")
	if got := LocalProjectsDir(); got != want {
		t.Errorf("LocalProjectsDir() = %q, want %q", got, want)
	}
}

func TestLocalProjectsDir_FallsBackToHome(t *testing.T) {
	home := "/home/tester"
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".claude", "projects")
	if got := LocalProjectsDir(); got != want {
		t.Errorf("LocalProjectsDir() = %q, want %q", got, want)
	}
}

func TestFindProjectsDirs(t *testing.T) {
	root := t.TempDir()

	// A devcontainer-style .claude/projects that should be found.
	dcProjects := filepath.Join(root, "repo", ".devcontainer", "containers", "app", "home", "vscode", ".claude", "projects")
	mkdirAll(t, dcProjects)

	// A .claude directory without a projects subdir should be ignored.
	mkdirAll(t, filepath.Join(root, "other", ".claude"))

	found := FindProjectsDirs([]string{root})

	if len(found) != 1 {
		t.Fatalf("expected 1 projects dir, got %d: %+v", len(found), found)
	}
	if found[0].Path != dcProjects {
		t.Errorf("path = %q, want %q", found[0].Path, dcProjects)
	}
	if found[0].Label != "repo/app" {
		t.Errorf("label = %q, want %q", found[0].Label, "repo/app")
	}
}

func TestFindProjectsDirs_DeduplicatesAndSkipsMissingRoots(t *testing.T) {
	root := t.TempDir()
	projects := filepath.Join(root, "x", ".claude", "projects")
	mkdirAll(t, projects)

	// Pass the same root twice plus a nonexistent one; result must be deduped.
	found := FindProjectsDirs([]string{root, root, filepath.Join(root, "does-not-exist")})

	if len(found) != 1 {
		t.Fatalf("expected 1 deduped projects dir, got %d: %+v", len(found), found)
	}
}

func TestFindProjectsDirs_RespectsMaxDepth(t *testing.T) {
	root := t.TempDir()

	// Bury a .claude/projects far deeper than maxSearchDepth.
	deep := root
	for range maxSearchDepth + 3 {
		deep = filepath.Join(deep, "d")
	}
	mkdirAll(t, filepath.Join(deep, ".claude", "projects"))

	if found := FindProjectsDirs([]string{root}); len(found) != 0 {
		t.Errorf("expected nothing beyond max depth, got %+v", found)
	}
}

func TestExpandHome(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	cases := map[string]string{
		"~":            "/home/tester",
		"~/code":       "/home/tester/code",
		"/abs/path":    "/abs/path",
		"relative/dir": "relative/dir",
	}
	for in, want := range cases {
		if got := expandHome(in); got != want {
			t.Errorf("expandHome(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRelDepth(t *testing.T) {
	root := filepath.FromSlash("/a/b")
	cases := []struct {
		path string
		want int
	}{
		{filepath.FromSlash("/a/b"), 0},
		{filepath.FromSlash("/a/b/c"), 1},
		{filepath.FromSlash("/a/b/c/d"), 2},
	}
	for _, c := range cases {
		if got := relDepth(root, c.path); got != c.want {
			t.Errorf("relDepth(%q, %q) = %d, want %d", root, c.path, got, c.want)
		}
	}
}

// mkdirAll is a test helper that fails the test on error.
func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("failed to create %q: %v", path, err)
	}
}
