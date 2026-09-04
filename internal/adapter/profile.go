package adapter

import (
	"encoding/json"
	"fmt"

	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/space"
)

// profile is this Adapter version’s documented Host mapping. It is Assess
// input, not AdapterDescriptor content.
type Profile struct {
	ActivationMode   string
	SOPSupport       string
	SkillSupport     string
	ContextSupport   string
	MCPSupport       string
	HookSupport      string
	IsolationScope   string
	IsolationEnforce string
	// MCPToolIsolationEnforce is Assess-time MCP tool visibility enforcement when
	// ControlMCP is planned. Empty defaults to unknown (safe for unverified Hosts).
	MCPToolIsolationEnforce string
	// No permGrantSubseteq here on purpose. The grant marker must be derived
	// from the projection, never a hand-written profile constant. It is
	// computed per run by each host's securityPolicy (see *_adapter.go).
	ApprovalGate        string
	ApprovalEnforce     string
	SandboxSupport      string
	SandboxEnforce      string
	OutputSchemaSupport string
	OutputValEnforce    string
}

type SecurityPolicy func(run *space.ResolvedAgentRun, probe ProbeResult, intent ControlIntent, p Profile) (compatibility.PolicyAssess, compatibility.PolicyAssess)

func Assess(run *space.ResolvedAgentRun, probe ProbeResult, p Profile, intent ControlIntent, security SecurityPolicy) compatibility.Assessment {
	a := compatibility.Assessment{Security: &compatibility.SecurityAssess{}}
	if !probe.Found {
		a.Activation = &compatibility.ActivationAssess{Mode: "unsupported", Confidence: "documented"}
	} else {
		a.Activation = &compatibility.ActivationAssess{Mode: p.ActivationMode, Confidence: "documented"}
	}

	turn, _ := json.Marshal(true)
	a.SOP = &compatibility.SOPAssess{
		Support: p.SOPSupport, Scope: "agent", Confidence: "documented",
		AppliesFromTurnOne: turn,
	}
	if !probe.Found {
		a.SOP.Support = "unsupported"
		a.SOP.AppliesFromTurnOne = nil
	}

	for _, sk := range run.Skills {
		sup := p.SkillSupport
		conf := "documented"
		if !probe.Found {
			sup, conf = "unsupported", "documented"
		}
		a.Skills = append(a.Skills, compatibility.SkillAssess{
			ID: sk.ID, Support: sup, Scope: "agent", Confidence: conf,
		})
	}

	for _, ctx := range run.Contexts {
		req := ctx.Required
		loading := "on_demand"
		iso := "required"
		if ctx.Loading != nil {
			loading = *ctx.Loading
		}
		if ctx.Isolation != nil {
			iso = *ctx.Isolation
		}
		sup := p.ContextSupport
		if !probe.Found {
			sup = "unsupported"
		}
		a.Contexts = append(a.Contexts, compatibility.ContextAssess{
			Path: ctx.Path, Required: req, Loading: loading, Isolation: iso,
			Support: sup, Scope: "agent", Confidence: "documented",
		})
		if iso == "required" {
			a.ContextIsolation = append(a.ContextIsolation, compatibility.IsolationAssess{
				Path: ctx.Path, Required: reqBool(req, true),
				Support: "mapped", Scope: p.IsolationScope,
				Enforcement: plannedEnforce(intent, ControlIsolation, p.IsolationEnforce),
				Confidence:  "documented",
			})
		}
	}

	for _, sid := range SortedMCPServerIDs(run) {
		srv := run.MCPServers[sid]
		connected := probe.Found
		for _, tool := range srv.Tools {
			req := true
			if tool.Required != nil {
				req = *tool.Required
			}
			sup := p.MCPSupport
			if !probe.Found {
				sup = "unsupported"
			}
			inv := probe.Found
			a.MCP = append(a.MCP, compatibility.MCPAssess{
				ServerID: sid, Name: tool.Name, Required: &req,
				Support: sup, Scope: "agent", Confidence: "documented",
				Invocable: &inv, ServerConnected: &connected,
			})
		}
		a.MCPToolIsolation = append(a.MCPToolIsolation, compatibility.MCPIsoAssess{
			ServerID:    sid,
			Support:     "approximate",
			Scope:       p.IsolationScope,
			Enforcement: mcpToolIsolationEnforcement(intent, p),
			Confidence:  "documented",
		})
	}

	if run.Hooks != nil {
		add := func(prefix string, n int, required *bool) {
			req := true
			if required != nil {
				req = *required
			}
			sup := p.HookSupport
			if !probe.Found {
				sup = "unsupported"
			}
			a.Hooks = append(a.Hooks, compatibility.HookAssess{
				Ref: fmt.Sprintf("%s/%d", prefix, n), Required: &req,
				Support: sup, Scope: "agent", Confidence: "documented",
			})
		}
		if run.Hooks.SessionStart != nil {
			for i, h := range *run.Hooks.SessionStart {
				add("/hooks/session_start", i, h.Required)
			}
		}
		if run.Hooks.BeforeToolCall != nil {
			for i, h := range *run.Hooks.BeforeToolCall {
				add("/hooks/before_tool_call", i, h.Required)
			}
		}
		if run.Hooks.AfterToolCall != nil {
			for i, h := range *run.Hooks.AfterToolCall {
				add("/hooks/after_tool_call", i, h.Required)
			}
		}
		if run.Hooks.AgentStop != nil {
			for i, h := range *run.Hooks.AgentStop {
				add("/hooks/agent_stop", i, h.Required)
			}
		}
	}

	if run.Output != nil {
		sup := p.OutputSchemaSupport
		if !probe.Found {
			sup = "unsupported"
		}
		a.OutputSchema = &compatibility.OrdinaryAssess{
			Support: sup, Scope: "agent", Confidence: "documented",
		}
		a.Security.OutputValidation = &compatibility.PolicyAssess{
			Support: "mapped", Scope: "agent", Enforcement: p.OutputValEnforce, Confidence: "documented",
		}
	}

	perm, appr := security(run, probe, intent, p)
	a.Security.Permissions = &perm
	a.Security.Approvals = &appr
	sbxEnforce := plannedEnforce(intent, ControlSandbox, p.SandboxEnforce)
	if !intent.Has(ControlSandbox) && (p.SandboxEnforce == "none" || p.SandboxSupport == "unsupported") {
		sbxEnforce = p.SandboxEnforce
	}
	a.Security.Sandbox = &compatibility.PolicyAssess{
		Support: p.SandboxSupport, Scope: "agent", Enforcement: sbxEnforce, Confidence: "documented",
	}
	if sbxEnforce == "none" || sbxEnforce == "prompt_only" {
		looser := "looser"
		a.Security.Sandbox.ModeVsDeclared = looser
	}

	a.SecretIsolation = secretFacts(run, probe, intent)
	return a
}

// plannedEnforce reports host_enforced only when this run's ControlIntent
// includes the control. It does not mean the mapped rule is a complete
// workspace-read whitelist — that is StrictReadEnforced plus support.
func plannedEnforce(intent ControlIntent, kind, claimed string) string {
	if intent.Has(kind) {
		return claimed
	}
	return "unknown"
}

func reqBool(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func secretFacts(run *space.ResolvedAgentRun, probe ProbeResult, intent ControlIntent) []compatibility.SecretAssess {
	var out []compatibility.SecretAssess
	add := func(consumer, target string, required bool) {
		scope, enforce, conf := "unknown", "unknown", "documented"
		if !probe.Found {
			scope, enforce, conf = "unknown", "unknown", "documented"
		} else if s, ok := intent.secretBinding(consumer, target); ok {
			scope = s.Scope
			enforce = s.Enforcement
			if enforce == "" {
				enforce = "host_enforced"
			}
		} else if HostProcessConsumer(consumer) {
			scope, enforce = "agent", "host_enforced"
		} else {
			scope = "host"
		}
		out = append(out, compatibility.SecretAssess{
			Consumer: consumer, Target: target, Required: required,
			Support: "mapped", Scope: scope, Enforcement: enforce, Confidence: conf,
		})
	}
	for _, b := range run.Environment {
		req := true
		if b.Required != nil {
			req = *b.Required
		}
		add("/environment", b.ValueFrom.Environment, req)
	}
	for _, sid := range SortedMCPServerIDs(run) {
		for _, b := range run.MCPServers[sid].Environment {
			req := true
			if b.Required != nil {
				req = *b.Required
			}
			add("/mcp_servers/"+sid, b.ValueFrom.Environment, req)
		}
	}
	if run.Hooks != nil {
		addHook := func(prefix string, entries *[]decode.HookEntry) {
			if entries == nil {
				return
			}
			for i, h := range *entries {
				if h.Environment == nil {
					continue
				}
				for _, b := range *h.Environment {
					req := true
					if b.Required != nil {
						req = *b.Required
					}
					add(fmt.Sprintf("%s/%d", prefix, i), b.ValueFrom.Environment, req)
				}
			}
		}
		addHook("/hooks/session_start", run.Hooks.SessionStart)
		addHook("/hooks/before_tool_call", run.Hooks.BeforeToolCall)
		addHook("/hooks/after_tool_call", run.Hooks.AfterToolCall)
		addHook("/hooks/agent_stop", run.Hooks.AgentStop)
	}
	return out
}

func mcpToolIsolationEnforcement(intent ControlIntent, p Profile) string {
	claimed := p.MCPToolIsolationEnforce
	if claimed == "" {
		claimed = "unknown"
	}
	return plannedEnforce(intent, ControlMCP, claimed)
}
