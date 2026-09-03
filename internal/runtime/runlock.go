package runtime

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

const liveLockName = "live.lock"

func liveLockPath(runRoot string) string {
	return filepath.Join(runRoot, liveLockName)
}

func writeLiveLock(p Prepared) error {
	if err := os.MkdirAll(p.Root, runtimeDirMode); err != nil {
		return err
	}
	return os.WriteFile(liveLockPath(p.Root), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600)
}

func clearLiveLock(p Prepared) {
	_ = os.Remove(liveLockPath(p.Root))
}

// runIsLive reports whether this run directory still has a living Agent2Host
// Execute process. Crash leftovers (kill -9) have a lock whose pid is dead.
func runIsLive(runRoot string) bool {
	body, err := os.ReadFile(liveLockPath(runRoot))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(string(bytes.TrimSpace(body)))
	if err != nil || pid <= 0 {
		return false
	}
	return pidAlive(pid)
}

func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return err == syscall.EPERM
}
