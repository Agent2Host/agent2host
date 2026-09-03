package space

import (
	"errors"
	"fmt"

	"github.com/agent2host/agent2host/internal/artifact"
	"github.com/agent2host/agent2host/internal/source/jsonreader"
	"github.com/agent2host/agent2host/internal/source/path"
	"github.com/agent2host/agent2host/internal/source/rule"
	"github.com/agent2host/agent2host/internal/source/semantic"
	"github.com/agent2host/agent2host/internal/space/registry"
	"github.com/agent2host/agent2host/internal/space/store"
)

// Space is the Agent Space library surface (register, later list/inspect/resolve).
type Space struct {
	Home     string
	engine   *semantic.Engine
	store    *store.Store
	registry *registry.Registry
}

// Report is the result of a successful register.
type Report struct {
	SystemID string
	Version  string
	Revision string
	Agents   []string
	Warnings []rule.Warning
}

// Open uses home as the Agent2Host home root (`--home` / A2H_HOME / ~/.a2h).
func Open(home string) (*Space, error) {
	abs, err := path.ResolveRoot(home)
	if err != nil {
		return nil, err
	}
	eng, err := semantic.New()
	if err != nil {
		return nil, err
	}
	st, err := store.New(abs)
	if err != nil {
		return nil, err
	}
	reg, err := registry.New(abs)
	if err != nil {
		return nil, err
	}
	return &Space{Home: abs, engine: eng, store: st, registry: reg}, nil
}

// InjectRegistryWriteError makes the next registry durable write fail after
// in-memory Put. Tests use this for publish-ok / write-fail atomicity.
func (s *Space) InjectRegistryWriteError(err error) {
	s.registry.InjectWriteError(err)
}

// Register validates a Source tree, publishes the Artifact, then updates the
// registry. Lock order: system → build/publish → registry write → re-read →
// commit. Registry lock is not held during Artifact build.
func (s *Space) Register(sourceDir string) (*Report, error) {
	provenance, err := registry.CanonicalSource(sourceDir)
	if err != nil {
		return nil, err
	}
	fs, err := path.NewFS(provenance, s.Home)
	if err != nil {
		return nil, err
	}
	bindSizeGuard(fs)
	snap := path.NewSnapshot(fs)
	sysRaw, err := snap.Load("system.json", path.RoleSystemJSON)
	if err != nil {
		return nil, err
	}
	id, err := peekSystemID(sysRaw)
	if err != nil || id == "" {
		_, terr := s.engine.TreeWithSnapshot(provenance, fs, snap)
		return nil, terr
	}
	unlock, err := s.registry.LockSystem(id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = unlock() }()

	if rec, err := s.registry.Get(id); err == nil {
		if rec.Source != provenance {
			return nil, failProvenance(id)
		}
	} else if !isUnknown(err) {
		return nil, err
	}

	res, err := s.engine.TreeWithSnapshot(provenance, fs, snap)
	if err != nil {
		return nil, err
	}
	if res.SystemID != id {
		return nil, fmt.Errorf("space: system id changed during register: %s -> %s", id, res.SystemID)
	}
	if err := checkInclusionSize(res.Payload); err != nil {
		return nil, err
	}

	art := &artifact.Artifact{
		Manifest: res.Manifest,
		Revision: res.Revision,
		Payload:  res.Payload,
	}
	if err := s.store.Publish(art); err != nil {
		return nil, err
	}

	rec := registry.Record{
		ActiveRevision: res.Revision,
		Source:         provenance,
		Version:        res.Version,
		Agents:         append([]string(nil), res.AgentIDs...),
	}
	if err := s.registry.WithWrite(func(doc *registry.Document) error {
		return registry.Put(doc, res.SystemID, rec)
	}); err != nil {
		return nil, err
	}
	return &Report{
		SystemID: res.SystemID,
		Version:  res.Version,
		Revision: res.Revision,
		Agents:   append([]string(nil), res.AgentIDs...),
		Warnings: res.Warnings,
	}, nil
}

func peekSystemID(raw []byte) (string, error) {
	doc, err := jsonreader.Read(raw)
	if err != nil {
		return "", err
	}
	m, ok := doc.Value.(map[string]any)
	if !ok {
		return "", fmt.Errorf("space: system.json is not an object")
	}
	id, _ := m["id"].(string)
	return id, nil
}

func isUnknown(err error) bool {
	var re *registry.Error
	return errors.As(err, &re) && re.Kind == registry.KindUnknown
}

func failProvenance(id string) error {
	return &registry.Error{Kind: registry.KindProvenance, Detail: id}
}
