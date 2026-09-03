package adapter

import (
	"sort"

	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/space"
)

func SortedMCPServerIDs(run *space.ResolvedAgentRun) []string {
	if run == nil || len(run.MCPServers) == 0 {
		return nil
	}
	ids := make([]string, 0, len(run.MCPServers))
	for id := range run.MCPServers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func IncludedMCPServerIDs(run *space.ResolvedAgentRun, report compatibility.Report) []string {
	keep := IncludedMCPServers(report)
	evalMCP := len(report.Capabilities.MCP.Items) > 0
	var ids []string
	for _, sid := range SortedMCPServerIDs(run) {
		if evalMCP && !keep[sid] {
			continue
		}
		ids = append(ids, sid)
	}
	return ids
}

// fsScopeAllowsWorkingDir reports whether a filesystem scope list includes
// working_directory. Nil (omitted) matches SRC-SEC-OMIT baseline → allowed.
func fsScopeAllowsWorkingDir(scopes *[]string) bool {
	if scopes == nil {
		return true
	}
	for _, s := range *scopes {
		if s == "working_directory" {
			return true
		}
	}
	return false
}

func FSReadAllowed(run *space.ResolvedAgentRun) bool {
	if run == nil || run.Permissions == nil || run.Permissions.Filesystem == nil {
		return true
	}
	return fsScopeAllowsWorkingDir(run.Permissions.Filesystem.Read)
}

func FSWriteAllowed(run *space.ResolvedAgentRun) bool {
	if run == nil || run.Permissions == nil || run.Permissions.Filesystem == nil {
		return true
	}
	return fsScopeAllowsWorkingDir(run.Permissions.Filesystem.Write)
}

func NetworkDenied(run *space.ResolvedAgentRun) bool {
	if run == nil || run.Permissions == nil || run.Permissions.Network == nil || run.Permissions.Network.Default == nil {
		return true
	}
	return *run.Permissions.Network.Default == "deny"
}

// FSCeilingWorkingDirectoryOnly is the shared declared FS ceiling: every
// listed read/write scope must be working_directory. One copy — do not
// reimplement per Host.
func FSCeilingWorkingDirectoryOnly(run *space.ResolvedAgentRun) bool {
	if run == nil || run.Permissions == nil || run.Permissions.Filesystem == nil {
		return true
	}
	fs := run.Permissions.Filesystem
	for _, scopes := range []*[]string{fs.Read, fs.Write} {
		if scopes == nil {
			continue
		}
		for _, s := range *scopes {
			if s != "working_directory" {
				return false
			}
		}
	}
	return true
}
