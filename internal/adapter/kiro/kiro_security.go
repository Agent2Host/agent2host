package kiro

import (
	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/space"
)

// kiroPermissionsGrantSubseteq reports whether projected Kiro capability rules
// stay within the declared permission ceiling for V2 agent JSON (tools +
// toolsSettings). Path-level FS is approximate — adapter.Assess uses support
// approximate, not mapped.
func PermissionsGrantSubseteq(run *space.ResolvedAgentRun) bool {
	if run == nil {
		return false
	}
	return adapter.FSCeilingWorkingDirectoryOnly(run) && adapter.NetworkDenied(run)
}

func kiroApprovalGateVsDeclared(string) string {
	return "equal"
}
