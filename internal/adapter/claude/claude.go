package claude

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/space"
)

// Claude Adapter dictionary (not Source). Isolation roots:
//
//	$A2H_AUTH_PROFILE/          == CLAUDE_CONFIG_DIR (stable Auth Profile)
//	  agents/<id>.md
//	  skills/<id>/SKILL.md
//	  Host-owned login (Keychain keyed to this dir; or Host-written files)
//
//	$A2H_PRIVATE/host-config/   official overlays only
//	  settings.json / mcp.json       (--settings, --mcp-config)
//
// SOP (CLAUDE.md) and Context files stay under the workspace root and are
// reached via --add-dir. Do not emit project-scoped .claude/agents or
// .claude/skills. Do not write ~/.claude or copy Keychain.
const (
	HomeDirRel   = "host-config"
	SettingsRel  = HomeDirRel + "/settings.json"
	MCPRel       = HomeDirRel + "/mcp.json"
	AgentsDirRel = "agents"
	SkillsDirRel = "skills"
	EnvConfig    = "CLAUDE_CONFIG_DIR"
)

func claudeIntent(run *space.ResolvedAgentRun, probe adapter.ProbeResult) adapter.ControlIntent {
	if run == nil || !probe.Found {
		return adapter.ControlIntent{}
	}
	var c adapter.ControlIntent
	c.Controls = []adapter.PlannedControl{
		{Kind: adapter.ControlSandbox, Via: adapter.ViaProjectionFile, Rel: SettingsRel, Derivation: "claude: settings.json sandbox.enabled"},
		{Kind: adapter.ControlPermissions, Via: adapter.ViaProjectionFile, Rel: SettingsRel, Derivation: "claude: settings.json permissions deny/ask/allow"},
		{Kind: adapter.ControlApprovals, Via: adapter.ViaProjectionFile, Rel: SettingsRel, Derivation: "claude: settings.json permissions ask/deny"},
		{Kind: adapter.ControlIsolation, Via: adapter.ViaNamedConfig, Rel: EnvConfig, Derivation: "claude: CLAUDE_CONFIG_DIR $A2H_AUTH_PROFILE"},
	}
	adapter.AppendEnvironmentSecrets(&c, run)
	for _, sid := range adapter.SortedMCPServerIDs(run) {
		c.Controls = append(c.Controls, adapter.PlannedControl{
			Kind: adapter.ControlMCP, ID: sid, Via: adapter.ViaProjectionFile, Rel: MCPRel,
			Derivation: "claude: mcp.json server env + tool allowlist",
		})
	}
	adapter.AppendMCPSecrets(&c, run)
	if run.Hooks != nil {
		addHook := func(prefix string, entries *[]decode.HookEntry) {
			if entries == nil || len(*entries) == 0 {
				return
			}
			c.Controls = append(c.Controls, adapter.PlannedControl{
				Kind: adapter.ControlHook, ID: prefix, Via: adapter.ViaProjectionFile, Rel: SettingsRel,
				Derivation: "claude: settings.json hooks",
			})
		}
		addHook("/hooks/session_start", run.Hooks.SessionStart)
		addHook("/hooks/before_tool_call", run.Hooks.BeforeToolCall)
		addHook("/hooks/after_tool_call", run.Hooks.AfterToolCall)
		addHook("/hooks/agent_stop", run.Hooks.AgentStop)
		adapter.AppendHookSecrets(&c, run)
	}
	return c
}

func projectClaude(run *space.ResolvedAgentRun, report compatibility.Report, files []adapter.ProjectionFile, lp adapter.LaunchPlan, pctx adapter.ProjectionContext) ([]adapter.ProjectionFile, adapter.LaunchPlan) {
	settings, mcpJSON, extra := claudeHostFiles(run, report, pctx)
	files = append(files, extra...)
	// Replace the sole agent card (under host-config/agents/) with tools allowlist.
	prefix := AgentsDirRel + "/"
	for i, f := range files {
		if strings.HasPrefix(f.RelPath, prefix) && strings.HasSuffix(f.RelPath, ".md") {
			files[i].Content = claudeAgentCard(run, report)
			break
		}
	}
	files = append(files, adapter.ProjectionFile{
		RelPath: SettingsRel, Class: adapter.DestHostPrivate, Content: settings, FromContent: "adapter:claude-settings",
	})
	if len(mcpJSON) == 0 && len(adapter.IncludedMCPServers(report)) > 0 {
		mcpJSON = []byte("{\n  \"mcpServers\": {}\n}\n")
	}
	if len(mcpJSON) > 0 {
		files = append(files, adapter.ProjectionFile{
			RelPath: MCPRel, Class: adapter.DestHostPrivate, Content: mcpJSON, FromContent: "adapter:claude-mcp",
		})
	}
	overlayDir := adapter.PrivateToken + "/" + HomeDirRel
	settingsPath := overlayDir + "/settings.json"
	args := []string{
		"--agent", run.AgentID,
		"--settings", settingsPath,
		"--setting-sources", "user",
		"--add-dir", adapter.WorkspaceToken,
	}
	if tools := claudeAgentTools(run, report); len(tools) > 0 {
		args = append(args, "--allowedTools", strings.Join(tools, ","))
	}
	if denied := claudeDisallowedTools(run); denied != "" {
		args = append(args, "--disallowedTools", denied)
	}
	if len(mcpJSON) > 0 {
		args = append(args, "--strict-mcp-config", "--mcp-config", overlayDir+"/mcp.json")
	}
	lp.Args = args
	if lp.Env == nil {
		lp.Env = map[string]string{}
	}
	lp.Env[EnvConfig] = adapter.AuthProfileToken
	return files, lp
}

func claudeHostFiles(run *space.ResolvedAgentRun, report compatibility.Report, pctx adapter.ProjectionContext) (settings, mcpJSON []byte, extras []adapter.ProjectionFile) {
	deny, ask, allow := claudePermissionRules(run, report, pctx)
	sandbox := map[string]any{
		"enabled":                  true,
		"failIfUnavailable":        true,
		"allowUnsandboxedCommands": false,
	}
	// Claude Code default autoAllowBashIfSandboxed=true skips bare ask:["Bash"]
	// for sandboxed commands (sandbox substitutes for the whole-tool prompt).
	// When Source requires a Shell ask floor, disable that substitution so the
	// projected ask rule is effective. See code.claude.com/docs/en/permissions.
	if claudeShellAskFloor(run) {
		sandbox["autoAllowBashIfSandboxed"] = false
	}
	// OS-level Bash-subprocess read fence. Independent of permissions.deny
	// (Read/Edit/Grep run in the Host process and bypass this layer). Known
	// denyRead bugs exist; this is partial protection, not a workspace whitelist.
	if adapter.ClaudeShouldDenyHomeReads(pctx) {
		sandbox["filesystem"] = map[string]any{
			"denyRead":  []string{"~/"},
			"allowRead": []string{"."},
		}
	}
	set := map[string]any{
		"sandbox": sandbox,
		"permissions": map[string]any{
			"defaultMode": "default",
			"deny":        deny,
			"ask":         ask,
			"allow":       allow,
		},
	}
	hooks, hookFiles := claudeHooks(run, report)
	if len(hooks) > 0 {
		set["hooks"] = hooks
	}
	extras = append(extras, hookFiles...)
	settings, _ = json.MarshalIndent(set, "", "  ")
	settings = append(settings, '\n')

	servers := map[string]any{}
	for _, sid := range adapter.IncludedMCPServerIDs(run, report) {
		srv := run.MCPServers[sid]
		packed := adapter.PackedFilesFromMCP(srv.Files)
		cmd, args := adapter.BindWorkspaceArgv(srv.Command, srv.Args, packed)
		entry := map[string]any{"command": cmd}
		if len(args) > 0 {
			entry["args"] = args
		}
		if len(srv.Environment) > 0 {
			env := map[string]string{}
			for _, b := range srv.Environment {
				env[b.ValueFrom.Environment] = adapter.SecretPlaceholder(b.ValueFrom.Environment)
			}
			entry["env"] = env
		}
		servers[sid] = entry
		for _, f := range srv.Files {
			extras = append(extras, adapter.File(f, run, f, nil))
		}
	}
	if len(servers) > 0 || len(run.MCPServers) > 0 {
		mcpJSON, _ = json.MarshalIndent(map[string]any{"mcpServers": servers}, "", "  ")
		mcpJSON = append(mcpJSON, '\n')
	}
	return settings, mcpJSON, extras
}

func claudeShellApproval(run *space.ResolvedAgentRun) string {
	if run != nil && run.Approvals != nil && run.Approvals.ShellExecute != nil {
		return *run.Approvals.ShellExecute
	}
	return "on_boundary"
}

// claudeShellAskFloor is true when Source wants a Host Shell approval gate
// (always or on_boundary). never → false.
func claudeShellAskFloor(run *space.ResolvedAgentRun) bool {
	switch claudeShellApproval(run) {
	case "always", "on_boundary":
		return true
	default:
		return false
	}
}

func claudePermissionRules(run *space.ResolvedAgentRun, report compatibility.Report, pctx adapter.ProjectionContext) (deny, ask, allow []string) {
	deny = []string{}
	ask = []string{}
	allow = []string{}
	// Network deny is approximate: WebFetch/WebSearch + common curl/wget/nc Bash
	// shapes. Other shell/MCP/browser exits may remain open — adapter.Assess must not
	// overclaim complete isolation.
	if adapter.NetworkDenied(run) {
		deny = append(deny,
			"WebFetch", "WebSearch",
			"Bash(curl *)", "Bash(wget *)", "Bash(nc *)", "Bash(ncat *)",
		)
	}
	switch claudeShellApproval(run) {
	case "always", "on_boundary":
		ask = append(ask, "Bash")
	case "never":
		allow = append(allow, "Bash")
	}
	// Filesystem → Claude Read/Write/Edit/Glob/Grep. V0 has no FS approval field;
	// in-scope working_directory maps to Host allow; empty scope → deny.
	if adapter.FSReadAllowed(run) {
		allow = append(allow, "Read", "Glob", "Grep")
	} else {
		deny = append(deny, "Read", "Glob", "Grep")
	}
	if adapter.FSWriteAllowed(run) {
		allow = append(allow, "Write", "Edit")
	} else {
		deny = append(deny, "Write", "Edit")
	}
	// Allow only declared MCP tools. Do not deny mcp__<id>__*: Host deny
	// wins across scopes and would hide the allowlisted tools too.
	for _, sid := range adapter.IncludedMCPServerIDs(run, report) {
		for _, tool := range run.MCPServers[sid].Tools {
			allow = append(allow, "mcp__"+sid+"__"+tool.Name)
		}
	}
	if adapter.ClaudeShouldDenyHomeReads(pctx) {
		deny = append(deny, adapter.ClaudeHomeReadDenyRule)
	}
	return deny, ask, allow
}

func claudeDisallowedTools(run *space.ResolvedAgentRun) string {
	if !adapter.NetworkDenied(run) {
		return ""
	}
	return "WebFetch,WebSearch"
}

func claudeAgentCard(run *space.ResolvedAgentRun, report compatibility.Report) []byte {
	var b strings.Builder
	b.WriteString("---\nname: ")
	b.WriteString(adapter.YAMLScalar(run.AgentID))
	b.WriteString("\ndescription: ")
	b.WriteString(adapter.YAMLScalar(run.Description))
	b.WriteString("\ntools:\n")
	for _, t := range claudeAgentTools(run, report) {
		b.WriteString("  - ")
		b.WriteString(adapter.YAMLScalar(t))
		b.WriteByte('\n')
	}
	b.WriteString("---\n\n# ")
	b.WriteString(adapter.DisplayName(run))
	b.WriteByte('\n')
	if body := adapter.CopyContent(run, run.SOP); len(body) > 0 {
		b.WriteByte('\n')
		b.Write(body)
		if body[len(body)-1] != '\n' {
			b.WriteByte('\n')
		}
	} else if run.Description != "" {
		b.WriteString("\n")
		b.WriteString(run.Description)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func claudeAgentTools(run *space.ResolvedAgentRun, report compatibility.Report) []string {
	// Built-ins the Agent may need; omit FS tools outside declared scope;
	// network tools omitted when Source denies.
	tools := []string{"Bash", "Skill"}
	if adapter.FSReadAllowed(run) {
		tools = append(tools, "Read", "Glob", "Grep")
	}
	if adapter.FSWriteAllowed(run) {
		tools = append(tools, "Write", "Edit")
	}
	if !adapter.NetworkDenied(run) {
		tools = append(tools, "WebFetch", "WebSearch")
	}
	for _, sid := range adapter.IncludedMCPServerIDs(run, report) {
		for _, tool := range run.MCPServers[sid].Tools {
			tools = append(tools, "mcp__"+sid+"__"+tool.Name)
		}
	}
	return tools
}

func claudeHooks(run *space.ResolvedAgentRun, report compatibility.Report) (map[string]any, []adapter.ProjectionFile) {
	if run.Hooks == nil {
		return nil, nil
	}
	keep := adapter.IncludedHooks(report)
	evalHooks := len(report.Capabilities.Hooks.Items) > 0
	out := map[string]any{}
	var extras []adapter.ProjectionFile
	add := func(hostEvent, prefix string, entries *[]decode.HookEntry) {
		if entries == nil {
			return
		}
		var hooks []any
		for i, h := range *entries {
			ref := fmt.Sprintf("%s/%d", prefix, i)
			if evalHooks && !keep[ref] {
				continue
			}
			packed := adapter.PackedFilesFromMCP(adapter.DerefFiles(h.Files))
			cmd, args := adapter.BindWorkspaceArgv(h.Command, adapter.DerefArgs(h.Args), packed)
			argv := []string{cmd}
			argv = append(argv, args...)
			entry := map[string]any{"type": "command", "command": adapter.ShellJoin(argv)}
			if h.Environment != nil {
				env := map[string]string{}
				for _, b := range *h.Environment {
					env[b.ValueFrom.Environment] = adapter.SecretPlaceholder(b.ValueFrom.Environment)
				}
				if len(env) > 0 {
					entry["env"] = env
				}
			}
			hooks = append(hooks, map[string]any{"hooks": []any{entry}})
			for _, f := range adapter.DerefFiles(h.Files) {
				extras = append(extras, adapter.File(f, run, f, nil))
			}
		}
		if len(hooks) > 0 {
			out[hostEvent] = hooks
		}
	}
	add("SessionStart", "/hooks/session_start", run.Hooks.SessionStart)
	add("PreToolUse", "/hooks/before_tool_call", run.Hooks.BeforeToolCall)
	add("PostToolUse", "/hooks/after_tool_call", run.Hooks.AfterToolCall)
	add("Stop", "/hooks/agent_stop", run.Hooks.AgentStop)
	if len(out) == 0 {
		return nil, extras
	}
	return out, extras
}
