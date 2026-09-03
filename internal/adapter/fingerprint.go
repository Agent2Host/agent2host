package adapter

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Fingerprint hashes Adapter-declared Assessment-affecting Probe fields.
// observed_at must not be included. Format matches Artifact revisions.
func Fingerprint(fields ...string) string {
	h := sha256.New()
	for _, f := range fields {
		_, _ = h.Write([]byte(f))
		_, _ = h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func normalizeExe(p string) string {
	return strings.TrimSpace(p)
}
