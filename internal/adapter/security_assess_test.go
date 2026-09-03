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

func TestPermissionsGrantSubseteqClaudeBaseline(t *testing.T) {
	if !claude.PermissionsGrantSubseteq(sampleRun(false, false)) {
		t.Fatal("claude baseline WD scopes should be grant-subseteq")
	}
}

func TestPermissionsGrantSubseteqKiroBaseline(t *testing.T) {
	if !kiro.PermissionsGrantSubseteq(sampleRun(false, false)) {
		t.Fatal("kiro baseline WD + network deny should be grant-subseteq at capability level")
	}
}

func TestPermissionsGrantSubseteqCodexBaseline(t *testing.T) {
	if !codex.PermissionsGrantSubseteq(sampleRun(false, false)) {
		t.Fatal("codex baseline WD + network deny should be grant-subseteq at sandbox level")
	}
}

func TestCodexApprovalGateWeakerForAlways(t *testing.T) {
	if got := codex.ApprovalGateVsDeclared("always", true); got != "weaker" {
		t.Fatalf("got %q", got)
	}
}

func TestCodexApprovalGateEqualForNever(t *testing.T) {
	if got := codex.ApprovalGateVsDeclared("never", true); got != "equal" {
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
	if claude.PermissionsGrantSubseteq(run) || codex.PermissionsGrantSubseteq(run) || kiro.PermissionsGrantSubseteq(run) {
		t.Fatal("host grant wrappers must use the same FS ceiling")
	}
}

func TestFsCeilingNilRun(t *testing.T) {
	if !adapter.FSCeilingWorkingDirectoryOnly(nil) {
		t.Fatal("omitted FS is within the WD-only ceiling")
	}
	if !claude.PermissionsGrantSubseteq(nil) {
		t.Fatal("claude omitted permissions stay grant-subseteq")
	}
	if kiro.PermissionsGrantSubseteq(nil) || codex.PermissionsGrantSubseteq(nil) {
		t.Fatal("codex/kiro nil run stays outside grant")
	}
}

func TestCodexApprovalGateEqualForOnBoundaryWithGrant(t *testing.T) {
	if got := codex.ApprovalGateVsDeclared("on_boundary", true); got != "equal" {
		t.Fatalf("on_boundary + sandbox grant should be equal, got %q", got)
	}
}
