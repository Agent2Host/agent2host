package adapter

import (
	"strings"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/space"
)

// BindWorkspaceLocal rewrites a Source-local path to a run-workspace token path.
// Only tokens that match a packed Source file (after canonicalize) are rewritten.
// PATH commands (e.g. "python3") and opaque args are left unchanged.
// Host process cwd stays ApprovedWorkingDirectory; these paths must not depend on it.
func BindWorkspaceLocal(token string, packedFiles []string) string {
	if token == "" {
		return token
	}
	for _, f := range packedFiles {
		if f == "" {
			continue
		}
		if token == f || token == "./"+f {
			return WorkspaceToken + "/" + f
		}
	}
	return token
}

// BindWorkspaceArgv rewrites command + args that are packed local files.
func BindWorkspaceArgv(command string, args, packedFiles []string) (string, []string) {
	cmd := BindWorkspaceLocal(command, packedFiles)
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = BindWorkspaceLocal(a, packedFiles)
	}
	return cmd, out
}

// SystemLocalCommands returns projected rel paths used directly as a
// ProcessSpec command (MCP server or Hook). PATH commands such as "python3"
// match no packed member and are excluded. Runtime marks these executable.
func SystemLocalCommands(run *space.ResolvedAgentRun) map[string]bool {
	out := map[string]bool{}
	if run == nil {
		return out
	}
	mark := func(command string, files []string) {
		if command == "" {
			return
		}
		for _, f := range PackedFilesFromMCP(files) {
			if command == f || command == "./"+f {
				out[f] = true
			}
		}
	}
	for _, srv := range run.MCPServers {
		mark(srv.Command, srv.Files)
	}
	if run.Hooks != nil {
		for _, list := range []*[]decode.HookEntry{
			run.Hooks.SessionStart, run.Hooks.BeforeToolCall,
			run.Hooks.AfterToolCall, run.Hooks.AgentStop,
		} {
			if list == nil {
				continue
			}
			for _, h := range *list {
				if h.Files == nil {
					continue
				}
				mark(h.Command, *h.Files)
			}
		}
	}
	return out
}

// MarkExecutableFiles sets Executable on projected members that are commands.
func MarkExecutableFiles(files []ProjectionFile, run *space.ResolvedAgentRun) []ProjectionFile {
	cmds := SystemLocalCommands(run)
	if len(cmds) == 0 {
		return files
	}
	for i, f := range files {
		if cmds[strings.TrimPrefix(f.RelPath, "./")] {
			files[i].Executable = true
		}
	}
	return files
}

// PackedFilesFromMCP is the projected file list for one MCP server.
func PackedFilesFromMCP(files []string) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		f = strings.TrimPrefix(f, "./")
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}
