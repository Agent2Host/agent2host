package claude

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/space"
)

type ProjectedSettings struct {
	Sandbox *struct {
		Enabled                  bool  `json:"enabled"`
		FailIfUnavailable        bool  `json:"failIfUnavailable"`
		AllowUnsandboxedCommands bool  `json:"allowUnsandboxedCommands"`
		AutoAllowBashIfSandboxed *bool `json:"autoAllowBashIfSandboxed"`
		Filesystem               *struct {
			DenyRead  []string `json:"denyRead"`
			AllowRead []string `json:"allowRead"`
		} `json:"filesystem"`
	} `json:"sandbox"`
	Permissions *struct {
		Deny  []string `json:"deny"`
		Ask   []string `json:"ask"`
		Allow []string `json:"allow"`
	} `json:"permissions"`
}

func reconcileSemanticClaude(run *space.ResolvedAgentRun, intent adapter.ControlIntent, report compatibility.Report, np adapter.NativeProjectionPlan, lp adapter.LaunchPlan, pctx adapter.ProjectionContext) error {
	if run == nil {
		return fmt.Errorf("%w: nil run", adapter.ErrSemanticMismatch)
	}
	if intent.Has(adapter.ControlPermissions) || intent.Has(adapter.ControlApprovals) || intent.Has(adapter.ControlSandbox) {
		if !adapter.LaunchArgPresent(lp.Args, "--settings") {
			return fmt.Errorf("%w: security controls planned but --settings missing", adapter.ErrSemanticMismatch)
		}
		settingsPath, ok := adapter.LaunchArgValue(lp.Args, "--settings")
		if !ok || !strings.Contains(settingsPath, SettingsRel) {
			return fmt.Errorf("%w: --settings must load %s", adapter.ErrSemanticMismatch, SettingsRel)
		}
	}
	if intent.Has(adapter.ControlIsolation) {
		v, ok := lp.Env[EnvConfig]
		want := adapter.AuthProfileToken
		if !ok || v != want {
			return fmt.Errorf("%w: %s must be %s", adapter.ErrSemanticMismatch, EnvConfig, want)
		}
	}
	if intent.Has(adapter.ControlMCP) {
		if !adapter.LaunchArgPresent(lp.Args, "--strict-mcp-config") {
			return fmt.Errorf("%w: MCP control planned but --strict-mcp-config missing", adapter.ErrSemanticMismatch)
		}
		if !adapter.LaunchArgPresent(lp.Args, "--mcp-config") {
			return fmt.Errorf("%w: MCP control planned but --mcp-config missing", adapter.ErrSemanticMismatch)
		}
	}
	if !adapter.LaunchArgPresent(lp.Args, "--agent") {
		return fmt.Errorf("%w: --agent missing", adapter.ErrSemanticMismatch)
	}
	if agent, ok := adapter.LaunchArgValue(lp.Args, "--agent"); !ok || agent != run.AgentID {
		return fmt.Errorf("%w: --agent must select %q", adapter.ErrSemanticMismatch, run.AgentID)
	}

	raw, ok := adapter.ProjectionContent(np, SettingsRel)
	if !ok && (intent.Has(adapter.ControlSandbox) || intent.Has(adapter.ControlPermissions) || intent.Has(adapter.ControlApprovals)) {
		return fmt.Errorf("%w: missing %s", adapter.ErrSemanticMismatch, SettingsRel)
	}
	if !ok {
		return nil
	}
	var set ProjectedSettings
	if err := json.Unmarshal(raw, &set); err != nil {
		return fmt.Errorf("%w: invalid %s: %v", adapter.ErrSemanticMismatch, SettingsRel, err)
	}
	if intent.Has(adapter.ControlSandbox) {
		if set.Sandbox == nil || !set.Sandbox.Enabled || !set.Sandbox.FailIfUnavailable || set.Sandbox.AllowUnsandboxedCommands {
			return fmt.Errorf("%w: sandbox must be enabled with fail-closed semantics", adapter.ErrSemanticMismatch)
		}
		if claudeShellAskFloor(run) {
			if set.Sandbox.AutoAllowBashIfSandboxed == nil || *set.Sandbox.AutoAllowBashIfSandboxed {
				return fmt.Errorf("%w: shell ask floor requires autoAllowBashIfSandboxed=false", adapter.ErrSemanticMismatch)
			}
		}
		if adapter.ClaudeShouldDenyHomeReads(pctx) {
			fs := set.Sandbox.Filesystem
			if fs == nil || !adapter.StringSetContains(fs.DenyRead, "~/") || !adapter.StringSetContains(fs.AllowRead, ".") {
				return fmt.Errorf("%w: sandbox.filesystem must denyRead ~/ and allowRead . when layout is outside home", adapter.ErrSemanticMismatch)
			}
		}
	}
	if intent.Has(adapter.ControlPermissions) || intent.Has(adapter.ControlApprovals) {
		if set.Permissions == nil {
			return fmt.Errorf("%w: permissions block missing in %s", adapter.ErrSemanticMismatch, SettingsRel)
		}
		wantDeny, wantAsk, wantAllow := claudePermissionRules(run, report, pctx)
		if !adapter.StringSetEqual(set.Permissions.Deny, wantDeny) ||
			!adapter.StringSetEqual(set.Permissions.Ask, wantAsk) ||
			!adapter.StringSetEqual(set.Permissions.Allow, wantAllow) {
			return fmt.Errorf("%w: projected permission rules do not match adapter.ControlIntent synthesis", adapter.ErrSemanticMismatch)
		}
	}
	return nil
}
