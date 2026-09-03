package codex

import (
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/space"
)

func testRun() *space.ResolvedAgentRun {
	sr := false
	return &space.ResolvedAgentRun{
		SystemID: "club-system",
		AgentID:  "club-faq",
		SOP:      "sops/a.sop.md",
		Content:  map[string][]byte{"sops/a.sop.md": []byte("# sop\n")},
		Sandbox:  &decode.Sandbox{Required: &sr},
	}
}

func TestCheckCodexProjectedConfigRejectsLegacySandbox(t *testing.T) {
	body := []byte(`
features.apps = false
default_permissions = "a2h"
approval_policy = "on-request"
sandbox_mode = "workspace-write"

[permissions.a2h]
extends = ":read-only"

[permissions.a2h.filesystem]
":root" = "deny"
":workspace_roots" = "write"

[permissions.a2h.network]
enabled = false
`)
	err := checkCodexProjectedConfig(testRun(), body)
	if err == nil || !strings.Contains(err.Error(), "sandbox_mode") {
		t.Fatalf("want sandbox_mode reject, got %v", err)
	}
}

func TestCheckCodexProjectedConfigAcceptsProfile(t *testing.T) {
	run := testRun()
	body, _ := codexConfigTOML(run, compatibility.Report{}, "# sop\n")
	if err := checkCodexProjectedConfig(run, body); err != nil {
		t.Fatal(err)
	}
}
