package path_test

import (
	"testing"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/path"
)

func TestSystemDeclsSkillOrderStable(t *testing.T) {
	sys := &decode.SystemSource{
		Agents: []string{"./agents/z.agent.json", "./agents/a.agent.json"},
		Skills: &map[string]decode.SkillEntry{
			"zeta":  {Document: "./skills/zeta.skill.md"},
			"alpha": {Document: "./skills/alpha.skill.md"},
		},
	}
	got := path.SystemDecls(sys)
	want := []string{
		"./agents/z.agent.json",
		"./agents/a.agent.json",
		"./skills/alpha.skill.md",
		"./skills/zeta.skill.md",
	}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d: %+v", len(got), len(want), got)
	}
	for i, d := range got {
		if d.Authoring != want[i] {
			t.Fatalf("decl[%d]=%q want %q", i, d.Authoring, want[i])
		}
	}
}

func TestAgentDeclsMCPOrderStable(t *testing.T) {
	a := &decode.AgentSource{
		SOP: "./sops/demo.sop.md",
		MCPServers: &map[string]decode.MCPServer{
			"z-server": {Files: &[]string{"./mcp/z.py"}},
			"a-server": {Files: &[]string{"./mcp/a.py"}},
		},
	}
	got := path.AgentDecls(a)
	want := []string{"./sops/demo.sop.md", "./mcp/a.py", "./mcp/z.py"}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d: %+v", len(got), len(want), got)
	}
	for i, d := range got {
		if d.Authoring != want[i] {
			t.Fatalf("decl[%d]=%q want %q", i, d.Authoring, want[i])
		}
	}
}
