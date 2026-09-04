package runtime

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
)

func TestRejectNativeArgs(t *testing.T) {
	if err := RejectNativeArgs(nil); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(RejectNativeArgs([]string{"--foo"}), ErrNativeArgs) {
		t.Fatal("unknown args must reject")
	}
}

func TestJoinUnderRejectsEscape(t *testing.T) {
	if _, err := joinUnder("/tmp/root", "../etc/passwd"); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("got %v", err)
	}
	if _, err := joinUnder("/tmp/root", "/abs"); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("got %v", err)
	}
}

func TestFilterParentEnvDropsSecrets(t *testing.T) {
	got := filterParentEnv([]string{
		"PATH=/usr/bin",
		"HOME=/home/u",
		"AWS_SECRET_ACCESS_KEY=leak",
		"GITHUB_TOKEN=leak",
		"REPO_TOOLS_TOKEN=leak",
		"ANTHROPIC_API_KEY=ok",
		"LC_TIME=C",
		"SSH_AUTH_SOCK=/tmp/ssh",
	})
	joined := strings.Join(got, "\n")
	for _, want := range []string{"PATH=/usr/bin", "HOME=/home/u", "ANTHROPIC_API_KEY=ok", "LC_TIME=C"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, got)
		}
	}
	for _, bad := range []string{"AWS_SECRET", "GITHUB_TOKEN", "REPO_TOOLS_TOKEN", "SSH_AUTH_SOCK"} {
		if strings.Contains(joined, bad) {
			t.Fatalf("must not inherit %s: %v", bad, got)
		}
	}
}

func TestExecuteDoesNotInheritParentSecrets(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dump := filepath.Join(t.TempDir(), "env.txt")
	stub := filepath.Join(t.TempDir(), "host")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nenv > "+dump+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Launch: adapter.LaunchPlan{Executable: stub},
	})
	getenv := func(k string) string {
		switch k {
		case "PATH":
			return "/bin:/usr/bin"
		case "AWS_SECRET_ACCESS_KEY":
			return "should-not-appear"
		case "ANTHROPIC_API_KEY":
			return "host-auth"
		default:
			return ""
		}
	}
	if _, err := Execute(p, auth, r, ExecOpts{Getenv: getenv}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte("AWS_SECRET_ACCESS_KEY")) {
		t.Fatalf("parent secret leaked into host env:\n%s", body)
	}
	if !bytes.Contains(body, []byte("ANTHROPIC_API_KEY=host-auth")) {
		t.Fatalf("host auth allowlist missing:\n%s", body)
	}
}

func TestCodexConfigLandsInPrivateNotAddDirSurface(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rel := "codex-home/config.toml"
	planFile := []byte("token = " + adapter.QuoteTOMLString(adapter.SecretPlaceholder("T")) + "\napproval_policy = \"on-request\"\n")
	dump := filepath.Join(t.TempDir(), "during.txt")
	stub := filepath.Join(t.TempDir(), "host")
	live := filepath.Join(p.Private, rel)
	script := "#!/bin/sh\ncat \"" + live + "\" > \"" + dump + "\"\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	r.Host.ID = adapter.HostCodex
	auth := mustAuth(r, adapter.Plans{
		Projection: adapter.NativeProjectionPlan{
			HostID: adapter.HostCodex,
			Files: []adapter.ProjectionFile{{
				RelPath: rel, Class: adapter.DestHostPrivate, Content: planFile,
			}},
		},
		Launch: adapter.LaunchPlan{
			Executable: stub,
			Env:        map[string]string{"CODEX_HOME": adapter.PrivateToken + "/codex-home"},
			Args:       []string{"--add-dir", adapter.WorkspaceToken},
			Secrets:    []adapter.SecretRef{{Name: "T", Consumer: "/mcp_servers/db", Required: true}},
		},
	})
	secret := "abc\"\napproval_policy = \"never"
	if _, err := Execute(p, auth, r, ExecOpts{
		Getenv: func(k string) string {
			if k == "T" {
				return secret
			}
			if k == "PATH" {
				return "/bin:/usr/bin"
			}
			return ""
		},
		OnHostStarting: func() {
			if _, err := os.Stat(filepath.Join(p.Workspace, rel)); err == nil {
				t.Error("codex config must not land under workspace/--add-dir surface")
			}
			if _, err := os.Stat(filepath.Join(p.Private, rel)); err != nil {
				t.Error("codex config must land in private")
			}
		},
	}); err != nil {
		t.Fatal(err)
	}
	during, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	want := "token = " + adapter.QuoteTOMLString(secret) + "\n"
	if !bytes.Contains(during, []byte(want)) {
		t.Fatalf("TOML overlay must escape quotes/newlines, got:\n%s\nwant substring:\n%s", during, want)
	}
	if bytes.Contains(during, []byte("\napproval_policy = \"never\"")) {
		t.Fatalf("newline in secret must not inject a real approval_policy line:\n%s", during)
	}
	if !bytes.Contains(during, []byte("approval_policy = \"on-request\"")) {
		t.Fatalf("original approval_policy must remain:\n%s", during)
	}
	assertWorkspaceGone(t, p)
}

func TestMaterializeSetsExecuteBitFromPlan(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "ran.txt")
	scriptRel := "mcp/local-server.sh"
	script := []byte("#!/bin/sh\necho ok > \"" + marker + "\"\n")
	docRel := "contexts/handbook.md"

	stub := filepath.Join(t.TempDir(), "host")
	// Host launches the projected System-local command directly.
	hostScript := "#!/bin/sh\n\"" + filepath.Join(p.Workspace, scriptRel) + "\"\n"
	if err := os.WriteFile(stub, []byte(hostScript), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Projection: adapter.NativeProjectionPlan{
			HostID: "claude-code",
			Files: []adapter.ProjectionFile{
				{RelPath: scriptRel, Class: adapter.DestProjection, Content: script, Executable: true},
				{RelPath: docRel, Class: adapter.DestProjection, Content: []byte("# handbook\n")},
			},
		},
		Launch: adapter.LaunchPlan{Executable: stub},
	})
	if _, err := Execute(p, auth, r, ExecOpts{
		Getenv: func(k string) string {
			if k == "PATH" {
				return "/bin:/usr/bin"
			}
			return ""
		},
		OnHostStarting: func() {
			fi, err := os.Stat(filepath.Join(p.Workspace, scriptRel))
			if err != nil {
				t.Error(err)
				return
			}
			if fi.Mode().Perm()&0o100 == 0 {
				t.Errorf("System-local command must be executable, mode %v", fi.Mode().Perm())
			}
			docFI, err := os.Stat(filepath.Join(p.Workspace, docRel))
			if err != nil {
				t.Error(err)
				return
			}
			if docFI.Mode().Perm()&0o111 != 0 {
				t.Errorf("non-command file must not be executable, mode %v", docFI.Mode().Perm())
			}
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("projected command did not run: %v", err)
	}
	assertWorkspaceGone(t, p)
}

func TestRestoreStaleSecretBaselinesAfterCrash(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rel := "codex-home/config.toml"
	plan := []byte("token = " + adapter.QuoteTOMLString(adapter.SecretPlaceholder("T")) + "\n")
	if err := persistSecretBaselines(p, adapter.NativeProjectionPlan{
		Files: []adapter.ProjectionFile{{
			RelPath: rel, Class: adapter.DestHostPrivate, Content: plan,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(p.Private, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		t.Fatal(err)
	}
	// Simulate kill -9 after overlay: live secret on disk, baseline still placeholders.
	leaked := []byte("token = \"leaked-secret-value\"\n")
	if err := os.WriteFile(dest, leaked, 0o600); err != nil {
		t.Fatal(err)
	}
	simulateCrashedLock(t, p)
	if _, err := Prepare(home, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(after, []byte("leaked-secret-value")) {
		t.Fatalf("Prepare must restore baseline placeholders, got %s", after)
	}
	if !bytes.Contains(after, []byte(adapter.SecretPlaceholder("T"))) {
		t.Fatalf("want placeholder restored, got %s", after)
	}
}

func TestRestoreSkipsPrivateCodexWhileRunLive(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rel := "codex-home/config.toml"
	plan := []byte("token = " + adapter.QuoteTOMLString(adapter.SecretPlaceholder("T")) + "\n")
	if err := persistSecretBaselines(p, adapter.NativeProjectionPlan{
		Files: []adapter.ProjectionFile{{
			RelPath: rel, Class: adapter.DestHostPrivate, Content: plan,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(p.Private, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		t.Fatal(err)
	}
	leaked := []byte("token = \"live-session-secret\"\n")
	if err := os.WriteFile(dest, leaked, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(liveLockPath(p.Root), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(home, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	during, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(during, []byte("live-session-secret")) {
		t.Fatalf("must not restore private codex-home while that run is live, got %s", during)
	}
	if err := os.Remove(liveLockPath(p.Root)); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(home, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(after, []byte("live-session-secret")) || !bytes.Contains(after, []byte(adapter.SecretPlaceholder("T"))) {
		t.Fatalf("after run exits, Prepare must restore placeholders, got %s", after)
	}
}

func TestPrepareDoesNotClobberLiveRunSecrets(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rel := "kiro-home/agents/example-agent.json"
	plan := []byte(`{"env":{"T":"` + adapter.SecretPlaceholder("T") + `"}}` + "\n")
	if err := persistSecretBaselines(p, adapter.NativeProjectionPlan{
		Files: []adapter.ProjectionFile{{
			RelPath: rel, Class: adapter.DestProjection, Content: plan,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(p.Workspace, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		t.Fatal(err)
	}
	live := []byte(`{"env":{"T":"live-token-must-stay"}}` + "\n")
	if err := os.WriteFile(dest, live, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeLiveLock(p); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clearLiveLock(p) })
	if _, err := Prepare(home, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(after, []byte("live-token-must-stay")) {
		t.Fatalf("second Prepare must not wipe a live run, got %s", after)
	}
}

func TestExecuteMissingAuth(t *testing.T) {
	p, err := Prepare(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(p, nil, compatibility.Report{Decision: "allowed"}, ExecOpts{})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("got %v", err)
	}
}

func TestExecuteMissingRequiredSecret(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Launch: adapter.LaunchPlan{
			Executable: "/bin/true",
			Secrets:    []adapter.SecretRef{{Name: "MUST_HAVE", Required: true}},
		},
	})
	started := false
	out, err := Execute(p, auth, r, ExecOpts{
		Getenv:         func(string) string { return "" },
		OnHostStarting: func() { started = true },
	})
	if !errors.Is(err, ErrMissingSecret) || out.Class != ClassPreLaunch {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if started {
		t.Fatal("host-start callback must not run before required secrets resolve")
	}
	body, _ := os.ReadFile(filepath.Join(home, "records", p.RunID+".json"))
	if bytes.Contains(body, []byte("secret-value")) || !bytes.Contains(body, []byte("MUST_HAVE")) {
		t.Fatalf("record %s", body)
	}
}

func TestExecuteRejectsBypassLaunchArg(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(t.TempDir(), "host")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Projection: adapter.NativeProjectionPlan{HostID: "claude-code"},
		Launch: adapter.LaunchPlan{
			Executable: stub,
			Args:       []string{"--dangerously-skip-permissions"},
		},
	})
	out, err := Execute(p, auth, r, ExecOpts{})
	if err == nil || out.Stage != "launch_reconcile" {
		t.Fatalf("want launch_reconcile refusal, out=%+v err=%v", out, err)
	}
}

func TestExecuteMaterializeAndLaunch(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(t.TempDir(), "host")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho host-ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Projection: adapter.NativeProjectionPlan{
			HostID: "claude-code",
			Files: []adapter.ProjectionFile{{
				RelPath: "CLAUDE.md", Class: adapter.DestProjection, Content: []byte("# sop\n"),
			}},
		},
		Launch: adapter.LaunchPlan{Executable: stub},
	})
	var stdout bytes.Buffer
	starts := 0
	out, err := Execute(p, auth, r, ExecOpts{
		Stdout: &stdout,
		Getenv: func(string) string { return "" },
		OnHostStarting: func() {
			starts++
			if _, err := os.Stat(filepath.Join(p.Workspace, "CLAUDE.md")); err != nil {
				t.Error("expected materialized projection before launch")
			}
		},
	})
	if err != nil || out.Class != ClassHostProcess || out.ExitCode != 0 {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if starts != 1 {
		t.Fatalf("host-start callback count = %d, want 1", starts)
	}
	if !strings.Contains(stdout.String(), "host-ok") {
		t.Fatalf("stdout %q", stdout.String())
	}
	assertWorkspaceGone(t, p)
	if _, err := os.Stat(filepath.Join(home, "CLAUDE.md")); err == nil {
		t.Fatal("must not write CLAUDE.md at home root")
	}
}

func TestLaunchCWDUsesApprovedWorkingDirectory(t *testing.T) {
	home := t.TempDir()
	wd := t.TempDir()
	p, err := Prepare(home, wd)
	if err != nil {
		t.Fatal(err)
	}
	got := filepath.Join(t.TempDir(), "cwd.txt")
	stub := filepath.Join(t.TempDir(), "host")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\npwd > "+got+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Launch: adapter.LaunchPlan{Executable: stub, WorkingDirClass: adapter.DestWorkingDir},
	})
	if _, err := Execute(p, auth, r, ExecOpts{}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	gotCWD, err := filepath.EvalSymlinks(strings.TrimSpace(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	wantCWD, err := filepath.EvalSymlinks(wd)
	if err != nil {
		t.Fatal(err)
	}
	if gotCWD != wantCWD {
		t.Fatalf("cwd %q want %q", gotCWD, wantCWD)
	}
}

func TestExpandWorkspaceInMCPThenWipeRestoresToken(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rel := "host-config/mcp.json"
	planFile := []byte("{\n  \"args\": [\"" + adapter.WorkspaceToken + "/mcp/x.py\"],\n  \"env\": {\n    \"T\": \"" + adapter.SecretPlaceholder("T") + "\"\n  }\n}\n")
	dump := filepath.Join(t.TempDir(), "during.txt")
	stub := filepath.Join(t.TempDir(), "host")
	script := "#!/bin/sh\ncat \"" + filepath.Join(p.Private, rel) + "\" > \"" + dump + "\"\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Projection: adapter.NativeProjectionPlan{
			HostID: "claude-code",
			Files: []adapter.ProjectionFile{{
				RelPath: rel, Class: adapter.DestHostPrivate, Content: planFile,
			}},
		},
		Launch: adapter.LaunchPlan{
			Executable: stub,
			Secrets:    []adapter.SecretRef{{Name: "T", Consumer: "/mcp_servers/db", Required: true}},
		},
	})
	if _, err := Execute(p, auth, r, ExecOpts{Getenv: func(k string) string {
		if k == "T" {
			return "secret-xyz"
		}
		if k == "PATH" {
			return "/bin:/usr/bin"
		}
		return ""
	}}); err != nil {
		t.Fatal(err)
	}
	during, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(during, []byte(p.Workspace+"/mcp/x.py")) {
		t.Fatalf("disk during run must expand workspace token, got %s", during)
	}
	if !bytes.Contains(during, []byte("secret-xyz")) {
		t.Fatalf("disk during run must overlay secret, got %s", during)
	}
	assertWorkspaceGone(t, p)
}

func TestOverlaySecretsJSONSafeThenWipe(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rel := "host-config/mcp.json"
	planFile := []byte("{\n  \"env\": {\n    \"T\": \"" + adapter.SecretPlaceholder("T") + "\"\n  }\n}\n")
	dump := filepath.Join(t.TempDir(), "during.txt")
	stub := filepath.Join(t.TempDir(), "host")
	script := "#!/bin/sh\ncat \"" + filepath.Join(p.Private, rel) + "\" > \"" + dump + "\"\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Projection: adapter.NativeProjectionPlan{
			HostID: "claude-code",
			Files: []adapter.ProjectionFile{{
				RelPath: rel, Class: adapter.DestHostPrivate, Content: planFile,
			}},
		},
		Launch: adapter.LaunchPlan{
			Executable: stub,
			Secrets:    []adapter.SecretRef{{Name: "T", Consumer: "/mcp_servers/db", Required: true}},
		},
	})
	if _, err := Execute(p, auth, r, ExecOpts{Getenv: func(k string) string {
		if k == "T" {
			return `say "hi"`
		}
		if k == "PATH" {
			return "/bin:/usr/bin"
		}
		return ""
	}}); err != nil {
		t.Fatal(err)
	}
	during, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(during, []byte(`"say \"hi\""`)) {
		t.Fatalf("overlay must JSON-escape, got %s", during)
	}
	assertWorkspaceGone(t, p)
}

func TestOptionalSecretOmitted(t *testing.T) {
	got, err := resolveSecrets([]adapter.SecretRef{{Name: "OPT", Required: false}}, func(string) string { return "" })
	if err != nil || len(got.omitted) != 1 || got.omitted[0] != "OPT" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestResolveSecretsScopesMCPConsumer(t *testing.T) {
	got, err := resolveSecrets([]adapter.SecretRef{{
		Name: "A2H_TEST_TOKEN", Consumer: "/mcp_servers/example-mcp", Required: true,
	}}, func(string) string { return "secret-value" })
	if err != nil || len(got.env) != 0 || got.values["A2H_TEST_TOKEN"] != "secret-value" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestResolveSecretsDeliverProcessEnv(t *testing.T) {
	got, err := resolveSecrets([]adapter.SecretRef{{
		Name: "A2H_TEST_TOKEN", Consumer: "/mcp_servers/example-mcp", Required: true, DeliverProcessEnv: true,
	}}, func(string) string { return "secret-value" })
	if err != nil || len(got.env) != 1 || got.env[0] != "A2H_TEST_TOKEN=secret-value" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestResolveSecretsOmitsMissingOptionalMCP(t *testing.T) {
	got, err := resolveSecrets([]adapter.SecretRef{{
		Name: "A2H_TEST_TOKEN", Consumer: "/mcp_servers/example-mcp", Required: false,
	}}, func(string) string { return "" })
	if err != nil || len(got.env) != 0 || len(got.omitted) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestValidForRejectsMutatedPlans(t *testing.T) {
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Launch: adapter.LaunchPlan{Executable: "/bin/true"},
	})
	auth.Plans.Launch.Args = []string{"--evil"}
	if err := auth.ValidFor(r, auth.Plans); !errors.Is(err, ErrBindingMismatch) {
		t.Fatalf("got %v", err)
	}
}

func TestMaterializeRejectsSymlinkEscape(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(p.Workspace, "escape")
	if err := os.Symlink(filepath.Dir(outside), link); err != nil {
		t.Fatal(err)
	}
	err = materialize(p, adapter.NativeProjectionPlan{
		Files: []adapter.ProjectionFile{{
			RelPath: "escape/pwned", Class: adapter.DestProjection, Content: []byte("x"),
		}},
	})
	if !errors.Is(err, ErrPathEscape) {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "pwned")); err == nil {
		t.Fatal("must not write through symlink")
	}
}

func mustAuth(r compatibility.Report, plans adapter.Plans) *Authorization {
	d, err := adapter.DigestPlans(plans)
	if err != nil {
		panic(err)
	}
	return &Authorization{
		Binding:     compatibility.BindingOf(r),
		Decision:    r.Decision,
		WarningSet:  compatibility.WarningSet(r),
		HostID:      r.Host.ID,
		Plans:       plans,
		PlansDigest: d,
		Executable:  plans.Launch.Executable,
	}
}

func sampleReport() compatibility.Report {
	return compatibility.Report{
		Decision:          "allowed",
		Agent2HostVersion: "0.0.0-dev",
		Subject:           compatibility.Subject{SystemID: "s", AgentID: "a", Revision: "sha256:1"},
		Host:              compatibility.HostRef{ID: "claude-code", Version: "1.0.0-test"},
		Adapter:           compatibility.AdapterRef{ID: "claude-code", Version: "0.1.0"},
		Probe:             compatibility.Probe{Fingerprint: "sha256:" + strings.Repeat("a", 64)},
	}
}

func assertWorkspaceGone(t *testing.T, p Prepared) {
	t.Helper()
	if _, err := os.Stat(p.Root); err == nil {
		t.Fatal("temporary run workspace must be gone after Execute")
	}
}

func TestWipeSecretsRestoresPlaceholders(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rel := "host-config/mcp.json"
	plan := adapter.NativeProjectionPlan{
		Files: []adapter.ProjectionFile{{
			RelPath: rel,
			Class:   adapter.DestHostPrivate,
			Content: []byte(`{"T":"` + adapter.SecretPlaceholder("T") + `"}`),
		}},
	}
	if err := materialize(p, plan); err != nil {
		t.Fatal(err)
	}
	if err := overlaySecrets(p, plan, map[string]string{"T": "secret-xyz"}, nil); err != nil {
		t.Fatal(err)
	}
	overlaid, err := os.ReadFile(filepath.Join(p.Private, rel))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(overlaid, []byte("secret-xyz")) {
		t.Fatalf("overlay missing secret: %s", overlaid)
	}
	if err := wipeSecrets(p, plan, map[string]string{"T": "secret-xyz"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(p.Private, rel))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(after, []byte("secret-xyz")) {
		t.Fatalf("wipe must remove secret: %s", after)
	}
	if !bytes.Contains(after, []byte(adapter.SecretPlaceholder("T"))) {
		t.Fatalf("wipe must restore placeholder: %s", after)
	}
}

func TestOverlayDoesNotWriteSecretsIntoDestProjection(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rel := "kiro-home/agents/example-agent.json"
	plan := adapter.NativeProjectionPlan{
		Files: []adapter.ProjectionFile{{
			RelPath: rel,
			Class:   adapter.DestProjection,
			Content: []byte(`{"env":{"T":"` + adapter.SecretPlaceholder("T") + `"}}` + "\n"),
		}},
	}
	if err := materialize(p, plan); err != nil {
		t.Fatal(err)
	}
	if err := overlaySecrets(p, plan, map[string]string{"T": "secret-xyz"}, nil); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(p.Workspace, rel))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte("secret-xyz")) {
		t.Fatalf("DestProjection must not receive secret bytes: %s", body)
	}
	if !bytes.Contains(body, []byte(adapter.SecretPlaceholder("T"))) {
		t.Fatalf("DestProjection must keep the slot, got %s", body)
	}
	if err := wipeSecrets(p, plan, map[string]string{"T": "secret-xyz"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(p.Workspace, rel))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(after, []byte("secret-xyz")) {
		t.Fatalf("wipe left secret bytes: %s", after)
	}
}

func TestScrubSkipsShortValuesAndKeepsExecutable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hook.sh")
	body := []byte("echo dev secret-xyz leftover\n")
	if err := os.WriteFile(path, body, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := scrubSecretValuesInFile(path, map[string]string{
		"SHORT": "dev",
		"LONG":  "secret-xyz",
	}, 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("dev")) {
		t.Fatalf("short value must stay: %s", got)
	}
	if bytes.Contains(got, []byte("secret-xyz")) {
		t.Fatalf("long value must be removed: %s", got)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o700 {
		t.Fatalf("mode %o", st.Mode().Perm())
	}
}
