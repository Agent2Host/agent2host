package graph

import (
	"sort"
	"strconv"
	"strings"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/path"
	"github.com/agent2host/agent2host/internal/source/rule"
)

// SecretItem is one SRC-SECRET-REPORT identity (consumer + target).
type SecretItem struct {
	Consumer string
	Target   string
}

// Agent checks collection uniqueness, argv, and path shape on one Agent Spec.
func Agent(a *decode.AgentSource) ([]rule.Warning, error) {
	var warns []rule.Warning
	if err := uniqueSkillRefs(a); err != nil {
		return warns, err
	}
	if err := uniqueContexts(a); err != nil {
		return warns, err
	}
	if err := uniqueSecrets(a); err != nil {
		return warns, err
	}
	if err := uniqueToolsAndArgv(a, &warns); err != nil {
		return warns, err
	}
	for _, d := range path.AgentDecls(a) {
		if _, err := path.Shape(d); err != nil {
			return warns, err
		}
	}
	return warns, nil
}

// System checks agents[] canonical uniqueness and Skill registry shape.
func System(s *decode.SystemSource) error {
	seen := map[string]struct{}{}
	var agentCanon []string
	for _, p := range s.Agents {
		c, err := path.Shape(path.Decl{Authoring: p, Role: path.RoleAgentSpec})
		if err != nil {
			return err
		}
		if _, ok := seen[c]; ok {
			return rule.Fail("SRC-REF-AGENTS-LIST", p)
		}
		seen[c] = struct{}{}
		agentCanon = append(agentCanon, c)
	}
	if err := path.Collide(agentCanon); err != nil {
		return err
	}
	if s.Skills == nil {
		return nil
	}
	for id, sk := range *s.Skills {
		if err := skillEntry(id, sk); err != nil {
			return err
		}
	}
	return nil
}

func skillEntry(id string, sk decode.SkillEntry) error {
	if _, err := path.Shape(path.Decl{Authoring: sk.Document, Role: path.RoleSkillDoc}); err != nil {
		return err
	}
	if err := uniquePaths(sk.Scripts); err != nil {
		return err
	}
	if err := uniquePaths(sk.Contexts); err != nil {
		return err
	}
	if err := uniquePaths(sk.Assets); err != nil {
		return err
	}
	if sk.MCPTools == nil {
		return nil
	}
	type pair struct{ s, t string }
	seen := map[pair]struct{}{}
	for _, t := range *sk.MCPTools {
		p := pair{t.ServerID, t.ToolName}
		if _, ok := seen[p]; ok {
			return rule.Fail("SRC-MCP-SKILL-REF", id)
		}
		seen[p] = struct{}{}
	}
	return nil
}

func uniquePaths(list *[]string) error {
	if list == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var canon []string
	for _, p := range *list {
		c, err := path.Shape(path.Decl{Authoring: p, Role: path.RoleOther})
		if err != nil {
			return err
		}
		if _, ok := seen[c]; ok {
			return rule.Fail("SRC-REF-DUP-DECL", p)
		}
		seen[c] = struct{}{}
		canon = append(canon, c)
	}
	return path.Collide(canon)
}

func uniqueSkillRefs(a *decode.AgentSource) error {
	if a.Skills == nil {
		return nil
	}
	seen := map[string]struct{}{}
	for _, s := range *a.Skills {
		if _, ok := seen[s.ID]; ok {
			return rule.Fail("SRC-REF-DUP-DECL", s.ID)
		}
		seen[s.ID] = struct{}{}
	}
	return nil
}

func uniqueContexts(a *decode.AgentSource) error {
	if a.Contexts == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var canon []string
	for _, c := range *a.Contexts {
		p, err := path.Shape(path.Decl{Authoring: c.Path, Role: path.RoleContext})
		if err != nil {
			return err
		}
		if _, ok := seen[p]; ok {
			return rule.Fail("SRC-REF-DUP-DECL", c.Path)
		}
		seen[p] = struct{}{}
		canon = append(canon, p)
	}
	return path.Collide(canon)
}

func uniqueSecrets(a *decode.AgentSource) error {
	if err := uniqueBindingTargets(a.Environment); err != nil {
		return err
	}
	if a.MCPServers != nil {
		ids := make([]string, 0, len(*a.MCPServers))
		for id := range *a.MCPServers {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if err := uniqueBindingTargets((*a.MCPServers)[id].Environment); err != nil {
				return err
			}
		}
	}
	if a.Hooks == nil {
		return nil
	}
	for _, list := range []*[]decode.HookEntry{a.Hooks.SessionStart, a.Hooks.BeforeToolCall, a.Hooks.AfterToolCall, a.Hooks.AgentStop} {
		if list == nil {
			continue
		}
		for _, h := range *list {
			if err := uniqueBindingTargets(h.Environment); err != nil {
				return err
			}
		}
	}
	return nil
}

func uniqueBindingTargets(list *[]decode.EnvironmentBinding) error {
	if list == nil {
		return nil
	}
	seen := map[string]struct{}{}
	for _, b := range *list {
		n := b.ValueFrom.Environment
		if _, ok := seen[n]; ok {
			return rule.Fail("SRC-SECRET-DUP", n)
		}
		seen[n] = struct{}{}
	}
	return nil
}

func uniqueToolsAndArgv(a *decode.AgentSource, warns *[]rule.Warning) error {
	if a.MCPServers != nil {
		ids := make([]string, 0, len(*a.MCPServers))
		for id := range *a.MCPServers {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			srv := (*a.MCPServers)[id]
			if err := uniqueToolNames(srv.Tools); err != nil {
				return err
			}
			if len(srv.Tools) == 0 {
				*warns = append(*warns, rule.Warning{ID: "SRC-MCP-TOOLS-REQ", Detail: "empty tools allowlist on " + id})
			}
			args := []string{}
			if srv.Args != nil {
				args = *srv.Args
			}
			files := []string{}
			if srv.Files != nil {
				files = *srv.Files
			}
			if err := path.CheckProcess(srv.Command, args, files); err != nil {
				return err
			}
		}
	}
	if a.Hooks == nil {
		return nil
	}
	for _, list := range []*[]decode.HookEntry{a.Hooks.SessionStart, a.Hooks.BeforeToolCall, a.Hooks.AfterToolCall, a.Hooks.AgentStop} {
		if list == nil {
			continue
		}
		for _, h := range *list {
			args := []string{}
			if h.Args != nil {
				args = *h.Args
			}
			files := []string{}
			if h.Files != nil {
				files = *h.Files
			}
			if err := path.CheckProcess(h.Command, args, files); err != nil {
				return err
			}
		}
	}
	return nil
}

func uniqueToolNames(tools []decode.ToolAllowlistEntry) error {
	seen := map[string]struct{}{}
	for _, t := range tools {
		if _, ok := seen[t.Name]; ok {
			return rule.Fail("SRC-REF-DUP-DECL", t.Name)
		}
		seen[t.Name] = struct{}{}
	}
	return nil
}

// Closure checks duplicate Agent ids, Skill refs, and Phase B MCP allowlist.
func Closure(s *decode.SystemSource, agents []*decode.AgentSource, agentPaths []string) ([]rule.Warning, error) {
	var warns []rule.Warning
	seenID := map[string]struct{}{}
	for i, a := range agents {
		if _, ok := seenID[a.ID]; ok {
			return warns, rule.Fail("SRC-REF-DUP-AGENT", a.ID)
		}
		seenID[a.ID] = struct{}{}
		if i < len(agentPaths) {
			if w := filenameWarning(agentPaths[i], a.ID); w != nil {
				warns = append(warns, *w)
			}
		}
		if err := skillRefs(s, a); err != nil {
			return warns, err
		}
		if err := phaseB(s, a); err != nil {
			return warns, err
		}
	}
	return warns, nil
}

func filenameWarning(specPath, id string) *rule.Warning {
	base := specPath
	if i := strings.LastIndex(specPath, "/"); i >= 0 {
		base = specPath[i+1:]
	}
	stem := strings.TrimSuffix(base, ".agent.json")
	if stem != id {
		return &rule.Warning{ID: "SRC-REF-FILENAME", Detail: specPath}
	}
	return nil
}

func skillRefs(s *decode.SystemSource, a *decode.AgentSource) error {
	if a.Skills == nil {
		return nil
	}
	reg := map[string]struct{}{}
	if s.Skills != nil {
		for id := range *s.Skills {
			reg[id] = struct{}{}
		}
	}
	for _, ref := range *a.Skills {
		if _, ok := reg[ref.ID]; !ok {
			return rule.Fail("SRC-REF-SKILL", ref.ID)
		}
	}
	return nil
}

func phaseB(s *decode.SystemSource, a *decode.AgentSource) error {
	if a.Skills == nil || s.Skills == nil {
		return nil
	}
	for _, ref := range *a.Skills {
		sk, ok := (*s.Skills)[ref.ID]
		if !ok || sk.MCPTools == nil {
			continue
		}
		for _, t := range *sk.MCPTools {
			if a.MCPServers == nil {
				return rule.Fail("SRC-MCP-SKILL-REF", mcpDetail(t.ServerID, t.ToolName))
			}
			srv, ok := (*a.MCPServers)[t.ServerID]
			if !ok {
				return rule.Fail("SRC-MCP-SKILL-REF", mcpDetail(t.ServerID, t.ToolName))
			}
			found := false
			for _, allow := range srv.Tools {
				if allow.Name == t.ToolName {
					found = true
					break
				}
			}
			if !found {
				return rule.Fail("SRC-MCP-SKILL-REF", mcpDetail(t.ServerID, t.ToolName))
			}
		}
	}
	return nil
}

func mcpDetail(serverID, toolName string) string {
	return `server_id="` + serverID + `" tool_name="` + toolName + `"`
}

// Secrets lists SRC-SECRET-REPORT items in stable order.
func Secrets(a *decode.AgentSource) []SecretItem {
	var out []SecretItem
	if a.Environment != nil {
		for _, b := range *a.Environment {
			out = append(out, SecretItem{Consumer: "/environment", Target: b.ValueFrom.Environment})
		}
	}
	if a.MCPServers != nil {
		ids := make([]string, 0, len(*a.MCPServers))
		for id := range *a.MCPServers {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if (*a.MCPServers)[id].Environment == nil {
				continue
			}
			for _, b := range *(*a.MCPServers)[id].Environment {
				out = append(out, SecretItem{
					Consumer: "/mcp_servers/" + id,
					Target:   b.ValueFrom.Environment,
				})
			}
		}
	}
	if a.Hooks == nil {
		return out
	}
	addHooks := func(event string, list *[]decode.HookEntry) {
		if list == nil {
			return
		}
		for i, h := range *list {
			if h.Environment == nil {
				continue
			}
			for _, b := range *h.Environment {
				out = append(out, SecretItem{
					Consumer: "/hooks/" + event + "/" + strconv.Itoa(i),
					Target:   b.ValueFrom.Environment,
				})
			}
		}
	}
	addHooks("session_start", a.Hooks.SessionStart)
	addHooks("before_tool_call", a.Hooks.BeforeToolCall)
	addHooks("after_tool_call", a.Hooks.AfterToolCall)
	addHooks("agent_stop", a.Hooks.AgentStop)
	return out
}
