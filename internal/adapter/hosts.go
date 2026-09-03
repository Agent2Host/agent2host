package adapter

// Committed Host ids (roadmap §2).
const (
	HostClaudeCode = "claude-code"
	HostKiro       = "kiro"
	HostCodex      = "codex"
)

// SupportedHost reports whether id is a committed Host.
func SupportedHost(id string) bool {
	switch id {
	case HostClaudeCode, HostKiro, HostCodex:
		return true
	default:
		return false
	}
}
