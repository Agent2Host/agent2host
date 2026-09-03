package path_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agent2host/agent2host/internal/source/path"
	"github.com/agent2host/agent2host/internal/source/rule"
)

func TestStoreInternalHardDenyUsesHomeSpaceOnly(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	art := filepath.Join(home, "space", "artifacts", "sha256-dead", "content")
	if err := os.MkdirAll(art, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(art, "system.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, "notes.md")
	if err := os.WriteFile(outside, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	fsys, err := path.NewFS(root, home)
	if err != nil {
		t.Fatal(err)
	}

	// Relocate a declared member so CheckDeclared's join is under artifacts.
	// CheckDeclared joins Root+canonical; storeInternal uses the absolute target.
	fsys.Root = art
	_, err = fsys.CheckDeclared("system.json", path.RoleSystemJSON)
	if rule.ID(err) != "SRC-PATH-HARD-DENY" {
		t.Fatalf("artifact-store path: %v", err)
	}

	fsys.Root = home
	raw, err := fsys.CheckDeclared("notes.md", path.RoleOther)
	if err != nil {
		t.Fatalf("path under home but not store internals must pass: %v", err)
	}
	if string(raw) != "ok" {
		t.Fatalf("bytes %q", raw)
	}
}

func TestCustomHomeNotDefaultHome(t *testing.T) {
	custom := t.TempDir()
	defLike := t.TempDir()
	root := t.TempDir()
	hidden := filepath.Join(custom, "space", "artifacts", "x")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "p.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	fsys, err := path.NewFS(root, custom)
	if err != nil {
		t.Fatal(err)
	}
	fsys.Root = hidden
	_, err = fsys.CheckDeclared("p.md", path.RoleOther)
	if rule.ID(err) != "SRC-PATH-HARD-DENY" {
		t.Fatalf("custom --home artifact-store must deny: %v", err)
	}

	fsys2, err := path.NewFS(root, defLike)
	if err != nil {
		t.Fatal(err)
	}
	fsys2.Root = hidden
	if _, err := fsys2.CheckDeclared("p.md", path.RoleOther); err != nil {
		t.Fatalf("path not under this home's store internals: %v", err)
	}
}

func TestStoreInternalHardDenyResolvesHomeSymlink(t *testing.T) {
	realHome := t.TempDir()
	linkDir := t.TempDir()
	link := filepath.Join(linkDir, "home-link")
	if err := os.Symlink(realHome, link); err != nil {
		t.Fatal(err)
	}
	art := filepath.Join(realHome, "space", "artifacts", "sha256-dead", "content")
	if err := os.MkdirAll(art, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(art, "system.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	fsys, err := path.NewFS(t.TempDir(), link)
	if err != nil {
		t.Fatal(err)
	}
	fsys.Root = art
	_, err = fsys.CheckDeclared("system.json", path.RoleSystemJSON)
	if rule.ID(err) != "SRC-PATH-HARD-DENY" {
		t.Fatalf("symlink --home must deny real store path: %v", err)
	}
}

func TestResolveRootLongestExistingAncestor(t *testing.T) {
	real := t.TempDir()
	realResolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	linkDir := t.TempDir()
	link := filepath.Join(linkDir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(link, "new-home")
	got, err := path.ResolveRoot(missing)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(realResolved, "new-home")
	if got != want {
		t.Fatalf("ResolveRoot(%q)=%q want %q", missing, got, want)
	}
	nested := filepath.Join(link, "a", "b", "c")
	got, err = path.ResolveRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Join(realResolved, "a", "b", "c")
	if got != want {
		t.Fatalf("nested ResolveRoot=%q want %q", got, want)
	}
}

func TestStoreInternalHardDenyMissingLeafUnderSymlinkHome(t *testing.T) {
	real := t.TempDir()
	linkDir := t.TempDir()
	link := filepath.Join(linkDir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	// Leaf home does not exist yet; ResolveRoot must still land on real/new-home.
	homeFlag := filepath.Join(link, "new-home")
	physHome := filepath.Join(real, "new-home")

	cases := []struct {
		name string
		dir  string
		file string
	}{
		{"artifacts", filepath.Join(physHome, "space", "artifacts", "sha256-dead", "content"), "system.json"},
		{"systems", filepath.Join(physHome, "space", "systems"), "deadbeef.lock"},
		{"registry", filepath.Join(physHome, "space"), "registry.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.MkdirAll(tc.dir, 0o755); err != nil {
				t.Fatal(err)
			}
			pathUnder := filepath.Join(tc.dir, tc.file)
			if err := os.WriteFile(pathUnder, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			fsys, err := path.NewFS(t.TempDir(), homeFlag)
			if err != nil {
				t.Fatal(err)
			}
			fsys.Root = tc.dir
			_, err = fsys.CheckDeclared(tc.file, path.RoleOther)
			if rule.ID(err) != "SRC-PATH-HARD-DENY" {
				t.Fatalf("%s under missing-leaf symlink home: %v", tc.name, err)
			}
		})
	}
}
