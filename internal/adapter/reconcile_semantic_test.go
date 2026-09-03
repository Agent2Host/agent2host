package adapter_test

import (
	"errors"
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/claude"
	"github.com/agent2host/agent2host/internal/adapter/codex"
	"github.com/agent2host/agent2host/internal/adapter/committed"
	"github.com/agent2host/agent2host/internal/compatibility"
)

func TestReconcilePlansClaudeSemanticOK(t *testing.T) {
	run := withNetworkAllow(sampleRun(false, false))
	reg := committed.New(foundLook(), stubVersion)
	ev, err := adapter.EvaluatePipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Report.Decision == "refused" {
		t.Fatalf("expected allowed, got refused")
	}
	plans, err := ev.Project(run, adapter.ProjectionContext{})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if err := claude.ReconcilePlans(run, ev.Intent, ev.Report, plans.Projection, plans.Launch, adapter.ProjectionContext{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func TestReconcileSemanticForbiddenBypass(t *testing.T) {
	run := withNetworkAllow(sampleRun(false, false))
	reg := committed.New(foundLook(), stubVersion)
	ev, err := adapter.EvaluatePipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	plans, err := ev.Project(run, adapter.ProjectionContext{})
	if err != nil {
		t.Fatal(err)
	}
	plans.Launch.Args = append(plans.Launch.Args, "--dangerously-skip-permissions")
	err = adapter.ReconcilePlansCommon(ev.Intent, ev.Report, plans.Projection, plans.Launch)
	if !errors.Is(err, adapter.ErrSemanticMismatch) {
		t.Fatalf("want adapter.ErrSemanticMismatch, got %v", err)
	}
}

func TestReconcileSemanticClaudeSettingsNotLoaded(t *testing.T) {
	run := withNetworkAllow(sampleRun(false, false))
	reg := committed.New(foundLook(), stubVersion)
	ev, err := adapter.EvaluatePipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	plans, err := ev.Project(run, adapter.ProjectionContext{})
	if err != nil {
		t.Fatal(err)
	}
	// Drop --settings while security controls remain in adapter.ControlIntent.
	filtered := plans.Launch.Args[:0]
	for _, a := range plans.Launch.Args {
		if a == "--settings" {
			continue
		}
		filtered = append(filtered, a)
	}
	plans.Launch.Args = filtered
	err = claude.ReconcilePlans(run, ev.Intent, ev.Report, plans.Projection, plans.Launch, adapter.ProjectionContext{})
	if !errors.Is(err, adapter.ErrSemanticMismatch) {
		t.Fatalf("want adapter.ErrSemanticMismatch, got %v", err)
	}
}

func TestReconcileSemanticClaudeWeakenedPermissions(t *testing.T) {
	run := withNetworkAllow(sampleRun(false, false))
	reg := committed.New(foundLook(), stubVersion)
	ev, err := adapter.EvaluatePipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	plans, err := ev.Project(run, adapter.ProjectionContext{})
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range plans.Projection.Files {
		if f.RelPath != claude.SettingsRel {
			continue
		}
		plans.Projection.Files[i].Content = []byte(`{
  "sandbox": {"enabled": true, "failIfUnavailable": true, "allowUnsandboxedCommands": false},
  "permissions": {"defaultMode": "default", "deny": [], "ask": [], "allow": ["Read"]}
}
`)
		break
	}
	err = claude.ReconcilePlans(run, ev.Intent, ev.Report, plans.Projection, plans.Launch, adapter.ProjectionContext{})
	if !errors.Is(err, adapter.ErrSemanticMismatch) {
		t.Fatalf("want adapter.ErrSemanticMismatch, got %v", err)
	}
}

func TestVerifyLaunchPlanCodexPrivateAddDir(t *testing.T) {
	run := withNetworkAllow(sampleRun(false, false))
	lp := adapter.LaunchPlan{
		Args: []string{
			"--ask-for-approval", codex.ApprovalFlag(run),
			"--add-dir", adapter.PrivateToken + "/codex-home",
		},
		Env: map[string]string{codex.EnvHome: adapter.HostAuthDir(adapter.HostCodex)},
	}
	err := codex.ReconcilePlans(run, codex.Intent(run, adapter.ProbeResult{Found: true}), compatibility.Report{}, adapter.NativeProjectionPlan{
		Files: []adapter.ProjectionFile{{RelPath: codex.ConfigRel, Content: []byte("x")}},
	}, lp, adapter.ProjectionContext{})
	if !errors.Is(err, adapter.ErrSemanticMismatch) {
		t.Fatalf("want adapter.ErrSemanticMismatch, got %v", err)
	}
}
