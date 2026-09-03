package space_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/source/rule"
	"github.com/agent2host/agent2host/internal/space"
	"github.com/agent2host/agent2host/internal/space/registry"
	"github.com/agent2host/agent2host/internal/space/store"
)

func TestRegisterRoundtrip(t *testing.T) {
	sp, err := space.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := fixtureTree(t, "valid", "markdown-leading-dashes")
	rep, err := sp.Register(src)
	if err != nil {
		t.Fatal(err)
	}
	if rep.SystemID != "fm" || rep.Version != "1.0.0" || len(rep.Agents) != 1 || rep.Agents[0] != "demo" {
		t.Fatalf("report %+v", rep)
	}
	if rep.Revision == "" {
		t.Fatal("empty revision")
	}
	reg, err := registry.New(sp.Home)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := reg.Get("fm")
	if err != nil {
		t.Fatal(err)
	}
	if rec.ActiveRevision != rep.Revision || rec.Version != "1.0.0" {
		t.Fatalf("record %+v", rec)
	}
	st, err := store.New(sp.Home)
	if err != nil {
		t.Fatal(err)
	}
	art, err := st.Load(rep.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if art.Revision != rep.Revision {
		t.Fatalf("artifact %s", art.Revision)
	}

	rep2, err := sp.Register(src)
	if err != nil {
		t.Fatal(err)
	}
	if rep2.Revision != rep.Revision {
		t.Fatalf("idempotent revision %s vs %s", rep2.Revision, rep.Revision)
	}
}

func TestRegisterUpdatesSameProvenance(t *testing.T) {
	sp, err := space.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := copyTree(t, fixtureTree(t, "valid", "markdown-leading-dashes"))
	rep1, err := sp.Register(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sops", "demo.sop.md"), []byte("# changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep2, err := sp.Register(src)
	if err != nil {
		t.Fatal(err)
	}
	if rep2.Revision == rep1.Revision {
		t.Fatal("content change must create a new revision")
	}
	reg, err := registry.New(sp.Home)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := reg.Get("fm")
	if err != nil {
		t.Fatal(err)
	}
	if rec.ActiveRevision != rep2.Revision {
		t.Fatalf("active %s want %s", rec.ActiveRevision, rep2.Revision)
	}
}

func TestRegisterProvenanceConflict(t *testing.T) {
	sp, err := space.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := fixtureTree(t, "valid", "markdown-leading-dashes")
	if _, err := sp.Register(src); err != nil {
		t.Fatal(err)
	}
	other := copyTree(t, src)
	_, err = sp.Register(other)
	var re *registry.Error
	if !errors.As(err, &re) || re.Kind != registry.KindProvenance {
		t.Fatalf("got %v", err)
	}
	reg, err := registry.New(sp.Home)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := reg.Get("fm")
	if err != nil {
		t.Fatal(err)
	}
	want, err := registry.CanonicalSource(src)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Source != want {
		t.Fatalf("source %q want %q", rec.Source, want)
	}
}

func TestRegisterWriteFailKeepsOldRevision(t *testing.T) {
	sp, err := space.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := copyTree(t, fixtureTree(t, "valid", "markdown-leading-dashes"))
	rep1, err := sp.Register(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sops", "demo.sop.md"), []byte("# new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sp.InjectRegistryWriteError(errors.New("injected registry write failure"))
	_, err = sp.Register(src)
	if err == nil {
		t.Fatal("expected injected write failure")
	}
	reg, err := registry.New(sp.Home)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := reg.Get("fm")
	if err != nil {
		t.Fatal(err)
	}
	if rec.ActiveRevision != rep1.Revision {
		t.Fatalf("active %s want old %s", rec.ActiveRevision, rep1.Revision)
	}
	st, err := store.New(sp.Home)
	if err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(filepath.Join(sp.Home, "space", "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) < 2 {
		t.Fatalf("orphan artifact should remain, got %d dirs", len(ents))
	}
	var extra string
	for _, e := range ents {
		rev := "sha256:" + strings.TrimPrefix(e.Name(), "sha256-")
		if rev == rep1.Revision {
			continue
		}
		if _, err := st.Load(rev); err == nil {
			extra = rev
		}
	}
	if extra == "" {
		t.Fatal("expected loadable orphan revision")
	}
}

func TestRegisterInvalidDoesNotInstall(t *testing.T) {
	home := t.TempDir()
	sp, err := space.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	src := fixtureTree(t, "invalid", "pem-private-key")
	_, err = sp.Register(src)
	if rule.ID(err) != "SRC-CONTENT-PRIVATE-KEY" {
		t.Fatalf("got %v", err)
	}
	reg, err := registry.New(home)
	if err != nil {
		t.Fatal(err)
	}
	_, err = reg.Get("pem")
	var re *registry.Error
	if !errors.As(err, &re) || re.Kind != registry.KindUnknown {
		t.Fatalf("must not install: %v", err)
	}
}

func TestRegisterConcurrentSystems(t *testing.T) {
	sp, err := space.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := fixtureTree(t, "valid", "markdown-leading-dashes")
	b := fixtureTree(t, "valid", "env-example")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, src := range []string{a, b} {
		src := src
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := sp.Register(src)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	reg, err := registry.New(sp.Home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Get("fm"); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Get("env-ex"); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterConcurrentSameSystem(t *testing.T) {
	sp, err := space.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := fixtureTree(t, "valid", "markdown-leading-dashes")
	var wg sync.WaitGroup
	revs := make(chan string, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rep, err := sp.Register(src)
			if err != nil {
				errs <- err
				return
			}
			revs <- rep.Revision
		}()
	}
	wg.Wait()
	close(errs)
	close(revs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first string
	for r := range revs {
		if first == "" {
			first = r
			continue
		}
		if r != first {
			t.Fatalf("revisions %s vs %s", first, r)
		}
	}
}

func TestRemoveUnregistersAndKeepsSource(t *testing.T) {
	home := t.TempDir()
	sp, err := space.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	src := fixtureTree(t, "valid", "markdown-leading-dashes")
	rep, err := sp.Register(src)
	if err != nil {
		t.Fatal(err)
	}
	hostState := filepath.Join(home, "auth-profiles", "claude-code", "keep")
	if err := os.MkdirAll(filepath.Dir(hostState), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hostState, []byte("login"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sp.Remove("fm"); err != nil {
		t.Fatal(err)
	}
	reg, err := registry.New(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Get("fm"); err == nil {
		t.Fatal("removed system must not remain")
	}
	st, err := store.New(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Load(rep.Revision); err == nil {
		t.Fatal("unreferenced artifact must be GCd")
	}
	if _, err := os.Stat(filepath.Join(src, "system.json")); err != nil {
		t.Fatal("must not delete user Source")
	}
	if _, err := os.Stat(hostState); err != nil {
		t.Fatal("must not delete Host state")
	}
}

func fixtureTree(t *testing.T, bucket, name string) string {
	t.Helper()
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "trees", bucket, name)
}

func copyTree(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}
