//go:build windows

package fsync

import (
	"errors"
	"syscall"
)

const (
	errorInvalidFunction  syscall.Errno = 1
	errorNotSupported     syscall.Errno = 50
	errorInvalidParameter syscall.Errno = 87
	errorAccessDenied     syscall.Errno = 5
)

func unsupported(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	switch errno {
	case errorInvalidFunction, errorNotSupported, errorInvalidParameter, errorAccessDenied:
		return true
	}
	return false
}
