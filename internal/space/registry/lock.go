package registry

import (
	"os"
	"path/filepath"
)

type fileLock struct {
	f *os.File
}

func acquire(path string) (*fileLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := lockExclusive(f); err != nil {
		_ = f.Close()
		return nil, fail(KindBusy, path)
	}
	return &fileLock{f: f}, nil
}

func (l *fileLock) Unlock() error {
	err := unlockFile(l.f)
	closeErr := l.f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
