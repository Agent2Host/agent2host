package registry_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/agent2host/agent2host/internal/space/registry"
)

func TestPutFirstAndSameProvenance(t *testing.T) {
	reg, err := registry.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec := registry.Record{
		ActiveRevision: "sha256:" + "a" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Source:         "/src/club",
		Version:        "0.1.0",
		Agents:         []string{"faq"},
	}
	if err := reg.WithWrite(func(doc *registry.Document) error {
		return registry.Put(doc, "club-system", rec)
	}); err != nil {
		t.Fatal(err)
	}
	got, err := reg.Get("club-system")
	if err != nil {
		t.Fatal(err)
	}
	if got.ActiveRevision != rec.ActiveRevision || got.Source != rec.Source || got.Version != rec.Version {
		t.Fatalf("got %+v", got)
	}

	rec.ActiveRevision = "sha256:" + "c" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	rec.Version = "0.2.0"
	if err := reg.WithWrite(func(doc *registry.Document) error {
		return registry.Put(doc, "club-system", rec)
	}); err != nil {
		t.Fatal(err)
	}
	got, err = reg.Get("club-system")
	if err != nil {
		t.Fatal(err)
	}
	if got.ActiveRevision != rec.ActiveRevision || got.Version != "0.2.0" {
		t.Fatalf("update %+v", got)
	}
}

func TestPutDifferentProvenanceRefused(t *testing.T) {
	reg, err := registry.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.WithWrite(func(doc *registry.Document) error {
		return registry.Put(doc, "club-system", registry.Record{
			ActiveRevision: "sha256:" + "a" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Source:         "/src/a",
			Version:        "0.1.0",
			Agents:         []string{"faq"},
		})
	}); err != nil {
		t.Fatal(err)
	}
	err = reg.WithWrite(func(doc *registry.Document) error {
		return registry.Put(doc, "club-system", registry.Record{
			ActiveRevision: "sha256:" + "d" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Source:         "/src/b",
			Version:        "0.1.0",
			Agents:         []string{"faq"},
		})
	})
	var re *registry.Error
	if !errors.As(err, &re) || re.Kind != registry.KindProvenance {
		t.Fatalf("got %v", err)
	}
	got, err := reg.Get("club-system")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "/src/a" {
		t.Fatalf("must keep original source: %+v", got)
	}
}

func TestGetUnknown(t *testing.T) {
	reg, err := registry.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = reg.Get("missing")
	var re *registry.Error
	if !errors.As(err, &re) || re.Kind != registry.KindUnknown {
		t.Fatalf("got %v", err)
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	reg, err := registry.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	doc, err := reg.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Systems) != 0 {
		t.Fatalf("got %+v", doc)
	}
}

func TestEmptyRecordRejected(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "space")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "registry.json"), []byte(`{"systems":{"x":{}}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := registry.New(home)
	if err != nil {
		t.Fatal(err)
	}
	_, err = reg.Load()
	var re *registry.Error
	if !errors.As(err, &re) || re.Kind != registry.KindCorrupt {
		t.Fatalf("got %v", err)
	}
}

func TestIllegalRevisionRejected(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "space")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"systems":{"club-system":{"active_revision":"../artifacts/x","source":"/tmp/src","version":"1.0.0","agents":["a"]}}}` + "\n")
	if err := os.WriteFile(filepath.Join(dir, "registry.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := registry.New(home)
	if err != nil {
		t.Fatal(err)
	}
	_, err = reg.Load()
	var re *registry.Error
	if !errors.As(err, &re) || re.Kind != registry.KindCorrupt {
		t.Fatalf("got %v", err)
	}
}

func TestCorruptJSON(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "space")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "registry.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := registry.New(home)
	if err != nil {
		t.Fatal(err)
	}
	_, err = reg.Load()
	var re *registry.Error
	if !errors.As(err, &re) || re.Kind != registry.KindCorrupt {
		t.Fatalf("got %v", err)
	}
}

func TestCanonicalSource(t *testing.T) {
	dir := t.TempDir()
	a, err := registry.CanonicalSource(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := registry.CanonicalSource(filepath.Join(dir, ".", "x", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("%q vs %q", a, b)
	}
}

func TestConcurrentWritesBothLand(t *testing.T) {
	reg, err := registry.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := fmt.Sprintf("sys-%d", i)
			errs <- reg.WithWrite(func(doc *registry.Document) error {
				return registry.Put(doc, id, registry.Record{
					ActiveRevision: "sha256:" + "e" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					Source:         "/src/" + id,
					Version:        "0.1.0",
					Agents:         []string{"a"},
				})
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	doc, err := reg.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Systems) != n {
		t.Fatalf("systems %d want %d: %+v", len(doc.Systems), n, doc.Systems)
	}
}

func TestLockSystemThenWrite(t *testing.T) {
	reg, err := registry.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := reg.LockSystem("club-system")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unlock(); err != nil {
			t.Fatal(err)
		}
	}()
	if err := reg.WithWrite(func(doc *registry.Document) error {
		return registry.Put(doc, "club-system", registry.Record{
			ActiveRevision: "sha256:" + "f" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Source:         "/src/club",
			Version:        "0.1.0",
			Agents:         []string{"faq"},
		})
	}); err != nil {
		t.Fatal(err)
	}
}
