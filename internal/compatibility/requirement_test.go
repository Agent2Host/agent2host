package compatibility

import (
	"testing"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/space"
)

func TestBuildRequirementEffectiveRequired(t *testing.T) {
	falseVal := false
	trueVal := true
	run := &space.ResolvedAgentRun{
		SystemID: "sys",
		AgentID:  "demo",
		Skills: []space.ResolvedSkill{
			{ID: "search-policy", Required: true, MCPTools: []decode.MCPToolRef{{ServerID: "db", ToolName: "from-skill"}}},
			{ID: "pretty", Required: false, MCPTools: []decode.MCPToolRef{{ServerID: "db", ToolName: "opt-only"}}},
		},
		MCPServers: map[string]space.ResolvedMCP{
			"db": {
				Tools: []space.ResolvedTool{
					{Name: "from-skill", Required: &falseVal},
					{Name: "opt-only", Required: &falseVal},
					{Name: "allow-req", Required: nil},
				},
			},
		},
		Sandbox: &decode.Sandbox{Required: &trueVal},
		Output:  &decode.Output{Schema: "out.json", Enforcement: strPtr("required")},
		Environment: []decode.EnvironmentBinding{
			{ValueFrom: decode.ValueFrom{Environment: "OPENAI_API_KEY"}, Required: &falseVal},
		},
	}
	req, err := BuildRequirement(run)
	if err != nil {
		t.Fatal(err)
	}
	if !req.SOP.Required {
		t.Fatal("SOP must be required")
	}
	if !req.Security.Sandbox.Required {
		t.Fatal("sandbox.required true must be hard")
	}
	if req.Security.OutputValidation == nil || !req.Security.OutputValidation.Required {
		t.Fatal("output.enforcement required must trigger output_validation")
	}
	want := map[string]bool{
		"db/from-skill": true,
		"db/opt-only":   false,
		"db/allow-req":  true,
	}
	got := map[string]bool{}
	for _, m := range req.MCP {
		got[m.ServerID+"/"+m.Name] = m.Required
	}
	if !mapsEq(want, got) {
		t.Fatalf("effective_required: got %v want %v", got, want)
	}
	if len(req.Secrets) != 1 || req.Secrets[0].Target != "OPENAI_API_KEY" || req.Secrets[0].Required {
		t.Fatalf("secrets: %+v", req.Secrets)
	}
}

func TestBuildRequirementSandboxOptional(t *testing.T) {
	run := &space.ResolvedAgentRun{SystemID: "s", AgentID: "a"}
	req, err := BuildRequirement(run)
	if err != nil {
		t.Fatal(err)
	}
	if req.Security.Sandbox.Required {
		t.Fatal("omitted sandbox must not be a hard gate")
	}
}

func strPtr(s string) *string { return &s }

func mapsEq(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
