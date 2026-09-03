package codex

import (
	"fmt"
	"github.com/agent2host/agent2host/internal/adapter"
	"strings"

	"github.com/agent2host/agent2host/internal/space"
)

func reconcileSemanticCodex(run *space.ResolvedAgentRun, intent adapter.ControlIntent, np adapter.NativeProjectionPlan, lp adapter.LaunchPlan) error {
	if intent.Has(adapter.ControlIsolation) {
		home, ok := lp.Env[EnvHome]
		want := adapter.PrivateToken + "/" + HomeDirRel
		if !ok || home != want {
			return fmt.Errorf("%w: %s must be %s", adapter.ErrSemanticMismatch, EnvHome, want)
		}
		if strings.Contains(home, adapter.WorkspaceToken) {
			return fmt.Errorf("%w: %s must not use workspace token", adapter.ErrSemanticMismatch, EnvHome)
		}
	}
	for i, a := range lp.Args {
		if a == "--add-dir" && i+1 < len(lp.Args) {
			if strings.Contains(lp.Args[i+1], adapter.PrivateToken) {
				return fmt.Errorf("%w: private assets must not appear in --add-dir", adapter.ErrSemanticMismatch)
			}
		}
	}
	if intent.Has(adapter.ControlSandbox) {
		if adapter.LaunchArgPresent(lp.Args, "--sandbox") {
			return fmt.Errorf("%w: --sandbox selects legacy sandbox_mode and ignores default_permissions", adapter.ErrSemanticMismatch)
		}
	}
	if intent.Has(adapter.ControlApprovals) {
		got, ok := adapter.LaunchArgValue(lp.Args, "--ask-for-approval")
		if !ok {
			return fmt.Errorf("%w: --ask-for-approval missing", adapter.ErrSemanticMismatch)
		}
		if run != nil && got != ApprovalFlag(run) {
			return fmt.Errorf("%w: --ask-for-approval %q does not match synthesis", adapter.ErrSemanticMismatch, got)
		}
	}
	if intent.Has(adapter.ControlPermissions) || intent.Has(adapter.ControlSandbox) || intent.Has(adapter.ControlApprovals) {
		raw, ok := adapter.ProjectionContent(np, ConfigRel)
		if !ok {
			return fmt.Errorf("%w: missing %s", adapter.ErrSemanticMismatch, ConfigRel)
		}
		if run != nil {
			if err := checkCodexProjectedConfig(run, raw); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkCodexProjectedConfig(run *space.ResolvedAgentRun, raw []byte) error {
	doc, err := adapter.ParseTOMLMap(raw)
	if err != nil {
		return fmt.Errorf("%w: invalid config.toml: %v", adapter.ErrSemanticMismatch, err)
	}
	if _, has := doc["sandbox_mode"]; has {
		return fmt.Errorf("%w: config must not set sandbox_mode (mutually exclusive with default_permissions)", adapter.ErrSemanticMismatch)
	}
	if _, has := doc["sandbox_workspace_write"]; has {
		return fmt.Errorf("%w: config must not set sandbox_workspace_write (mutually exclusive with default_permissions)", adapter.ErrSemanticMismatch)
	}
	gotPerm, ok := adapter.TOMLString(doc, "default_permissions")
	if !ok || gotPerm != codexPermissionProfileName {
		return fmt.Errorf("%w: config default_permissions must be %q", adapter.ErrSemanticMismatch, codexPermissionProfileName)
	}
	gotApproval, ok := adapter.TOMLString(doc, "approval_policy")
	if !ok || gotApproval != ApprovalFlag(run) {
		return fmt.Errorf("%w: config approval_policy mismatch", adapter.ErrSemanticMismatch)
	}
	features, ok := adapter.TOMLTable(doc, "features")
	if !ok {
		return fmt.Errorf("%w: config must disable global Apps MCP", adapter.ErrSemanticMismatch)
	}
	apps, ok := adapter.TOMLBool(features, "apps")
	if !ok || apps {
		return fmt.Errorf("%w: config must disable global Apps MCP", adapter.ErrSemanticMismatch)
	}
	perms, ok := adapter.TOMLTable(doc, "permissions")
	if !ok {
		return fmt.Errorf("%w: config missing permissions table", adapter.ErrSemanticMismatch)
	}
	profile, ok := adapter.TOMLTable(perms, codexPermissionProfileName)
	if !ok {
		return fmt.Errorf("%w: config missing permissions.%s table", adapter.ErrSemanticMismatch, codexPermissionProfileName)
	}
	fs, ok := adapter.TOMLTable(profile, "filesystem")
	if !ok {
		return fmt.Errorf("%w: config missing permissions.%s.filesystem", adapter.ErrSemanticMismatch, codexPermissionProfileName)
	}
	ws, ok := adapter.TOMLString(fs, ":workspace_roots")
	if !ok || ws != codexWorkspaceFSAccess(run) {
		return fmt.Errorf("%w: config workspace filesystem access mismatch", adapter.ErrSemanticMismatch)
	}
	if root, ok := adapter.TOMLString(fs, ":root"); ok && root == "deny" {
		return fmt.Errorf("%w: config must not deny :root (blocks Codex TUI bootstrap)", adapter.ErrSemanticMismatch)
	}
	if adapter.NetworkDenied(run) {
		net, ok := adapter.TOMLTable(profile, "network")
		if !ok {
			return fmt.Errorf("%w: config network must be disabled", adapter.ErrSemanticMismatch)
		}
		enabled, ok := adapter.TOMLBool(net, "enabled")
		if !ok || enabled {
			return fmt.Errorf("%w: config network must be disabled", adapter.ErrSemanticMismatch)
		}
	}
	return nil
}
