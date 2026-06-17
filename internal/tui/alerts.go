package tui

import (
	"time"

	"cc_session_mon/internal/config"
	"cc_session_mon/internal/session"
)

// evaluateAlerts runs every configured rule against every session at time now,
// emitting one bell per alert-threshold crossing per (session, rule) and
// maintaining the action-threshold streak (Step 4 consumes the streak; this step
// does not act). Latches clear when a metric drops back below its alert threshold.
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

			if v >= rule.ActionThreshold {
				m.actionStreak[key]++
			} else {
				m.actionStreak[key] = 0
			}

			// Corrective action (Step 4). All gates must hold: master opt-in,
			// session not already acted on, value at/over the action threshold,
			// and the over-threshold streak has reached the rule's
			// action_sustained_ticks (default 1 → first such tick).
			if cfg.EnableCorrectiveActions && !m.actionLatch[sess.FilePath] &&
				v >= rule.ActionThreshold && m.actionStreak[key] >= rule.ActionSustainedTicks {
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
			}
		}
	}
	return m
}
