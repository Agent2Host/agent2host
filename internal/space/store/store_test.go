package store_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/artifact"
	"github.com/agent2host/agent2host/internal/space/store"
)

func testArtifact(t *testing.T) *artifact.Artifact {
	t.Helper()
	files := map[string][]byte{
		"system.json":            []byte(`{"ok":true}`),
		"agents/demo.agent.json": []byte(`{"id":"demo"}`),
	}
	art, err := artifact.Build([]string{"system.json", "agents/demo.agent.json"}, func(c string) ([]byte, error) {
		return files[c], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return art
}

func TestPublishLoadRoundtrip(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	art := testArtifact(t)
	if err := st.Publish(art); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load(art.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != art.Revision {
		t.Fatalf("revision %s want %s", got.Revision, art.Revision)
	}
	for p, raw := range art.Payload {
		if !bytes.Equal(got.Payload[p], raw) {
			t.Fatalf("payload %s mismatch", p)
		}
	}
	if err := st.Publish(art); err != nil {
		t.Fatal(err)
	}
}

func TestPublishRefusesCorruptExisting(t *testing.T) {
	home := t.TempDir()
	st, err := store.New(home)
	if err != nil {
		t.Fatal(err)
	}
	art := testArtifact(t)
	if err := st.Publish(art); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load(art.Revision)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "space", "artifacts", "sha256-"+art.Revision[len("sha256:"):])
	if err := os.WriteFile(filepath.Join(dir, "content", "system.json"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = st.Load(art.Revision)
	var se *store.Error
	if !errors.As(err, &se) || se.Kind != store.KindCorrupt {
		t.Fatalf("load corrupt: %v", err)
	}
	if err := st.Publish(got); err == nil || !errors.As(err, &se) || se.Kind != store.KindCorrupt {
		t.Fatalf("publish must refuse corrupt dest: %v", err)
	}
}

func TestLoadUndeclaredAndMissing(t *testing.T) {
	home := t.TempDir()
	st, err := store.New(home)
	if err != nil {
		t.Fatal(err)
	}
	art := testArtifact(t)
	if err := st.Publish(art); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "space", "artifacts", "sha256-"+art.Revision[len("sha256:"):], "content")
	if err := os.WriteFile(filepath.Join(dir, "extra.md"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	var se *store.Error
	if _, err := st.Load(art.Revision); !errors.As(err, &se) || se.Kind != store.KindCorrupt {
		t.Fatalf("extra file: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "extra.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "agents", "demo.agent.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Load(art.Revision); !errors.As(err, &se) || se.Kind != store.KindCorrupt {
		t.Fatalf("missing member: %v", err)
	}
}

func TestLoadMissingRevision(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	art := testArtifact(t)
	var se *store.Error
	if _, err := st.Load(art.Revision); !errors.As(err, &se) || se.Kind != store.KindMissing {
		t.Fatalf("missing: %v", err)
	}
	if _, err := st.Load("not-a-revision"); !errors.As(err, &se) || se.Kind != store.KindRevision {
		t.Fatalf("bad revision: %v", err)
	}
}

func TestLoadRejectsSymlinkManifest(t *testing.T) {
	home := t.TempDir()
	st, err := store.New(home)
	if err != nil {
		t.Fatal(err)
	}
	art := testArtifact(t)
	if err := st.Publish(art); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "space", "artifacts", "sha256-"+art.Revision[len("sha256:"):])
	man := filepath.Join(dir, "manifest.json")
	bak := man + ".real"
	if err := os.Rename(man, bak); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(bak, man); err != nil {
		t.Fatal(err)
	}
	var se *store.Error
	if _, err := st.Load(art.Revision); !errors.As(err, &se) || se.Kind != store.KindCorrupt {
		t.Fatalf("symlink manifest: %v", err)
	}
}

func TestLoadRejectsDuplicateKeyManifest(t *testing.T) {
	home := t.TempDir()
	st, err := store.New(home)
	if err != nil {
		t.Fatal(err)
	}
	art := testArtifact(t)
	if err := st.Publish(art); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "space", "artifacts", "sha256-"+art.Revision[len("sha256:"):])
	dup := []byte(`{"digest_format":"sha256","digest_format":"sha256","files":[]}` + "\n")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), dup, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = st.Load(art.Revision)
	var se *store.Error
	if !errors.As(err, &se) || se.Kind != store.KindCorrupt {
		t.Fatalf("duplicate-key manifest: %v", err)
	}
	if !strings.Contains(err.Error(), "SRC-JSON-DUP") {
		t.Fatalf("want jsonreader DUP via ValidateBytes, got %v", err)
	}
}
