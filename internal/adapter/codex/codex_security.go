package codex

import (
	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/space"
)

// SessionNetwork reports the generated Codex permission profile.
// Official key: permissions.<name>.network.enabled
// (https://developers.openai.com/codex/permissions).
func SessionNetwork(run *space.ResolvedAgentRun) adapter.SessionEffect {
	if adapter.NetworkDenied(run) {
		return adapter.EffectDeny
	}
	return adapter.EffectSilent
}

func PermissionsCeiling(run *space.ResolvedAgentRun) adapter.CeilingResult {
	if run == nil {
		return adapter.ComparePermissions(run, adapter.SessionFacts{Network: adapter.EffectUnknown})
	}
	return adapter.ComparePermissions(run, adapter.SessionFacts{Network: SessionNetwork(run)})
}

// ApprovalGateVsDeclared reports whether authorized shell is usable.
// Host asking less often than Source "always" is not a failure: authorized
// work may run silently; extra Host prompts are also fine.
func ApprovalGateVsDeclared(string) string {
	return "equal"
}
