package adapter_test

import (
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/claude"
	"github.com/agent2host/agent2host/internal/adapter/codex"
	"github.com/agent2host/agent2host/internal/adapter/kiro"
	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/space"
)

func TestPermissionsCeilingClaudeBaselineUnverified(t *testing.T) {
	if claude.PermissionsCeiling(sampleRun(false, false)) != adapter.CeilingUnverified {
		t.Fatal("claude default network deny is unproven and must be unverified")
	}
}

func TestPermissionsCeilingKiroBaselineUnverified(t *testing.T) {
	if kiro.PermissionsCeiling(sampleRun(false, false)) != adapter.CeilingUnverified {
		t.Fatal("kiro default network deny is unproven and must be unverified")
	}
}

func TestPermissionsCeilingCodexBaselineWithin(t *testing.T) {
	if codex.PermissionsCeiling(sampleRun(false, false)) != adapter.CeilingWithin {
		t.Fatal("codex baseline WD + documented network deny should be within")
	}
}

func TestPermissionsCeilingNetworkAllow(t *testing.T) {
	allow := "allow"
	run := sampleRun(false, false)
	run.Permissions = &decode.Permissions{
		Network: &decode.NetworkPermissions{Default: &allow},
	}
	if claude.SessionNetwork(run) != adapter.EffectSilent ||
		codex.SessionNetwork(run) != adapter.EffectSilent ||
		kiro.SessionNetwork(run) != adapter.EffectSilent {
		t.Fatal("each host must report silent when it enables network")
	}
	if claude.PermissionsCeiling(run) != adapter.CeilingWithin ||
		codex.PermissionsCeiling(run) != adapter.CeilingWithin ||
		kiro.PermissionsCeiling(run) != adapter.CeilingWithin {
		t.Fatal("declared network allow plus usable session must be within")
	}
}

func TestCodexApprovalGateUsableForAlways(t *testing.T) {
	if got := codex.ApprovalGateVsDeclared("always"); got != "equal" {
		t.Fatalf("authorized shell must stay usable, got %q", got)
	}
}

func TestCodexApprovalGateEqualForNever(t *testing.T) {
	if got := codex.ApprovalGateVsDeclared("never"); got != "equal" {
		t.Fatalf("got %q", got)
	}
}

func TestFsCeilingWorkingDirectoryOnlyRejectsExtraScope(t *testing.T) {
	home := "home"
	wd := "working_directory"
	run := &space.ResolvedAgentRun{Permissions: &decode.Permissions{
		Filesystem: &decode.FilesystemPermissions{Read: &[]string{wd, home}},
	}}
	if adapter.FSCeilingWorkingDirectoryOnly(run) {
		t.Fatal("home scope must fail the shared FS ceiling")
	}
	if claude.PermissionsCeiling(run) != adapter.CeilingOvergrant ||
		codex.PermissionsCeiling(run) != adapter.CeilingOvergrant ||
		kiro.PermissionsCeiling(run) != adapter.CeilingOvergrant {
		t.Fatal("host ceiling wrappers must use the same FS ceiling")
	}
}

func TestFsCeilingNilRun(t *testing.T) {
	if !adapter.FSCeilingWorkingDirectoryOnly(nil) {
		t.Fatal("omitted FS is within the WD-only ceiling")
	}
	if claude.PermissionsCeiling(nil) != adapter.CeilingUnverified ||
		kiro.PermissionsCeiling(nil) != adapter.CeilingUnverified ||
		codex.PermissionsCeiling(nil) != adapter.CeilingUnverified {
		t.Fatal("nil run cannot prove a network deny")
	}
}

func TestCodexApprovalGateEqualForOnBoundary(t *testing.T) {
	if got := codex.ApprovalGateVsDeclared("on_boundary"); got != "equal" {
		t.Fatalf("on_boundary should be equal without chaining grant, got %q", got)
	}
}
