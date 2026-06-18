package session

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cc_session_mon/internal/config"
)

// representativeFilterPy is a realistic filter.py body used across fixture tests.
// It deliberately has one line matching "api.anthropic.com" and one that does not,
// plus a comment header and a trailing newline, so we can assert byte-for-byte
// preservation of everything except the targeted line.
const representativeFilterPy = `# proxy allow list
ALLOW = [
    "api.anthropic.com",
    "github.com",
]
`

// devcontainerFixture builds a devcontainer directory tree under tmp and returns
// a *Session whose FilePath sits inside it with a non-local Origin.
//
// Layout:
//
//	<tmp>/repo/.devcontainer/proxy/filter.py   ← the filter file (when createFilter=true)
//	<tmp>/repo/.devcontainer/containers/app/.claude/projects/enc/<uuid>.jsonl  ← session file
//
// The session FilePath is constructed so DevcontainerAnchor returns
// <tmp>/repo/.devcontainer, and the filter path resolves to
// <tmp>/repo/.devcontainer/proxy/filter.py.
func devcontainerFixture(t *testing.T, createFilter bool, filterContent string, filterMode os.FileMode) (sess *Session, filterPath string) {
	t.Helper()
	tmp := t.TempDir()

	anchor := filepath.Join(tmp, "repo", ".devcontainer")
	sessionDir := filepath.Join(anchor, "containers", "app", ".claude", "projects", "enc")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}

	filterDir := filepath.Join(anchor, "proxy")
	filterPath = filepath.Join(filterDir, "filter.py")

	if createFilter {
		if err := os.MkdirAll(filterDir, 0o755); err != nil {
			t.Fatalf("failed to create filter dir: %v", err)
		}
		if err := os.WriteFile(filterPath, []byte(filterContent), filterMode); err != nil {
			t.Fatalf("failed to write filter.py: %v", err)
		}
	}

	sessionFilePath := filepath.Join(sessionDir, "abc123.jsonl")
	sess = &Session{
		ID:       "test-devcontainer-session",
		FilePath: sessionFilePath,
		Origin:   "repo/app",
	}
	return
}

// devcontainerCfg returns a *config.Config set up for devcontainer filter tests.
func devcontainerCfg() *config.Config {
	cfg := config.DefaultConfig()
	cfg.DevcontainerFilterRelPath = "proxy/filter.py"
	cfg.AnthropicAllowPattern = `api\.anthropic\.com`
	return cfg
}

// TestNeutralizeSession_Devcontainer_HappyPath verifies that the matching line is
// prefix-commented, every other line is byte-for-byte unchanged, and the file
// mode is preserved.
func TestNeutralizeSession_Devcontainer_HappyPath(t *testing.T) {
	// No t.Parallel(): does real FS writes under TempDir.
	sess, filterPath := devcontainerFixture(t, true, representativeFilterPy, 0o600)
	cfg := devcontainerCfg()

	outcome := NeutralizeSession(sess, cfg, false)

	if outcome.Action != "filter-cut" {
		t.Fatalf("Action: want %q, got %q (detail: %s)", "filter-cut", outcome.Action, outcome.Detail)
	}

	got, err := os.ReadFile(filterPath) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("ReadFile after cut: %v", err)
	}

	// Build the expected content: only the api.anthropic.com line is commented.
	wantLines := strings.Split(representativeFilterPy, "\n")
	for i, line := range wantLines {
		if strings.Contains(line, "api.anthropic.com") && !strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			wantLines[i] = "# " + line
		}
	}
	want := strings.Join(wantLines, "\n")

	if string(got) != want {
		t.Errorf("file content mismatch after cut.\ngot:  %q\nwant: %q", string(got), want)
	}

	// Verify unchanged lines are byte-identical.
	origLines := strings.Split(representativeFilterPy, "\n")
	gotLines := strings.Split(string(got), "\n")
	if len(origLines) != len(gotLines) {
		t.Fatalf("line count changed: orig %d, got %d", len(origLines), len(gotLines))
	}
	for i, orig := range origLines {
		if strings.Contains(orig, "api.anthropic.com") {
			// This is the modified line; skip identical-check.
			continue
		}
		if gotLines[i] != orig {
			t.Errorf("line %d changed unexpectedly: orig=%q, got=%q", i, orig, gotLines[i])
		}
	}

	// Verify file mode is preserved (0600).
	info, err := os.Stat(filterPath)
	if err != nil {
		t.Fatalf("Stat after cut: %v", err)
	}
	if info.Mode() != 0o600 {
		t.Errorf("file mode changed: want 0600, got %v", info.Mode())
	}
}

// TestNeutralizeSession_Devcontainer_DryRunNoModify verifies that dry-run returns
// Action=="dry-run" and does not modify the filter file.
func TestNeutralizeSession_Devcontainer_DryRunNoModify(t *testing.T) {
	// No t.Parallel(): reads the FS.
	sess, filterPath := devcontainerFixture(t, true, representativeFilterPy, 0o644)
	cfg := devcontainerCfg()

	before, err := os.ReadFile(filterPath) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("ReadFile before dry-run: %v", err)
	}

	outcome := NeutralizeSession(sess, cfg, true)

	if outcome.Action != "dry-run" {
		t.Fatalf("Action: want %q, got %q (detail: %s)", "dry-run", outcome.Action, outcome.Detail)
	}

	after, err := os.ReadFile(filterPath) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("ReadFile after dry-run: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("file was modified during dry-run:\nbefore: %q\nafter:  %q", string(before), string(after))
	}
}

// TestNeutralizeSession_Devcontainer_Idempotent verifies that running the cut
// twice leaves the file unchanged after the second call and does not produce
// double-comments (e.g. "# # original line").
func TestNeutralizeSession_Devcontainer_Idempotent(t *testing.T) {
	// No t.Parallel(): real FS writes.
	sess, filterPath := devcontainerFixture(t, true, representativeFilterPy, 0o644)
	cfg := devcontainerCfg()

	// First cut: matches and comments the allow-rule line.
	first := NeutralizeSession(sess, cfg, false)
	if first.Action != "filter-cut" {
		t.Fatalf("first cut: Action want %q, got %q (detail: %s)", "filter-cut", first.Action, first.Detail)
	}

	// Snapshot the file after the first cut.
	afterFirst, err := os.ReadFile(filterPath) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("ReadFile after first cut: %v", err)
	}

	// Second cut: no uncommented matching lines remain.
	second := NeutralizeSession(sess, cfg, false)
	if second.Action != "filter-cut" {
		t.Fatalf("second cut: Action want %q, got %q (detail: %s)", "filter-cut", second.Action, second.Detail)
	}
	if !strings.Contains(second.Detail, "no uncommented allow-rule lines") {
		t.Errorf("second cut: Detail should mention no uncommented lines, got %q", second.Detail)
	}

	// File must be byte-identical to the post-first-cut snapshot.
	afterSecond, err := os.ReadFile(filterPath) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("ReadFile after second cut: %v", err)
	}
	if !bytes.Equal(afterSecond, afterFirst) {
		t.Errorf("file changed on second (idempotent) cut:\nfirst:  %q\nsecond: %q", string(afterFirst), string(afterSecond))
	}

	// Extra guard: the commented line must not be double-commented.
	for _, line := range strings.Split(string(afterSecond), "\n") {
		if strings.HasPrefix(line, "# # ") {
			t.Errorf("double-commented line detected: %q", line)
		}
	}
}

// TestNeutralizeSession_Devcontainer_MissingFilterFile verifies that when the
// filter.py file does not exist the outcome is Action=="failed" mentioning "read filter.py".
func TestNeutralizeSession_Devcontainer_MissingFilterFile(t *testing.T) {
	// No t.Parallel(): FS reads.
	// createFilter=false: anchor dir exists but filter.py does not.
	sess, _ := devcontainerFixture(t, false, "", 0o644)
	cfg := devcontainerCfg()

	outcome := NeutralizeSession(sess, cfg, false)

	if outcome.Action != "failed" {
		t.Fatalf("Action: want %q, got %q (detail: %s)", "failed", outcome.Action, outcome.Detail)
	}
	if !strings.Contains(outcome.Detail, "read filter.py") {
		t.Errorf("Detail should mention reading filter.py, got %q", outcome.Detail)
	}
}

// TestNeutralizeSession_Devcontainer_OutOfAnchorRejected verifies that a
// DevcontainerFilterRelPath that escapes the anchor via path traversal is
// rejected before any file is read or written.
func TestNeutralizeSession_Devcontainer_OutOfAnchorRejected(t *testing.T) {
	// No t.Parallel(): FS reads.
	sess, _ := devcontainerFixture(t, true, representativeFilterPy, 0o644)
	cfg := devcontainerCfg()
	// This resolves to something outside the anchor after filepath.Clean.
	cfg.DevcontainerFilterRelPath = "../../../../etc/hosts"

	outcome := NeutralizeSession(sess, cfg, false)

	if outcome.Action != "failed" {
		t.Fatalf("Action: want %q, got %q (detail: %s)", "failed", outcome.Action, outcome.Detail)
	}
	if !strings.Contains(strings.ToLower(outcome.Detail), "escapes") && !strings.Contains(strings.ToLower(outcome.Detail), "escape") {
		t.Errorf("Detail should mention path escaping anchor, got %q", outcome.Detail)
	}
	// No file outside the temp tree must have been touched (function returned before
	// any write; we verify by asserting Action=="failed" above — real hosts file is
	// never written because IsTrustedPath rejects the path first).
}

// TestNeutralizeSession_Devcontainer_SymlinkEscapeRejected verifies that a
// filter.py which is a symlink pointing OUTSIDE the .devcontainer anchor is
// rejected (Action=="failed", detail mentions "symlink") and the link target is
// left byte-for-byte unchanged. The lexical containment check alone would not
// catch this — os.ReadFile/os.Rename follow symlinks — so EvalSymlinks must run.
func TestNeutralizeSession_Devcontainer_SymlinkEscapeRejected(t *testing.T) {
	// No t.Parallel(): real FS writes and a symlink.
	tmp := t.TempDir()

	anchor := filepath.Join(tmp, "repo", ".devcontainer")
	sessionDir := filepath.Join(anchor, "containers", "app", ".claude", "projects", "enc")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll sessionDir: %v", err)
	}
	proxyDir := filepath.Join(anchor, "proxy")
	if err := os.MkdirAll(proxyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll proxyDir: %v", err)
	}

	// A file OUTSIDE the anchor, and a filter.py symlink that targets it.
	outsideDir := filepath.Join(tmp, "outside")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("MkdirAll outsideDir: %v", err)
	}
	target := filepath.Join(outsideDir, "secret.py")
	if err := os.WriteFile(target, []byte(representativeFilterPy), 0o644); err != nil {
		t.Fatalf("WriteFile target: %v", err)
	}
	link := filepath.Join(proxyDir, "filter.py")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	sess := &Session{
		ID:       "symlink-escape-session",
		FilePath: filepath.Join(sessionDir, "abc123.jsonl"),
		Origin:   "repo/app",
	}

	outcome := NeutralizeSession(sess, devcontainerCfg(), false)

	if outcome.Action != "failed" {
		t.Fatalf("Action: want %q, got %q (detail: %s)", "failed", outcome.Action, outcome.Detail)
	}
	if !strings.Contains(outcome.Detail, "symlink") {
		t.Errorf("Detail should mention symlink escape, got %q", outcome.Detail)
	}

	// The out-of-anchor target must be untouched.
	got, err := os.ReadFile(target) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("ReadFile target after rejected cut: %v", err)
	}
	if string(got) != representativeFilterPy {
		t.Errorf("out-of-anchor target was modified:\ngot:  %q\nwant: %q", string(got), representativeFilterPy)
	}
}

// TestNeutralizeSession_Devcontainer_NoAnchorFails verifies that a session whose
// FilePath has no .devcontainer ancestor (plain path) returns Action=="failed"
// mentioning the missing anchor.
func TestNeutralizeSession_Devcontainer_NoAnchorFails(t *testing.T) {
	// No t.Parallel(): FS reads.
	tmp := t.TempDir()
	plainPath := filepath.Join(tmp, "plain", "x.jsonl")
	if err := os.MkdirAll(filepath.Dir(plainPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	sess := &Session{
		ID:       "plain-session",
		FilePath: plainPath,
		Origin:   "somerepo/container", // non-local so cutDevcontainer is invoked
	}
	cfg := devcontainerCfg()

	outcome := NeutralizeSession(sess, cfg, false)

	if outcome.Action != "failed" {
		t.Fatalf("Action: want %q, got %q (detail: %s)", "failed", outcome.Action, outcome.Detail)
	}
	if !strings.Contains(outcome.Detail, ".devcontainer anchor") {
		t.Errorf("Detail should mention missing .devcontainer anchor, got %q", outcome.Detail)
	}
}

// TestNeutralizeSession_Devcontainer_BadRegexFails verifies that an invalid
// AnthropicAllowPattern returns Action=="failed" mentioning the bad pattern.
// Per the source, regex compilation happens BEFORE the file is read, so the
// filter.py need not exist for this case to be exercised; however we provide
// the full fixture to avoid any earlier failure path obscuring the regex error.
func TestNeutralizeSession_Devcontainer_BadRegexFails(t *testing.T) {
	// No t.Parallel(): FS reads.
	sess, _ := devcontainerFixture(t, true, representativeFilterPy, 0o644)
	cfg := devcontainerCfg()
	cfg.AnthropicAllowPattern = "(" // invalid regex

	outcome := NeutralizeSession(sess, cfg, false)

	if outcome.Action != "failed" {
		t.Fatalf("Action: want %q, got %q (detail: %s)", "failed", outcome.Action, outcome.Detail)
	}
	if !strings.Contains(strings.ToLower(outcome.Detail), "anthropicallowpattern") &&
		!strings.Contains(strings.ToLower(outcome.Detail), "bad") &&
		!strings.Contains(strings.ToLower(outcome.Detail), "pattern") {
		t.Errorf("Detail should mention the bad pattern, got %q", outcome.Detail)
	}
}
