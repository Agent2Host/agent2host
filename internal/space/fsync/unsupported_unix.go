//go:build unix

package fsync

import (
	"errors"
	"syscall"
)

func unsupported(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	switch errno {
	case syscall.ENOTSUP, syscall.EINVAL, syscall.ENOSYS:
		return true
	}
	if syscall.EOPNOTSUPP != syscall.ENOTSUP && errno == syscall.EOPNOTSUPP {
		return true
	}
	return false
}
