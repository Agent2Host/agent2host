package codex

import (
	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/space"
)

type codexAdapter struct {
	look    adapter.LookPathFunc
	version adapter.VersionFunc
}

func New(look adapter.LookPathFunc, version adapter.VersionFunc) adapter.HostAdapter {
	return &codexAdapter{look: look, version: version}
}

func (a *codexAdapter) HostState() adapter.HostStateBinder { return a }

func (a *codexAdapter) DescribeAuth() adapter.AuthDescription {
	return adapter.AuthDescription{
		Profile: adapter.AuthProfileKey{
			Host:                adapter.HostCodex,
			Provider:            "openai",
			NativeAuthNamespace: "codex-home",
		},
		Topology:    adapter.AuthTopologySeparated,
		Concurrency: adapter.AuthConcurrencyUnverified,
		Materials: []adapter.AuthMaterial{{
			StoreRel: "auth.json",
			BindRoot: adapter.AuthRootPrivate,
			BindRel:  HomeDirRel + "/auth.json",
			Lock:     true,
		}},
	}
}

func (a *codexAdapter) BindForRun(adapter.AuthBindRequest) (adapter.AuthBindDirective, error) {
	return adapter.AuthBindDirective{
		Copies: a.DescribeAuth().Materials,
		Env:    map[string]string{EnvHome: adapter.PrivateToken + "/" + HomeDirRel},
	}, nil
}

func (a *codexAdapter) FinalizeRun(adapter.AuthFinalizeRequest) (adapter.AuthFinalizeDirective, error) {
	return adapter.AuthFinalizeDirective{Copies: a.DescribeAuth().Materials}, nil
}

func (a *codexAdapter) Descriptor() adapter.Descriptor {
	return adapter.Descriptor{AdapterID: adapter.HostCodex, HostID: adapter.HostCodex, AdapterVersion: adapter.AdapterVersion}
}

func (a *codexAdapter) Probe() (adapter.ProbeResult, error) {
	return adapter.ProbeBinary(adapter.HostCodex, "codex", a.look, a.version)
}

func (a *codexAdapter) Assess(run *space.ResolvedAgentRun, probe adapter.ProbeResult) (compatibility.Assessment, adapter.ControlIntent, error) {
	if run == nil {
		return compatibility.Assessment{}, adapter.ControlIntent{}, adapter.ErrNilRun
	}
	intent := Intent(run, probe)
	assess := adapter.Assess(run, probe, codexProFile(), intent, codexSecurityPolicy)
	return assess, intent, nil
}

func (a *codexAdapter) Project(run *space.ResolvedAgentRun, probe adapter.ProbeResult, report compatibility.Report, pctx adapter.ProjectionContext) (adapter.NativeProjectionPlan, adapter.LaunchPlan, error) {
	if err := adapter.RefuseIfNeeded(report); err != nil {
		return adapter.NativeProjectionPlan{}, adapter.LaunchPlan{}, err
	}
	if run == nil {
		return adapter.NativeProjectionPlan{}, adapter.LaunchPlan{}, adapter.ErrNilRun
	}
	files := adapter.ProjectSOPAndSkills(run, report, adapter.PrefixedSOP("AGENTS.md"), adapter.SkillDir(SkillsDirRel), adapter.DestHostPrivate)
	files = adapter.BannerSOPFiles(files, "AGENTS.md", run)
	lp := adapter.Launch(probe, "codex:AGENTS.md")
	files, lp = projectCodex(run, report, files, lp)
	lp.Secrets = adapter.SecretRefs(run)
	files = adapter.MarkExecutableFiles(files, run)
	np := adapter.NativeProjectionPlan{HostID: adapter.HostCodex, Files: files}
	intent := Intent(run, probe)
	if err := ReconcilePlans(run, intent, report, np, lp, pctx); err != nil {
		return adapter.NativeProjectionPlan{}, adapter.LaunchPlan{}, err
	}
	return np, lp, nil
}

func codexProFile() adapter.Profile {
	return adapter.Profile{
		ActivationMode: "primary_mapped",
		SOPSupport:     "mapped", SkillSupport: "mapped", ContextSupport: "mapped",
		MCPSupport: "mapped", HookSupport: "approximate",
		IsolationScope: "agent", IsolationEnforce: "host_enforced",
		MCPToolIsolationEnforce: "host_enforced",
		ApprovalGate:            "equal", ApprovalEnforce: "host_enforced",
		SandboxSupport: "mapped", SandboxEnforce: "host_enforced",
		OutputSchemaSupport: "mapped", OutputValEnforce: "none",
	}
}

func codexSecurityPolicy(run *space.ResolvedAgentRun, probe adapter.ProbeResult, intent adapter.ControlIntent, p adapter.Profile) (compatibility.PolicyAssess, compatibility.PolicyAssess) {
	ceiling := PermissionsCeiling(run)
	grant, vs := adapter.PermissionPolicyFields(ceiling)
	permEnforce := adapter.PermissionsEnforcement(grant, intent)
	gate := ApprovalGateVsDeclared(adapter.ShellExecuteDeclared(run))
	apprEnforce := adapter.ApprovalsEnforcement(gate, intent, p)

	permSupport := "approximate"
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
func ReconcilePlans(run *space.ResolvedAgentRun, intent adapter.ControlIntent, _ compatibility.Report, np adapter.NativeProjectionPlan, lp adapter.LaunchPlan, _ adapter.ProjectionContext) error {
	return reconcileSemanticCodex(run, intent, np, lp)
}
