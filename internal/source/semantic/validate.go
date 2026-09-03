package semantic

import (
	"github.com/agent2host/agent2host/internal/artifact"
	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/graph"
	"github.com/agent2host/agent2host/internal/source/normalize"
	"github.com/agent2host/agent2host/internal/source/path"
	"github.com/agent2host/agent2host/internal/source/rule"
	"github.com/agent2host/agent2host/internal/source/schema"
)

// Result is register-time validation without Store publish.
type Result struct {
	SystemID string
	Version  string
	AgentIDs []string
	Members  []string
	Revision string
	Manifest decode.ArtifactManifest
	Warnings []rule.Warning
	Secrets  []graph.SecretItem
	SkillIDs map[string][]string
	Payload  map[string][]byte
}

// Engine is the Source semantic + Artifact pipeline (SP-050–SP-080).
type Engine struct {
	Schema *schema.Validator
}

// New loads the embedded published schemas.
func New() (*Engine, error) {
	v, err := schema.Load()
	if err != nil {
		return nil, err
	}
	return &Engine{Schema: v}, nil
}

// AgentBytes validates a standalone Agent Spec (no filesystem).
func (e *Engine) AgentBytes(raw []byte) ([]rule.Warning, error) {
	if err := e.Schema.ValidateBytes(schema.KindAgent, raw); err != nil {
		return nil, err
	}
	a, err := decode.Agent(raw)
	if err != nil {
		return nil, err
	}
	normalize.Agent(a)
	return graph.Agent(a)
}

// SystemBytes validates a standalone AgentSystem document (no filesystem).
func (e *Engine) SystemBytes(raw []byte) error {
	if err := e.Schema.ValidateBytes(schema.KindSystem, raw); err != nil {
		return err
	}
	s, err := decode.System(raw)
	if err != nil {
		return err
	}
	normalize.System(s)
	return graph.System(s)
}

// Tree validates a Source tree, plans inclusion, and builds the Artifact.
func (e *Engine) Tree(root string) (*Result, error) {
	return e.tree(root, nil, nil)
}

// TreeWithHome is Tree using Agent2Host home for SRC-PATH-HARD-DENY store internals.
func (e *Engine) TreeWithHome(root, home string) (*Result, error) {
	fs, err := path.NewFS(root, home)
	if err != nil {
		return nil, err
	}
	return e.tree(root, fs, nil)
}

func (e *Engine) tree(root string, fs *path.FS, snap *path.Snapshot) (*Result, error) {
	var err error
	if snap == nil {
		if fs == nil {
			fs, err = path.NewFS(root, "")
			if err != nil {
				return nil, err
			}
		}
		snap = path.NewSnapshot(fs)
	}
	sysRaw, err := snap.Load("system.json", path.RoleSystemJSON)
	if err != nil {
		return nil, err
	}
	if err := e.Schema.ValidateBytes(schema.KindSystem, sysRaw); err != nil {
		return nil, err
	}
	sys, err := decode.System(sysRaw)
	if err != nil {
		return nil, err
	}
	normalize.System(sys)
	if err := graph.System(sys); err != nil {
		return nil, err
	}

	var warns []rule.Warning
	var agents []artifact.AgentMember
	var specs []*decode.AgentSource
	var agentPaths []string
	var secrets []graph.SecretItem
	skillIDs := map[string][]string{}

	for _, p := range sys.Agents {
		c, err := path.Shape(path.Decl{Authoring: p, Role: path.RoleAgentSpec})
		if err != nil {
			return nil, err
		}
		raw, err := snap.Load(c, path.RoleAgentSpec)
		if err != nil {
			return nil, err
		}
		if err := e.Schema.ValidateBytes(schema.KindAgent, raw); err != nil {
			return nil, err
		}
		a, err := decode.Agent(raw)
		if err != nil {
			return nil, err
		}
		normalize.Agent(a)
		w, err := graph.Agent(a)
		if err != nil {
			return nil, err
		}
		warns = append(warns, w...)
		agents = append(agents, artifact.AgentMember{Canonical: c, Spec: a})
		specs = append(specs, a)
		agentPaths = append(agentPaths, c)
		secrets = append(secrets, graph.Secrets(a)...)
		if a.Skills != nil {
			var ids []string
			for _, ref := range *a.Skills {
				ids = append(ids, ref.ID)
			}
			skillIDs[a.ID] = ids
		} else {
			skillIDs[a.ID] = nil
		}
	}

	cw, err := graph.Closure(sys, specs, agentPaths)
	if err != nil {
		return nil, err
	}
	warns = append(warns, cw...)

	roles := map[string]path.Role{}
	var declared []string
	add := func(d path.Decl) error {
		c, err := path.Shape(d)
		if err != nil {
			return err
		}
		if prev, ok := roles[c]; ok {
			roles[c] = path.MergeRoles(prev, d.Role)
			return nil
		}
		roles[c] = d.Role
		declared = append(declared, c)
		return nil
	}
	if err := add(path.Decl{Authoring: "system.json", Role: path.RoleSystemJSON}); err != nil {
		return nil, err
	}
	for _, d := range path.SystemDecls(sys) {
		if err := add(d); err != nil {
			return nil, err
		}
	}
	for _, a := range agents {
		for _, d := range path.AgentDecls(a.Spec) {
			if err := add(d); err != nil {
				return nil, err
			}
		}
	}
	if err := path.Collide(declared); err != nil {
		return nil, err
	}

	for _, c := range declared {
		raw, err := snap.Load(c, roles[c])
		if err != nil {
			return nil, err
		}
		if path.ScanWarn(raw) {
			warns = append(warns, rule.Warning{ID: "SRC-CONTENT-SCAN-WARN", Detail: c})
		}
	}

	members, err := artifact.Inclusion(sys, agents)
	if err != nil {
		return nil, err
	}
	art, err := artifact.Build(members, snap.Bytes)
	if err != nil {
		return nil, err
	}
	var agentIDs []string
	for _, a := range agents {
		agentIDs = append(agentIDs, a.Spec.ID)
	}

	return &Result{
		SystemID: sys.ID,
		Version:  sys.Version,
		AgentIDs: agentIDs,
		Members:  members,
		Revision: art.Revision,
		Manifest: art.Manifest,
		Warnings: warns,
		Secrets:  secrets,
		SkillIDs: skillIDs,
		Payload:  art.Payload,
	}, nil
}

// TreeWithFS is for SRC-PATH-TYPE device stubs.
func (e *Engine) TreeWithFS(root string, fs *path.FS) (*Result, error) {
	return e.tree(root, fs, nil)
}

// TreeWithSnapshot continues a register that already loaded system.json into snap.
func (e *Engine) TreeWithSnapshot(root string, fs *path.FS, snap *path.Snapshot) (*Result, error) {
	return e.tree(root, fs, snap)
}
