package artifact

import (
	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/path"
)

// AgentMember is one listed Agent Spec after path validation.
type AgentMember struct {
	Canonical string
	Spec      *decode.AgentSource
}

// Inclusion is the Registered System Artifact member set (Space §6).
// Named Agent resolve closure is not computed here.
func Inclusion(s *decode.SystemSource, agents []AgentMember) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	addCanon := func(c string) {
		if _, ok := seen[c]; ok {
			return
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	add := func(authoring string) error {
		c, err := path.Canonicalize(authoring)
		if err != nil {
			return err
		}
		addCanon(c)
		return nil
	}

	addCanon("system.json")
	for _, a := range agents {
		addCanon(a.Canonical)
		if err := add(a.Spec.SOP); err != nil {
			return nil, err
		}
		if a.Spec.Contexts != nil {
			for _, c := range *a.Spec.Contexts {
				if err := add(c.Path); err != nil {
					return nil, err
				}
			}
		}
		if a.Spec.Output != nil {
			if err := add(a.Spec.Output.Schema); err != nil {
				return nil, err
			}
		}
		if a.Spec.Hooks != nil {
			for _, list := range []*[]decode.HookEntry{a.Spec.Hooks.SessionStart, a.Spec.Hooks.BeforeToolCall, a.Spec.Hooks.AfterToolCall, a.Spec.Hooks.AgentStop} {
				if list == nil {
					continue
				}
				for _, h := range *list {
					if h.Files == nil {
						continue
					}
					for _, f := range *h.Files {
						if err := add(f); err != nil {
							return nil, err
						}
					}
				}
			}
		}
		if a.Spec.MCPServers != nil {
			for _, srv := range *a.Spec.MCPServers {
				if len(srv.Tools) == 0 || srv.Files == nil {
					continue
				}
				for _, f := range *srv.Files {
					if err := add(f); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	if s.Skills != nil {
		for _, sk := range *s.Skills {
			if err := add(sk.Document); err != nil {
				return nil, err
			}
			if sk.Scripts != nil {
				for _, p := range *sk.Scripts {
					if err := add(p); err != nil {
						return nil, err
					}
				}
			}
			if sk.Contexts != nil {
				for _, p := range *sk.Contexts {
					if err := add(p); err != nil {
						return nil, err
					}
				}
			}
			if sk.Assets != nil {
				for _, p := range *sk.Assets {
					if err := add(p); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return out, nil
}
