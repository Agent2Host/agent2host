package adapter_test

import (
	"errors"
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/committed"
	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/space"
)

func registerOfficial(t *testing.T, system, agent string) *space.ResolvedAgentRun {
	t.Helper()
	src, err := fixtures.OfficialSystem(system)
	if err != nil {
		t.Fatal(err)
	}
	sp, err := space.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sp.Register(src); err != nil {
		t.Fatal(err)
	}
	run, err := sp.Resolve(system, agent, "")
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestOfficialSystemsRegister(t *testing.T) {
	for _, system := range []string{"dev-studio", "ops-desk", "research-lab"} {
		src, err := fixtures.OfficialSystem(system)
		if err != nil {
			t.Fatal(err)
		}
		sp, err := space.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		rep, err := sp.Register(src)
		if err != nil {
			t.Fatalf("%s register: %v", system, err)
		}
		if rep.SystemID != system {
			t.Fatalf("%s id %q", system, rep.SystemID)
		}
	}
}

func TestCodeReviewerDefaultNetworkDeny(t *testing.T) {
	run := registerOfficial(t, "dev-studio", "code-reviewer")
	reg := committed.New(foundLook(), stubVersion)
	want := map[string]string{
		adapter.HostClaudeCode: "allowed_with_warnings",
		adapter.HostKiro:       "allowed_with_warnings",
		adapter.HostCodex:      "allowed_with_warnings",
	}
	for host, decision := range want {
		out, err := adapter.RunPipeline(reg, host, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
		if err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		if out.Report.Decision != decision {
			t.Fatalf("%s code-reviewer decision %s, want %s perms=%+v appr=%+v",
				host, out.Report.Decision, decision, out.Report.Security.Permissions, out.Report.Security.Approvals)
		}
	}
}

func TestWebResearcherNetworkAllow(t *testing.T) {
	run := registerOfficial(t, "research-lab", "web-researcher")
	reg := committed.New(foundLook(), stubVersion)
	for _, host := range []string{adapter.HostClaudeCode, adapter.HostKiro} {
		out, err := adapter.RunPipeline(reg, host, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
		if err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		if out.Report.Decision == "refused" {
			t.Fatalf("%s web-researcher refused: perms=%+v appr=%+v",
				host, out.Report.Security.Permissions, out.Report.Security.Approvals)
		}
		if out.Report.Security.Permissions.ReasonCode == "permission_overgrant" {
			t.Fatalf("%s treated network allow as overgrant: %+v", host, out.Report.Security.Permissions)
		}
	}
}

func TestDeployGuardSandbox(t *testing.T) {
	run := registerOfficial(t, "ops-desk", "deploy-guard")
	reg := committed.New(foundLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostKiro, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision != "refused" {
		t.Fatalf("kiro deploy-guard must refuse, got %s sbx=%+v", out.Report.Decision, out.Report.Security.Sandbox)
	}
	out, err = adapter.RunPipeline(reg, adapter.HostCodex, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision == "refused" && out.Report.Security.Approvals.ReasonCode == "approval_weaker" {
		t.Fatalf("codex must not refuse for asking less on authorized shell: appr=%+v",
			out.Report.Security.Approvals)
	}
}

func TestProjectRejectsTamperedAssessIntent(t *testing.T) {
	run := withNetworkAllow(sampleRun(false, false))
	reg := committed.New(foundLook(), stubVersion)
	ev, err := adapter.EvaluatePipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Report.Decision == "refused" {
		t.Fatal("expected allowed evaluation")
	}
	ev.Intent.Controls = append(ev.Intent.Controls, adapter.PlannedControl{
		Kind: adapter.ControlSandbox, Via: adapter.ViaProjectionFile, Rel: "nonexistent/settings.json",
	})
	_, err = ev.Project(run, adapter.ProjectionContext{})
	if !errors.Is(err, adapter.ErrIntentMismatch) {
		t.Fatalf("want adapter.ErrIntentMismatch, got %v", err)
	}
}

func TestMCPIsolationHostEnforcedWhenPlanned(t *testing.T) {
	run := registerOfficial(t, "dev-studio", "code-reviewer")
	reg := committed.New(foundLook(), stubVersion)
	for _, host := range []string{adapter.HostClaudeCode, adapter.HostKiro, adapter.HostCodex} {
		ev, err := adapter.EvaluatePipeline(reg, host, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
		if err != nil {
			t.Fatal(err)
		}
		if !ev.Intent.Has(adapter.ControlMCP) {
			t.Fatalf("%s: code-reviewer must plan MCP controls", host)
		}
		if len(ev.Report.Security.MCPToolIsolation.Items) == 0 {
			t.Fatalf("%s: expected mcp_tool_isolation items", host)
		}
		for _, it := range ev.Report.Security.MCPToolIsolation.Items {
			if it.Enforcement != "host_enforced" {
				t.Fatalf("%s server %s: want host_enforced, got %+v", host, it.ServerID, it)
			}
			if it.RequirementResult != "satisfied" {
				t.Fatalf("%s server %s: %+v", host, it.ServerID, it)
			}
		}
	}
}
