package adapter

import (
	"github.com/agent2host/agent2host/internal/compatibility"
)

// RunPolicy carries CLI/runtime gates applied after Evaluate.
type RunPolicy struct {
	// RequireStrictRead treats workspace read-boundary enforcement as hard.
	// No committed Host satisfies this today (partial Claude mitigation ≠ strict).
	RequireStrictRead bool
	// ForExecute is true on the run path (version mismatch refuses); false on check (warn).
	ForExecute bool
}

// ApplyRunPolicy applies post-Evaluate gates without changing frozen Report fields.
func ApplyRunPolicy(report compatibility.Report, probe ProbeResult, hostID string, pctx ProjectionContext, policy RunPolicy) compatibility.Report {
	if policy.RequireStrictRead && !StrictReadEnforced(hostID, pctx) {
		report.Decision = "refused"
		return report
	}
	if probe.Found && probe.HostVersion != "" && probe.HostVersion != "unknown" {
		if !HostVersionVerified(hostID, probe.HostVersion) {
			if policy.ForExecute {
				report.Decision = "refused"
			} else if report.Decision == "allowed" {
				report.Decision = "allowed_with_warnings"
			}
		}
	}
	return report
}

// StrictReadEnforced reports probe-backed strict workspace read isolation.
// Partial Claude home-tree deny does not satisfy strict mode.
func StrictReadEnforced(hostID string, _ ProjectionContext) bool {
	_ = hostID
	return false
}
