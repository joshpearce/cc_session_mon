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
						Time:   now,
						Metric: rule.Metric,
						Action: "config-error",
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
		}
	}
	return m
}
