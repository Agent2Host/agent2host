//go:build windows

package fsync

import (
	"syscall"
	"testing"
)

func TestDirErrorAccessDeniedWindows(t *testing.T) {
	if got := DirError(syscall.ERROR_ACCESS_DENIED); got != nil {
		t.Fatalf("ERROR_ACCESS_DENIED must be ignored for directory sync: %v", got)
	}
}
