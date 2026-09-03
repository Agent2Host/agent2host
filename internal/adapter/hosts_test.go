package adapter_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/claude"
	"github.com/agent2host/agent2host/internal/adapter/codex"
	"github.com/agent2host/agent2host/internal/adapter/committed"
	"github.com/agent2host/agent2host/internal/adapter/kiro"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/space"
)

func digestN(n byte) string {
	return "sha256:" + strings.Repeat("0", 63) + string('0'+n)
}

func foundLook() adapter.LookPathFunc {
	return func(file string) (string, error) { return "/opt/" + file, nil }
}

func missingLook() adapter.LookPathFunc {
	return func(string) (string, error) { return "", errors.New("not found") }
}

func stubVersion(string) (string, error) { return "1.0.0-test", nil }

func sampleRun(sandboxRequired bool, extraSkill bool) *space.ResolvedAgentRun {
	sr := sandboxRequired
	run := &space.ResolvedAgentRun{
		SystemID:         "club-system",
		AgentID:          "club-faq",
		Name:             "Club FAQ",
		Description:      "I can help with club events, registration, and policies.",
		ArtifactRevision: digestN(1),
		SOP:              "sops/a.sop.md",
		Skills: []space.ResolvedSkill{
			{ID: "search-policy", Required: true, Name: "Search", Description: "d", Document: "skills/s.skill.md"},
		},
		Content: map[string][]byte{
			"sops/a.sop.md":     []byte("# sop\n"),
			"skills/s.skill.md": []byte("# skill\n"),
		},
		Sandbox: &decode.Sandbox{Required: &sr},
	}
	if extraSkill {
		run.Skills = append(run.Skills, space.ResolvedSkill{
			ID: "pretty", Required: false, Name: "Pretty", Description: "p", Document: "skills/p.skill.md",
		})
		run.Content["skills/p.skill.md"] = []byte("# pretty\n")
	}
	return run
}

func TestSelectUnknownHost(t *testing.T) {
	_, err := committed.Default().Select("not-a-host")
	if !errors.Is(err, adapter.ErrUnsupportedHost) {
		t.Fatalf("got %v", err)
	}
}

func TestDefaultRegistryIDs(t *testing.T) {
	got := committed.Default().HostIDs()
	want := []string{adapter.HostClaudeCode, adapter.HostCodex, adapter.HostKiro}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestClaudeCodeAllowedProjects(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	run := sampleRun(true, false)
	out, err := adapter.RunPipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision == "refused" {
		t.Fatalf("claude-code refused: %+v", out.Report.Activation)
	}
	if out.Report.Activation.Mode != "primary_native" {
		t.Fatalf("activation %s", out.Report.Activation.Mode)
	}
	if out.Plans == nil {
		t.Fatal("expected plans")
	}
	if out.Plans.Launch.Executable != "/opt/claude" {
		t.Fatalf("exe %s", out.Plans.Launch.Executable)
	}
	rels := map[string]bool{}
	for _, f := range out.Plans.Projection.Files {
		rels[f.RelPath] = true
		switch {
		case strings.HasPrefix(f.RelPath, claude.HomeDirRel+"/"):
			if f.Class != adapter.DestHostPrivate {
				t.Fatalf("%s class %s want host_private", f.RelPath, f.Class)
			}
		case strings.HasPrefix(f.RelPath, claude.SkillsDirRel+"/") || strings.HasPrefix(f.RelPath, claude.AgentsDirRel+"/"):
			if f.Class != adapter.DestAuthProfile {
				t.Fatalf("%s class %s want auth_profile", f.RelPath, f.Class)
			}
		default:
			if f.Class != adapter.DestProjection {
				t.Fatalf("class %s %s", f.RelPath, f.Class)
			}
		}
	}
	agentCardRel := claude.AgentsDirRel + "/club-faq.md"
	if !rels["CLAUDE.md"] || !rels[claude.SkillsDirRel+"/search-policy/SKILL.md"] || !rels[agentCardRel] {
		t.Fatalf("files %v", rels)
	}
	if rels[".claude/agents/club-faq.md"] {
		t.Fatal("must not emit project-scoped .claude/agents")
	}
	if rels[".claude/skills/search-policy/SKILL.md"] {
		t.Fatal("must not emit project-scoped .claude/skills")
	}
	overlayDir := adapter.PrivateToken + "/" + claude.HomeDirRel
	args := strings.Join(out.Plans.Launch.Args, " ")
	if !strings.Contains(args, "--agent club-faq") ||
		!strings.Contains(args, "--settings "+overlayDir+"/settings.json") ||
		!strings.Contains(args, "--setting-sources user") ||
		!strings.Contains(args, "--add-dir "+adapter.WorkspaceToken) ||
		!strings.Contains(args, "--disallowedTools WebFetch,WebSearch") {
		t.Fatalf("launch args %v", out.Plans.Launch.Args)
	}
	if out.Plans.Launch.WorkingDirClass != adapter.DestWorkingDir {
		t.Fatalf("cwd class %s", out.Plans.Launch.WorkingDirClass)
	}
	if out.Plans.Launch.Env[claude.EnvConfig] != adapter.AuthProfileToken {
		t.Fatalf("CLAUDE_CONFIG_DIR must be the stable Auth Profile, got %v", out.Plans.Launch.Env)
	}
	body := fileBody(out.Plans.Projection.Files, claude.SettingsRel)
	if !strings.Contains(body, `"enabled": true`) || !strings.Contains(body, `"failIfUnavailable": true`) {
		t.Fatalf("settings:\n%s", body)
	}
	if !strings.Contains(body, `"autoAllowBashIfSandboxed": false`) {
		t.Fatalf("sandboxed Bash must not auto-skip ask:\n%s", body)
	}
	if !strings.Contains(body, `"WebFetch"`) || !strings.Contains(body, `"Bash"`) {
		t.Fatalf("permission rules missing:\n%s", body)
	}
	if !strings.Contains(body, `"Read"`) || !strings.Contains(body, `"Write"`) {
		t.Fatalf("baseline FS allow missing:\n%s", body)
	}
	if strings.Contains(body, `Read(./host-config/**)`) || strings.Contains(body, `Read(host-config/**)`) {
		t.Fatalf("workspace host-config deny is obsolete:\n%s", body)
	}
	card := fileBody(out.Plans.Projection.Files, agentCardRel)
	if !strings.Contains(card, "tools:") || strings.Contains(card, "WebFetch") {
		t.Fatalf("agent tools:\n%s", card)
	}
	if out.Report.Security.Sandbox.Enforcement != "host_enforced" ||
		out.Report.Security.Permissions.Enforcement != "host_enforced" {
		t.Fatalf("security %+v / %+v", out.Report.Security.Sandbox, out.Report.Security.Permissions)
	}
	assertCard(t, out.Plans.Projection.Files, agentCardRel, "club-faq", run.Description)
	assertSkillFrontmatter(t, out.Plans.Projection.Files, claude.SkillsDirRel+"/search-policy/SKILL.md", "search-policy", "d")
}

func TestClaudeAuthProfileIsNotPerRunConfigDir(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	a, err := reg.Select(adapter.HostClaudeCode)
	if err != nil {
		t.Fatal(err)
	}
	d := a.HostState().DescribeAuth()
	if err := adapter.ValidateAuthDescription(d); err != nil {
		t.Fatal(err)
	}
	if d.Topology != adapter.AuthTopologyBoundRoot {
		t.Fatalf("topology %s", d.Topology)
	}
	if d.Profile.NativeAuthNamespace == "" || d.Profile.Provider == "" {
		t.Fatalf("profile must name provider and native namespace: %+v", d.Profile)
	}
	out, err := adapter.RunPipeline(reg, adapter.HostClaudeCode, sampleRun(true, false), adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	got := out.Plans.Launch.Env[claude.EnvConfig]
	if got != adapter.AuthProfileToken {
		t.Fatalf("CLAUDE_CONFIG_DIR=%s want stable Auth Profile token", got)
	}
	if strings.Contains(got, adapter.PrivateToken) || strings.Contains(got, claude.HomeDirRel) {
		t.Fatal("per-run host-config must not be the Claude login namespace")
	}
}

func TestClaudeCodeFilesystemEmptyScopesDeny(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	run := sampleRun(true, false)
	empty := []string{}
	run.Permissions = &decode.Permissions{
		Filesystem: &decode.FilesystemPermissions{
			Read:  &empty,
			Write: &empty,
		},
	}
	out, err := adapter.RunPipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Plans == nil {
		t.Fatal("expected plans")
	}
	body := fileBody(out.Plans.Projection.Files, claude.SettingsRel)
	var set struct {
		Permissions struct {
			Deny  []string `json:"deny"`
			Allow []string `json:"allow"`
			Ask   []string `json:"ask"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(body), &set); err != nil {
		t.Fatal(err)
	}
	has := func(list []string, want string) bool {
		for _, s := range list {
			if s == want {
				return true
			}
		}
		return false
	}
	for _, tool := range []string{"Read", "Write", "Edit", "Glob", "Grep"} {
		if !has(set.Permissions.Deny, tool) {
			t.Fatalf("want deny %s; deny=%v", tool, set.Permissions.Deny)
		}
		if has(set.Permissions.Allow, tool) {
			t.Fatalf("must not allow %s; allow=%v", tool, set.Permissions.Allow)
		}
	}
	if !has(set.Permissions.Ask, "Bash") {
		t.Fatalf("want ask Bash; ask=%v", set.Permissions.Ask)
	}
	for _, rule := range []string{"Read(./host-config/**)", "Read(host-config/**)"} {
		if has(set.Permissions.Deny, rule) {
			t.Fatalf("workspace host-config deny is obsolete; deny=%v", set.Permissions.Deny)
		}
	}
	card := fileBody(out.Plans.Projection.Files, claude.AgentsDirRel+"/club-faq.md")
	if strings.Contains(card, "Read") || strings.Contains(card, "Write") {
		t.Fatalf("agent card must omit FS tools:\n%s", card)
	}
}

func TestKiroSandboxRequiredRefuses(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostKiro, sampleRun(true, false), adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision != "refused" {
		t.Fatalf("want refused, got %s", out.Report.Decision)
	}
	if out.Report.Security.Sandbox.ReasonCode != "acceptance_failed" {
		t.Fatalf("sandbox %s %s", out.Report.Security.Sandbox.RequirementResult, out.Report.Security.Sandbox.ReasonCode)
	}
	if out.Plans != nil {
		t.Fatal("refused must not Project")
	}
}

func TestKiroAllowedBaselinePermissions(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostKiro, sampleRun(false, false), adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision == "refused" {
		t.Fatalf("kiro baseline should be allowed after permission synthesis, got %s perms=%+v appr=%+v",
			out.Report.Decision, out.Report.Security.Permissions, out.Report.Security.Approvals)
	}
	if out.Report.Security.Permissions.Support != "approximate" ||
		out.Report.Security.Permissions.Enforcement != "host_enforced" ||
		out.Report.Security.Permissions.RequirementResult != "satisfied" {
		t.Fatalf("permissions %+v", out.Report.Security.Permissions)
	}
	if out.Plans == nil {
		t.Fatal("expected plans")
	}
}

func TestKiroRefusesNonWorkingDirectoryScope(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	run := sampleRun(false, false)
	home := "home_directory"
	run.Permissions = &decode.Permissions{
		Filesystem: &decode.FilesystemPermissions{
			Read: &[]string{home},
		},
	}
	out, err := adapter.RunPipeline(reg, adapter.HostKiro, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision != "refused" {
		t.Fatalf("want refused for unsupported scope token, got %s", out.Report.Decision)
	}
	if out.Report.Security.Permissions.ReasonCode != "permission_overgrant" {
		t.Fatalf("permissions %+v", out.Report.Security.Permissions)
	}
}

// TestKiroProjectionDictionary checks agent JSON shape via Project on an allowed report.
func TestKiroProjectionDictionary(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	run := sampleRun(false, false)
	a, err := reg.Select(adapter.HostKiro)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := a.Probe()
	if err != nil {
		t.Fatal(err)
	}
	assess, _, err := a.Assess(run, probe)
	if err != nil {
		t.Fatal(err)
	}
	req, err := compatibility.BuildRequirement(run)
	if err != nil {
		t.Fatal(err)
	}
	report := compatibility.Evaluate(compatibility.Envelope{
		SchemaVersion:     "agent2host/compatibility/v1alpha1",
		Agent2HostVersion: "0.0.0-test",
		Subject:           compatibility.Subject{SystemID: run.SystemID, AgentID: run.AgentID, Revision: run.ArtifactRevision},
		Host:              compatibility.HostRef{ID: adapter.HostKiro, Version: probe.HostVersion},
		Adapter:           compatibility.AdapterRef{ID: adapter.HostKiro, Version: adapter.AdapterVersion},
		Probe:             compatibility.Probe{Fingerprint: "sha256:" + strings.Repeat("a", 64)},
	}, req, assess)
	report.Decision = "allowed" // exercise Project dictionary only
	np, lp, err := a.Project(run, probe, report, adapter.ProjectionContext{})
	if err != nil {
		t.Fatal(err)
	}
	agentRel := kiro.AgentsDirRel + "/club-faq.json"
	rels := map[string]bool{}
	for _, f := range np.Files {
		rels[f.RelPath] = true
	}
	if !rels[agentRel] || !rels[kiro.SkillsDirRel+"/search-policy/SKILL.md"] || !rels[kiro.SettingsRel] {
		t.Fatalf("files %v", rels)
	}
	if rels[".kiro/agents/club-faq.json"] {
		t.Fatal("must not emit project .kiro/agents; card lives under kiro-home/agents")
	}
	args := strings.Join(lp.Args, " ")
	if strings.Contains(args, "--v3") || !strings.Contains(args, "--agent club-faq") {
		t.Fatalf("launch args want V2 --agent only, got %v", lp.Args)
	}
	if lp.Env[kiro.EnvHome] != adapter.WorkspaceToken+"/"+kiro.HomeDirRel {
		t.Fatalf("KIRO_HOME %v", lp.Env)
	}
	body := fileBody(np.Files, agentRel)
	if !strings.Contains(body, `"includeMcpJson": false`) {
		t.Fatalf("agent missing includeMcpJson false:\n%s", body)
	}
	if strings.Contains(body, `"includePowers"`) || strings.Contains(body, `"permissions"`) {
		t.Fatalf("agent must omit includePowers/permissions (2.20.1 drops agent):\n%s", body)
	}
	if !strings.Contains(body, `"toolsSettings"`) || !strings.Contains(body, `"deniedCommands"`) {
		t.Fatalf("agent toolsSettings missing:\n%s", body)
	}
	sopURI := "file://" + adapter.WorkspaceToken + "/" + kiro.SOPRel
	if !strings.Contains(body, sopURI) {
		t.Fatalf("agent prompt must be file:// SOP:\n%s", body)
	}
	sopBody := fileBody(np.Files, kiro.SOPRel)
	if !strings.Contains(sopBody, `Named Agent "club-faq"`) {
		t.Fatalf("sop identity banner missing:\n%s", sopBody)
	}
}

func TestKiroHooksCamelCaseValidShape(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	run := sampleRun(false, false)
	run.Hooks = &decode.Hooks{
		SessionStart:   &[]decode.HookEntry{{Command: "true"}},
		BeforeToolCall: &[]decode.HookEntry{{Command: "true"}},
		AfterToolCall:  &[]decode.HookEntry{{Command: "true"}},
		AgentStop:      &[]decode.HookEntry{{Command: "true"}},
	}
	a, err := reg.Select(adapter.HostKiro)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := a.Probe()
	if err != nil {
		t.Fatal(err)
	}
	report := compatibility.Report{Decision: "allowed", Host: compatibility.HostRef{ID: adapter.HostKiro}}
	np, _, err := a.Project(run, probe, report, adapter.ProjectionContext{})
	if err != nil {
		t.Fatal(err)
	}
	body := fileBody(np.Files, kiro.AgentsDirRel+"/club-faq.json")
	for _, bad := range []string{`"AgentSpawn"`, `"PreToolUse"`, `"PostToolUse"`, `"Stop"`} {
		if strings.Contains(body, bad) {
			t.Fatalf("PascalCase hook key %s invalidates Kiro 2.20.1 agent:\n%s", bad, body)
		}
	}
	for _, good := range []string{`"agentSpawn"`, `"preToolUse"`, `"postToolUse"`, `"stop"`} {
		if !strings.Contains(body, good) {
			t.Fatalf("missing V2 hook key %s:\n%s", good, body)
		}
	}
}

func TestCodexAllowedBaseline(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostCodex, sampleRun(false, false), adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Activation.Mode != "primary_mapped" || out.Report.Activation.ReasonCode != "mapped_activation" {
		t.Fatalf("codex activation %+v", out.Report.Activation)
	}
	if out.Report.Decision == "refused" {
		t.Fatalf("codex baseline on_boundary should be allowed, got %s perms=%+v appr=%+v",
			out.Report.Decision, out.Report.Security.Permissions, out.Report.Security.Approvals)
	}
	if out.Report.Security.Approvals.RequirementResult != "satisfied" ||
		out.Report.Security.Approvals.ReasonCode == "approval_weaker" {
		t.Fatalf("approvals %+v", out.Report.Security.Approvals)
	}
	if out.Plans == nil {
		t.Fatal("expected plans")
	}
}

func TestCodexRefusesAlwaysApproval(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	run := sampleRun(false, false)
	always := "always"
	run.Approvals = &decode.Approvals{ShellExecute: &always}
	out, err := adapter.RunPipeline(reg, adapter.HostCodex, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision != "refused" {
		t.Fatalf("codex must refuse always (on-request is weaker), got %s appr=%+v",
			out.Report.Decision, out.Report.Security.Approvals)
	}
	if out.Report.Security.Approvals.ReasonCode != "approval_weaker" {
		t.Fatalf("approvals %+v", out.Report.Security.Approvals)
	}
}

func TestCodexSandboxRequiredProjects(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostCodex, sampleRun(true, false), adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision == "refused" {
		t.Fatalf("codex projects OS sandbox; sandbox.required should not refuse alone, got %s sbx=%+v",
			out.Report.Decision, out.Report.Security.Sandbox)
	}
	if out.Report.Security.Sandbox.RequirementResult != "satisfied" {
		t.Fatalf("sandbox %+v", out.Report.Security.Sandbox)
	}
	if out.Plans == nil {
		t.Fatal("expected plans")
	}
}

func TestCodexProjectionDictionary(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	run := sampleRun(false, false)
	a, err := reg.Select(adapter.HostCodex)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := a.Probe()
	if err != nil {
		t.Fatal(err)
	}
	report := compatibility.Report{
		Decision: "allowed",
		Host:     compatibility.HostRef{ID: adapter.HostCodex},
		Capabilities: compatibility.Capabilities{
			SOP: compatibility.OrdinaryRow{Disposition: "include"},
			Skills: compatibility.SkillCollection{Items: []compatibility.SkillItem{
				{ID: "search-policy", Disposition: "include"},
			}},
		},
	}
	np, lp, err := a.Project(run, probe, report, adapter.ProjectionContext{})
	if err != nil {
		t.Fatal(err)
	}
	rels := map[string]bool{}
	for _, f := range np.Files {
		rels[f.RelPath] = true
	}
	skillRel := codex.SkillsDirRel + "/search-policy/SKILL.md"
	if !rels["AGENTS.md"] || !rels[codex.ConfigRel] || !rels[codex.AgentsRel] || !rels[skillRel] {
		t.Fatalf("files %v", rels)
	}
	// Codex scans .agents/skills relative to cwd (ApprovedWorkingDirectory),
	// so workspace-projected skills are invisible. Skills must live in CODEX_HOME.
	if rels[".agents/skills/search-policy/SKILL.md"] {
		t.Fatal("skills must not be projected to the workspace-only .agents/skills path")
	}
	for _, f := range np.Files {
		if f.RelPath == codex.ConfigRel || f.RelPath == codex.AgentsRel || f.RelPath == skillRel {
			if f.Class != adapter.DestHostPrivate {
				t.Fatalf("%s class %s want host_private", f.RelPath, f.Class)
			}
		}
	}
	args := strings.Join(lp.Args, " ")
	if !strings.Contains(args, "--add-dir "+adapter.WorkspaceToken) || strings.Contains(args, adapter.PrivateToken) {
		t.Fatalf("launch args %v", lp.Args)
	}
	if lp.Env[codex.EnvHome] != adapter.PrivateToken+"/"+codex.HomeDirRel {
		t.Fatalf("CODEX_HOME %v", lp.Env)
	}
}

func TestCodexConfigUsesPermissionProfile(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	run := registerClubFAQRun(t)
	out, err := adapter.RunPipeline(reg, adapter.HostCodex, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Plans == nil {
		t.Fatal("expected plans")
	}
	var cfg []byte
	for _, f := range out.Plans.Projection.Files {
		if f.RelPath == codex.ConfigRel {
			cfg = f.Content
		}
	}
	if len(cfg) == 0 {
		t.Fatal("missing codex config.toml")
	}
	body := string(cfg)
	for _, forbidden := range []string{"sandbox_mode", "[sandbox_workspace_write]", "exclude_slash_tmp"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("legacy sandbox key %q must not appear:\n%s", forbidden, body)
		}
	}
	if strings.Contains(body, `":root" = "deny"`) {
		t.Fatalf("interactive Codex cannot start with :root deny:\n%s", body)
	}
	for _, want := range []string{
		`default_permissions = "a2h"`,
		`[permissions.a2h]`,
		`":workspace_roots" = "write"`,
		"enabled = false",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("config missing %q:\n%s", want, body)
		}
	}
	args := strings.Join(out.Plans.Launch.Args, " ")
	if strings.Contains(args, "--sandbox") {
		t.Fatalf("--sandbox would force legacy model: %v", out.Plans.Launch.Args)
	}
}

func TestCodexClubFAQIdentityAndAppsDisabled(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	run := registerClubFAQRun(t)
	out, err := adapter.RunPipeline(reg, adapter.HostCodex, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Plans == nil {
		t.Fatal("expected plans")
	}
	var cfg, agents []byte
	for _, f := range out.Plans.Projection.Files {
		switch f.RelPath {
		case codex.ConfigRel:
			cfg = f.Content
		case codex.AgentsRel:
			agents = f.Content
		}
	}
	body := string(cfg)
	if strings.Contains(body, "[features]") {
		t.Fatalf("[features] table swallows subsequent keys; use features.apps dotted form:\n%s", body)
	}
	for _, want := range []string{
		"features.apps = false",
		"developer_instructions = ",
		"club-faq",
		"Do not identify as the default Host assistant",
	} {
		if !strings.Contains(body, want) && !strings.Contains(string(agents), want) {
			t.Fatalf("missing %q in codex private config/agents:\nconfig=%s\nagents=%s", want, body, agents)
		}
	}
	if !strings.Contains(body, "[mcp_servers.club-database]") {
		t.Fatalf("missing club-database MCP:\n%s", body)
	}
}

func TestKiroClubFAQNamedAgentPrompt(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	run := registerClubFAQRun(t)
	out, err := adapter.RunPipeline(reg, adapter.HostKiro, run, adapter.ProjectionContext{}, "test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Plans == nil {
		t.Fatal("expected plans")
	}
	body := fileBody(out.Plans.Projection.Files, kiro.SOPRel)
	for _, want := range []string{
		"club-faq",
		"Do not identify as the default Host assistant",
		"Use them when the user asks for file or shell operations",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in kiro SOP projection:\n%s", want, body)
		}
	}
}

func registerClubFAQRun(t *testing.T) *space.ResolvedAgentRun {
	t.Helper()
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	sp, err := space.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sp.Register(filepath.Join(root, "trees", "valid", "club-system")); err != nil {
		t.Fatal(err)
	}
	run, err := sp.Resolve("club-system", "club-faq", "")
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestMissingBinaryRefuses(t *testing.T) {
	reg := committed.New(missingLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostClaudeCode, sampleRun(false, false), adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision != "refused" {
		t.Fatalf("missing host: %s", out.Report.Decision)
	}
	if out.Plans != nil {
		t.Fatal("no plans when missing")
	}
}

func TestProjectOmitsOptionalSkill(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	run := sampleRun(false, true)
	// Force pretty skill unsupported via a one-off assess? Easier: mark pretty
	// required false and temporarily use a host... default profile maps skills.
	// Omit by evaluating a fixture-like path: Project filters disposition.
	// Build a report with pretty omitted.
	a, _ := reg.Select(adapter.HostClaudeCode)
	probe, _ := a.Probe()
	assess, _, _ := a.Assess(run, probe)
	for i, s := range assess.Skills {
		if s.ID == "pretty" {
			assess.Skills[i].Support = "unsupported"
		}
	}
	req, err := compatibility.BuildRequirement(run)
	if err != nil {
		t.Fatal(err)
	}
	env := (adapter.FixtureDriver{Observed: probe, AdapterID: adapter.HostClaudeCode, AdapterVer: adapter.AdapterVersion}).Envelope(
		compatibility.Subject{SystemID: run.SystemID, AgentID: run.AgentID, Revision: run.ArtifactRevision}, "0.0.0-test")
	report := compatibility.Evaluate(env, req, assess)
	np, _, err := a.Project(run, probe, report, adapter.ProjectionContext{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range np.Files {
		if strings.Contains(f.RelPath, "pretty") {
			t.Fatalf("omitted skill leaked: %s", f.RelPath)
		}
	}
}

func TestFingerprintStableAndExcludesClock(t *testing.T) {
	a := adapter.Fingerprint("claude-code", "1.0.0", "/opt/claude")
	b := adapter.Fingerprint("claude-code", "1.0.0", "/opt/claude")
	if a != b || !strings.HasPrefix(a, "sha256:") || len(a) != 7+64 {
		t.Fatalf("fingerprint %s", a)
	}
	if adapter.Fingerprint("claude-code", "1.0.1", "/opt/claude") == a {
		t.Fatal("version must affect fingerprint")
	}
}

func TestBindWorkspaceLocalOnlyPackedFiles(t *testing.T) {
	packed := []string{"mcp/club-database.py", "hooks/before-tool.py"}
	if got := adapter.BindWorkspaceLocal("mcp/club-database.py", packed); got != adapter.WorkspaceToken+"/mcp/club-database.py" {
		t.Fatalf("got %q", got)
	}
	if got := adapter.BindWorkspaceLocal("./hooks/before-tool.py", packed); got != adapter.WorkspaceToken+"/hooks/before-tool.py" {
		t.Fatalf("got %q", got)
	}
	if got := adapter.BindWorkspaceLocal("python3", packed); got != "python3" {
		t.Fatalf("PATH command rewritten: %q", got)
	}
	if got := adapter.BindWorkspaceLocal("--flag", packed); got != "--flag" {
		t.Fatalf("opaque arg rewritten: %q", got)
	}
}

func TestClaudePermissionsApproximateWhenNetworkDenyIncomplete(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	// Omitted permissions.network → deny (SRC-SEC-NET). Claude can only project
	// WebFetch/WebSearch + curl/wget/nc-shaped Bash, so SRC-SEC-INTENT forbids
	// claiming complete isolation.
	out, err := adapter.RunPipeline(reg, adapter.HostClaudeCode, sampleRun(false, false), adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	perm := out.Report.Security.Permissions
	if perm.Support != "approximate" {
		t.Fatalf("permissions support %q, want approximate (incomplete network deny)", perm.Support)
	}
	if perm.RequirementResult != "satisfied" || perm.Enforcement != "host_enforced" {
		t.Fatalf("honesty downgrade must not change the outcome: %+v", perm)
	}
	if out.Report.Decision == "refused" {
		t.Fatalf("decision %s", out.Report.Decision)
	}
}

func TestSystemLocalCommandMarkedExecutable(t *testing.T) {
	run := sampleRun(false, false)
	run.MCPServers = map[string]space.ResolvedMCP{
		"club-database": {
			Command: "mcp/club-database.py",
			Files:   []string{"mcp/club-database.py"},
		},
		"other": {
			Command: "python3",
			Args:    []string{"mcp/helper.py"},
			Files:   []string{"mcp/helper.py"},
		},
	}
	run.Hooks = &decode.Hooks{
		BeforeToolCall: &[]decode.HookEntry{{
			Command: "./hooks/before-tool.py",
			Files:   &[]string{"hooks/before-tool.py"},
		}},
	}
	run.Content["mcp/club-database.py"] = []byte("#!/usr/bin/env python3\n")
	run.Content["mcp/helper.py"] = []byte("#!/usr/bin/env python3\n")
	run.Content["hooks/before-tool.py"] = []byte("#!/usr/bin/env python3\n")

	reg := committed.New(foundLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Plans == nil {
		t.Fatal("expected plans")
	}
	want := map[string]bool{
		"mcp/club-database.py": true,
		"hooks/before-tool.py": true,
		"mcp/helper.py":        false,
		claude.SettingsRel:     false,
	}
	seen := map[string]bool{}
	for _, f := range out.Plans.Projection.Files {
		if exp, ok := want[f.RelPath]; ok {
			seen[f.RelPath] = true
			if f.Executable != exp {
				t.Fatalf("%s executable=%v want %v", f.RelPath, f.Executable, exp)
			}
		}
	}
	for rel := range want {
		if !seen[rel] {
			t.Fatalf("projection missing %s", rel)
		}
	}
}

func TestClaudeCodeProjectsMCPAllowlistAndQuotedHooks(t *testing.T) {
	req := true
	run := sampleRun(true, false)
	run.MCPServers = map[string]space.ResolvedMCP{
		"club-database": {
			Command: "python3",
			Args:    []string{"mcp/club-database.py"},
			Files:   []string{"mcp/club-database.py"},
			Tools: []space.ResolvedTool{{
				Name: "search_policy", Required: &req,
			}},
		},
	}
	run.Hooks = &decode.Hooks{
		BeforeToolCall: &[]decode.HookEntry{{
			Command: "python3",
			Args:    &[]string{"hooks/before-tool.py", "arg with space"},
			Files:   &[]string{"hooks/before-tool.py"},
		}},
	}
	reg := committed.New(foundLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision == "refused" || out.Plans == nil {
		t.Fatalf("decision %s", out.Report.Decision)
	}
	mcp := fileBody(out.Plans.Projection.Files, claude.MCPRel)
	if !strings.Contains(mcp, adapter.WorkspaceToken+"/mcp/club-database.py") {
		t.Fatalf("MCP args must bind workspace, got %s", mcp)
	}
	if strings.Contains(mcp, `"args": [
    "mcp/club-database.py"
  ]`) {
		t.Fatal("unbound relative MCP arg leaked into plan")
	}
	args := strings.Join(out.Plans.Launch.Args, " ")
	if !strings.Contains(args, "mcp__club-database__search_policy") {
		t.Fatalf("allowedTools missing declared MCP tool: %v", out.Plans.Launch.Args)
	}
	if strings.Contains(args, "forbidden_echo") {
		t.Fatal("non-allowlisted MCP tool leaked into launch args")
	}
	card := fileBody(out.Plans.Projection.Files, claude.AgentsDirRel+"/club-faq.md")
	if !strings.Contains(card, "mcp__club-database__search_policy") || strings.Contains(card, "forbidden_echo") {
		t.Fatalf("agent card tools:\n%s", card)
	}
	settings := fileBody(out.Plans.Projection.Files, claude.SettingsRel)
	if !strings.Contains(settings, adapter.WorkspaceToken+"/hooks/before-tool.py") || !strings.Contains(settings, `'arg with space'`) {
		t.Fatalf("hook argv not workspace-bound/quoted:\n%s", settings)
	}
	if len(out.Report.Security.MCPToolIsolation.Items) == 0 ||
		out.Report.Security.MCPToolIsolation.Items[0].Enforcement != "host_enforced" {
		t.Fatalf("Claude strict MCP closure must be host_enforced, got %+v", out.Report.Security.MCPToolIsolation)
	}
}

func TestHookSecretsAreConsumerScopedOnClaude(t *testing.T) {
	req := true
	run := sampleRun(false, false)
	run.Hooks = &decode.Hooks{
		BeforeToolCall: &[]decode.HookEntry{{
			Command: "true",
			Environment: &[]decode.EnvironmentBinding{{
				Required:  &req,
				ValueFrom: decode.ValueFrom{Environment: "AUDIT_TOKEN"},
			}},
		}},
	}
	reg := committed.New(foundLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision == "refused" {
		t.Fatalf("planned hook consumer must not refuse, got %s %+v", out.Report.Decision, out.Report.Security.SecretIsolation)
	}
	found := false
	for _, s := range out.Report.Security.SecretIsolation.Items {
		if s.Consumer == "/hooks/before_tool_call/0" && s.Target == "AUDIT_TOKEN" &&
			s.Scope == "agent" && s.RequirementResult == "satisfied" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing scoped hook secret row: %+v", out.Report.Security.SecretIsolation)
	}
	if out.Plans == nil || !strings.Contains(fileBody(out.Plans.Projection.Files, claude.SettingsRel), "AUDIT_TOKEN") {
		t.Fatal("settings must carry hook secret placeholder")
	}
}

func TestRequiredMCPSecretIsConsumerScopedOnClaude(t *testing.T) {
	req := true
	run := sampleRun(false, false)
	run.MCPServers = map[string]space.ResolvedMCP{
		"club-database": {
			Command: "python3",
			Args:    []string{"mcp/club-database.py"},
			Files:   []string{"mcp/club-database.py"},
			Environment: []decode.EnvironmentBinding{{
				Required:  &req,
				ValueFrom: decode.ValueFrom{Environment: "CLUB_DB_TOKEN"},
			}},
		},
	}
	reg := committed.New(foundLook(), stubVersion)
	out, err := adapter.RunPipeline(reg, adapter.HostClaudeCode, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Decision == "refused" {
		t.Fatalf("planned MCP consumer must not refuse, got %s %+v", out.Report.Decision, out.Report.Security.SecretIsolation)
	}
	found := false
	for _, s := range out.Report.Security.SecretIsolation.Items {
		if s.Consumer == "/mcp_servers/club-database" && s.Target == "CLUB_DB_TOKEN" &&
			s.Scope == "agent" && s.RequirementResult == "satisfied" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing scoped MCP secret row: %+v", out.Report.Security.SecretIsolation)
	}
	if out.Plans == nil || !strings.Contains(fileBody(out.Plans.Projection.Files, claude.MCPRel), adapter.SecretPlaceholder("CLUB_DB_TOKEN")) {
		t.Fatal("mcp.json must carry secret placeholder")
	}
	if !strings.Contains(fileBody(out.Plans.Projection.Files, claude.MCPRel), adapter.WorkspaceToken+"/mcp/club-database.py") {
		t.Fatal("mcp.json must bind local script to workspace")
	}
}

func TestReconcileIntentMissingFile(t *testing.T) {
	err := adapter.ReconcilePlansCommon(adapter.ControlIntent{
		Controls: []adapter.PlannedControl{{Kind: adapter.ControlSandbox, Via: adapter.ViaProjectionFile, Rel: claude.SettingsRel}},
	}, compatibility.Report{}, adapter.NativeProjectionPlan{}, adapter.LaunchPlan{})
	if !errors.Is(err, adapter.ErrIntentMismatch) {
		t.Fatalf("got %v", err)
	}
}

func TestProjectRefusedError(t *testing.T) {
	d := adapter.FixtureDriver{Observed: adapter.ProbeResult{HostID: "codex"}}
	_, _, err := d.Project(sampleRun(false, false), d.Observed, compatibility.Report{Decision: "refused"}, adapter.ProjectionContext{})
	if !errors.Is(err, adapter.ErrProjectRefused) {
		t.Fatalf("got %v", err)
	}
}

func fileBody(files []adapter.ProjectionFile, rel string) string {
	for _, f := range files {
		if f.RelPath == rel {
			return string(f.Content)
		}
	}
	return ""
}

func assertCard(t *testing.T, files []adapter.ProjectionFile, rel, id, desc string) {
	t.Helper()
	body := fileBody(files, rel)
	if body == "" {
		t.Fatalf("missing %s", rel)
	}
	if !strings.HasPrefix(body, "---\n") || !strings.Contains(body, "name: "+id) || !strings.Contains(body, desc) {
		t.Fatalf("%s card:\n%s", rel, body)
	}
}

func assertSkillFrontmatter(t *testing.T, files []adapter.ProjectionFile, rel, id, desc string) {
	t.Helper()
	body := fileBody(files, rel)
	if !strings.HasPrefix(body, "---\n") || !strings.Contains(body, "name: "+id) || !strings.Contains(body, "description: "+desc) {
		t.Fatalf("%s skill frontmatter:\n%s", rel, body)
	}
}
