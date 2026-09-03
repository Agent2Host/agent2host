package adapter

import (
	"github.com/agent2host/agent2host/internal/space"
)

func ShellExecuteDeclared(run *space.ResolvedAgentRun) string {
	shell := "on_boundary"
	if run != nil && run.Approvals != nil && run.Approvals.ShellExecute != nil {
		shell = *run.Approvals.ShellExecute
	}
	return shell
}

func PermissionsEnforcement(grant bool, intent ControlIntent) string {
	if !intent.Has(ControlPermissions) {
		return "unknown"
	}
	if !grant {
		return "none"
	}
	return plannedEnforce(intent, ControlPermissions, "host_enforced")
}

func ApprovalsEnforcement(gate string, intent ControlIntent, p Profile) string {
	if !intent.Has(ControlApprovals) {
		return "unknown"
	}
	if gate == "weaker" {
		return "prompt_only"
	}
	return plannedEnforce(intent, ControlApprovals, p.ApprovalEnforce)
}
