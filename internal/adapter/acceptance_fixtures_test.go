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

func TestAcceptanceWebResearcherAllowedOnClaude(t *testing.T) {
	src, err := fixtures.OfficialSystem("research-lab")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	sp, err := space.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := sp.Register(src)
	if err != nil {
		t.Fatal(err)
	}
	if rep.SystemID != "research-lab" {
		t.Fatalf("system %q", rep.SystemID)
	}
	run, err := sp.Resolve("research-lab", "web-researcher", "")
	if err != nil {
		t.Fatal(err)
	}
	reg := committed.New(foundLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision == "refused" {
		t.Fatalf("web-researcher refused: %+v", out.Report)
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
	run := withNetworkAllow(sampleRun(false, false))
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
	run := withNetworkAllow(sampleRun(false, false))
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
