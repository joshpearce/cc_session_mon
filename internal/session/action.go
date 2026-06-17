package session

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"cc_session_mon/internal/config"
	"cc_session_mon/internal/discovery"
)

// ActionOutcome is the recorded result of a neutralization attempt.
type ActionOutcome struct {
	Action string // "pkill" | "filter-cut" | "dry-run" | "skipped" | "failed"
	Detail string // human-readable detail for the audit log
}

// IsTrustedPath reports whether filePath lies within one of the watched projects
// roots. Corrective actions fire ONLY for trusted paths so a crafted transcript
// path can never steer a kill/file-edit outside the monitored tree.
func IsTrustedPath(filePath string, roots []string) bool {
	clean := filepath.Clean(filePath)
	for _, root := range roots {
		if root == "" {
			continue
		}
		root = filepath.Clean(root)
		if clean == root || strings.HasPrefix(clean, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// isLocalOrigin reports whether a session came from the user's own projects dir.
// Origin is empty or "local" for local sessions; devcontainer sessions carry a
// derived label such as "myrepo/mycontainer".
func isLocalOrigin(origin string) bool {
	return origin == "" || origin == "local"
}

// runPkill runs `pkill -f <pattern>` (no shell, SIGTERM). Indirected through a
// var so tests can stub it without signalling real processes. pkill exit code 1
// means "no process matched" — reported as matched=false, not an error.
var runPkill = func(pattern string) (matched bool, err error) {
	cmd := exec.Command("pkill", "-f", pattern) //nolint:gosec // pattern is regexp.QuoteMeta'd; no shell
	if runErr := cmd.Run(); runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) && ee.ExitCode() == 1 {
			return false, nil
		}
		return false, runErr
	}
	return true, nil
}

// NeutralizeSession dispatches the corrective action by origin: local sessions
// are killed with pkill; devcontainer sessions get the filter.py cut.
// dryRun returns the intended action without executing it.
func NeutralizeSession(sess *Session, cfg *config.Config, dryRun bool) ActionOutcome {
	if isLocalOrigin(sess.Origin) {
		return killLocal(sess, dryRun)
	}
	return cutDevcontainer(sess, cfg, dryRun)
}

func killLocal(sess *Session, dryRun bool) ActionOutcome {
	// Guard: ProjectPath must be an absolute path of meaningful length before it
	// is used as a pkill -f substring. An empty or very short pattern (e.g. "/")
	// would match unrelated processes on the host — an irreversible mistake.
	// filepath.IsAbs rejects both empty strings and relative paths.
	if !filepath.IsAbs(sess.ProjectPath) || len(sess.ProjectPath) < 4 {
		return ActionOutcome{
			Action: "skipped",
			Detail: "ProjectPath too short or not absolute for safe pkill: " + sess.ProjectPath,
		}
	}
	// ProjectPath is untrusted transcript content; QuoteMeta so it is matched
	// literally by pkill's regex, never interpreted as a pattern.
	pattern := regexp.QuoteMeta(sess.ProjectPath)
	if dryRun {
		return ActionOutcome{Action: "dry-run", Detail: "would run: pkill -f " + pattern}
	}
	matched, err := runPkill(pattern)
	switch {
	case err != nil:
		return ActionOutcome{Action: "failed", Detail: "pkill -f " + pattern + ": " + err.Error()}
	case !matched:
		return ActionOutcome{Action: "pkill", Detail: "no matching process: " + pattern}
	default:
		return ActionOutcome{Action: "pkill", Detail: "SIGTERM via pkill -f " + pattern}
	}
}

// cutDevcontainer comments every uncommented line in the devcontainer proxy's
// filter.py that matches cfg.AnthropicAllowPattern. The filter file's path is
// derived from the monitor-discovered directory tree (FilePath anchor +
// DevcontainerFilterRelPath) and is verified to stay under the anchor before
// the file is read or written. The write is atomic (temp + os.Rename). The cut
// is idempotent: already-commented matching lines are left untouched.
func cutDevcontainer(sess *Session, cfg *config.Config, dryRun bool) ActionOutcome {
	anchor, ok := discovery.DevcontainerAnchor(sess.FilePath)
	if !ok {
		return ActionOutcome{
			Action: "failed",
			Detail: "no .devcontainer anchor for " + sess.FilePath,
		}
	}

	filterPath := filepath.Join(anchor, cfg.DevcontainerFilterRelPath)

	// Reject traversal: the joined path must remain under the anchor.
	if !IsTrustedPath(filterPath, []string{anchor}) {
		return ActionOutcome{
			Action: "failed",
			Detail: "filter path escapes anchor: " + filterPath,
		}
	}

	re, err := regexp.Compile(cfg.AnthropicAllowPattern)
	if err != nil {
		return ActionOutcome{
			Action: "failed",
			Detail: "bad AnthropicAllowPattern: " + err.Error(),
		}
	}

	data, err := os.ReadFile(filterPath) //nolint:gosec // path is anchor-verified above
	if err != nil {
		return ActionOutcome{
			Action: "failed",
			Detail: "read filter.py: " + err.Error(),
		}
	}

	lines := strings.Split(string(data), "\n")
	changed, firstOriginal := applyFilterCut(lines, re)

	if dryRun {
		return ActionOutcome{
			Action: "dry-run",
			Detail: fmt.Sprintf("would comment %d line(s) in %s", changed, filterPath),
		}
	}

	if changed == 0 {
		return ActionOutcome{
			Action: "filter-cut",
			Detail: "no uncommented allow-rule lines in " + filterPath,
		}
	}

	if err := atomicWrite(filterPath, strings.Join(lines, "\n")); err != nil {
		return ActionOutcome{
			Action: "failed",
			Detail: "write filter.py: " + err.Error(),
		}
	}

	return ActionOutcome{
		Action: "filter-cut",
		Detail: fmt.Sprintf("commented %d line(s) in %s; first: %s", changed, filterPath, firstOriginal),
	}
}

// applyFilterCut iterates lines in-place, prefix-commenting each uncommented
// line that matches re. It returns the count of lines changed and the text of
// the first original line that was changed (empty string when count is 0).
func applyFilterCut(lines []string, re *regexp.Regexp) (changed int, firstOriginal string) {
	for i, line := range lines {
		if !re.MatchString(line) {
			continue
		}
		// Already commented — idempotent, leave untouched.
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		if changed == 0 {
			firstOriginal = line
		}
		lines[i] = "# " + line
		changed++
	}
	return changed, firstOriginal
}

// atomicWrite writes content to path via a temp file in the same directory,
// preserving the original file's mode. The temp file is renamed over path only
// on success; on any earlier error the temp file is removed.
func atomicWrite(path, content string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	mode := info.Mode()

	tmp, err := os.CreateTemp(filepath.Dir(path), ".filter-cut-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename temp to %s: %w", path, err)
	}
	return nil
}
