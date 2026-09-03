package claude

import (
	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/space"
)

func PermissionsGrantSubseteq(run *space.ResolvedAgentRun) bool {
	return adapter.FSCeilingWorkingDirectoryOnly(run)
}

func claudeApprovalGate(string) string {
	return "equal"
}
