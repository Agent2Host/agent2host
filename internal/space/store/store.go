package store

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/agent2host/agent2host/internal/artifact"
	"github.com/agent2host/agent2host/internal/source/decode"
	pathpkg "github.com/agent2host/agent2host/internal/source/path"
	"github.com/agent2host/agent2host/internal/source/schema"
	"github.com/agent2host/agent2host/internal/space/fsync"
)

// Store is the on-disk Canonical System Artifact store under <home>/space/artifacts.
// Publish is content-addressed and idempotent for an intact revision.
type Store struct {
	Home   string
	schema *schema.Validator
}

// New resolves home to an absolute path. It does not create directories.
func New(home string) (*Store, error) {
	abs, err := pathpkg.ResolveRoot(home)
	if err != nil {
		return nil, err
	}
	v, err := schema.Load()
	if err != nil {
		return nil, err
	}
	return &Store{Home: abs, schema: v}, nil
}

func (s *Store) artifactsRoot() string {
	return filepath.Join(s.Home, "space", "artifacts")
}

// dirName is a portable directory name for revision sha256:<hex>.
func dirName(revision string) (string, error) {
	const prefix = "sha256:"
	if !strings.HasPrefix(revision, prefix) {
		return "", fail(KindRevision, revision)
	}
	hex := revision[len(prefix):]
	if len(hex) != 64 {
		return "", fail(KindRevision, revision)
	}
	for i := 0; i < len(hex); i++ {
		c := hex[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return "", fail(KindRevision, revision)
	}
	return "sha256-" + hex, nil
}

func (s *Store) artifactDir(revision string) (string, error) {
	name, err := dirName(revision)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.artifactsRoot(), name), nil
}

// Publish writes an already-verified Artifact. Same revision is reused when
// the stored payload is intact and refused when it is corrupt.
func (s *Store) Publish(art *artifact.Artifact) error {
	if err := artifact.Verify(art); err != nil {
		return asCorrupt(err)
	}
	dest, err := s.artifactDir(art.Revision)
	if err != nil {
		return err
	}
	if st, err := os.Lstat(dest); err == nil {
		if !st.IsDir() {
			return fail(KindCorrupt, dest)
		}
		if _, err := s.loadFrom(dest, art.Revision); err != nil {
			return asCorrupt(err)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	root := s.artifactsRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(root, ".tmp-")
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(tmp)
		}
	}()
	if err := writeArtifact(tmp, art); err != nil {
		return err
	}
	if _, err := s.loadFrom(tmp, art.Revision); err != nil {
		return asCorrupt(err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		if _, loadErr := s.loadFrom(dest, art.Revision); loadErr == nil {
			return nil
		}
		return fail(KindCorrupt, dest)
	}
	committed = true
	fsync.SyncCommittedDirBestEffort(root)
	return nil
}

// Load reads and verifies the Artifact for revision.
func (s *Store) Load(revision string) (*artifact.Artifact, error) {
	dest, err := s.artifactDir(revision)
	if err != nil {
		return nil, err
	}
	st, err := os.Lstat(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fail(KindMissing, revision)
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, fail(KindCorrupt, dest)
	}
	art, err := s.loadFrom(dest, revision)
	if err != nil {
		return nil, asCorrupt(err)
	}
	return art, nil
}

func writeArtifact(dir string, art *artifact.Artifact) error {
	content := filepath.Join(dir, "content")
	for _, f := range art.Manifest.Files {
		raw, ok := art.Payload[f.Path]
		if !ok {
			return fail(KindCorrupt, f.Path)
		}
		if !pathpkg.IsCanonicalMember(f.Path) || !memberInsideContent(content, f.Path) {
			return fail(KindCorrupt, f.Path)
		}
		target := filepath.Join(content, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := fsync.WriteFile(target, raw); err != nil {
			return err
		}
	}
	if err := fsync.Tree(content); err != nil {
		return err
	}
	man, err := json.MarshalIndent(art.Manifest, "", "  ")
	if err != nil {
		return err
	}
	man = append(man, '\n')
	if err := fsync.WriteFile(filepath.Join(dir, "manifest.json"), man); err != nil {
		return err
	}
	return fsync.Dir(dir)
}

// memberInsideContent requires the member path to stay a relative child of
// content/ after Join/Clean. Lexical escape, absolute paths, and rewritten
// `.` / `..` segments fail closed even if CheckManifest was skipped.
func memberInsideContent(contentDir, memberPath string) bool {
	if memberPath == "" || filepath.IsAbs(memberPath) {
		return false
	}
	converted := filepath.FromSlash(memberPath)
	if filepath.IsAbs(converted) {
		return false
	}
	target := filepath.Join(contentDir, converted)
	rel, err := filepath.Rel(contentDir, target)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	if filepath.IsAbs(rel) {
		return false
	}
	return filepath.ToSlash(rel) == memberPath
}

func (s *Store) loadFrom(dir, revision string) (*artifact.Artifact, error) {
	manPath := filepath.Join(dir, "manifest.json")
	st, err := os.Lstat(manPath)
	if err != nil {
		return nil, err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() {
		return nil, fail(KindCorrupt, manPath)
	}
	raw, err := os.ReadFile(manPath)
	if err != nil {
		return nil, err
	}
	if err := s.schema.ValidateBytes(schema.KindArtifact, raw); err != nil {
		return nil, err
	}
	m, err := decode.Artifact(raw)
	if err != nil {
		return nil, err
	}
	content := filepath.Join(dir, "content")
	payload := map[string][]byte{}
	err = filepath.WalkDir(content, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		st, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() {
			return fail(KindCorrupt, p)
		}
		rel, err := filepath.Rel(content, p)
		if err != nil {
			return err
		}
		canon := filepath.ToSlash(rel)
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		payload[canon] = body
		return nil
	})
	if err != nil {
		return nil, err
	}
	art := &artifact.Artifact{Manifest: *m, Revision: revision, Payload: payload}
	if err := artifact.Verify(art); err != nil {
		return nil, err
	}
	return art, nil
}

// GCUnreferenced deletes artifact directories whose revision is not in keep.
func (s *Store) GCUnreferenced(keep map[string]struct{}) error {
	entries, err := os.ReadDir(s.artifactsRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if !strings.HasPrefix(e.Name(), "sha256-") {
			continue
		}
		rev := "sha256:" + strings.TrimPrefix(e.Name(), "sha256-")
		if _, ok := keep[rev]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(s.artifactsRoot(), e.Name())); err != nil {
			return err
		}
	}
	return nil
}
