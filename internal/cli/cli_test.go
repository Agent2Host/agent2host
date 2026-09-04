package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/adapter/committed"
	"github.com/agent2host/agent2host/internal/cli"
	"github.com/agent2host/agent2host/internal/source/fixtures"
)

func officialSystem(t *testing.T, name string) string {
	t.Helper()
	src, err := fixtures.OfficialSystem(name)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func TestCLIRegisterListInspect(t *testing.T) {
	src := officialSystem(t, "dev-studio")
	home := t.TempDir()

	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "--json", "register", src}, &out, &errb)
	if code != 0 {
		t.Fatalf("register %d: %s", code, errb.String())
	}
	var reg map[string]any
	if err := json.Unmarshal(out.Bytes(), &reg); err != nil {
		t.Fatal(err)
	}
	if reg["system_id"] != "dev-studio" {
		t.Fatalf("register json %+v", reg)
	}
	if _, ok := reg["warnings"]; !ok {
		t.Fatal("register --json must include warnings")
	}

	out.Reset()
	errb.Reset()
	code = cli.Main([]string{"a2h", "--home", home, "list", "--json"}, &out, &errb)
	if code != 0 {
		t.Fatalf("list %d: %s", code, errb.String())
	}
	var listed struct {
		Systems []struct {
			ID             string `json:"id"`
			AgentCount     int    `json:"agent_count"`
			ActiveRevision string `json:"active_revision"`
		} `json:"systems"`
	}
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Systems) != 1 || listed.Systems[0].ID != "dev-studio" || listed.Systems[0].AgentCount != 5 {
		t.Fatalf("list %+v", listed)
	}

	out.Reset()
	errb.Reset()
	code = cli.Main([]string{"a2h", "--home", home, "--json", "inspect", "dev-studio/code-reviewer"}, &out, &errb)
	if code != 0 {
		t.Fatalf("inspect %d: %s", code, errb.String())
	}
	var ins map[string]any
	if err := json.Unmarshal(out.Bytes(), &ins); err != nil {
		t.Fatal(err)
	}
	if ins["agent_id"] != "code-reviewer" {
		t.Fatalf("inspect %+v", ins)
	}
	skills, _ := ins["skills"].([]any)
	if len(skills) != 2 {
		t.Fatalf("skills %v", skills)
	}

	out.Reset()
	errb.Reset()
	code = cli.Main([]string{"a2h", "--home", home, "resolve", "dev-studio/code-reviewer"}, &out, &errb)
	if code != 0 {
		t.Fatalf("resolve %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), `"system_id": "dev-studio"`) {
		t.Fatalf("resolve json %s", out.String())
	}
	if strings.Contains(out.String(), "unused-catalog") {
		t.Fatal("resolve JSON must not include unreferenced skill")
	}
}

func TestCLIVerboseOnlyOnRun(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", t.TempDir(), "--verbose", "check", "s/a", "--host", "claude-code"}, &out, &errb)
	if code != cli.ExitUsage || out.Len() != 0 || !strings.Contains(errb.String(), "--verbose is only valid with run") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func TestCLIListRejectsHostFlag(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", t.TempDir(), "list", "--host", "claude-code"}, &out, &errb)
	if code != cli.ExitUsage || out.Len() != 0 || !strings.Contains(errb.String(), "--host is not valid with list") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func TestCLIRegisterShowsWarnings(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", "filename-id-mismatch")
	home := t.TempDir()
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb)
	if code != 0 {
		t.Fatalf("register %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "registered fn-mismatch") {
		t.Fatalf("stdout %q", out.String())
	}
	if !strings.Contains(errb.String(), "SRC-REF-FILENAME") {
		t.Fatalf("stderr must show register warning: %q", errb.String())
	}

	out.Reset()
	errb.Reset()
	code = cli.Main([]string{"a2h", "--home", home, "--json", "register", src}, &out, &errb)
	if code != 0 {
		t.Fatalf("re-register %d: %s", code, errb.String())
	}
	var reg struct {
		Warnings []struct {
			ID     string `json:"id"`
			Detail string `json:"detail"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(out.Bytes(), &reg); err != nil {
		t.Fatal(err)
	}
	if len(reg.Warnings) == 0 || reg.Warnings[0].ID != "SRC-REF-FILENAME" {
		t.Fatalf("warnings %+v", reg.Warnings)
	}
}

func TestCLIRemoveUnregistersBeforeInvalidate(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", "markdown-leading-dashes")
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	left := filepath.Join(home, "runs", "abandoned")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(left, "begun"), []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "records"), []byte("not-a-dir"), 0o600); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "remove", "fm"}, &out, &errb)
	if code != cli.ExitPrecondition {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "removed fm") || !strings.Contains(errb.String(), "a2h clean") {
		t.Fatalf("stderr %q", errb.String())
	}
	if _, err := os.Stat(left); err != nil {
		t.Fatal("leftover must remain when leftover cleanup fails")
	}

	out.Reset()
	errb.Reset()
	if cli.Main([]string{"a2h", "--home", home, "list"}, &out, &errb) != 0 {
		t.Fatalf("list %s", errb.String())
	}
	if strings.Contains(out.String(), "fm") {
		t.Fatalf("system must already be unregistered: %s", out.String())
	}
}

func TestCLIUnknownCommand(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "nope"}, &out, &errb)
	if code != cli.ExitUsage {
		t.Fatalf("code %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout must be empty on failure: %q", out.String())
	}
	if errb.Len() == 0 {
		t.Fatal("stderr must be non-empty")
	}
}

func TestCLIHelp(t *testing.T) {
	for _, args := range [][]string{{"a2h", "--help"}, {"a2h", "-h"}, {"a2h", "help"}} {
		var out, errb bytes.Buffer
		code := cli.Main(args, &out, &errb)
		if code != 0 {
			t.Fatalf("%v: code %d stderr=%q", args, code, errb.String())
		}
		if !strings.Contains(out.String(), "register <system-source-dir>") || !strings.Contains(out.String(), "help") {
			t.Fatalf("%v: stdout %q", args, out.String())
		}
		if strings.Contains(out.String(), "resolve") {
			t.Fatalf("%v: help must not list resolve", args)
		}
		if errb.Len() != 0 {
			t.Fatalf("%v: stderr %q", args, errb.String())
		}
	}
}

func TestCLICommandHelp(t *testing.T) {
	for _, args := range [][]string{
		{"a2h", "run", "--help"},
		{"a2h", "--help", "run"},
		{"a2h", "help", "run"},
	} {
		var out, errb bytes.Buffer
		code := cli.Main(args, &out, &errb)
		if code != 0 {
			t.Fatalf("%v: code %d stderr=%q", args, code, errb.String())
		}
		if !strings.Contains(out.String(), "a2h run") || !strings.Contains(out.String(), "--project") {
			t.Fatalf("%v: stdout %q", args, out.String())
		}
		if strings.Contains(out.String(), "resolve") {
			t.Fatalf("%v: must not list resolve", args)
		}
	}
}

func TestCLIUsageOmitsResolve(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h"}, &out, &errb)
	if code != cli.ExitUsage {
		t.Fatalf("code %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout %q", out.String())
	}
	if strings.Contains(errb.String(), "resolve") {
		t.Fatalf("public usage must not list resolve: %s", errb.String())
	}
}

func TestCLIFailureEmptyStdout(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", t.TempDir(), "inspect", "noshift"}, &out, &errb)
	if code != cli.ExitUsage || out.Len() != 0 || errb.Len() == 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func TestCLICheckRequiresHost(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", t.TempDir(), "check", "dev-studio/code-reviewer"}, &out, &errb)
	if code != cli.ExitUsage || out.Len() != 0 || !strings.Contains(errb.String(), "--host") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func TestCLICheckUnknownHost(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", "markdown-leading-dashes")
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "fm/demo", "--host", "not-a-host"}, &out, &errb)
	if code != cli.ExitPrecondition || out.Len() != 0 || !strings.Contains(errb.String(), "unsupported host") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func TestCLICheckJSONPlansNotMaterialized(t *testing.T) {
	src := officialSystem(t, "research-lab")
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	restore := cli.SetCheckHostsForTest(committed.New(
		func(file string) (string, error) { return "/opt/" + file, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()

	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "research-lab/web-researcher", "--host", "claude-code"}, &out, &errb)
	if code != 0 {
		t.Fatalf("check %d: %s\n%s", code, errb.String(), out.String())
	}
	report := decodeCheckReport(t, out.Bytes())
	if report.Decision == "refused" {
		t.Fatalf("decision %s", report.Decision)
	}
	if _, err := os.Stat(filepath.Join(home, "CLAUDE.md")); err == nil {
		t.Fatal("check must not materialize Host files under --home")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); err == nil {
		t.Fatal("check must not write .claude under --home")
	}
}

func TestCLICheckCodexAllowedWithWarnings(t *testing.T) {
	src := officialSystem(t, "dev-studio")
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	restore := cli.SetCheckHostsForTest(committed.New(
		func(file string) (string, error) { return "/opt/" + file, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()

	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "dev-studio/code-reviewer", "--host", "codex"}, &out, &errb)
	if code != 0 {
		t.Fatalf("codex baseline check must exit 0 (allowed or allowed_with_warnings), got %d: %s\n%s", code, errb.String(), out.String())
	}
	report := decodeCheckReport(t, out.Bytes())
	if report.Decision == "refused" {
		t.Fatalf("codex baseline should not refuse, got %s", report.Decision)
	}
}

func TestCLICheckProjectsScopedSecrets(t *testing.T) {
	home, restore := registerOfficialCLI(t, "dev-studio")
	defer restore()
	t.Setenv("A2H_TEST_TOKEN", "token-value-must-not-be-recorded")
	t.Setenv("A2H_TEST_HOOK_TOKEN", "audit-value-must-not-be-recorded")
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "dev-studio/code-reviewer", "--host", "claude-code"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code-reviewer default network deny on claude-code must start with warnings, got %d %s", code, out.String())
	}
	report := decodeCheckReport(t, out.Bytes())
	if report.Decision == "refused" {
		t.Fatalf("code-reviewer on claude-code must not be refused, got %s", report.Decision)
	}
	if bytes.Contains(out.Bytes(), []byte("token-value")) || bytes.Contains(out.Bytes(), []byte("audit-value")) {
		t.Fatal("secret values leaked in check output")
	}
}

func TestCLICheckCodeReviewerAllCommittedHosts(t *testing.T) {
	home, restore := registerOfficialCLI(t, "dev-studio")
	defer restore()

	want := map[string]string{
		"claude-code": "allowed_with_warnings",
		"kiro":        "allowed_with_warnings",
		"codex":       "allowed_with_warnings",
	}
	for host, decision := range want {
		var out, errb bytes.Buffer
		code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "dev-studio/code-reviewer", "--host", host}, &out, &errb)
		if decision == "refused" {
			if code != cli.ExitRefused {
				t.Fatalf("%s check exit %d: %s", host, code, errb.String())
			}
		} else if code != 0 {
			t.Fatalf("%s check exit %d: %s", host, code, errb.String())
		}
		report := decodeCheckReport(t, out.Bytes())
		if report.Decision != decision {
			t.Fatalf("%s decision %q want %q", host, report.Decision, decision)
		}
	}
}

func TestCLICheckRefusedNoPlans(t *testing.T) {
	src := officialSystem(t, "dev-studio")
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	restore := cli.SetCheckHostsForTest(committed.New(
		func(string) (string, error) { return "", os.ErrNotExist },
		func(string) (string, error) { return "", os.ErrNotExist },
	))
	defer restore()

	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "dev-studio/code-reviewer", "--host", "claude-code"}, &out, &errb)
	if code != cli.ExitRefused {
		t.Fatalf("want exit 1, got %d %s", code, errb.String())
	}
	report := decodeCheckReport(t, out.Bytes())
	if report.Decision != "refused" {
		t.Fatalf("refused must emit a Report, got %s", report.Decision)
	}
}

func registerOfficialCLI(t *testing.T, system string) (home string, restore func()) {
	t.Helper()
	src := officialSystem(t, system)
	home = t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	restore = cli.SetCheckHostsForTest(committed.New(
		func(file string) (string, error) { return "/opt/" + file, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	return home, restore
}

func TestCLICheckStrictReadRefusesAllCommittedHosts(t *testing.T) {
	home, restore := registerOfficialCLI(t, "dev-studio")
	defer restore()
	hosts := []string{"claude-code", "kiro", "codex"}
	for _, host := range hosts {
		var out, errb bytes.Buffer
		code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "dev-studio/code-reviewer", "--host", host, "--require-strict-read"}, &out, &errb)
		if code != cli.ExitRefused {
			t.Fatalf("%s strict check exit %d stderr=%s stdout=%s", host, code, errb.String(), out.String())
		}
		if !strings.Contains(errb.String(), "Read protection:") {
			t.Fatalf("%s missing read protection line: %s", host, errb.String())
		}
		report := decodeCheckReport(t, out.Bytes())
		if report.Decision != "refused" {
			t.Fatalf("%s decision %q", host, report.Decision)
		}
	}
}

func TestCLICheckDeployGuard(t *testing.T) {
	home, restore := registerOfficialCLI(t, "ops-desk")
	defer restore()
	want := map[string]string{
		"claude-code": "allowed_with_warnings",
		"kiro":        "refused",
		"codex":       "allowed_with_warnings",
	}
	for host, decision := range want {
		var out, errb bytes.Buffer
		code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "ops-desk/deploy-guard", "--host", host}, &out, &errb)
		if decision == "refused" && code != cli.ExitRefused {
			t.Fatalf("%s exit %d want refused", host, code)
		}
		if decision != "refused" && code != 0 {
			t.Fatalf("%s exit %d stderr=%s", host, code, errb.String())
		}
		report := decodeCheckReport(t, out.Bytes())
		if report.Decision != decision {
			t.Fatalf("%s decision %q want %q", host, report.Decision, decision)
		}
	}
}

func TestCLICheckWithoutStrictReadStillAllowed(t *testing.T) {
	home, restore := registerOfficialCLI(t, "dev-studio")
	defer restore()
	want := map[string]string{
		"claude-code": "allowed_with_warnings",
		"kiro":        "allowed_with_warnings",
		"codex":       "allowed_with_warnings",
	}
	for host, decision := range want {
		var out, errb bytes.Buffer
		code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "dev-studio/code-reviewer", "--host", host}, &out, &errb)
		if decision == "refused" {
			if code != cli.ExitRefused {
				t.Fatalf("%s check exit %d: %s", host, code, errb.String())
			}
		} else if code != 0 {
			t.Fatalf("%s check exit %d: %s", host, code, errb.String())
		}
		if decision != "refused" && !strings.Contains(errb.String(), "not strictly confined") {
			t.Fatalf("%s missing ordinary read protection: %s", host, errb.String())
		}
		report := decodeCheckReport(t, out.Bytes())
		if report.Decision != decision {
			t.Fatalf("%s decision %q want %q", host, report.Decision, decision)
		}
	}
}

func TestCLIJSONSuccessIsObject(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", "markdown-leading-dashes")
	home := t.TempDir()
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "--json", "register", src}, &out, &errb)
	if code != 0 {
		t.Fatalf("register %d: %s", code, errb.String())
	}
	if errb.Len() != 0 {
		t.Fatalf("stderr on success: %s", errb.String())
	}
	var v map[string]any
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v["system_id"] != "fm" {
		t.Fatalf("%+v", v)
	}
}

func TestCLIRemoveAndClean(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", "markdown-leading-dashes")
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	hostState := filepath.Join(home, "auth-profiles", "claude-code", "keep")
	if err := os.MkdirAll(filepath.Dir(hostState), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hostState, []byte("login"), 0o600); err != nil {
		t.Fatal(err)
	}
	q := filepath.Join(home, "quarantine", "bad")
	if err := os.MkdirAll(q, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(q, "x"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	left := filepath.Join(home, "runs", "old")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(left, "recovered"), []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "--dry-run", "clean"}, &out, &errb)
	if code != 0 {
		t.Fatalf("dry-run %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), left) {
		t.Fatalf("dry-run should list leftover run: %s", out.String())
	}
	if _, err := os.Stat(left); err != nil {
		t.Fatal("dry-run must not delete")
	}

	out.Reset()
	errb.Reset()
	if cli.Main([]string{"a2h", "--home", home, "clean"}, &out, &errb) != 0 {
		t.Fatalf("clean %s", errb.String())
	}
	if _, err := os.Stat(left); err == nil {
		t.Fatal("default clean should delete leftover run")
	}
	if _, err := os.Stat(q); err != nil {
		t.Fatal("default clean must keep quarantine")
	}
	if _, err := os.Stat(hostState); err != nil {
		t.Fatal("default clean must keep host state")
	}

	out.Reset()
	errb.Reset()
	if cli.Main([]string{"a2h", "--home", home, "remove", "fm"}, &out, &errb) != 0 {
		t.Fatalf("remove %s", errb.String())
	}
	out.Reset()
	errb.Reset()
	if cli.Main([]string{"a2h", "--home", home, "list"}, &out, &errb) != 0 {
		t.Fatalf("list %s", errb.String())
	}
	if strings.Contains(out.String(), "fm") {
		t.Fatalf("removed system still listed: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(src, "system.json")); err != nil {
		t.Fatal("remove must keep user Source")
	}
	if _, err := os.Stat(hostState); err != nil {
		t.Fatal("remove must keep Host state")
	}
}

func TestCLICleanHostStateRequiresHost(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", t.TempDir(), "clean", "--host-state"}, &out, &errb)
	if code != cli.ExitUsage || !strings.Contains(errb.String(), "--host-state requires --host") {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
}

func TestCLICleanHostStateRejectsTraversal(t *testing.T) {
	home := t.TempDir()
	outside := filepath.Join(home, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(outside, "keep")
	if err := os.WriteFile(keep, []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "clean", "--host-state", "--host", "../outside"}, &out, &errb)
	if code != cli.ExitUsage {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("must not delete a path outside Host state")
	}
}

func writeMiniSystem(t *testing.T, workRoot string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sops"), 0o755); err != nil {
		t.Fatal(err)
	}
	sys := `{
  "schema_version": "agent2host/v1alpha2",
  "kind": "AgentSystem",
  "id": "mini-sys",
  "version": "0.1.0",
  "agents": ["./agents/demo.agent.json"],
  "work_root": ` + workRoot + `
}`
	if err := os.WriteFile(filepath.Join(dir, "system.json"), []byte(sys), 0o644); err != nil {
		t.Fatal(err)
	}
	agent := `{
  "schema_version": "agent2host/v1alpha1",
  "kind": "Agent",
  "id": "demo",
  "name": "Demo",
  "sop": "./sops/demo.sop.md"
}`
	if err := os.WriteFile(filepath.Join(dir, "agents", "demo.agent.json"), []byte(agent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sops", "demo.sop.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCLICheckPrintsInvocationWorkRoot(t *testing.T) {
	src := writeMiniSystem(t, `{"mode":"invocation"}`)
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	restore := cli.SetCheckHostsForTest(committed.New(
		func(file string) (string, error) { return "/opt/" + file, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()
	project := t.TempDir()
	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "check", "mini-sys/demo", "--host", "claude-code", "--project", project}, &out, &errb)
	if code != 0 {
		t.Fatalf("check %d: %s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "Work root (invocation):") {
		t.Fatalf("stderr %q", errb.String())
	}
	if !strings.Contains(errb.String(), project) && !strings.Contains(errb.String(), filepath.Base(project)) {
		t.Fatalf("project path missing: %q", errb.String())
	}
}

func TestCLICheckFixedRejectsProject(t *testing.T) {
	src := writeMiniSystem(t, `{"mode":"fixed","path_from_home":"Desktop/A2HWorkRootTest"}`)
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	restore := cli.SetCheckHostsForTest(committed.New(
		func(file string) (string, error) { return "/opt/" + file, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()
	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "check", "mini-sys/demo", "--host", "claude-code", "--project", t.TempDir()}, &out, &errb)
	if code != cli.ExitUsage {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "invocation") {
		t.Fatalf("stderr %q", errb.String())
	}
}

func TestCLICheckFixedDoesNotCreate(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	rel := "A2HWorkRootCheckNoCreate"
	abs := filepath.Join(homeDir, rel)
	if err := os.RemoveAll(abs); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(abs) })

	src := writeMiniSystem(t, `{"mode":"fixed","path_from_home":"`+rel+`"}`)
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	restore := cli.SetCheckHostsForTest(committed.New(
		func(file string) (string, error) { return "/opt/" + file, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()
	out.Reset()
	errb.Reset()
	if cli.Main([]string{"a2h", "--home", home, "check", "mini-sys/demo", "--host", "claude-code"}, &out, &errb) != 0 {
		t.Fatalf("check %s", errb.String())
	}
	if _, err := os.Stat(abs); err == nil {
		t.Fatal("check must not create a missing fixed work root")
	}
}

func TestCLIRunFixedDoesNotCreateWhenRefused(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	rel := "A2HWorkRootRunNoCreate"
	abs := filepath.Join(homeDir, rel)
	if err := os.RemoveAll(abs); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(abs) })

	src := writeMiniSystem(t, `{"mode":"fixed","path_from_home":"`+rel+`"}`)
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	restore := cli.SetCheckHostsForTest(committed.New(
		func(file string) (string, error) { return "/opt/" + file, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()
	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "run", "mini-sys/demo", "--host", "not-a-host"}, &out, &errb)
	if code == 0 {
		t.Fatal("unknown host must not run")
	}
	if _, err := os.Stat(abs); err == nil {
		t.Fatal("run must not create a fixed work root when start is refused")
	}
}

func TestCLIRunFixedCreatesAfterAuthorize(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	rel := "A2HWorkRootRunCreate"
	abs := filepath.Join(homeDir, rel)
	if err := os.RemoveAll(abs); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(abs) })

	src := writeMiniSystem(t, `{"mode":"fixed","path_from_home":"`+rel+`"}`)
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	stub := filepath.Join(t.TempDir(), "host")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := cli.SetCheckHostsForTest(committed.New(
		func(string) (string, error) { return stub, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()
	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "--accept-warnings", "run", "mini-sys/demo", "--host", "claude-code"}, &out, &errb)
	if code != 0 {
		t.Fatalf("run %d: %s", code, errb.String())
	}
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		t.Fatalf("authorized run must create the fixed work root: %v", err)
	}
}
