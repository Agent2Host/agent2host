package kiro

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/space"
)

// Kiro Adapter dictionary (not Source). One isolation root per run:
//
//	$A2H_WORKSPACE/kiro-home/   == KIRO_HOME
//	  agents/<id>.json
//	  skills/<id>/SKILL.md
//	  settings/cli.json
//
// SOP body is embedded in the agent prompt. Contexts stay under the workspace
// root and are referenced via file://$A2H_WORKSPACE/…. Local Kiro CLI has
// no OS sandbox — Assess stays unsupported/none; required:true refuses.
//
// Auth: login is not under KIRO_HOME (Application Support sqlite / IdP).
// Isolated KIRO_HOME still whoami-logged-in. Runtime does not copy Kiro auth.
const (
	HomeDirRel   = "kiro-home"
	AgentsDirRel = HomeDirRel + "/agents"
	SkillsDirRel = HomeDirRel + "/skills"
	SettingsRel  = HomeDirRel + "/settings/cli.json"
	SOPRel       = HomeDirRel + "/projection/sop.md"
	EnvHome      = "KIRO_HOME"
)

func kiroAgentJSON(run *space.ResolvedAgentRun) []byte {
	if run == nil {
		return []byte("{}\n")
	}
	doc := struct {
		Name           string `json:"name"`
		Description    string `json:"description,omitempty"`
		Prompt         string `json:"prompt,omitempty"`
		WelcomeMessage string `json:"welcomeMessage,omitempty"`
	}{
		Name:           run.AgentID,
		Description:    run.Description,
		WelcomeMessage: run.Description,
	}
	if body := adapter.CopyContent(run, run.SOP); len(body) > 0 {
		doc.Prompt = strings.TrimRight(string(body), "\n")
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return []byte("{}\n")
	}
	return append(b, '\n')
}

func kiroIntent(run *space.ResolvedAgentRun, probe adapter.ProbeResult) adapter.ControlIntent {
	if run == nil || !probe.Found {
		return adapter.ControlIntent{}
	}
	agentRel := AgentsDirRel + "/" + run.AgentID + ".json"
	var c adapter.ControlIntent
	c.Controls = []adapter.PlannedControl{
		{Kind: adapter.ControlPermissions, Via: adapter.ViaProjectionFile, Rel: agentRel, Derivation: "kiro: agent.json tools list (FS-scoped allow/deny)"},
		{Kind: adapter.ControlApprovals, Via: adapter.ViaProjectionFile, Rel: agentRel, Derivation: "kiro: agent.json tools list (implicit on_boundary)"},
		{Kind: adapter.ControlIsolation, Via: adapter.ViaNamedConfig, Rel: EnvHome, Derivation: "kiro: KIRO_HOME run-private kiro-home"},
	}
	adapter.AppendEnvironmentSecrets(&c, run)
	for _, sid := range adapter.SortedMCPServerIDs(run) {
		c.Controls = append(c.Controls, adapter.PlannedControl{
			Kind: adapter.ControlMCP, ID: sid, Via: adapter.ViaProjectionFile, Rel: agentRel,
			Derivation: "kiro: agent.json mcpServers + tools allowlist",
		})
	}
	adapter.AppendMCPSecrets(&c, run)
	if run.Hooks != nil {
		c.Controls = append(c.Controls, adapter.PlannedControl{
			Kind: adapter.ControlHook, Via: adapter.ViaProjectionFile, Rel: agentRel,
			Derivation: "kiro: agent.json hooks (approximate projection)",
		})
		adapter.AppendHookSecrets(&c, run)
	}
	return c
}

// kiroAssessSOP records documented SOP mapping for Kiro. Probe.Found proves the
// binary exists, not turn-one SOP binding — do not set applies_from_turn_one.
func kiroAssessSOP(a compatibility.Assessment, probe adapter.ProbeResult) compatibility.Assessment {
	if !probe.Found {
		return a
	}
	if a.SOP == nil {
		a.SOP = &compatibility.SOPAssess{Support: "mapped", Scope: "agent"}
	}
	a.SOP.Support = "mapped"
	a.SOP.AppliesFromTurnOne = nil
	a.SOP.Confidence = "documented"
	return a
}

func projectKiro(run *space.ResolvedAgentRun, report compatibility.Report, files []adapter.ProjectionFile, lp adapter.LaunchPlan) ([]adapter.ProjectionFile, adapter.LaunchPlan) {
	// Projection dictionary for Kiro Custom Agent under kiro-home/ (KIRO_HOME).
	// Identity-perfect SOP is still approximate; Launch is allowed for functional use.
	for i, f := range files {
		if f.RelPath != SOPRel {
			continue
		}
		files[i].Content = kiroSOPBody(run, f.Content)
		break
	}
	agentRel := AgentsDirRel + "/" + run.AgentID + ".json"
	card, extras := kiroAgentDoc(run, report)
	files = append(files, extras...)
	replaced := false
	for i, f := range files {
		if f.RelPath == agentRel {
			files[i].Content = card
			replaced = true
			break
		}
	}
	if !replaced {
		files = append(files, adapter.ProjectionFile{
			RelPath: agentRel, Class: adapter.DestProjection, Content: card, FromContent: run.AgentSpec,
		})
	}
	settings, _ := json.MarshalIndent(map[string]any{
		// Feasibility: avoid ambient steering / default skills under isolated home.
		"chat.disableInheritingDefaultResources": true,
	}, "", "  ")
	settings = append(settings, '\n')
	files = append(files, adapter.ProjectionFile{
		RelPath: SettingsRel, Class: adapter.DestProjection, Content: settings, FromContent: "adapter:kiro-settings",
	})
	// Default launch is CLI 2.x agent discovery under KIRO_HOME.
	// Do NOT force --v3: V3 (early-access harness) does not load the same
	// agent JSON that `kiro-cli agent list` shows under KIRO_HOME on 2.20.1
	// ("agent not found, using default"). Probe records the installed binary
	// version; a later slice may emit a V3-native card and pass --v3 only when
	// that load path is verified for the probed build.
	lp.Args = []string{"--agent", run.AgentID}
	if lp.Env == nil {
		lp.Env = map[string]string{}
	}
	lp.Env[EnvHome] = adapter.WorkspaceToken + "/" + HomeDirRel
	return files, lp
}

func kiroSOPBody(run *space.ResolvedAgentRun, sop []byte) []byte {
	return adapter.WrapNamedAgentSOP(run, sop)
}

func kiroAgentDoc(run *space.ResolvedAgentRun, report compatibility.Report) ([]byte, []adapter.ProjectionFile) {
	var extras []adapter.ProjectionFile
	sopURI := "file://" + adapter.WorkspaceToken + "/" + SOPRel
	// kiro-cli 2.20.1 silently drops agents that include V3-only fields
	// `permissions` or `includePowers` (even false). Use V2-loadable JSON:
	// toolsSettings for shell/network controls; omit includePowers.
	// SOP: file:// prompt (docs: system-like) + resources entry (startup load;
	// handbook already proved resources work when inline prompt did not).
	doc := map[string]any{
		"name":           run.AgentID,
		"description":    run.Description,
		"welcomeMessage": run.Description,
		"includeMcpJson": false,
		"prompt":         sopURI,
		"tools":          kiroTools(run, report),
		"allowedTools":   kiroAllowedTools(run, report),
		"toolsSettings":  kiroToolsSettings(run),
		"resources":      append([]string{sopURI}, kiroResources(run, report)...),
	}
	servers, mcpFiles := kiroMCPServers(run, report)
	extras = append(extras, mcpFiles...)
	if len(servers) > 0 {
		doc["mcpServers"] = servers
	}
	hooks, hookFiles := kiroHooks(run, report)
	extras = append(extras, hookFiles...)
	if len(hooks) > 0 {
		doc["hooks"] = hooks
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return []byte("{}\n"), extras
	}
	return append(b, '\n'), extras
}

func kiroTools(run *space.ResolvedAgentRun, report compatibility.Report) []string {
	var tools []string
	if adapter.FSReadAllowed(run) {
		tools = append(tools, "read")
	}
	if adapter.FSWriteAllowed(run) {
		tools = append(tools, "write")
	}
	tools = append(tools, "shell")
	if !adapter.NetworkDenied(run) {
		tools = append(tools, "web")
	}
	for _, sid := range adapter.IncludedMCPServerIDs(run, report) {
		for _, tool := range run.MCPServers[sid].Tools {
			tools = append(tools, "@"+sid+"/"+tool.Name)
		}
	}
	return tools
}

func kiroResources(run *space.ResolvedAgentRun, report compatibility.Report) []string {
	var out []string
	for _, sk := range run.Skills {
		if !skillIncluded(report, sk.ID) {
			continue
		}
		out = append(out, "skill://"+sk.ID)
	}
	keep := adapter.IncludedContexts(report)
	evalCtx := len(report.Capabilities.Context.Items) > 0
	for _, ctx := range run.Contexts {
		if evalCtx && !keep[ctx.Path] {
			continue
		}
		out = append(out, "file://"+adapter.WorkspaceToken+"/"+ctx.Path)
	}
	return out
}

func skillIncluded(report compatibility.Report, id string) bool {
	for _, it := range report.Capabilities.Skills.Items {
		if it.ID == id && it.Disposition == "include" {
			return true
		}
	}
	// No skill rows → Project may still emit declared skills via adapter.ProjectSOPAndSkills.
	return len(report.Capabilities.Skills.Items) == 0
}

// kiroAllowedTools: tools that skip the approval prompt. Shell stays off this
// list when Source wants always/on_boundary (Host asks). MCP allowlisted tools
// are auto-approved so smoke is not blocked on every tool call.
func kiroAllowedTools(run *space.ResolvedAgentRun, report compatibility.Report) []string {
	var out []string
	shell := "on_boundary"
	if run.Approvals != nil && run.Approvals.ShellExecute != nil {
		shell = *run.Approvals.ShellExecute
	}
	if shell == "never" {
		out = append(out, "shell", "read", "write")
	} else {
		out = append(out, "read")
	}
	for _, sid := range adapter.IncludedMCPServerIDs(run, report) {
		for _, tool := range run.MCPServers[sid].Tools {
			out = append(out, "@"+sid+"/"+tool.Name)
		}
	}
	return out
}

// kiroToolsSettings maps network deny / shell floor onto CLI 2.x toolsSettings
// (loadable on 2.20.1). V3 `permissions.rules` is not used until that field
// stops making agents invisible to `kiro-cli agent list` / `--agent`.
func kiroToolsSettings(run *space.ResolvedAgentRun) map[string]any {
	bash := map[string]any{
		"denyByDefault": false,
	}
	if adapter.NetworkDenied(run) {
		bash["deniedCommands"] = []string{`^curl\b`, `^wget\b`, `^nc\b`, `^ncat\b`}
	}
	shell := "on_boundary"
	if run.Approvals != nil && run.Approvals.ShellExecute != nil {
		shell = *run.Approvals.ShellExecute
	}
	if shell == "always" {
		bash["denyByDefault"] = true
		bash["allowedCommands"] = []string{}
	}
	out := map[string]any{"execute_bash": bash, "shell": bash}
	if adapter.NetworkDenied(run) {
		out["web_fetch"] = map[string]any{"allowedUrls": []string{}, "deniedUrls": []string{".*"}}
		out["web_search"] = map[string]any{"allowedUrls": []string{}, "deniedUrls": []string{".*"}}
	}
	return out
}

func kiroMCPServers(run *space.ResolvedAgentRun, report compatibility.Report) (map[string]any, []adapter.ProjectionFile) {
	servers := map[string]any{}
	var extras []adapter.ProjectionFile
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
	return servers, extras
}

func kiroHooks(run *space.ResolvedAgentRun, report compatibility.Report) (map[string]any, []adapter.ProjectionFile) {
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
			hooks = append(hooks, entry)
			for _, f := range adapter.DerefFiles(h.Files) {
				extras = append(extras, adapter.File(f, run, f, nil))
			}
		}
		if len(hooks) > 0 {
			out[hostEvent] = hooks
		}
	}
	// kiro-cli 2.20.1 embeds hooks in the agent JSON with camelCase triggers
	// (agentSpawn / preToolUse / postToolUse / stop). PascalCase names
	// (AgentSpawn, …) invalidate the entire agent file → "agent not found".
	add("agentSpawn", "/hooks/session_start", run.Hooks.SessionStart)
	add("preToolUse", "/hooks/before_tool_call", run.Hooks.BeforeToolCall)
	add("postToolUse", "/hooks/after_tool_call", run.Hooks.AfterToolCall)
	add("stop", "/hooks/agent_stop", run.Hooks.AgentStop)
	if len(out) == 0 {
		return nil, extras
	}
	return out, extras
}
