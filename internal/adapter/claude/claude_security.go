package claude

import (
	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/space"
)

// SessionNetwork reports the generated Claude settings/tools list.
// Official model: permissions allow/deny/ask
// (https://code.claude.com/docs/en/permissions).
// Allow adds WebFetch/WebSearch (usable without a further ask).
// Deny only blocks those plus curl/wget/nc-shaped Bash; other shell/MCP/browser
// exits are not proven closed.
func SessionNetwork(run *space.ResolvedAgentRun) adapter.SessionEffect {
	if !adapter.NetworkDenied(run) {
		return adapter.EffectSilent
	}
	return adapter.EffectUnknown
}

func PermissionsCeiling(run *space.ResolvedAgentRun) adapter.CeilingResult {
	return adapter.ComparePermissions(run, adapter.SessionFacts{Network: SessionNetwork(run)})
}

func claudeApprovalGate(string) string {
	return "equal"
}
