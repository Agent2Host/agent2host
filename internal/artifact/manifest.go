package artifact

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/gowebpki/jcs"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/path"
	"github.com/agent2host/agent2host/internal/source/schema"
)

const digestFormat = "agent2host-system-v1"

// Artifact is the in-memory Canonical System Artifact (not Store-published).
type Artifact struct {
	Manifest decode.ArtifactManifest
	Revision string
	Payload  map[string][]byte
}

// Build hashes raw member bytes, sorts by path UTF-8, and computes the revision.
func Build(members []string, read func(canonical string) ([]byte, error)) (*Artifact, error) {
	payload := make(map[string][]byte, len(members))
	files := make([]decode.ArtifactMember, 0, len(members))
	for _, m := range path.UniqueCanonical(members) {
		raw, err := read(m)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(raw)
		payload[m] = append([]byte(nil), raw...)
		files = append(files, decode.ArtifactMember{
			Path:   m,
			SHA256: hex.EncodeToString(sum[:]),
			Type:   "file",
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	man := decode.ArtifactManifest{DigestFormat: digestFormat, Files: files}
	if err := CheckManifest(&man); err != nil {
		return nil, err
	}
	canon, err := canonicalManifestBytes(&man)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(canon)
	rev := "sha256:" + hex.EncodeToString(sum[:])
	art := &Artifact{Manifest: man, Revision: rev, Payload: payload}
	if err := Verify(art); err != nil {
		return nil, err
	}
	return art, nil
}

// CheckManifest applies semantic Artifact rules beyond JSON Schema.
func CheckManifest(m *decode.ArtifactManifest) error {
	if m.DigestFormat != digestFormat {
		return fail(KindDigestFormat, m.DigestFormat)
	}
	if len(m.Files) == 0 {
		return fail(KindEmptyFiles, "")
	}
	var paths []string
	seen := map[string]struct{}{}
	sys := 0
	prev := ""
	for _, f := range m.Files {
		if _, ok := seen[f.Path]; ok {
			return fail(KindDuplicatePath, f.Path)
		}
		seen[f.Path] = struct{}{}
		if prev != "" && f.Path < prev {
			return fail(KindNoncanonicalOrder, f.Path)
		}
		prev = f.Path
		if f.Type != "file" {
			return fail(KindMemberType, f.Path)
		}
		if !path.IsCanonicalMember(f.Path) {
			return fail(KindNoncanonicalPath, f.Path)
		}
		if f.Path == "system.json" {
			sys++
		}
		paths = append(paths, f.Path)
	}
	if sys != 1 {
		return fail(KindSystemJSONCount, "")
	}
	return path.Collide(paths)
}

// Verify checks payload ↔ manifest identity and revision bytes.
func Verify(a *Artifact) error {
	if err := CheckManifest(&a.Manifest); err != nil {
		return err
	}
	if len(a.Payload) != len(a.Manifest.Files) {
		return fmt.Errorf("artifact: payload count %d != manifest %d", len(a.Payload), len(a.Manifest.Files))
	}
	for _, f := range a.Manifest.Files {
		raw, ok := a.Payload[f.Path]
		if !ok {
			return fmt.Errorf("artifact: missing payload member %s", f.Path)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != f.SHA256 {
			return fmt.Errorf("artifact: hash mismatch for %s", f.Path)
		}
	}
	for p := range a.Payload {
		found := false
		for _, f := range a.Manifest.Files {
			if f.Path == p {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("artifact: undeclared payload member %s", p)
		}
	}
	canon, err := canonicalManifestBytes(&a.Manifest)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(canon)
	want := "sha256:" + hex.EncodeToString(sum[:])
	if a.Revision != want {
		return fmt.Errorf("artifact: revision mismatch")
	}
	return nil
}

func canonicalManifestBytes(m *decode.ArtifactManifest) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return jcs.Transform(b)
}

// ValidateJSON checks a standalone Artifact manifest instance (schema + semantic).
func ValidateJSON(v *schema.Validator, raw []byte) error {
	if err := v.ValidateBytes(schema.KindArtifact, raw); err != nil {
		return err
	}
	m, err := decode.Artifact(raw)
	if err != nil {
		return err
	}
	return CheckManifest(m)
}
