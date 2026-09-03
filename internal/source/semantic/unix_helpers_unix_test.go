//go:build !windows

package semantic_test

import (
	"net"
	"os"
	"syscall"
)

func mkfifo(p string) error {
	return syscall.Mkfifo(p, 0o644)
}

func listenUnix(p string) (net.Listener, error) {
	_ = os.Remove(p)
	return net.Listen("unix", p)
}
