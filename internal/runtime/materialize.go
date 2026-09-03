package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agent2host/agent2host/internal/adapter"
)

const (
	runtimeDirMode  = 0o750
	runtimeFileMode = 0o640
	runtimeExecMode = 0o750
)

func materialize(p Prepared, plan adapter.NativeProjectionPlan) error {
	for _, f := range plan.Files {
		root, err := rootFor(p, f.Class)
		if err != nil {
			return err
		}
		dest, err := joinUnder(root, f.RelPath)
		if err != nil {
			return err
		}
		if err := mkdirAllNoFollow(root, filepath.Dir(dest)); err != nil {
			return err
		}
		if err := rejectExistingSymlink(dest); err != nil {
			return err
		}
		mode := os.FileMode(runtimeFileMode)
		if f.Executable {
			mode = runtimeExecMode
		}
		if err := os.WriteFile(dest, f.Content, mode); err != nil {
			return err
		}
		// WriteFile keeps the mode of an existing file; force Plan mode.
		if err := os.Chmod(dest, mode); err != nil {
			return err
		}
	}
	return nil
}

func rootFor(p Prepared, class adapter.DestinationClass) (string, error) {
	switch class {
	case adapter.DestProjection, "":
		return p.Workspace, nil
	case adapter.DestWorkspace:
		return p.Workspace, nil
	case adapter.DestHostPrivate:
		return p.Private, nil
	case adapter.DestHostAuth:
		if p.Home == "" {
			return "", fmt.Errorf("%w: host_auth requires Agent2Host home", ErrPathEscape)
		}
		return filepath.Join(p.Home, adapter.HostAuthDirName), nil
	case adapter.DestAuthProfile:
		if p.AuthProfile == "" {
			return "", fmt.Errorf("%w: auth_profile requires a bound Auth Profile", ErrPathEscape)
		}
		return p.AuthProfile, nil
	case adapter.DestWorkingDir:
		return "", fmt.Errorf("%w: refusing user working-directory destinations", ErrPathEscape)
	default:
		return "", fmt.Errorf("%w: unknown class %s", ErrPathEscape, class)
	}
}

func joinUnder(root, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", ErrPathEscape
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrPathEscape
	}
	dest := filepath.Join(root, clean)
	relTo, err := filepath.Rel(root, dest)
	if err != nil || strings.HasPrefix(relTo, "..") {
		return "", ErrPathEscape
	}
	return dest, nil
}

func mkdirAllNoFollow(root, dir string) error {
	if dir == "" || dir == root {
		return nil
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ErrPathEscape
	}
	cur := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		if os.IsNotExist(err) {
			if err := os.Mkdir(cur, runtimeDirMode); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink %s", ErrPathEscape, cur)
		}
		if !fi.IsDir() {
			return fmt.Errorf("%w: not a directory %s", ErrPathEscape, cur)
		}
	}
	return nil
}

func rejectExistingSymlink(dest string) error {
	fi, err := os.Lstat(dest)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: symlink %s", ErrPathEscape, dest)
	}
	return nil
}

func launchCWD(p Prepared, class adapter.DestinationClass) string {
	if class == adapter.DestWorkingDir && p.WorkingDir != "" {
		return p.WorkingDir
	}
	return p.Workspace
}
