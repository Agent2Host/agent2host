package registry

import (
	"path/filepath"
)

// CanonicalSource is the provenance identity of a System Root: absolute path
// after evaluating `.` / `..` and symlinks (spec §4.3).
func CanonicalSource(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// Put applies spec §8.3 to doc: first install, same provenance updates
// active_revision, different provenance refuses.
func Put(doc *Document, systemID string, rec Record) error {
	if doc.Systems == nil {
		doc.Systems = map[string]Record{}
	}
	cur, ok := doc.Systems[systemID]
	if !ok {
		doc.Systems[systemID] = rec
		return nil
	}
	if cur.Source == rec.Source {
		doc.Systems[systemID] = rec
		return nil
	}
	return fail(KindProvenance, systemID)
}

// Delete removes systemID from the registry document.
func Delete(doc *Document, systemID string) error {
	if doc.Systems == nil {
		return fail(KindUnknown, systemID)
	}
	if _, ok := doc.Systems[systemID]; !ok {
		return fail(KindUnknown, systemID)
	}
	delete(doc.Systems, systemID)
	return nil
}
