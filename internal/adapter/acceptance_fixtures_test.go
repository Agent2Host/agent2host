package adapter_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/claude"
	"github.com/agent2host/agent2host/internal/adapter/committed"
	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/space"
)

func TestAcceptanceBasicSystemRegisters(t *testing.T) {
	root := acceptanceFixtureRoot(t, "basic-system")
	home := t.TempDir()
	sp, err := space.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := sp.Register(root)
	if err != nil {
		t.Fatal(err)
	}
	if rep.SystemID != "basic-system" {
		t.Fatalf("system %q", rep.SystemID)
	}
	run, err := sp.Resolve("basic-system", "help", "")
	if err != nil {
		t.Fatal(err)
	}
	reg := committed.New(foundLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision == "refused" {
		t.Fatalf("basic-system refused: %+v", out.Report)
	}
}

func TestAcceptanceSecurityStrictKiroRefusesSandbox(t *testing.T) {
	root := acceptanceFixtureRoot(t, "security-strict")
	home := t.TempDir()
	sp, err := space.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sp.Register(root); err != nil {
		t.Fatal(err)
	}
	run, err := sp.Resolve("security-strict", "guard", "")
	if err != nil {
		t.Fatal(err)
	}
	reg := committed.New(foundLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostKiro, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision != "refused" {
		t.Fatalf("kiro must refuse sandbox.required, got %q", out.Report.Decision)
	}
}

func TestAcceptanceSecurityStrictAllCommittedHosts(t *testing.T) {
	root := acceptanceFixtureRoot(t, "security-strict")
	home := t.TempDir()
	sp, err := space.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sp.Register(root); err != nil {
		t.Fatal(err)
	}
	run, err := sp.Resolve("security-strict", "guard", "")
	if err != nil {
		t.Fatal(err)
	}
	reg := committed.New(foundLook(), stubVersion)
	want := map[string]string{
		adapter.HostClaudeCode: "allowed",
		adapter.HostKiro:       "refused",
		adapter.HostCodex:      "refused",
	}
	for host, decision := range want {
		out, err := adapter.RunPipeline(reg, host, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
		if err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		if out.Report.Decision != decision {
			t.Fatalf("%s decision %q want %q", host, out.Report.Decision, decision)
		}
	}
}

func TestClaudeHomeReadDenyOutsideHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	wd := filepath.Join(os.TempDir(), "a2h-claude-wd")
	private := filepath.Join(os.TempDir(), "a2h-claude-private")
	if adapter.PathUnderHome(wd, home) {
		t.Skip("temp layout not outside $HOME")
	}
	pctx := adapter.ProjectionContext{ApprovedWorkingDirectory: wd, RunPrivateDirectory: private}

	reg := committed.New(foundLook(), stubVersion)
	run := sampleRun(false, false)
	ev, err := adapter.EvaluatePipeline(reg, adapter.HostClaudeCode, run, pctx, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	plans, err := ev.Project(run, pctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := adapter.ProjectionContent(plans.Projection, claude.SettingsRel)
	if !ok {
		t.Fatal("missing settings")
	}
	var set claude.ProjectedSettings
	if err := json.Unmarshal(raw, &set); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range set.Permissions.Deny {
		if d == adapter.ClaudeHomeReadDenyRule {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("deny list missing %q: %v", adapter.ClaudeHomeReadDenyRule, set.Permissions.Deny)
	}
	if set.Sandbox == nil || set.Sandbox.Filesystem == nil {
		t.Fatal("sandbox.filesystem required when layout is outside home")
	}
	if !adapter.StringSetContains(set.Sandbox.Filesystem.DenyRead, "~/") || !adapter.StringSetContains(set.Sandbox.Filesystem.AllowRead, ".") {
		t.Fatalf("sandbox.filesystem %+v", set.Sandbox.Filesystem)
	}
}

func TestClaudeHomeReadDenySkippedInsideHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	wd := filepath.Join(home, "a2h-inside-home-wd")
	private := filepath.Join(home, "a2h-inside-home-private")
	pctx := adapter.ProjectionContext{ApprovedWorkingDirectory: wd, RunPrivateDirectory: private}

	reg := committed.New(foundLook(), stubVersion)
	run := sampleRun(false, false)
	ev, err := adapter.EvaluatePipeline(reg, adapter.HostClaudeCode, run, pctx, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	plans, err := ev.Project(run, pctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := adapter.ProjectionContent(plans.Projection, claude.SettingsRel)
	if !ok {
		t.Fatal("missing settings")
	}
	var set claude.ProjectedSettings
	if err := json.Unmarshal(raw, &set); err != nil {
		t.Fatal(err)
	}
	for _, d := range set.Permissions.Deny {
		if d == adapter.ClaudeHomeReadDenyRule {
			t.Fatalf("home deny must not project when layout under $HOME: %v", set.Permissions.Deny)
		}
	}
	if set.Sandbox != nil && set.Sandbox.Filesystem != nil && adapter.StringSetContains(set.Sandbox.Filesystem.DenyRead, "~/") {
		t.Fatalf("denyRead ~/ must not project when workspace is under $HOME: %+v", set.Sandbox.Filesystem)
	}
}

func TestProbeReadBoundaryScriptExists(t *testing.T) {
	mod := repoRootFromFixtures(t)
	script := filepath.Join(mod, "test", "scripts", "probe-read-boundary.sh")
	st, err := os.Stat(script)
	if os.IsNotExist(err) {
		t.Skip("local test/scripts not present (test/ is gitignored)")
	}
	if err != nil || st.Mode()&0o111 == 0 {
		t.Fatalf("missing or non-executable %s: %v", script, err)
	}
}

func repoRootFromFixtures(t *testing.T) string {
	t.Helper()
	fx, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	mod, err := filepath.Abs(filepath.Join(fx, "..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return mod
}

func acceptanceFixtureRoot(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join(repoRootFromFixtures(t), "test", "acceptance", name)
	if _, err := os.Stat(filepath.Join(root, "system.json")); err != nil {
		t.Skipf("local test/acceptance/%s not present (test/ is gitignored): %v", name, err)
	}
	return root
}
