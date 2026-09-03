package kiro

import (
	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/space"
)

// SessionNetwork reports the generated Kiro V2 agent tools list.
// Official tag: "web" (https://kiro.dev/docs/custom-agents/configuration-reference/).
// Allowed network adds "web" (usable). Denied network only drops "web" and
// adds curl/wget deny patterns; other shell/MCP exits are not proven closed.
func SessionNetwork(run *space.ResolvedAgentRun) adapter.SessionEffect {
	if !adapter.NetworkDenied(run) {
		return adapter.EffectSilent
	}
	return adapter.EffectUnknown
}

func PermissionsCeiling(run *space.ResolvedAgentRun) adapter.CeilingResult {
	if run == nil {
		return adapter.ComparePermissions(run, adapter.SessionFacts{Network: adapter.EffectUnknown})
	}
	return adapter.ComparePermissions(run, adapter.SessionFacts{Network: SessionNetwork(run)})
}

func kiroApprovalGateVsDeclared(string) string {
	return "equal"
}
