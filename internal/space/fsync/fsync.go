package fsync

import (
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFile writes data and Syncs the file. File Sync failure is returned.
func WriteFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// Dir Syncs a directory. Errors that only mean “this platform/FS does not
// support directory fsync” are ignored; permission and I/O errors are returned.
func Dir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	return DirError(d.Sync())
}

// Tree Syncs every directory under root, deepest first, then root itself.
func Tree(root string) error {
	var dirs []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := Dir(dirs[i]); err != nil {
			return err
		}
	}
	return nil
}

// SyncCommittedDirBestEffort syncs a directory after an atomic rename that
// already made the new state visible. Failures are discarded on purpose:
// returning an ordinary error would look like “state unchanged” while the
// rename commit has already succeeded. V0 accepts a
// committed-but-durability-uncertain window here; there is no warning or
// telemetry channel yet. Do not convert this into a fail-closed register error.
func SyncCommittedDirBestEffort(path string) {
	_ = Dir(path)
}

// DirError applies the directory-sync policy to an error from File.Sync on a directory.
func DirError(err error) error {
	if err == nil || unsupported(err) {
		return nil
	}
	return err
}
