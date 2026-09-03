package path

import (
	"sort"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/rule"
)

// Role is why a structured path was declared (encoding / suffix).
type Role int

const (
	RoleOther Role = iota
	RoleSOP
	RoleSkillDoc
	RoleContext
	RoleAsset
	RoleOutputSchema
	RoleScript
	RoleAgentSpec
	RoleSystemJSON
)

// Decl is one authoring structured path.
type Decl struct {
	Authoring string
	Role      Role
}

// AgentDecls returns structured paths declared on one Agent Spec.
func AgentDecls(a *decode.AgentSource) []Decl {
	var out []Decl
	out = append(out, Decl{a.SOP, RoleSOP})
	if a.Contexts != nil {
		for _, c := range *a.Contexts {
			out = append(out, Decl{c.Path, RoleContext})
		}
	}
	if a.Output != nil {
		out = append(out, Decl{a.Output.Schema, RoleOutputSchema})
	}
	if a.Hooks != nil {
		out = append(out, hookFiles(a.Hooks.SessionStart)...)
		out = append(out, hookFiles(a.Hooks.BeforeToolCall)...)
		out = append(out, hookFiles(a.Hooks.AfterToolCall)...)
		out = append(out, hookFiles(a.Hooks.AgentStop)...)
	}
	if a.MCPServers != nil {
		for _, id := range sortedKeys(*a.MCPServers) {
			srv := (*a.MCPServers)[id]
			if srv.Files == nil {
				continue
			}
			for _, f := range *srv.Files {
				out = append(out, Decl{f, RoleScript})
			}
		}
	}
	return out
}

func hookFiles(list *[]decode.HookEntry) []Decl {
	if list == nil {
		return nil
	}
	var out []Decl
	for _, h := range *list {
		if h.Files == nil {
			continue
		}
		for _, f := range *h.Files {
			out = append(out, Decl{f, RoleScript})
		}
	}
	return out
}

// SystemDecls returns structured paths declared on system.json (not system.json itself).
func SystemDecls(s *decode.SystemSource) []Decl {
	var out []Decl
	for _, p := range s.Agents {
		out = append(out, Decl{p, RoleAgentSpec})
	}
	if s.Skills == nil {
		return out
	}
	for _, id := range sortedKeys(*s.Skills) {
		sk := (*s.Skills)[id]
		out = append(out, Decl{sk.Document, RoleSkillDoc})
		if sk.Scripts != nil {
			for _, p := range *sk.Scripts {
				out = append(out, Decl{p, RoleScript})
			}
		}
		if sk.Contexts != nil {
			for _, p := range *sk.Contexts {
				out = append(out, Decl{p, RoleContext})
			}
		}
		if sk.Assets != nil {
			for _, p := range *sk.Assets {
				out = append(out, Decl{p, RoleAsset})
			}
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Shape canonicalizes, applies hard-deny and SOP/Skill suffixes.
func Shape(d Decl) (string, error) {
	c, err := Canonicalize(d.Authoring)
	if err != nil {
		return "", err
	}
	if err := HardDeny(c); err != nil {
		return "", err
	}
	switch d.Role {
	case RoleSOP:
		if !hasSuffix(c, ".sop.md") {
			return "", rule.Fail("SRC-MD-SOP", d.Authoring)
		}
	case RoleSkillDoc:
		if !hasSuffix(c, ".skill.md") {
			return "", rule.Fail("SRC-MD-SKILL", d.Authoring)
		}
	}
	return c, nil
}

func hasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}

// MergeRoles prefers a more specific encoding role when the same path is declared twice.
func MergeRoles(a, b Role) Role {
	rank := func(r Role) int {
		switch r {
		case RoleOutputSchema:
			return 6
		case RoleSOP, RoleSkillDoc:
			return 5
		case RoleContext:
			return 4
		case RoleAsset:
			return 3
		case RoleScript:
			return 2
		case RoleAgentSpec, RoleSystemJSON:
			return 1
		default:
			return 0
		}
	}
	if rank(b) > rank(a) {
		return b
	}
	return a
}
