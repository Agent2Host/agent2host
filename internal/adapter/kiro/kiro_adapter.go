package kiro

import (
	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/space"
)

type kiroAdapter struct {
	look    adapter.LookPathFunc
	version adapter.VersionFunc
}

func New(look adapter.LookPathFunc, version adapter.VersionFunc) adapter.HostAdapter {
	return &kiroAdapter{look: look, version: version}
}

func (a *kiroAdapter) HostState() adapter.HostStateBinder { return a }

func (a *kiroAdapter) DescribeAuth() adapter.AuthDescription {
	return adapter.AuthDescription{
		Profile: adapter.AuthProfileKey{
			Host:                adapter.HostKiro,
			Provider:            "kiro",
			NativeAuthNamespace: "external-session",
		},
		Topology:       adapter.AuthTopologyExternal,
		Concurrency:    adapter.AuthConcurrencySafe,
		ExecSecretEnvs: []string{"KIRO_API_KEY"},
	}
}

func (a *kiroAdapter) BindForRun(adapter.AuthBindRequest) (adapter.AuthBindDirective, error) {
	return adapter.AuthBindDirective{}, nil
}

func (a *kiroAdapter) FinalizeRun(adapter.AuthFinalizeRequest) (adapter.AuthFinalizeDirective, error) {
	return adapter.AuthFinalizeDirective{}, nil
}

func (a *kiroAdapter) Descriptor() adapter.Descriptor {
	return adapter.Descriptor{AdapterID: adapter.HostKiro, HostID: adapter.HostKiro, AdapterVersion: adapter.AdapterVersion}
}

func (a *kiroAdapter) Probe() (adapter.ProbeResult, error) {
	return adapter.ProbeBinary(adapter.HostKiro, "kiro-cli", a.look, a.version)
}

func (a *kiroAdapter) Assess(run *space.ResolvedAgentRun, probe adapter.ProbeResult) (compatibility.Assessment, adapter.ControlIntent, error) {
	if run == nil {
		return compatibility.Assessment{}, adapter.ControlIntent{}, adapter.ErrNilRun
	}
	intent := kiroIntent(run, probe)
	assess := adapter.Assess(run, probe, kiroProFile(), intent, kiroSecurityPolicy)
	assess = kiroAssessSOP(assess, probe)
	return assess, intent, nil
}

func (a *kiroAdapter) Project(run *space.ResolvedAgentRun, probe adapter.ProbeResult, report compatibility.Report, pctx adapter.ProjectionContext) (adapter.NativeProjectionPlan, adapter.LaunchPlan, error) {
	if err := adapter.RefuseIfNeeded(report); err != nil {
		return adapter.NativeProjectionPlan{}, adapter.LaunchPlan{}, err
	}
	if run == nil {
		return adapter.NativeProjectionPlan{}, adapter.LaunchPlan{}, adapter.ErrNilRun
	}
	files := adapter.ProjectSOPAndSkills(run, report, adapter.PrefixedSOP(SOPRel), adapter.SkillDir(SkillsDirRel), adapter.DestProjection)
	files = append(files, adapter.ProjectionFile{
		RelPath:     AgentsDirRel + "/" + run.AgentID + ".json",
		Class:       adapter.DestProjection,
		Content:     kiroAgentJSON(run),
		FromContent: run.SOP,
	})
	lp := adapter.Launch(probe, "kiro:--agent")
	files, lp = projectKiro(run, report, files, lp)
	lp.Secrets = adapter.SecretRefs(run)
	files = adapter.MarkExecutableFiles(files, run)
	np := adapter.NativeProjectionPlan{HostID: adapter.HostKiro, Files: files}
	intent := kiroIntent(run, probe)
	if err := ReconcilePlans(run, intent, report, np, lp, pctx); err != nil {
		return adapter.NativeProjectionPlan{}, adapter.LaunchPlan{}, err
	}
	return np, lp, nil
}

func kiroProFile() adapter.Profile {
	return adapter.Profile{
		ActivationMode: "primary_native",
		SOPSupport:     "mapped", SkillSupport: "mapped", ContextSupport: "mapped",
		MCPSupport: "mapped", HookSupport: "mapped",
		IsolationScope: "agent", IsolationEnforce: "host_enforced",
		MCPToolIsolationEnforce: "host_enforced",
		ApprovalGate:            "equal", ApprovalEnforce: "host_enforced",
		// Local Kiro CLI has no OS sandbox (plan §5.3). Evaluate refuses when sandbox.required.
		SandboxSupport: "unsupported", SandboxEnforce: "none",
		OutputSchemaSupport: "mapped", OutputValEnforce: "none",
	}
}

func kiroSecurityPolicy(run *space.ResolvedAgentRun, probe adapter.ProbeResult, intent adapter.ControlIntent, p adapter.Profile) (compatibility.PolicyAssess, compatibility.PolicyAssess) {
	grant := PermissionsGrantSubseteq(run)
	permEnforce := adapter.PermissionsEnforcement(grant, intent)
	gate := kiroApprovalGateVsDeclared(adapter.ShellExecuteDeclared(run))
	apprEnforce := adapter.ApprovalsEnforcement(gate, intent, p)

	permSupport := "approximate"
	if !probe.Found {
		permSupport = "unsupported"
		grant = false
		gate = ""
	}

	perm := compatibility.PolicyAssess{
		Support:               permSupport,
		Scope:                 "agent",
		Enforcement:           permEnforce,
		Confidence:            "documented",
		GrantSubseteqDeclared: &grant,
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
func ReconcilePlans(run *space.ResolvedAgentRun, intent adapter.ControlIntent, report compatibility.Report, np adapter.NativeProjectionPlan, lp adapter.LaunchPlan, _ adapter.ProjectionContext) error {
	return reconcileSemanticKiro(run, intent, report, np, lp)
}
