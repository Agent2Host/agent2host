package fsync

import (
	"errors"
	"io"
	"path/filepath"
	"syscall"
	"testing"
)

func TestDirErrorPolicy(t *testing.T) {
	if err := DirError(nil); err != nil {
		t.Fatalf("nil: %v", err)
	}
	if err := DirError(io.ErrUnexpectedEOF); err == nil {
		t.Fatal("real I/O must return")
	}
	if err := DirError(errors.New("permission denied")); err == nil {
		t.Fatal("plain error must return")
	}
	if err := DirError(syscall.EPERM); err == nil {
		t.Fatal("EPERM must return")
	}
}

func TestSyncCommittedDirBestEffortSwallowsErrors(t *testing.T) {
	// Missing path: Dir fails; best-effort must not panic or return (void API).
	SyncCommittedDirBestEffort(filepath.Join(t.TempDir(), "no-such-dir"))
	// Existing dir: should succeed silently.
	SyncCommittedDirBestEffort(t.TempDir())
}
