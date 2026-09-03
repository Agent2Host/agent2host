package adapter

import (
	"strings"
)

// verifiedHostVersions is the run-admission prefix set per committed Host.
var verifiedHostVersions = map[string][]string{
	HostClaudeCode: {"2.1."},
	HostKiro:       {"2.20.", "2.21."},
	HostCodex:      {"0.149."},
}

// HostVersionVerified reports whether a probed version string matches a verified prefix.
// Versions ending in "-test" are accepted for unit tests. "unknown" and missing Host
// entries are not verified.
func HostVersionVerified(hostID, version string) bool {
	v := normalizeHostVersion(version)
	if v == "" || v == "unknown" {
		return false
	}
	if strings.HasSuffix(v, "-test") {
		return true
	}
	prefixes, ok := verifiedHostVersions[hostID]
	if !ok {
		return false
	}
	for _, p := range prefixes {
		if strings.HasPrefix(v, p) {
			return true
		}
	}
	return false
}

func normalizeHostVersion(version string) string {
	v := strings.TrimSpace(version)
	if v == "" {
		return ""
	}
	if i := strings.IndexByte(v, '('); i > 0 {
		v = strings.TrimSpace(v[:i])
	}
	parts := strings.Fields(v)
	for i := len(parts) - 1; i >= 0; i-- {
		if len(parts[i]) > 0 && parts[i][0] >= '0' && parts[i][0] <= '9' {
			return parts[i]
		}
	}
	return v
}
