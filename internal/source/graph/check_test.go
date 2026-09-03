package graph_test

import (
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/graph"
	"github.com/agent2host/agent2host/internal/source/normalize"
	"github.com/agent2host/agent2host/internal/source/rule"
)

func boolPtr(b bool) *bool { return &b }

func skillSys() *decode.SystemSource {
	tools := []decode.MCPToolRef{{ServerID: "club-database", ToolName: "search_policy"}}
	return &decode.SystemSource{
		Skills: &map[string]decode.SkillEntry{
			"search-policy": {
				Name:     "Search",
				Document: "./skills/search-policy.skill.md",
				MCPTools: &tools,
			},
		},
	}
}

func TestPhaseB(t *testing.T) {
	sys := skillSys()

	t.Run("server-missing", func(t *testing.T) {
		a := &decode.AgentSource{
			ID:     "demo",
			SOP:    "./sops/demo.sop.md",
			Skills: &[]decode.SkillRef{{ID: "search-policy"}},
		}
		normalize.Agent(a)
		_, err := graph.Closure(sys, []*decode.AgentSource{a}, []string{"agents/demo.agent.json"})
		if rule.ID(err) != "SRC-MCP-SKILL-REF" {
			t.Fatalf("got %v", err)
		}
		if strings.Contains(err.Error(), "club-database/search_policy") {
			t.Fatalf("must not join identity with /: %v", err)
		}
		if !strings.Contains(err.Error(), `server_id="club-database"`) || !strings.Contains(err.Error(), `tool_name="search_policy"`) {
			t.Fatalf("detail %v", err)
		}
	})

	t.Run("tools-empty", func(t *testing.T) {
		a := &decode.AgentSource{
			ID:     "demo",
			SOP:    "./sops/demo.sop.md",
			Skills: &[]decode.SkillRef{{ID: "search-policy"}},
			MCPServers: &map[string]decode.MCPServer{
				"club-database": {Transport: "stdio", Command: "python", Tools: []decode.ToolAllowlistEntry{}},
			},
		}
		normalize.Agent(a)
		_, err := graph.Closure(sys, []*decode.AgentSource{a}, []string{"agents/demo.agent.json"})
		if rule.ID(err) != "SRC-MCP-SKILL-REF" {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("tool-missing", func(t *testing.T) {
		a := &decode.AgentSource{
			ID:     "demo",
			SOP:    "./sops/demo.sop.md",
			Skills: &[]decode.SkillRef{{ID: "search-policy"}},
			MCPServers: &map[string]decode.MCPServer{
				"club-database": {
					Transport: "stdio",
					Command:   "python",
					Tools:     []decode.ToolAllowlistEntry{{Name: "other_tool"}},
				},
			},
		}
		normalize.Agent(a)
		_, err := graph.Closure(sys, []*decode.AgentSource{a}, []string{"agents/demo.agent.json"})
		if rule.ID(err) != "SRC-MCP-SKILL-REF" {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("tool-allowed", func(t *testing.T) {
		a := &decode.AgentSource{
			ID:     "demo",
			SOP:    "./sops/demo.sop.md",
			Skills: &[]decode.SkillRef{{ID: "search-policy"}},
			MCPServers: &map[string]decode.MCPServer{
				"club-database": {
					Transport: "stdio",
					Command:   "python",
					Tools:     []decode.ToolAllowlistEntry{{Name: "search_policy"}},
				},
			},
		}
		normalize.Agent(a)
		if _, err := graph.Closure(sys, []*decode.AgentSource{a}, []string{"agents/demo.agent.json"}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("short-skill-ref", func(t *testing.T) {
		a := &decode.AgentSource{
			ID:     "demo",
			SOP:    "./sops/demo.sop.md",
			Skills: &[]decode.SkillRef{{ID: "search-policy"}},
			MCPServers: &map[string]decode.MCPServer{
				"club-database": {
					Transport: "stdio",
					Command:   "python",
					Tools:     []decode.ToolAllowlistEntry{{Name: "search_policy"}},
				},
			},
		}
		normalize.Agent(a)
		if _, err := graph.Closure(sys, []*decode.AgentSource{a}, []string{"agents/demo.agent.json"}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("long-skill-ref", func(t *testing.T) {
		a := &decode.AgentSource{
			ID:     "demo",
			SOP:    "./sops/demo.sop.md",
			Skills: &[]decode.SkillRef{{ID: "search-policy", Required: boolPtr(true)}},
			MCPServers: &map[string]decode.MCPServer{
				"club-database": {
					Transport: "stdio",
					Command:   "python",
					Tools:     []decode.ToolAllowlistEntry{{Name: "search_policy"}},
				},
			},
		}
		normalize.Agent(a)
		if _, err := graph.Closure(sys, []*decode.AgentSource{a}, []string{"agents/demo.agent.json"}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("optional-skill-still-phase-b", func(t *testing.T) {
		a := &decode.AgentSource{
			ID:     "demo",
			SOP:    "./sops/demo.sop.md",
			Skills: &[]decode.SkillRef{{ID: "search-policy", Required: boolPtr(false)}},
		}
		normalize.Agent(a)
		_, err := graph.Closure(sys, []*decode.AgentSource{a}, []string{"agents/demo.agent.json"})
		if rule.ID(err) != "SRC-MCP-SKILL-REF" {
			t.Fatalf("optional skill must not skip Phase B: %v", err)
		}
	})
}

func TestSkillCollectionDuplicatePaths(t *testing.T) {
	cases := []struct {
		name string
		sk   decode.SkillEntry
	}{
		{
			name: "scripts",
			sk: decode.SkillEntry{
				Name:     "Search",
				Document: "./skills/search-policy.skill.md",
				Scripts:  &[]string{"./scripts/a.sh", "./scripts/a.sh"},
			},
		},
		{
			name: "contexts",
			sk: decode.SkillEntry{
				Name:     "Search",
				Document: "./skills/search-policy.skill.md",
				Contexts: &[]string{"./contexts/a.md", "./contexts/a.md"},
			},
		},
		{
			name: "assets",
			sk: decode.SkillEntry{
				Name:     "Search",
				Document: "./skills/search-policy.skill.md",
				Assets:   &[]string{"./assets/a.bin", "./assets/a.bin"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sys := &decode.SystemSource{
				Skills: &map[string]decode.SkillEntry{"search-policy": tc.sk},
			}
			if err := graph.System(sys); rule.ID(err) != "SRC-REF-DUP-DECL" {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestSkillDuplicateMCPToolRef(t *testing.T) {
	tools := []decode.MCPToolRef{
		{ServerID: "club-database", ToolName: "search_policy"},
		{ServerID: "club-database", ToolName: "search_policy"},
	}
	sys := &decode.SystemSource{
		Skills: &map[string]decode.SkillEntry{
			"search-policy": {
				Name:     "Search",
				Document: "./skills/search-policy.skill.md",
				MCPTools: &tools,
			},
		},
	}
	if err := graph.System(sys); rule.ID(err) != "SRC-MCP-SKILL-REF" {
		t.Fatalf("got %v", err)
	}
}

func TestEmptyToolsAllowlistWarns(t *testing.T) {
	a := &decode.AgentSource{
		ID:  "demo",
		SOP: "./sops/demo.sop.md",
		MCPServers: &map[string]decode.MCPServer{
			"club-database": {Transport: "stdio", Command: "python", Tools: []decode.ToolAllowlistEntry{}},
		},
	}
	normalize.Agent(a)
	warns, err := graph.Agent(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 || warns[0].ID != "SRC-MCP-TOOLS-REQ" {
		t.Fatalf("warns %+v", warns)
	}
}
