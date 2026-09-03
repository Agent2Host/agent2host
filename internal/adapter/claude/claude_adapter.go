package claude

import (
	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/space"
)

type claudeCodeAdapter struct {
	look    adapter.LookPathFunc
	version adapter.VersionFunc
}

func New(look adapter.LookPathFunc, version adapter.VersionFunc) adapter.HostAdapter {
	return &claudeCodeAdapter{look: look, version: version}
}

func (a *claudeCodeAdapter) HostState() adapter.HostStateBinder { return a }

func (a *claudeCodeAdapter) DescribeAuth() adapter.AuthDescription {
	return adapter.AuthDescription{
		Profile: adapter.AuthProfileKey{
			Host:                adapter.HostClaudeCode,
			Provider:            "anthropic",
			NativeAuthNamespace: "claude-config-dir",
		},
		// Official: CLAUDE_CONFIG_DIR is the macOS Keychain identity space.
		// A new per-run config dir is a new login. Do not copy Keychain or
		// ~/.claude. Settings/MCP use official --settings / --mcp-config overlays.
		Topology:    adapter.AuthTopologyBoundRoot,
		Concurrency: adapter.AuthConcurrencyUnverified,
	}
}

func (a *claudeCodeAdapter) BindForRun(adapter.AuthBindRequest) (adapter.AuthBindDirective, error) {
	return adapter.AuthBindDirective{
		Env: map[string]string{EnvConfig: adapter.AuthProfileToken},
	}, nil
}

func (a *claudeCodeAdapter) FinalizeRun(adapter.AuthFinalizeRequest) (adapter.AuthFinalizeDirective, error) {
	return adapter.AuthFinalizeDirective{}, nil
}

func (a *claudeCodeAdapter) Descriptor() adapter.Descriptor {
	return adapter.Descriptor{AdapterID: adapter.HostClaudeCode, HostID: adapter.HostClaudeCode, AdapterVersion: adapter.AdapterVersion}
}

func (a *claudeCodeAdapter) Probe() (adapter.ProbeResult, error) {
	return adapter.ProbeBinary(adapter.HostClaudeCode, "claude", a.look, a.version)
}

func (a *claudeCodeAdapter) Assess(run *space.ResolvedAgentRun, probe adapter.ProbeResult) (compatibility.Assessment, adapter.ControlIntent, error) {
	if run == nil {
		return compatibility.Assessment{}, adapter.ControlIntent{}, adapter.ErrNilRun
	}
	intent := claudeIntent(run, probe)
	assess := adapter.Assess(run, probe, claudeProFile(), intent, claudeSecurityPolicy)
	return assess, intent, nil
}

func (a *claudeCodeAdapter) Project(run *space.ResolvedAgentRun, probe adapter.ProbeResult, report compatibility.Report, pctx adapter.ProjectionContext) (adapter.NativeProjectionPlan, adapter.LaunchPlan, error) {
	if err := adapter.RefuseIfNeeded(report); err != nil {
		return adapter.NativeProjectionPlan{}, adapter.LaunchPlan{}, err
	}
	if run == nil {
		return adapter.NativeProjectionPlan{}, adapter.LaunchPlan{}, adapter.ErrNilRun
	}
	files := adapter.ProjectSOPAndSkills(run, report, adapter.PrefixedSOP("CLAUDE.md"), adapter.SkillDir(SkillsDirRel), adapter.DestAuthProfile)
	files = append(files, adapter.ProjectionFile{
		RelPath:     AgentsDirRel + "/" + run.AgentID + ".md",
		Class:       adapter.DestAuthProfile,
		Content:     adapter.AgentCard(run, false),
		FromContent: run.AgentSpec,
	})
	lp := adapter.Launch(probe, "claude-code:--agent")
	files, lp = projectClaude(run, report, files, lp, pctx)
	lp.Secrets = adapter.SecretRefs(run)
	files = adapter.MarkExecutableFiles(files, run)
	np := adapter.NativeProjectionPlan{HostID: adapter.HostClaudeCode, Files: files}
	intent := claudeIntent(run, probe)
	if err := ReconcilePlans(run, intent, report, np, lp, pctx); err != nil {
		return adapter.NativeProjectionPlan{}, adapter.LaunchPlan{}, err
	}
	return np, lp, nil
}

func claudeProFile() adapter.Profile {
	return adapter.Profile{
		ActivationMode: "primary_native",
		SOPSupport:     "mapped", SkillSupport: "mapped", ContextSupport: "mapped",
		MCPSupport: "mapped", HookSupport: "mapped",
		IsolationScope: "agent", IsolationEnforce: "host_enforced",
		MCPToolIsolationEnforce: "host_enforced",
		ApprovalGate:            "equal", ApprovalEnforce: "host_enforced",
		SandboxSupport: "mapped", SandboxEnforce: "host_enforced",
		OutputSchemaSupport: "mapped", OutputValEnforce: "none",
	}
}

func claudeSecurityPolicy(run *space.ResolvedAgentRun, probe adapter.ProbeResult, intent adapter.ControlIntent, p adapter.Profile) (compatibility.PolicyAssess, compatibility.PolicyAssess) {
	ceiling := PermissionsCeiling(run)
	grant, vs := adapter.PermissionPolicyFields(ceiling)
	permEnforce := adapter.PermissionsEnforcement(grant, intent)
	gate := claudeApprovalGate(adapter.ShellExecuteDeclared(run))
	apprEnforce := adapter.ApprovalsEnforcement(gate, intent, p)

	permSupport := "mapped"
	// SRC-SEC-INTENT: the projected network deny only covers WebFetch/WebSearch
	// and curl/wget/nc-shaped Bash. Other shell, MCP, and browser exits stay
	// open, so this is not complete network isolation.
	if ceiling != adapter.CeilingWithin || adapter.NetworkDenied(run) {
		permSupport = "approximate"
	}
	if !probe.Found {
		permSupport = "unsupported"
		grant = false
		vs = string(adapter.CeilingOvergrant)
		gate = ""
	}

	perm := compatibility.PolicyAssess{
		Support:               permSupport,
		Scope:                 "agent",
		Enforcement:           permEnforce,
		Confidence:            "documented",
		GrantSubseteqDeclared: &grant,
		CeilingVsDeclared:     vs,
	}
	appr := compatibility.PolicyAssess{
		Support:        "mapped",
		Scope:          "agent",
		Enforcement:    apprEnforce,
		Confidence:     "documented",
		GateVsDeclared: gate,
	}
	if !probe.Found {
		appr.Support = "unsupported"
	}
	return perm, appr
}

// Semantic only. Do not call reconcilePlansCommon here — Evaluation.Project owns that.
func ReconcilePlans(run *space.ResolvedAgentRun, intent adapter.ControlIntent, report compatibility.Report, np adapter.NativeProjectionPlan, lp adapter.LaunchPlan, pctx adapter.ProjectionContext) error {
	return reconcileSemanticClaude(run, intent, report, np, lp, pctx)
}
