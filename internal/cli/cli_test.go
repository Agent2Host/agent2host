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

func TestCLIRegisterListInspect(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", "club-system")
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
	if reg["system_id"] != "club-system" {
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
	if len(listed.Systems) != 1 || listed.Systems[0].ID != "club-system" || listed.Systems[0].AgentCount != 2 {
		t.Fatalf("list %+v", listed)
	}

	out.Reset()
	errb.Reset()
	code = cli.Main([]string{"a2h", "--home", home, "--json", "inspect", "club-system/club-faq"}, &out, &errb)
	if code != 0 {
		t.Fatalf("inspect %d: %s", code, errb.String())
	}
	var ins map[string]any
	if err := json.Unmarshal(out.Bytes(), &ins); err != nil {
		t.Fatal(err)
	}
	if ins["agent_id"] != "club-faq" {
		t.Fatalf("inspect %+v", ins)
	}
	skills, _ := ins["skills"].([]any)
	if len(skills) != 2 {
		t.Fatalf("skills %v", skills)
	}

	out.Reset()
	errb.Reset()
	code = cli.Main([]string{"a2h", "--home", home, "resolve", "club-system/club-faq"}, &out, &errb)
	if code != 0 {
		t.Fatalf("resolve %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), `"system_id": "club-system"`) {
		t.Fatalf("resolve json %s", out.String())
	}
	if strings.Contains(out.String(), "unused-declared") {
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
	code := cli.Main([]string{"a2h", "--home", t.TempDir(), "check", "club-system/club-faq"}, &out, &errb)
	if code != cli.ExitUsage || out.Len() != 0 || !strings.Contains(errb.String(), "--host") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func TestCLICheckUnknownHost(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", "club-system")
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "club-system/club-faq", "--host", "not-a-host"}, &out, &errb)
	if code != cli.ExitPrecondition || out.Len() != 0 || !strings.Contains(errb.String(), "unsupported host") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func TestCLICheckJSONPlansNotMaterialized(t *testing.T) {
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
	restore := cli.SetCheckHostsForTest(committed.New(
		func(file string) (string, error) { return "/opt/" + file, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()

	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "fm/demo", "--host", "claude-code"}, &out, &errb)
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
	restore := cli.SetCheckHostsForTest(committed.New(
		func(file string) (string, error) { return "/opt/" + file, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()

	out.Reset()
	errb.Reset()
	code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "fm/demo", "--host", "codex"}, &out, &errb)
	if code != 0 {
		t.Fatalf("codex baseline check must exit 0 (allowed or allowed_with_warnings), got %d: %s\n%s", code, errb.String(), out.String())
	}
	report := decodeCheckReport(t, out.Bytes())
	if report.Decision == "refused" {
		t.Fatalf("codex baseline should not refuse, got %s", report.Decision)
	}
}

func TestCLICheckClubFAQProjectsScopedSecrets(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", "club-system")
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
	code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "club-system/club-faq", "--host", "claude-code"}, &out, &errb)
	if code != 0 {
		t.Fatalf("scoped MCP/hook secrets must allow, got %d %s", code, out.String())
	}
	report := decodeCheckReport(t, out.Bytes())
	if report.Decision == "refused" {
		t.Fatalf("club-faq on claude-code must be allowed, got %s", report.Decision)
	}
	if bytes.Contains(out.Bytes(), []byte("token-value")) || bytes.Contains(out.Bytes(), []byte("audit-value")) {
		t.Fatal("secret values leaked in check output")
	}
}

func TestCLICheckClubFAQAllCommittedHosts(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", "club-system")
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

	want := map[string]string{
		"claude-code": "allowed",
		"kiro":        "allowed",
		"codex":       "allowed_with_warnings",
	}
	for host, decision := range want {
		out.Reset()
		errb.Reset()
		code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "club-system/club-faq", "--host", host}, &out, &errb)
		if code != 0 {
			t.Fatalf("%s check exit %d: %s", host, code, errb.String())
		}
		report := decodeCheckReport(t, out.Bytes())
		if report.Decision != decision {
			t.Fatalf("%s decision %q want %q", host, report.Decision, decision)
		}
	}
}

func TestCLICheckRefusedNoPlans(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", "club-system")
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
	code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "club-system/club-faq", "--host", "claude-code"}, &out, &errb)
	if code != cli.ExitRefused {
		t.Fatalf("want exit 1, got %d %s", code, errb.String())
	}
	report := decodeCheckReport(t, out.Bytes())
	if report.Decision != "refused" {
		t.Fatalf("refused must emit a Report, got %s", report.Decision)
	}
}

func registerClubFAQ(t *testing.T) (home string, restore func()) {
	t.Helper()
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", "club-system")
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
	home, restore := registerClubFAQ(t)
	defer restore()
	hosts := []string{"claude-code", "kiro", "codex"}
	for _, host := range hosts {
		var out, errb bytes.Buffer
		code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "club-system/club-faq", "--host", host, "--require-strict-read"}, &out, &errb)
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

func TestCLICheckSecurityStrictGuard(t *testing.T) {
	home := registerAcceptanceFixture(t, "security-strict")
	restore := cli.SetCheckHostsForTest(committed.New(
		func(file string) (string, error) { return "/opt/" + file, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()
	want := map[string]string{
		"claude-code": "allowed",
		"kiro":        "refused",
		"codex":       "refused",
	}
	for host, decision := range want {
		var out, errb bytes.Buffer
		code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "security-strict/guard", "--host", host}, &out, &errb)
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

func registerAcceptanceFixture(t *testing.T, name string) string {
	t.Helper()
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	mod, err := filepath.Abs(filepath.Join(root, "..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(mod, "test", "acceptance", name)
	if _, err := os.Stat(filepath.Join(src, "system.json")); err != nil {
		t.Skipf("local test/acceptance/%s not present (test/ is gitignored): %v", name, err)
	}
	home := t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s: %s", errb.String(), out.String())
	}
	return home
}

func TestCLICheckWithoutStrictReadStillAllowed(t *testing.T) {
	home, restore := registerClubFAQ(t)
	defer restore()
	want := map[string]string{
		"claude-code": "allowed",
		"kiro":        "allowed",
		"codex":       "allowed_with_warnings",
	}
	for host, decision := range want {
		var out, errb bytes.Buffer
		code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "club-system/club-faq", "--host", host}, &out, &errb)
		if code != 0 {
			t.Fatalf("%s check exit %d: %s", host, code, errb.String())
		}
		if !strings.Contains(errb.String(), "not strictly confined") {
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
