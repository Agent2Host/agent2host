//go:build windows

package semantic_test

import (
	"errors"
	"net"
)

func mkfifo(string) error {
	return errors.New("fifo not supported on windows")
}

func listenUnix(string) (net.Listener, error) {
	return nil, errors.New("unix socket not supported on windows")
}
