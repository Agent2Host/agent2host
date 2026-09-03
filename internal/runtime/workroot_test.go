package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent2host/agent2host/internal/source/decode"
)

func TestResolveWorkRootInvocationCwd(t *testing.T) {
	cwd := t.TempDir()
	got, err := ResolveWorkRoot(decode.WorkRoot{Mode: decode.WorkRootInvocation}, "", cwd, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != want && got.Path != cwd {
		// Abs may not eval; compare cleaned abs
		abs, _ := filepath.Abs(cwd)
		if got.Path != abs {
			t.Fatalf("path %q want %q", got.Path, abs)
		}
	}
	if got.Mode != decode.WorkRootInvocation || !got.Exists {
		t.Fatalf("%+v", got)
	}
}

func TestResolveWorkRootInvocationProject(t *testing.T) {
	dir := t.TempDir()
	got, err := ResolveWorkRoot(decode.WorkRoot{Mode: decode.WorkRootInvocation}, dir, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(dir)
	if got.Path != abs {
		t.Fatalf("path %q want %q", got.Path, abs)
	}
}

func TestResolveWorkRootFixedRejectsProject(t *testing.T) {
	home := t.TempDir()
	_, err := ResolveWorkRoot(decode.WorkRoot{Mode: decode.WorkRootFixed, PathFromHome: "Desktop/Events"}, home, home, home)
	if !errors.Is(err, ErrWorkRootProject) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveWorkRootFixedCreatesUnderHome(t *testing.T) {
	home := t.TempDir()
	realHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		realHome = home
	}
	got, err := ResolveWorkRoot(decode.WorkRoot{Mode: decode.WorkRootFixed, PathFromHome: "Desktop/Crossroads/Events"}, "", t.TempDir(), home)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(realHome, "Desktop", "Crossroads", "Events")
	if got.Path != want {
		t.Fatalf("path %q want %q", got.Path, want)
	}
	if got.Exists {
		t.Fatal("must not exist yet")
	}
	ensured, err := EnsureWorkRoot(got)
	if err != nil {
		t.Fatal(err)
	}
	if !ensured.Created || !ensured.Exists {
		t.Fatalf("%+v", ensured)
	}
	st, err := os.Stat(want)
	if err != nil || !st.IsDir() {
		t.Fatalf("created: %v %v", st, err)
	}
}

func TestResolveWorkRootFixedRejectsDotDot(t *testing.T) {
	home := t.TempDir()
	_, err := ResolveWorkRoot(decode.WorkRoot{Mode: decode.WorkRootFixed, PathFromHome: "Desktop/../secret"}, "", t.TempDir(), home)
	if !errors.Is(err, ErrWorkRootBadRel) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveWorkRootFixedRejectsEscapingSymlink(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(home, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveWorkRoot(decode.WorkRoot{Mode: decode.WorkRootFixed, PathFromHome: "escape"}, "", t.TempDir(), home)
	if !errors.Is(err, ErrWorkRootSymlink) && !errors.Is(err, ErrWorkRootEscape) {
		t.Fatalf("got %v", err)
	}
}

func TestEnsureInvocationMissing(t *testing.T) {
	_, err := EnsureWorkRoot(WorkRootResolution{Mode: decode.WorkRootInvocation, Path: filepath.Join(t.TempDir(), "nope")})
	if !errors.Is(err, ErrWorkRootMissing) {
		t.Fatalf("got %v", err)
	}
}

func TestNilDeclIsInvocation(t *testing.T) {
	cwd := t.TempDir()
	got, err := ResolveWorkRoot(decode.WorkRoot{}, "", cwd, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != decode.WorkRootInvocation {
		t.Fatalf("%+v", got)
	}
}
