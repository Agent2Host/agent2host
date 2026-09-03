package runtime

// Host-state bind: apply a Host's declared copies/env/args so the next run
// finds the same native home. This is not login, token parse, or refresh.
// Execute authorization lives in auth.go.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/agent2host/agent2host/internal/adapter"
)

// authPersistInterval is how often declared opaque materials are written back
// while the Host is still running. Exit-only save is not enough: an
// interrupted parent (`go run`, Ctrl-C) can kill Agent2Host before defer runs.
var authPersistInterval = 500 * time.Millisecond

const (
	hostAuthFileMode = 0o600
	hostAuthDirMode  = 0o700
)

func authProfileDir(home string, key adapter.AuthProfileKey) string {
	return filepath.Join(home, adapter.AuthProfilesDirName, key.Host, key.DirName())
}

func attachAuthProfile(p Prepared, binder adapter.HostStateBinder) (Prepared, error) {
	if binder == nil {
		return p, nil
	}
	d := binder.DescribeAuth()
	if err := adapter.ValidateAuthDescription(d); err != nil {
		return p, err
	}
	p.AuthProfile = authProfileDir(p.Home, d.Profile)
	if err := os.MkdirAll(p.AuthProfile, hostAuthDirMode); err != nil {
		return p, err
	}
	return p, nil
}

func bindRootPath(p Prepared, root adapter.AuthRoot) (string, error) {
	switch root {
	case adapter.AuthRootPrivate:
		return p.Private, nil
	case adapter.AuthRootProfile:
		if p.AuthProfile == "" {
			return "", fmt.Errorf("host-auth: auth profile directory is empty")
		}
		return p.AuthProfile, nil
	default:
		return "", fmt.Errorf("host-auth: unknown bind root %s", root)
	}
}

// bindHostAuth applies the adapter's declared Host-state bind: opaque copies,
// env, and extra args. Missing store files are not an error: the Host will
// prompt natively. Runtime never imports the user's global Host home, browser
// sessions, or OS credential stores, and never decides whether the Host is logged in.
func bindHostAuth(p Prepared, binder adapter.HostStateBinder) (adapter.AuthBindDirective, error) {
	if binder == nil {
		return adapter.AuthBindDirective{}, nil
	}
	dir, err := binder.BindForRun(adapter.AuthBindRequest{
		ProfileDir: p.AuthProfile,
		PrivateDir: p.Private,
	})
	if err != nil {
		return adapter.AuthBindDirective{}, err
	}
	if err := applyAuthCopies(p, dir.Copies, true); err != nil {
		return adapter.AuthBindDirective{}, err
	}
	return dir, nil
}

// finalizeHostAuth writes adapter-declared opaque blobs back to the stable
// Host-state directory. It does not scan directories or parse credentials.
func finalizeHostAuth(p Prepared, binder adapter.HostStateBinder) error {
	if binder == nil {
		return nil
	}
	dir, err := binder.FinalizeRun(adapter.AuthFinalizeRequest{
		ProfileDir: p.AuthProfile,
		PrivateDir: p.Private,
	})
	if err != nil {
		return err
	}
	return applyAuthCopies(p, dir.Copies, false)
}

// watchHostAuth copies declared materials into the stable Host-state directory
// while the Host runs so a first native login is not lost if Agent2Host is killed
// before exit finalization.
func watchHostAuth(p Prepared, binder adapter.HostStateBinder) func() {
	if binder == nil {
		return func() {}
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(authPersistInterval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				_ = finalizeHostAuth(p, binder)
			}
		}
	}()
	return func() {
		close(done)
		wg.Wait()
	}
}

func applyAuthCopies(p Prepared, copies []adapter.AuthMaterial, storeToBind bool) error {
	for _, m := range copies {
		if err := copyDeclaredAuth(p, m, storeToBind); err != nil {
			return err
		}
	}
	return nil
}

func copyDeclaredAuth(p Prepared, m adapter.AuthMaterial, storeToBind bool) error {
	root, err := bindRootPath(p, m.BindRoot)
	if err != nil {
		return err
	}
	bindPath, err := joinUnder(root, m.BindRel)
	if err != nil {
		return err
	}
	storePath, err := joinUnder(p.AuthProfile, m.StoreRel)
	if err != nil {
		return err
	}
	src, dest := storePath, bindPath
	if !storeToBind {
		src, dest = bindPath, storePath
	}
	if filepath.Clean(src) == filepath.Clean(dest) {
		return nil
	}
	run := func() error {
		ok, err := regularFile(src)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		return copyAuthFile(src, dest)
	}
	if m.Lock {
		return withAuthStoreFileLock(storePath, run)
	}
	return run()
}

// withAuthStoreFileLock serializes read/write of one declared store file.
func withAuthStoreFileLock(storePath string, fn func() error) error {
	lockPath := storePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), hostAuthDirMode); err != nil {
		return err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, hostAuthFileMode)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	return fn()
}

func regularFile(path string) (bool, error) {
	fi, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("host-auth: refusing symlink %s", path)
	}
	return fi.Mode().IsRegular(), nil
}

func copyAuthFile(src, dest string) error {
	ok, err := regularFile(src)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("host-auth: not a regular file %s", src)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	body, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	return writeAuthFile(dest, body)
}

func writeAuthFile(dest string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(dest), hostAuthDirMode); err != nil {
		return err
	}
	if err := rejectExistingSymlink(dest); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".auth-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(hostAuthFileMode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return err
	}
	ok = true
	return nil
}

func hostAuthNote(w io.Writer, stage string, err error) {
	if w == nil || err == nil {
		return
	}
	fmt.Fprintf(w, "a2h: host-state %s: %v\n", stage, err)
}
