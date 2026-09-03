package artifact_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/artifact"
	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/source/rule"
	"github.com/agent2host/agent2host/internal/source/schema"
)

func TestBuildRevisionStableAndSensitive(t *testing.T) {
	files := map[string][]byte{
		"system.json":            []byte(`{"ok":true}`),
		"agents/demo.agent.json": []byte(`{"id":"demo"}`),
		"sops/demo.sop.md":       []byte("# sop\n"),
	}
	read := func(c string) ([]byte, error) { return files[c], nil }
	members := []string{"system.json", "agents/demo.agent.json", "sops/demo.sop.md"}
	a1, err := artifact.Build(members, read)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := artifact.Build(members, read)
	if err != nil {
		t.Fatal(err)
	}
	if a1.Revision != a2.Revision || !strings.HasPrefix(a1.Revision, "sha256:") {
		t.Fatalf("revision %q vs %q", a1.Revision, a2.Revision)
	}
	files["sops/demo.sop.md"] = []byte("# sop changed\n")
	a3, err := artifact.Build(members, read)
	if err != nil {
		t.Fatal(err)
	}
	if a3.Revision == a1.Revision {
		t.Fatal("content change must change revision")
	}
	for _, f := range a3.Manifest.Files {
		if f.Type != "file" || len(f.SHA256) != 64 {
			t.Fatalf("member %+v", f)
		}
	}
}

func TestVerifyPayloadCorrupt(t *testing.T) {
	files := map[string][]byte{"system.json": []byte("x"), "a.md": []byte("y")}
	art, err := artifact.Build([]string{"system.json", "a.md"}, func(c string) ([]byte, error) { return files[c], nil })
	if err != nil {
		t.Fatal(err)
	}
	art.Payload["a.md"] = []byte("z")
	if err := artifact.Verify(art); err == nil {
		t.Fatal("corrupt payload must fail")
	}
	art.Payload["a.md"] = []byte("y")
	delete(art.Payload, "a.md")
	if err := artifact.Verify(art); err == nil {
		t.Fatal("missing payload must fail")
	}
}

func TestVerifyExtraPayload(t *testing.T) {
	files := map[string][]byte{"system.json": []byte("x")}
	art, err := artifact.Build([]string{"system.json"}, func(c string) ([]byte, error) { return files[c], nil })
	if err != nil {
		t.Fatal(err)
	}
	art.Payload["extra.md"] = []byte("no")
	if err := artifact.Verify(art); err == nil {
		t.Fatal("extra payload must fail")
	}
}

func TestArtifactFixtureManifests(t *testing.T) {
	v, err := schema.Load()
	if err != nil {
		t.Fatal(err)
	}
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	valid := filepath.Join(root, "artifact", "valid", "two-members.json")
	raw, err := os.ReadFile(valid)
	if err != nil {
		t.Fatal(err)
	}
	// Schema-valid only: files[] is not UTF-8 sorted (system.json before agents/…).
	if err := v.ValidateBytes(schema.KindArtifact, raw); err != nil {
		t.Fatalf("schema-valid fixture: %v", err)
	}
	m, err := decode.Artifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	err = artifact.CheckManifest(m)
	var ae *artifact.Error
	if !errors.As(err, &ae) || ae.Kind != artifact.KindNoncanonicalOrder {
		t.Fatalf("unsorted files: %v", err)
	}
	if err := artifact.ValidateJSON(v, raw); err == nil {
		t.Fatal("ValidateJSON must apply semantic order, not only Schema")
	}

	dup := *m
	dup.Files = append([]decode.ArtifactMember{}, m.Files...)
	dup.Files[1].Path = "system.json"
	dup.Files[1].SHA256 = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if err := artifact.CheckManifest(&dup); !errors.As(err, &ae) || ae.Kind != artifact.KindDuplicatePath {
		t.Fatalf("same path different hash must be duplicate_path, got %v", err)
	}
}

func TestCheckManifestTypedErrors(t *testing.T) {
	ok := decode.ArtifactManifest{
		DigestFormat: "agent2host-system-v1",
		Files: []decode.ArtifactMember{
			{Path: "system.json", SHA256: strings.Repeat("a", 64), Type: "file"},
		},
	}
	if err := artifact.CheckManifest(&ok); err != nil {
		t.Fatal(err)
	}

	wrong := ok
	wrong.DigestFormat = "other"
	err := artifact.CheckManifest(&wrong)
	var ae *artifact.Error
	if !errors.As(err, &ae) || ae.Kind != artifact.KindDigestFormat {
		t.Fatalf("digest_format: %v", err)
	}
	if rule.ID(err) == "SRC-PATH-CANON" {
		t.Fatal("must not report SRC-PATH-CANON")
	}

	empty := ok
	empty.Files = nil
	if err := artifact.CheckManifest(&empty); !errors.As(err, &ae) || ae.Kind != artifact.KindEmptyFiles {
		t.Fatalf("empty: %v", err)
	}

	dup := ok
	dup.Files = []decode.ArtifactMember{
		{Path: "system.json", SHA256: strings.Repeat("a", 64), Type: "file"},
		{Path: "system.json", SHA256: strings.Repeat("b", 64), Type: "file"},
	}
	if err := artifact.CheckManifest(&dup); !errors.As(err, &ae) || ae.Kind != artifact.KindDuplicatePath {
		t.Fatalf("dup: %v", err)
	}

	badType := ok
	badType.Files = []decode.ArtifactMember{
		{Path: "system.json", SHA256: strings.Repeat("a", 64), Type: "dir"},
	}
	if err := artifact.CheckManifest(&badType); !errors.As(err, &ae) || ae.Kind != artifact.KindMemberType {
		t.Fatalf("type: %v", err)
	}

	none := ok
	none.Files = []decode.ArtifactMember{
		{Path: "agents/a.agent.json", SHA256: strings.Repeat("a", 64), Type: "file"},
	}
	if err := artifact.CheckManifest(&none); !errors.As(err, &ae) || ae.Kind != artifact.KindSystemJSONCount {
		t.Fatalf("system.json count: %v", err)
	}

	unsorted := ok
	unsorted.Files = []decode.ArtifactMember{
		{Path: "system.json", SHA256: strings.Repeat("a", 64), Type: "file"},
		{Path: "agents/a.agent.json", SHA256: strings.Repeat("b", 64), Type: "file"},
	}
	if err := artifact.CheckManifest(&unsorted); !errors.As(err, &ae) || ae.Kind != artifact.KindNoncanonicalOrder {
		t.Fatalf("order: %v", err)
	}
}

func TestCheckManifestRejectsNoncanonicalPaths(t *testing.T) {
	hash := strings.Repeat("a", 64)
	sys := decode.ArtifactMember{Path: "system.json", SHA256: hash, Type: "file"}
	cases := []string{
		"../registry.json",
		"../../space/registry.json",
		"/etc/passwd",
		"agents/./demo.agent.json",
		"agents//demo.agent.json",
		"./system.json",
		`agents\demo.agent.json`,
	}
	for _, p := range cases {
		files := []decode.ArtifactMember{sys, {Path: p, SHA256: hash, Type: "file"}}
		if p < "system.json" {
			files = []decode.ArtifactMember{{Path: p, SHA256: hash, Type: "file"}, sys}
		}
		if p == "./system.json" {
			files = []decode.ArtifactMember{{Path: p, SHA256: hash, Type: "file"}}
		}
		err := artifact.CheckManifest(&decode.ArtifactManifest{
			DigestFormat: "agent2host-system-v1",
			Files:        files,
		})
		var ae *artifact.Error
		if !errors.As(err, &ae) || ae.Kind != artifact.KindNoncanonicalPath {
			t.Fatalf("path %q: %v", p, err)
		}
	}
}

func TestVerifyRejectsUnsortedFiles(t *testing.T) {
	files := map[string][]byte{
		"system.json":            []byte("x"),
		"agents/demo.agent.json": []byte("y"),
	}
	art, err := artifact.Build([]string{"system.json", "agents/demo.agent.json"}, func(c string) ([]byte, error) {
		return files[c], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	art.Manifest.Files[0], art.Manifest.Files[1] = art.Manifest.Files[1], art.Manifest.Files[0]
	err = artifact.Verify(art)
	var ae *artifact.Error
	if !errors.As(err, &ae) || ae.Kind != artifact.KindNoncanonicalOrder {
		t.Fatalf("Verify unsorted: %v", err)
	}
}

func TestRFC8785IndependentGolden(t *testing.T) {
	// Expected bytes come from testdata (Node canonicalize@2.0.0), not production JCS.
	wantCanon, err := os.ReadFile(filepath.Join("testdata", "rfc8785_independent.jcs"))
	if err != nil {
		t.Fatal(err)
	}
	wantCanon = bytes.TrimSuffix(wantCanon, []byte("\n"))
	wantRev, err := os.ReadFile(filepath.Join("testdata", "rfc8785_independent.revision"))
	if err != nil {
		t.Fatal(err)
	}
	wantRev = bytes.TrimSpace(wantRev)

	files := map[string][]byte{
		"system.json":            []byte(`{"ok":true}`),
		"agents/demo.agent.json": []byte(`{"id":"demo"}`),
	}
	for p, raw := range files {
		sum := sha256.Sum256(raw)
		t.Logf("%s %s", p, hex.EncodeToString(sum[:]))
	}
	art, err := artifact.Build([]string{"system.json", "agents/demo.agent.json"}, func(c string) ([]byte, error) {
		return files[c], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.Revision != string(wantRev) {
		t.Fatalf("revision %s want %s", art.Revision, wantRev)
	}
	// Re-hash independently: SHA-256 of the committed JCS bytes must equal the committed revision.
	sum := sha256.Sum256(wantCanon)
	if "sha256:"+hex.EncodeToString(sum[:]) != string(wantRev) {
		t.Fatal("committed revision is not SHA-256 of committed JCS bytes")
	}
	if len(art.Manifest.Files) != 2 || art.Manifest.Files[0].Path != "agents/demo.agent.json" {
		t.Fatalf("files must be sorted by path: %+v", art.Manifest.Files)
	}
}
