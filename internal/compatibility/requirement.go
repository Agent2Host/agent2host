package compatibility

import (
	"fmt"
	"sort"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/space"
)

// Requirement is Evaluate's Agent-side input (fixture-local or built from IR).
type Requirement struct {
	Activation *struct {
		ModeIntent string `json:"mode_intent"`
	} `json:"activation,omitempty"`
	SOP *struct {
		Required bool `json:"required"`
	} `json:"sop,omitempty"`
	Skills       []SkillReq   `json:"skills,omitempty"`
	Contexts     []ContextReq `json:"contexts,omitempty"`
	MCP          []MCPReq     `json:"mcp,omitempty"`
	Hooks        []HookReq    `json:"hooks,omitempty"`
	OutputSchema *struct {
		Required bool `json:"required"`
	} `json:"output_schema,omitempty"`
	Secrets  []SecretReq  `json:"secrets,omitempty"`
	Security *SecurityReq `json:"security,omitempty"`
}

type SkillReq struct {
	ID          string `json:"id"`
	Required    bool   `json:"required"`
	ExclusiveOf string `json:"exclusive_of,omitempty"`
}

type ContextReq struct {
	Path      string `json:"path"`
	Required  bool   `json:"required"`
	Isolation string `json:"isolation,omitempty"`
	Loading   string `json:"loading,omitempty"`
}

type MCPReq struct {
	ServerID string `json:"server_id"`
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

type HookReq struct {
	Ref      string `json:"ref"`
	Required bool   `json:"required"`
}

type SecretReq struct {
	Consumer string `json:"consumer"`
	Target   string `json:"target"`
	Required bool   `json:"required"`
}

type FlagReq struct {
	Required bool `json:"required"`
}

type SecurityReq struct {
	Permissions      *FlagReq `json:"permissions,omitempty"`
	Approvals        *FlagReq `json:"approvals,omitempty"`
	Sandbox          *FlagReq `json:"sandbox,omitempty"`
	OutputValidation *FlagReq `json:"output_validation,omitempty"`
}

func (r Requirement) sandboxRequired() bool {
	return r.Security != nil && r.Security.Sandbox != nil && r.Security.Sandbox.Required
}

func (r Requirement) outputValidationRequired() bool {
	return r.Security != nil && r.Security.OutputValidation != nil && r.Security.OutputValidation.Required
}

// BuildRequirement maps a ResolvedAgentRun onto Evaluate inputs.
// effective_required is computed on the Source graph before omit.
func BuildRequirement(run *space.ResolvedAgentRun) (Requirement, error) {
	if run == nil {
		return Requirement{}, fmt.Errorf("compatibility: nil ResolvedAgentRun")
	}
	req := Requirement{
		SOP: &struct {
			Required bool `json:"required"`
		}{Required: true},
		Activation: &struct {
			ModeIntent string `json:"mode_intent"`
		}{ModeIntent: "primary"},
		Security: &SecurityReq{
			Permissions: &FlagReq{Required: true},
			Approvals:   &FlagReq{Required: true},
			Sandbox:     &FlagReq{Required: sandboxHard(run)},
		},
	}

	for _, sk := range run.Skills {
		req.Skills = append(req.Skills, SkillReq{ID: sk.ID, Required: sk.Required})
	}

	for _, ctx := range run.Contexts {
		c := ContextReq{Path: ctx.Path, Required: true, Loading: "on_demand"}
		if ctx.Required != nil {
			c.Required = *ctx.Required
		}
		if ctx.Loading != nil {
			c.Loading = *ctx.Loading
		}
		if ctx.Isolation != nil {
			c.Isolation = *ctx.Isolation
		}
		req.Contexts = append(req.Contexts, c)
	}

	requiredSkillTools := map[string]bool{}
	for _, sk := range run.Skills {
		if !sk.Required {
			continue
		}
		for _, ref := range sk.MCPTools {
			requiredSkillTools[ref.ServerID+"\x00"+ref.ToolName] = true
		}
	}

	serverIDs := make([]string, 0, len(run.MCPServers))
	for id := range run.MCPServers {
		serverIDs = append(serverIDs, id)
	}
	sort.Strings(serverIDs)
	for _, sid := range serverIDs {
		srv := run.MCPServers[sid]
		for _, tool := range srv.Tools {
			allowRequired := true
			if tool.Required != nil {
				allowRequired = *tool.Required
			}
			eff := allowRequired || requiredSkillTools[sid+"\x00"+tool.Name]
			req.MCP = append(req.MCP, MCPReq{ServerID: sid, Name: tool.Name, Required: eff})
		}
	}

	if run.Hooks != nil {
		req.Hooks = append(req.Hooks, hookReqs("/hooks/session_start", run.Hooks.SessionStart)...)
		req.Hooks = append(req.Hooks, hookReqs("/hooks/before_tool_call", run.Hooks.BeforeToolCall)...)
		req.Hooks = append(req.Hooks, hookReqs("/hooks/after_tool_call", run.Hooks.AfterToolCall)...)
		req.Hooks = append(req.Hooks, hookReqs("/hooks/agent_stop", run.Hooks.AgentStop)...)
	}

	if run.Output != nil {
		req.OutputSchema = &struct {
			Required bool `json:"required"`
		}{Required: true}
		enforcement := "best_effort"
		if run.Output.Enforcement != nil && *run.Output.Enforcement != "" {
			enforcement = *run.Output.Enforcement
		}
		if enforcement == "required" {
			req.Security.OutputValidation = &FlagReq{Required: true}
		} else {
			req.Security.OutputValidation = &FlagReq{Required: false}
		}
	}

	req.Secrets = append(req.Secrets, secretReqs("/environment", run.Environment)...)
	for _, sid := range serverIDs {
		srv := run.MCPServers[sid]
		req.Secrets = append(req.Secrets, secretReqs("/mcp_servers/"+sid, srv.Environment)...)
	}
	if run.Hooks != nil {
		req.Secrets = append(req.Secrets, hookSecrets("/hooks/session_start", run.Hooks.SessionStart)...)
		req.Secrets = append(req.Secrets, hookSecrets("/hooks/before_tool_call", run.Hooks.BeforeToolCall)...)
		req.Secrets = append(req.Secrets, hookSecrets("/hooks/after_tool_call", run.Hooks.AfterToolCall)...)
		req.Secrets = append(req.Secrets, hookSecrets("/hooks/agent_stop", run.Hooks.AgentStop)...)
	}
	return req, nil
}

func sandboxHard(run *space.ResolvedAgentRun) bool {
	if run.Sandbox == nil || run.Sandbox.Required == nil {
		return false
	}
	return *run.Sandbox.Required
}

func hookReqs(prefix string, entries *[]decode.HookEntry) []HookReq {
	if entries == nil {
		return nil
	}
	out := make([]HookReq, 0, len(*entries))
	for i, h := range *entries {
		req := true
		if h.Required != nil {
			req = *h.Required
		}
		out = append(out, HookReq{Ref: fmt.Sprintf("%s/%d", prefix, i), Required: req})
	}
	return out
}

func secretReqs(consumer string, env []decode.EnvironmentBinding) []SecretReq {
	if len(env) == 0 {
		return nil
	}
	out := make([]SecretReq, 0, len(env))
	for _, b := range env {
		req := true
		if b.Required != nil {
			req = *b.Required
		}
		out = append(out, SecretReq{Consumer: consumer, Target: b.ValueFrom.Environment, Required: req})
	}
	return out
}

func hookSecrets(prefix string, entries *[]decode.HookEntry) []SecretReq {
	if entries == nil {
		return nil
	}
	var out []SecretReq
	for i, h := range *entries {
		if h.Environment == nil {
			continue
		}
		out = append(out, secretReqs(fmt.Sprintf("%s/%d", prefix, i), *h.Environment)...)
	}
	return out
}
