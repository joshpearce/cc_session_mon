package tui

import (
	"time"

	"cc_session_mon/internal/config"
	"cc_session_mon/internal/session"
)

// evaluateAlerts runs every configured rule against every session at time now.
// For each (session, rule) pair it:
//
//   - Alert tier (only when AlertThreshold > 0): emits one bell per
//     alert-threshold crossing; clears the latch when the metric drops back
//     below. A non-positive AlertThreshold disables the alert tier entirely —
//     neither the bell nor the latch is touched.
//
//   - Action tier (only when ActionThreshold > 0): maintains the
//     over-threshold streak and, when EnableCorrectiveActions is on and all
//     safety gates pass (per-session one-shot latch, trusted-path check,
//     sustained-tick count), dispatches the gated corrective action
//     (dry-run–aware, trusted-path-verified, per-session one-shot). A
//     non-positive ActionThreshold keeps the streak at zero and never acts.
func (m Model) evaluateAlerts(sessions []*session.Session, now time.Time) Model {
	cfg := config.Global()
	for _, sess := range sessions {
		for i := range cfg.Alerts {
			rule := cfg.Alerts[i]
			mf, ok := session.Metric(rule.Metric)
			if !ok {
				if !m.warnedMetrics[rule.Metric] {
					m.warnedMetrics[rule.Metric] = true
					m.audit.append(AuditEntry{
						Time:    now,
						Metric:  rule.Metric,
						Action:  "config-error",
						Outcome: "unknown metric",
					})
				}
				continue
			}
			v := mf(sess, now)
			key := alertKey{sess.FilePath, rule.Metric}
			m = m.applyAlertTier(sess, now, rule, key, v)
			m = m.applyActionTier(sess, now, cfg, rule, key, v)
		}
	}
	return m
}

// applyAlertTier handles the bell-and-latch logic for one (session, rule, value)
// triple. It is a no-op when rule.AlertThreshold <= 0 ("disabled").
func (m Model) applyAlertTier(sess *session.Session, now time.Time, rule config.AlertRule, key alertKey, v float64) Model {
	if rule.AlertThreshold <= 0 {
		return m
	}
	if v >= rule.AlertThreshold {
		if !m.alertLatch[key] {
			m.alertLatch[key] = true
			m.bell()
			m.audit.append(AuditEntry{
				Time:      now,
				SessionID: sess.ID,
				FilePath:  sess.FilePath,
				Origin:    sess.Origin,
				Metric:    rule.Metric,
				Value:     v,
				Threshold: rule.AlertThreshold,
				Action:    "alert",
				Outcome:   "bell",
			})
		}
	} else {
		delete(m.alertLatch, key)
	}
	return m
}

// applyActionTier maintains the action streak and, when all safety gates pass,
// dispatches the corrective action for one (session, rule, value) triple.
// It is a no-op (streak stays 0, nothing dispatched) when rule.ActionThreshold <= 0.
func (m Model) applyActionTier(sess *session.Session, now time.Time, cfg *config.Config, rule config.AlertRule, key alertKey, v float64) Model {
	if rule.ActionThreshold <= 0 || v < rule.ActionThreshold {
		m.actionStreak[key] = 0
		return m
	}
	m.actionStreak[key]++

	// Corrective action (Step 4). All gates must hold: master opt-in,
	// session not already acted on, value at/over the action threshold,
	// and the over-threshold streak has reached the rule's
	// action_sustained_ticks (default 1 → first such tick).
	if !cfg.EnableCorrectiveActions || m.actionLatch[sess.FilePath] ||
		m.actionStreak[key] < rule.ActionSustainedTicks {
		return m
	}

	// One-shot: latch the session regardless of branch so it cannot
	// re-fire or spam the audit log on subsequent ticks.
	m.actionLatch[sess.FilePath] = true

	entry := AuditEntry{
		Time:      now,
		SessionID: sess.ID,
		FilePath:  sess.FilePath,
		Origin:    sess.Origin,
		Metric:    rule.Metric,
		Value:     v,
		Threshold: rule.ActionThreshold,
	}
	if !session.IsTrustedPath(sess.FilePath, m.trustedRoots) {
		// Trusted-path gate: never act on a path outside the watched roots.
		entry.Action = "skipped"
		entry.Outcome = "untrusted path: " + sess.FilePath
	} else {
		outcome := session.NeutralizeSession(sess, cfg, cfg.ActionDryRun)
		entry.Action = outcome.Action
		entry.Outcome = outcome.Detail
	}
	m.audit.append(entry)
	return m
}
