package adapter_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/committed"
	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/space"
)

// registerClubFAQ registers the published club-system fixture and resolves club-faq.
func registerClubFAQ(t *testing.T) *space.ResolvedAgentRun {
	t.Helper()
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	sp, err := space.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sp.Register(filepath.Join(root, "trees", "valid", "club-system")); err != nil {
		t.Fatal(err)
	}
	run, err := sp.Resolve("club-system", "club-faq", "")
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestClubFAQCheckAllowedAllCommittedHosts(t *testing.T) {
	run := registerClubFAQ(t)
	reg := committed.New(foundLook(), stubVersion)
	for _, host := range []string{adapter.HostClaudeCode, adapter.HostKiro, adapter.HostCodex} {
		out, err := adapter.RunPipeline(reg, host, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
		if err != nil {
			t.Fatalf("%s pipeline: %v", host, err)
		}
		if out.Report.Decision == "refused" {
			t.Fatalf("%s refused club-faq: activation=%+v perms=%+v appr=%+v mcp_iso=%+v",
				host, out.Report.Activation, out.Report.Security.Permissions,
				out.Report.Security.Approvals, out.Report.Security.MCPToolIsolation)
		}
		if out.Plans == nil {
			t.Fatalf("%s: expected plans", host)
		}
	}
}

func TestProjectRejectsTamperedAssessIntent(t *testing.T) {
	run := sampleRun(false, false)
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
	run := registerClubFAQ(t)
	reg := committed.New(foundLook(), stubVersion)
	for _, host := range []string{adapter.HostClaudeCode, adapter.HostKiro, adapter.HostCodex} {
		ev, err := adapter.EvaluatePipeline(reg, host, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
		if err != nil {
			t.Fatal(err)
		}
		if !ev.Intent.Has(adapter.ControlMCP) {
			t.Fatalf("%s: club-faq must plan MCP controls", host)
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
