package session

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"cc_session_mon/internal/config"
)

// TestIsTrustedPath verifies that IsTrustedPath correctly classifies paths as
// trusted or untrusted, including the sibling-prefix false-positive guard.
func TestIsTrustedPath(t *testing.T) {
	// No t.Parallel(): pure function; but kept serial for consistency.
	type tc struct {
		name     string
		filePath string
		roots    []string
		want     bool
	}

	tests := []tc{
		{
			name:     "path_under_root",
			filePath: "/a/projects/p/session.jsonl",
			roots:    []string{"/a/projects"},
			want:     true,
		},
		{
			name:     "path_equals_root",
			filePath: "/a/projects",
			roots:    []string{"/a/projects"},
			want:     true,
		},
		{
			name:     "unrelated_path",
			filePath: "/b/other/session.jsonl",
			roots:    []string{"/a/projects"},
			want:     false,
		},
		{
			name:     "sibling_prefix_not_trusted",
			filePath: "/a/projects-evil/x.jsonl",
			roots:    []string{"/a/projects"},
			want:     false,
		},
		{
			name:     "empty_root_skipped",
			filePath: "/a/projects/x.jsonl",
			roots:    []string{"", "/a/projects"},
			want:     true,
		},
		{
			name:     "only_empty_roots",
			filePath: "/a/projects/x.jsonl",
			roots:    []string{""},
			want:     false,
		},
		{
			name:     "no_roots",
			filePath: "/a/projects/x.jsonl",
			roots:    []string{},
			want:     false,
		},
		{
			name:     "path_under_second_root",
			filePath: "/b/sessions/p/x.jsonl",
			roots:    []string{"/a/projects", "/b/sessions"},
			want:     true,
		},
		{
			name:     "path_with_trailing_slash_in_root",
			filePath: "/a/projects/p/x.jsonl",
			roots:    []string{"/a/projects/"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTrustedPath(tt.filePath, tt.roots)
			if got != tt.want {
				t.Fatalf("IsTrustedPath(%q, %v) = %v, want %v",
					tt.filePath, tt.roots, got, tt.want)
			}
		})
	}
}

// TestNeutralizeSession_MetacharQuoted verifies that a ProjectPath containing
// regex metacharacters is passed through regexp.QuoteMeta before reaching pkill,
// so crafted paths cannot inject a kill-all pattern.
func TestNeutralizeSession_MetacharQuoted(t *testing.T) {
	// No t.Parallel(): mutates package-global runPkill.
	var capturedPattern string
	orig := runPkill
	runPkill = func(pattern string) (bool, error) {
		capturedPattern = pattern
		return true, nil
	}
	t.Cleanup(func() { runPkill = orig })

	sess := &Session{
		Origin:      "local",
		ProjectPath: "/tmp/foo(bar)+.*x",
	}
	outcome := NeutralizeSession(sess, &config.Config{}, false)

	want := regexp.QuoteMeta("/tmp/foo(bar)+.*x")
	if capturedPattern != want {
		t.Fatalf("pkill pattern: want %q, got %q", want, capturedPattern)
	}
	if outcome.Action != "pkill" {
		t.Fatalf("outcome.Action: want %q, got %q", "pkill", outcome.Action)
	}
}

// TestNeutralizeSession_DryRunNoExec verifies that dry-run mode returns an
// "dry-run" outcome and never invokes the real pkill.
func TestNeutralizeSession_DryRunNoExec(t *testing.T) {
	// No t.Parallel(): mutates package-global runPkill.
	called := false
	orig := runPkill
	runPkill = func(_ string) (bool, error) {
		called = true
		return true, nil
	}
	t.Cleanup(func() { runPkill = orig })

	sess := &Session{
		Origin:      "local",
		ProjectPath: "/tmp/safe",
	}
	outcome := NeutralizeSession(sess, &config.Config{}, true)

	if outcome.Action != "dry-run" {
		t.Fatalf("outcome.Action: want %q, got %q", "dry-run", outcome.Action)
	}
	if called {
		t.Fatal("runPkill was called despite dry-run=true")
	}
}

// TestNeutralizeSession_NoMatchMapping verifies that pkill exit-1 (no processes
// matched) is mapped to Action=="pkill" with a "no matching process" detail,
// not to "failed".
func TestNeutralizeSession_NoMatchMapping(t *testing.T) {
	// No t.Parallel(): mutates package-global runPkill.
	orig := runPkill
	runPkill = func(_ string) (bool, error) {
		return false, nil // exit-1 mapped to matched=false, err=nil
	}
	t.Cleanup(func() { runPkill = orig })

	sess := &Session{Origin: "local", ProjectPath: "/tmp/gone"}
	outcome := NeutralizeSession(sess, &config.Config{}, false)

	if outcome.Action != "pkill" {
		t.Fatalf("outcome.Action: want %q, got %q", "pkill", outcome.Action)
	}
	if outcome.Detail == "" {
		t.Fatal("outcome.Detail should mention no matching process, got empty string")
	}
}

// TestNeutralizeSession_PkillError verifies that a non-exit-1 error from pkill
// is reported as Action=="failed".
func TestNeutralizeSession_PkillError(t *testing.T) {
	// No t.Parallel(): mutates package-global runPkill.
	someErr := errors.New("permission denied")
	orig := runPkill
	runPkill = func(_ string) (bool, error) {
		return false, someErr
	}
	t.Cleanup(func() { runPkill = orig })

	sess := &Session{Origin: "local", ProjectPath: "/tmp/badpath"}
	outcome := NeutralizeSession(sess, &config.Config{}, false)

	if outcome.Action != "failed" {
		t.Fatalf("outcome.Action: want %q, got %q", "failed", outcome.Action)
	}
}

// TestNeutralizeSession_NonLocalOrigin verifies that a non-local origin routes
// to cutDevcontainer (not pkill). When FilePath has no .devcontainer ancestor,
// the outcome is Action=="failed" with a "no .devcontainer anchor" detail.
func TestNeutralizeSession_NonLocalOrigin(t *testing.T) {
	// No t.Parallel(): mutates package-global runPkill.
	called := false
	orig := runPkill
	runPkill = func(_ string) (bool, error) {
		called = true
		return true, nil
	}
	t.Cleanup(func() { runPkill = orig })

	sess := &Session{
		Origin:      "repo/app",
		FilePath:    "/containers/repo/app/project/session.jsonl", // no .devcontainer in path
		ProjectPath: "/containers/repo/app/project",
	}
	outcome := NeutralizeSession(sess, &config.Config{
		DevcontainerFilterRelPath: "proxy/filter.py",
		AnthropicAllowPattern:     `api\.anthropic\.com`,
	}, false)

	if outcome.Action != "failed" {
		t.Fatalf("outcome.Action: want %q (no .devcontainer anchor), got %q",
			"failed", outcome.Action)
	}
	if !strings.Contains(outcome.Detail, "no .devcontainer anchor") {
		t.Fatalf("outcome.Detail should mention missing anchor, got %q", outcome.Detail)
	}
	if called {
		t.Fatal("runPkill was called for a non-local origin")
	}
}
