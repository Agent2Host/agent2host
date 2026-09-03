package space

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/normalize"
	"github.com/agent2host/agent2host/internal/source/path"
)

// ResolvedSkill is one Skill in a Named Agent closure.
type ResolvedSkill struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Required    bool                `json:"required"`
	Document    string              `json:"document"`
	Scripts     []string            `json:"scripts,omitempty"`
	Contexts    []string            `json:"contexts,omitempty"`
	Assets      []string            `json:"assets,omitempty"`
	MCPTools    []decode.MCPToolRef `json:"mcp_tools,omitempty"`
}

// ResolvedContext is one Agent-level Context with declared required/loading/isolation.
type ResolvedContext struct {
	Path      string  `json:"path"`
	Loading   *string `json:"loading,omitempty"`
	Isolation *string `json:"isolation,omitempty"`
	Required  *bool   `json:"required,omitempty"`
}

// ResolvedTool is one MCP allowlist entry; required is the declared edge, not effective_required.
type ResolvedTool struct {
	Name     string `json:"name"`
	Required *bool  `json:"required,omitempty"`
}

// ResolvedMCP is one MCP server with a non-empty tools allowlist.
type ResolvedMCP struct {
	Transport   string                      `json:"transport"`
	Command     string                      `json:"command"`
	Args        []string                    `json:"args"`
	Files       []string                    `json:"files"`
	Tools       []ResolvedTool              `json:"tools"`
	Environment []decode.EnvironmentBinding `json:"environment,omitempty"`
}

// ResolvedAgentRun is the internal compiler IR (spec §10). Content holds only
// the selected Agent’s reachable files. Artifact still stores full system.json;
// that snapshot is not enumerable Adapter input.
type ResolvedAgentRun struct {
	SystemID         string                      `json:"system_id"`
	AgentID          string                      `json:"agent_id"`
	Name             string                      `json:"name,omitempty"`
	Description      string                      `json:"description,omitempty"`
	Version          string                      `json:"version"`
	ArtifactRevision string                      `json:"artifact_revision"`
	AgentSpec        string                      `json:"agent_spec"`
	SOP              string                      `json:"sop"`
	Skills           []ResolvedSkill             `json:"skills"`
	Contexts         []ResolvedContext           `json:"contexts"`
	Scripts          []string                    `json:"scripts"`
	Assets           []string                    `json:"assets"`
	MCPServers       map[string]ResolvedMCP      `json:"mcp_servers"`
	Hooks            *decode.Hooks               `json:"hooks,omitempty"`
	WorkRoot         decode.WorkRoot             `json:"work_root"`
	Permissions      *decode.Permissions         `json:"permissions,omitempty"`
	Approvals        *decode.Approvals           `json:"approvals,omitempty"`
	Sandbox          *decode.Sandbox             `json:"sandbox,omitempty"`
	Environment      []decode.EnvironmentBinding `json:"environment,omitempty"`
	Output           *decode.Output              `json:"output,omitempty"`
	Content          map[string][]byte           `json:"-"`
}

// Resolve loads the Artifact (default: active revision), verifies integrity,
// binds the Artifact’s system id to the request, and returns the selected
// Named Agent’s reachable closure. Agent existence is taken from that Artifact,
// not from the registry’s current agent index.
func (s *Space) Resolve(systemID, agentID, revision string) (*ResolvedAgentRun, error) {
	rec, err := s.registry.Get(systemID)
	if err != nil {
		return nil, err
	}
	if revision == "" {
		revision = rec.ActiveRevision
	}
	art, err := s.store.Load(revision)
	if err != nil {
		return nil, err
	}
	return closure(systemID, agentID, art.Revision, art.Payload)
}

func closure(systemID, agentID, revision string, payload map[string][]byte) (*ResolvedAgentRun, error) {
	sysRaw, ok := payload["system.json"]
	if !ok {
		return nil, fail(KindMismatch, "artifact missing system.json")
	}
	sys, err := decode.System(sysRaw)
	if err != nil {
		return nil, err
	}
	normalize.System(sys)
	if sys.ID != systemID {
		return nil, fail(KindMismatch, fmt.Sprintf("artifact system_id %s != %s", sys.ID, systemID))
	}

	specPath, spec, err := findAgent(sys, payload, agentID)
	if err != nil {
		return nil, err
	}

	hooks, err := copyHooks(spec.Hooks)
	if err != nil {
		return nil, err
	}
	out, err := copyOutput(spec.Output)
	if err != nil {
		return nil, err
	}

	run := &ResolvedAgentRun{
		SystemID:         systemID,
		AgentID:          agentID,
		Version:          sys.Version,
		ArtifactRevision: revision,
		AgentSpec:        specPath,
		MCPServers:       map[string]ResolvedMCP{},
		Content:          map[string][]byte{},
		Hooks:            hooks,
		WorkRoot:         normalize.EffectiveWorkRoot(sys),
		Permissions:      spec.Permissions,
		Approvals:        spec.Approvals,
		Sandbox:          spec.Sandbox,
		Output:           out,
	}
	if spec.Name != nil {
		run.Name = *spec.Name
	}
	if spec.Description != nil {
		run.Description = *spec.Description
	}
	if spec.Environment != nil {
		run.Environment = append([]decode.EnvironmentBinding(nil), *spec.Environment...)
	}

	add := func(authoring string) (string, error) {
		c, err := path.Canonicalize(authoring)
		if err != nil {
			return "", err
		}
		return c, addPath(run.Content, payload, c)
	}
	if err := addPath(run.Content, payload, specPath); err != nil {
		return nil, err
	}
	sop, err := add(spec.SOP)
	if err != nil {
		return nil, err
	}
	run.SOP = sop

	if spec.Skills != nil && sys.Skills != nil {
		for _, ref := range *spec.Skills {
			sk, ok := (*sys.Skills)[ref.ID]
			if !ok {
				return nil, fmt.Errorf("space: skill %s missing from system registry", ref.ID)
			}
			doc, err := add(sk.Document)
			if err != nil {
				return nil, err
			}
			rs := ResolvedSkill{
				ID:          ref.ID,
				Name:        sk.Name,
				Description: sk.Description,
				Document:    doc,
				Required:    true,
			}
			if ref.Required != nil {
				rs.Required = *ref.Required
			}
			if sk.Scripts != nil {
				for _, p := range *sk.Scripts {
					c, err := add(p)
					if err != nil {
						return nil, err
					}
					rs.Scripts = append(rs.Scripts, c)
					run.Scripts = append(run.Scripts, c)
				}
			}
			if sk.Contexts != nil {
				for _, p := range *sk.Contexts {
					c, err := add(p)
					if err != nil {
						return nil, err
					}
					rs.Contexts = append(rs.Contexts, c)
				}
			}
			if sk.Assets != nil {
				for _, p := range *sk.Assets {
					c, err := add(p)
					if err != nil {
						return nil, err
					}
					rs.Assets = append(rs.Assets, c)
					run.Assets = append(run.Assets, c)
				}
			}
			if sk.MCPTools != nil {
				rs.MCPTools = append([]decode.MCPToolRef(nil), *sk.MCPTools...)
			}
			rs.Scripts = path.UniqueCanonical(rs.Scripts)
			rs.Assets = path.UniqueCanonical(rs.Assets)
			run.Skills = append(run.Skills, rs)
		}
	}
	if spec.Contexts != nil {
		for _, c := range *spec.Contexts {
			p, err := add(c.Path)
			if err != nil {
				return nil, err
			}
			run.Contexts = append(run.Contexts, ResolvedContext{
				Path:      p,
				Loading:   c.Loading,
				Isolation: c.Isolation,
				Required:  c.Required,
			})
		}
	}
	if spec.Output != nil {
		if _, err := add(spec.Output.Schema); err != nil {
			return nil, err
		}
	}
	if spec.Hooks != nil {
		for _, list := range []*[]decode.HookEntry{spec.Hooks.SessionStart, spec.Hooks.BeforeToolCall, spec.Hooks.AfterToolCall, spec.Hooks.AgentStop} {
			if list == nil {
				continue
			}
			for _, h := range *list {
				if h.Files == nil {
					continue
				}
				for _, f := range *h.Files {
					c, err := add(f)
					if err != nil {
						return nil, err
					}
					run.Scripts = append(run.Scripts, c)
				}
			}
		}
	}
	if spec.MCPServers != nil {
		ids := make([]string, 0, len(*spec.MCPServers))
		for id := range *spec.MCPServers {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			srv := (*spec.MCPServers)[id]
			if len(srv.Tools) == 0 {
				continue
			}
			rm := ResolvedMCP{Transport: srv.Transport, Command: canonCommand(srv.Command)}
			if srv.Args != nil {
				for _, a := range *srv.Args {
					rm.Args = append(rm.Args, canonArg(a))
				}
			}
			for _, t := range srv.Tools {
				rm.Tools = append(rm.Tools, ResolvedTool{Name: t.Name, Required: t.Required})
			}
			if srv.Files != nil {
				for _, f := range *srv.Files {
					c, err := add(f)
					if err != nil {
						return nil, err
					}
					rm.Files = append(rm.Files, c)
					run.Scripts = append(run.Scripts, c)
				}
			}
			if srv.Environment != nil {
				rm.Environment = append([]decode.EnvironmentBinding(nil), *srv.Environment...)
			}
			run.MCPServers[id] = rm
		}
	}
	run.Scripts = path.UniqueCanonical(run.Scripts)
	run.Assets = path.UniqueCanonical(run.Assets)
	return run, nil
}

func canonCommand(cmd string) string {
	c, err := path.Canonicalize(cmd)
	if err == nil {
		return c
	}
	return cmd
}

func canonArg(a string) string {
	if strings.HasPrefix(a, "./") || strings.HasPrefix(a, "../") {
		c, err := path.Canonicalize(a)
		if err == nil {
			return c
		}
	}
	return a
}

func copyOutput(o *decode.Output) (*decode.Output, error) {
	if o == nil {
		return nil, nil
	}
	c, err := path.Canonicalize(o.Schema)
	if err != nil {
		return nil, err
	}
	cp := *o
	cp.Schema = c
	return &cp, nil
}

func copyHooks(h *decode.Hooks) (*decode.Hooks, error) {
	if h == nil {
		return nil, nil
	}
	out := &decode.Hooks{}
	var err error
	if out.SessionStart, err = copyHookList(h.SessionStart); err != nil {
		return nil, err
	}
	if out.BeforeToolCall, err = copyHookList(h.BeforeToolCall); err != nil {
		return nil, err
	}
	if out.AfterToolCall, err = copyHookList(h.AfterToolCall); err != nil {
		return nil, err
	}
	if out.AgentStop, err = copyHookList(h.AgentStop); err != nil {
		return nil, err
	}
	return out, nil
}

func copyHookList(list *[]decode.HookEntry) (*[]decode.HookEntry, error) {
	if list == nil {
		return nil, nil
	}
	cp := make([]decode.HookEntry, 0, len(*list))
	for _, h := range *list {
		e := h
		e.Command = canonCommand(h.Command)
		if h.Files != nil {
			files := make([]string, 0, len(*h.Files))
			for _, f := range *h.Files {
				c, err := path.Canonicalize(f)
				if err != nil {
					return nil, err
				}
				files = append(files, c)
			}
			e.Files = &files
		}
		if h.Args != nil {
			args := make([]string, 0, len(*h.Args))
			for _, a := range *h.Args {
				args = append(args, canonArg(a))
			}
			e.Args = &args
		}
		cp = append(cp, e)
	}
	return &cp, nil
}

func findAgent(sys *decode.SystemSource, payload map[string][]byte, agentID string) (string, *decode.AgentSource, error) {
	for _, authoring := range sys.Agents {
		c, err := path.Canonicalize(authoring)
		if err != nil {
			return "", nil, err
		}
		raw, ok := payload[c]
		if !ok {
			return "", nil, fmt.Errorf("space: missing agent spec %s", c)
		}
		a, err := decode.Agent(raw)
		if err != nil {
			return "", nil, err
		}
		normalize.Agent(a)
		if a.ID == agentID {
			return c, a, nil
		}
	}
	return "", nil, fail(KindUnknownAgent, agentID)
}

func addPath(content, payload map[string][]byte, canonical string) error {
	raw, ok := payload[canonical]
	if !ok {
		return fmt.Errorf("space: closure member %s missing from artifact", canonical)
	}
	content[canonical] = append([]byte(nil), raw...)
	return nil
}
