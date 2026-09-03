//go:build unix

package fsync

import (
	"fmt"
	"syscall"
	"testing"
)

func TestDirErrorUnsupportedUnix(t *testing.T) {
	for _, err := range []error{syscall.ENOTSUP, syscall.EINVAL, syscall.ENOSYS} {
		if got := DirError(err); got != nil {
			t.Errorf("%v: want ignore, got %v", err, got)
		}
	}
	if syscall.EOPNOTSUPP != syscall.ENOTSUP {
		if got := DirError(syscall.EOPNOTSUPP); got != nil {
			t.Errorf("EOPNOTSUPP: want ignore, got %v", got)
		}
	}
	wrapped := fmt.Errorf("sync dir: %w", syscall.ENOTSUP)
	if got := DirError(wrapped); got != nil {
		t.Fatalf("wrapped ENOTSUP must be ignored: %v", got)
	}
}
