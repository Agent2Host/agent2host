package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/space"
)

var (
	ErrWorkRootEscape   = errors.New("runtime: work root escapes the home directory")
	ErrWorkRootProject  = errors.New("runtime: --project is only valid when work_root.mode is invocation")
	ErrWorkRootMissing  = errors.New("runtime: --project directory does not exist")
	ErrWorkRootNotDir   = errors.New("runtime: work root is not a directory")
	ErrWorkRootSymlink  = errors.New("runtime: work root resolves through a symlink outside the home directory")
	ErrWorkRootBadRel   = errors.New("runtime: path_from_home is not a home-relative archive path")
	ErrWorkRootNeedHome = errors.New("runtime: cannot resolve a fixed work root without a user home directory")
	ErrWorkRootNeedCwd  = errors.New("runtime: cannot resolve an invocation work root without a current directory")
)

// WorkRootResolution is the Host process directory for this check/run.
type WorkRootResolution struct {
	Mode    string
	Path    string
	Exists  bool
	Created bool
}

// ResolveWorkRoot turns a System declaration into an absolute directory.
// projectFlag is the --project value; it is rejected for fixed mode.
func ResolveWorkRoot(decl decode.WorkRoot, projectFlag, cwd, userHome string) (WorkRootResolution, error) {
	if decl.Mode == "" {
		decl.Mode = decode.WorkRootInvocation
	}
	switch decl.Mode {
	case decode.WorkRootFixed:
		if projectFlag != "" {
			return WorkRootResolution{}, ErrWorkRootProject
		}
		return resolveFixed(decl.PathFromHome, userHome)
	case decode.WorkRootInvocation:
		return resolveInvocation(projectFlag, cwd)
	default:
		return WorkRootResolution{}, fmt.Errorf("runtime: unknown work_root.mode %q", decl.Mode)
	}
}

// ResolveRunWorkRoot reads the declaration from a resolved Agent.
func ResolveRunWorkRoot(run *space.ResolvedAgentRun, projectFlag, cwd, userHome string) (WorkRootResolution, error) {
	decl := decode.WorkRoot{Mode: decode.WorkRootInvocation}
	if run != nil {
		decl = run.WorkRoot
	}
	return ResolveWorkRoot(decl, projectFlag, cwd, userHome)
}

// EnsureWorkRoot creates a missing fixed archive root. Invocation roots must already exist.
func EnsureWorkRoot(res WorkRootResolution) (WorkRootResolution, error) {
	if res.Path == "" {
		return res, fmt.Errorf("runtime: empty work root")
	}
	st, err := os.Lstat(res.Path)
	if err == nil {
		if !st.IsDir() {
			return res, ErrWorkRootNotDir
		}
		res.Exists = true
		return res, nil
	}
	if !os.IsNotExist(err) {
		return res, err
	}
	if res.Mode != decode.WorkRootFixed {
		return res, ErrWorkRootMissing
	}
	if err := os.MkdirAll(res.Path, 0o755); err != nil {
		return res, err
	}
	res.Exists = true
	res.Created = true
	return res, nil
}

func resolveFixed(rel, userHome string) (WorkRootResolution, error) {
	if userHome == "" {
		return WorkRootResolution{}, ErrWorkRootNeedHome
	}
	if err := checkPathFromHome(rel); err != nil {
		return WorkRootResolution{}, err
	}
	home, err := filepath.Abs(userHome)
	if err != nil {
		return WorkRootResolution{}, err
	}
	if real, err := filepath.EvalSymlinks(home); err == nil {
		home = real
	}
	abs := filepath.Clean(filepath.Join(home, filepath.FromSlash(rel)))
	if err := confinedToHome(abs, home); err != nil {
		return WorkRootResolution{}, err
	}
	exists := false
	if st, err := os.Lstat(abs); err == nil {
		if !st.IsDir() && st.Mode()&os.ModeSymlink == 0 {
			return WorkRootResolution{}, ErrWorkRootNotDir
		}
		exists = st.IsDir() || st.Mode()&os.ModeSymlink != 0
		if err := rejectEscapingSymlink(abs, home); err != nil {
			return WorkRootResolution{}, err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(abs)
			if err != nil {
				return WorkRootResolution{}, err
			}
			if err := confinedToHome(target, home); err != nil {
				return WorkRootResolution{}, ErrWorkRootSymlink
			}
			abs = target
			exists = true
		}
	} else if !os.IsNotExist(err) {
		return WorkRootResolution{}, err
	} else if err := rejectEscapingAncestorSymlink(abs, home); err != nil {
		return WorkRootResolution{}, err
	}
	return WorkRootResolution{Mode: decode.WorkRootFixed, Path: abs, Exists: exists}, nil
}

func resolveInvocation(projectFlag, cwd string) (WorkRootResolution, error) {
	raw := projectFlag
	if raw == "" {
		if cwd == "" {
			return WorkRootResolution{}, ErrWorkRootNeedCwd
		}
		raw = cwd
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return WorkRootResolution{}, err
	}
	abs = filepath.Clean(abs)
	st, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return WorkRootResolution{}, fmt.Errorf("%w: %s", ErrWorkRootMissing, abs)
		}
		return WorkRootResolution{}, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return WorkRootResolution{}, err
		}
		abs = target
		st, err = os.Stat(abs)
		if err != nil {
			return WorkRootResolution{}, err
		}
	}
	if !st.IsDir() {
		return WorkRootResolution{}, ErrWorkRootNotDir
	}
	return WorkRootResolution{Mode: decode.WorkRootInvocation, Path: abs, Exists: true}, nil
}

func checkPathFromHome(rel string) error {
	if rel == "" || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "\\") ||
		strings.Contains(rel, "\\") || strings.HasPrefix(rel, "~") {
		return ErrWorkRootBadRel
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return ErrWorkRootBadRel
		}
	}
	return nil
}

func confinedToHome(abs, home string) error {
	realHome := home
	if h, err := filepath.EvalSymlinks(home); err == nil {
		realHome = h
	}
	rel, err := filepath.Rel(realHome, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return ErrWorkRootEscape
	}
	return nil
}

func rejectEscapingSymlink(path, home string) error {
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if st.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if err := confinedToHome(target, home); err != nil {
		return ErrWorkRootSymlink
	}
	return nil
}

func rejectEscapingAncestorSymlink(abs, home string) error {
	cur := abs
	for {
		parent := filepath.Dir(cur)
		if parent == cur {
			return nil
		}
		if err := confinedToHome(parent, home); err != nil {
			return err
		}
		if st, err := os.Lstat(parent); err == nil && st.Mode()&os.ModeSymlink != 0 {
			if err := rejectEscapingSymlink(parent, home); err != nil {
				return err
			}
		}
		if samePath(parent, home) {
			return nil
		}
		cur = parent
	}
}

func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil {
		ra = a
	}
	if errB != nil {
		rb = b
	}
	return filepath.Clean(ra) == filepath.Clean(rb)
}
