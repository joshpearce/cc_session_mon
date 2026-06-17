package session

import (
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"cc_session_mon/internal/config"
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
// are killed with pkill; devcontainer sessions get the filter.py cut (Step 5 —
// here a logged placeholder). dryRun returns the intended action without
// executing it.
func NeutralizeSession(sess *Session, _ *config.Config, dryRun bool) ActionOutcome {
	if isLocalOrigin(sess.Origin) {
		return killLocal(sess, dryRun)
	}
	return cutDevcontainer(sess, dryRun)
}

func killLocal(sess *Session, dryRun bool) ActionOutcome {
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

// cutDevcontainer is implemented in Step 5; here it only records intent so the
// dispatch path and audit wiring are exercised end-to-end.
func cutDevcontainer(sess *Session, _ bool) ActionOutcome {
	return ActionOutcome{Action: "skipped", Detail: "devcontainer cut not yet implemented (origin " + sess.Origin + ")"}
}
