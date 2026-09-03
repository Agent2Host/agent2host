package path

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/agent2host/agent2host/internal/source/rule"
)

// FS checks declared paths under a canonical System Root. Lstat/ReadFile
// may be stubbed (SRC-PATH-TYPE device recipe).
type FS struct {
	Root     string
	Home     string
	Lstat    func(string) (os.FileInfo, error)
	ReadFile func(string) ([]byte, error)
}

// NewFS resolves root once (CLI symlink allowed) and uses os.Lstat / os.ReadFile.
func NewFS(root, home string) (*FS, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	if home == "" {
		home = os.Getenv("A2H_HOME")
	}
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = filepath.Join(h, ".a2h")
		}
	}
	if home != "" {
		home, err = ResolveRoot(home)
		if err != nil {
			return nil, err
		}
	}
	return &FS{
		Root:     resolved,
		Home:     home,
		Lstat:    os.Lstat,
		ReadFile: os.ReadFile,
	}, nil
}

// ResolveRoot returns an absolute path with existing symlink components
// resolved. When the final component (or a trailing suffix) does not exist
// yet, the longest existing ancestor is EvalSymlinks'd and the missing
// suffix is reattached. Permission and I/O errors are returned; only
// os.IsNotExist triggers the ancestor walk. Concurrent symlink replacement
// during register remains outside the V0 threat model (see CheckDeclared).
func ResolveRoot(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	cur := abs
	var missing []string
	for {
		parent := filepath.Dir(cur)
		if parent == cur {
			// No existing ancestor; keep lexical Abs (volume root case).
			return abs, nil
		}
		missing = append(missing, filepath.Base(cur))
		cur = parent
		resolved, err = filepath.EvalSymlinks(cur)
		if err == nil {
			parts := make([]string, 0, 1+len(missing))
			parts = append(parts, resolved)
			for i := len(missing) - 1; i >= 0; i-- {
				parts = append(parts, missing[i])
			}
			return filepath.Join(parts...), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
	}
}

// Read returns raw bytes of a already-validated canonical member.
func (fs *FS) Read(canonical string) ([]byte, error) {
	return fs.ReadFile(fs.join(canonical))
}

func (fs *FS) join(canonical string) string {
	parts := strings.Split(canonical, "/")
	return filepath.Join(append([]string{fs.Root}, parts...)...)
}

// CheckDeclared verifies existence, type, containment, hard-deny store
// internals, PEM, and encoding for one canonical member.
//
// Threat boundary: this walk uses path-based Lstat then ReadFile. It rejects a
// symlink observed at check time. It does not implement descriptor-relative
// openat/O_NOFOLLOW for every component, so a concurrent replacement of an
// intermediate directory with a symlink after Lstat can still race. V0 register
// assumes the Source tree is not being concurrently mutated by an adversary
// during that walk. Do not treat leaf O_NOFOLLOW as closing this race.
func (fs *FS) CheckDeclared(canonical string, role Role) ([]byte, error) {
	absRoot := fs.Root
	target := absRoot
	parts := strings.Split(canonical, "/")
	for i, p := range parts {
		target = filepath.Join(target, p)
		rel, err := filepath.Rel(absRoot, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return nil, rule.Fail("SRC-PATH-ESCAPE", canonical)
		}
		st, err := fs.Lstat(target)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, rule.Fail("SRC-PATH-DECLARED", canonical)
			}
			return nil, err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, rule.Fail("SRC-PATH-TYPE", canonical)
		}
		if i < len(parts)-1 {
			if !st.IsDir() {
				return nil, rule.Fail("SRC-PATH-TYPE", canonical)
			}
			continue
		}
		if !st.Mode().IsRegular() {
			return nil, rule.Fail("SRC-PATH-TYPE", canonical)
		}
	}
	if fs.storeInternal(target) {
		return nil, rule.Fail("SRC-PATH-HARD-DENY", canonical)
	}
	raw, err := fs.ReadFile(target)
	if err != nil {
		return nil, err
	}
	if HasPEM(raw) {
		return nil, rule.Fail("SRC-CONTENT-PRIVATE-KEY", canonical)
	}
	if err := checkEncoding(role, canonical, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (fs *FS) storeInternal(absPath string) bool {
	if fs.Home == "" {
		return false
	}
	home, err := ResolveRoot(fs.Home)
	if err != nil {
		return false
	}
	target, err := ResolveRoot(absPath)
	if err != nil {
		return false
	}
	space := filepath.Join(home, "space")
	for _, root := range []string{
		filepath.Join(space, "artifacts"),
		filepath.Join(space, "systems"),
		filepath.Join(space, "registry.json"),
		filepath.Join(space, "registry.lock"),
	} {
		if containedOrEqual(root, target) {
			return true
		}
	}
	return false
}

func containedOrEqual(root, absPath string) bool {
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func checkEncoding(role Role, canonical string, raw []byte) error {
	switch role {
	case RoleSOP, RoleSkillDoc:
		if !utf8.Valid(raw) {
			return rule.Fail("SRC-ENC-MD", canonical)
		}
	case RoleContext:
		if !utf8.Valid(raw) {
			return rule.Fail("SRC-ENC-CTX", canonical)
		}
	case RoleOutputSchema:
		return CheckOutputSchema(raw)
	}
	return nil
}
