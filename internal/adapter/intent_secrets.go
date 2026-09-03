package adapter

import (
	"fmt"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/space"
)

// AppendEnvironmentSecrets copies Source environment bindings into Intent.
// Host-independent — do not reimplement per Host.
func AppendEnvironmentSecrets(c *ControlIntent, run *space.ResolvedAgentRun) {
	if c == nil || run == nil {
		return
	}
	for _, b := range run.Environment {
		c.Secrets = append(c.Secrets, PlannedSecret{
			Consumer: "/environment", Target: b.ValueFrom.Environment, Scope: "agent",
		})
	}
}

// AppendMCPSecrets copies MCP server environment bindings into Intent.
// Host-independent — do not reimplement per Host. MCP controls stay Host-specific.
func AppendMCPSecrets(c *ControlIntent, run *space.ResolvedAgentRun) {
	if c == nil || run == nil {
		return
	}
	for _, sid := range SortedMCPServerIDs(run) {
		for _, b := range run.MCPServers[sid].Environment {
			c.Secrets = append(c.Secrets, PlannedSecret{
				Consumer: "/mcp_servers/" + sid, Target: b.ValueFrom.Environment, Scope: "agent",
			})
		}
	}
}

// AppendHookSecrets copies hook environment bindings into Intent.
// Host-independent — do not reimplement per Host. Hook controls stay Host-specific.
func AppendHookSecrets(c *ControlIntent, run *space.ResolvedAgentRun) {
	if c == nil || run == nil || run.Hooks == nil {
		return
	}
	add := func(prefix string, entries *[]decode.HookEntry) {
		if entries == nil {
			return
		}
		for i, h := range *entries {
			if h.Environment == nil {
				continue
			}
			for _, b := range *h.Environment {
				c.Secrets = append(c.Secrets, PlannedSecret{
					Consumer: fmt.Sprintf("%s/%d", prefix, i),
					Target:   b.ValueFrom.Environment,
					Scope:    "agent",
				})
			}
		}
	}
	add("/hooks/session_start", run.Hooks.SessionStart)
	add("/hooks/before_tool_call", run.Hooks.BeforeToolCall)
	add("/hooks/after_tool_call", run.Hooks.AfterToolCall)
	add("/hooks/agent_stop", run.Hooks.AgentStop)
}
