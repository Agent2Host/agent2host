package codex

import (
	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/space"
)

// codexPermissionsGrantSubseteq reports whether the projected permission
// profile stays within the declared permission ceiling for the V0 slice.
func PermissionsGrantSubseteq(run *space.ResolvedAgentRun) bool {
	if run == nil {
		return false
	}
	return adapter.FSCeilingWorkingDirectoryOnly(run) && adapter.NetworkDenied(run)
}

func ApprovalGateVsDeclared(shell string, grantSubseteq bool) string {
	switch shell {
	case "never":
		return "equal"
	case "on_boundary":
		// on_boundary is satisfied when out-of-bound Shell is denied by
		// permissions/sandbox; Codex on-request is equal only with a clean grant.
		if grantSubseteq {
			return "equal"
		}
		return "weaker"
	case "always":
		return "weaker"
	default:
		return "weaker"
	}
}
