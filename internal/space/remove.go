package space

import "github.com/agent2host/agent2host/internal/space/registry"

// Remove unregisters a system and GCs artifacts that no remaining system
// references. It does not delete user Source or Host state.
func (s *Space) Remove(systemID string) error {
	unlock, err := s.registry.LockSystem(systemID)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()

	if err := s.registry.WithWrite(func(doc *registry.Document) error {
		return registry.Delete(doc, systemID)
	}); err != nil {
		return err
	}
	keep := map[string]struct{}{}
	doc, err := s.registry.Load()
	if err != nil {
		return err
	}
	for _, rec := range doc.Systems {
		keep[rec.ActiveRevision] = struct{}{}
	}
	return s.store.GCUnreferenced(keep)
}
