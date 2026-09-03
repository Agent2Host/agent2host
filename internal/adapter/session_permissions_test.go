package adapter

import (
	"testing"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/space"
)

func TestComparePermissionsDeclaredAllowNeedsUsableEffect(t *testing.T) {
	allow := "allow"
	run := &space.ResolvedAgentRun{Permissions: &decode.Permissions{
		Network: &decode.NetworkPermissions{Default: &allow},
	}}
	if ComparePermissions(run, SessionFacts{Network: EffectSilent}) != CeilingWithin {
		t.Fatal("declared allow + silent must be within")
	}
	if ComparePermissions(run, SessionFacts{Network: EffectAsk}) != CeilingWithin {
		t.Fatal("declared allow + ask is a usable grant")
	}
	if ComparePermissions(run, SessionFacts{Network: EffectDeny}) != CeilingOvergrant {
		t.Fatal("declared allow + deny is not a usable grant")
	}
	if ComparePermissions(run, SessionFacts{Network: EffectUnknown}) != CeilingOvergrant {
		t.Fatal("declared allow + unknown does not satisfy a grant")
	}
}

func TestComparePermissionsUndeclaredNetwork(t *testing.T) {
	run := &space.ResolvedAgentRun{} // omitted network → deny
	if ComparePermissions(run, SessionFacts{Network: EffectSilent}) != CeilingOvergrant {
		t.Fatal("undeclared + silent is overgrant")
	}
	if ComparePermissions(run, SessionFacts{Network: EffectDeny}) != CeilingWithin {
		t.Fatal("undeclared + proven deny is controlled")
	}
	if ComparePermissions(run, SessionFacts{Network: EffectAsk}) != CeilingWithin {
		t.Fatal("undeclared + effective ask is controlled")
	}
	if ComparePermissions(run, SessionFacts{Network: EffectUnknown}) != CeilingUnverified {
		t.Fatal("undeclared + unknown must be unverified, not overgrant")
	}
}

func TestComparePermissionsRejectsExtraFSScope(t *testing.T) {
	home := "home"
	wd := "working_directory"
	run := &space.ResolvedAgentRun{Permissions: &decode.Permissions{
		Filesystem: &decode.FilesystemPermissions{Read: &[]string{wd, home}},
	}}
	if ComparePermissions(run, SessionFacts{Network: EffectDeny}) != CeilingOvergrant {
		t.Fatal("extra FS scope is outside the shared ceiling")
	}
}

func TestPermissionPolicyFields(t *testing.T) {
	grant, vs := PermissionPolicyFields(CeilingUnverified)
	if !grant || vs != "unverified" {
		t.Fatalf("unverified: grant=%v vs=%s", grant, vs)
	}
	grant, vs = PermissionPolicyFields(CeilingOvergrant)
	if grant || vs != "overgrant" {
		t.Fatalf("overgrant: grant=%v vs=%s", grant, vs)
	}
}
