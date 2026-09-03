package space

import (
	"fmt"
	"os"

	"github.com/agent2host/agent2host/internal/source/path"
)

const (
	maxInclusionBytes = 64 << 20
	maxMemberBytes    = 16 << 20
)

func checkInclusionSize(payload map[string][]byte) error {
	var total int64
	for path, body := range payload {
		n := int64(len(body))
		if err := checkMemberBytes(path, n); err != nil {
			return err
		}
		total += n
	}
	if total > maxInclusionBytes {
		return fail(KindTooLarge, "the Agent System is larger than 64 MiB")
	}
	return nil
}

func checkMemberBytes(name string, n int64) error {
	if n > maxMemberBytes {
		if name == "" {
			return fail(KindTooLarge, "a file is larger than 16 MiB")
		}
		return fail(KindTooLarge, fmt.Sprintf("%s is larger than 16 MiB", name))
	}
	return nil
}

// bindSizeGuard is a memory protection on each ReadFile. It is not the
// Artifact inclusion policy (SPACE-SIZE-*); that is checkInclusionSize.
func bindSizeGuard(fs *path.FS) {
	if fs == nil {
		return
	}
	origRead := fs.ReadFile
	origLstat := fs.Lstat
	if origRead == nil {
		origRead = os.ReadFile
	}
	if origLstat == nil {
		origLstat = os.Lstat
	}
	fs.ReadFile = func(name string) ([]byte, error) {
		st, err := origLstat(name)
		if err != nil {
			return nil, err
		}
		if st.Mode().IsRegular() {
			if err := checkMemberBytes(name, st.Size()); err != nil {
				return nil, err
			}
		}
		return origRead(name)
	}
}
