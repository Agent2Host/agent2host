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

func TestCLIRunJSONLaunchesStub(t *testing.T) {
	home, stub := registerTree(t, "markdown-leading-dashes")
	restore := cli.SetCheckHostsForTest(committed.New(
		func(string) (string, error) { return stub, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()

	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "--json", "--accept-warnings", "run", "fm/demo", "--host", "claude-code"}, &out, &errb)
	if code != 0 {
		t.Fatalf("run %d: %s\n%s", code, errb.String(), out.String())
	}
	var doc struct {
		Kind   string `json:"kind"`
		Report struct {
			Decision string `json:"decision"`
		} `json:"report"`
		Outcome struct {
			Class    string `json:"class"`
			ExitCode int    `json:"exit_code"`
			RunID    string `json:"run_id"`
		} `json:"outcome"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("%s: %v", out.String(), err)
	}
	if doc.Kind != "run" || doc.Report.Decision == "refused" || doc.Outcome.Class != "host_process" || doc.Outcome.ExitCode != 0 {
		t.Fatalf("%+v", doc)
	}
	if strings.Contains(out.String(), "token-value") || strings.Contains(errb.String(), "token-value") {
		t.Fatal("secret values must not appear")
	}
	if _, err := os.Stat(filepath.Join(home, "runs", doc.Outcome.RunID)); err == nil {
		t.Fatal("successful run must not leave a temporary run workspace")
	}
	if _, err := os.Stat(filepath.Join(home, "CLAUDE.md")); err == nil {
		t.Fatal("run must not write CLAUDE.md at home root")
	}
	rec, err := os.ReadFile(filepath.Join(home, "records", doc.Outcome.RunID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(rec, []byte("token-value")) || bytes.Contains(rec, []byte("audit-value")) {
		t.Fatalf("record leaked secrets: %s", rec)
	}
}

func TestCLIRunDefaultUsesHumanSessionMessages(t *testing.T) {
	home, stub := registerTree(t, "markdown-leading-dashes")
	restore := cli.SetCheckHostsForTest(committed.New(
		func(string) (string, error) { return stub, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()

	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "run", "fm/demo", "--host", "claude-code"}, &out, &errb)
	if code != cli.ExitOK {
		t.Fatalf("run %d: stdout=%q stderr=%q", code, out.String(), errb.String())
	}
	for _, want := range []string{"Starting demo in Claude Code…", "host-ok", "Session ended."} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout missing %q: %q", want, out.String())
		}
	}
	if !strings.Contains(errb.String(), "This session can read files outside your project folder.") {
		t.Fatalf("startup read notice missing: %q", errb.String())
	}
	for _, internal := range []string{"decision:", "run:", "class:", "exit:"} {
		if strings.Contains(out.String(), internal) || strings.Contains(errb.String(), internal) {
			t.Fatalf("default output leaked internal field %q: stdout=%q stderr=%q", internal, out.String(), errb.String())
		}
	}
}

func TestCLIRunRejectsJSONAndVerboseTogether(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--json", "--verbose", "run", "s/a", "--host", "claude-code"}, &out, &errb)
	if code != cli.ExitUsage || !strings.Contains(errb.String(), "--json and --verbose cannot be used together") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func TestCLIRunUnexpectedHostExitUsesHumanMessage(t *testing.T) {
	home, _ := registerTree(t, "markdown-leading-dashes")
	failingHost := filepath.Join(t.TempDir(), "host-fails")
	if err := os.WriteFile(failingHost, []byte("#!/bin/sh\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := cli.SetCheckHostsForTest(committed.New(
		func(string) (string, error) { return failingHost, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()

	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "run", "fm/demo", "--host", "claude-code"}, &out, &errb)
	if code != cli.ExitHostProcess {
		t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, cli.ExitHostProcess, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "Session ended unexpectedly. Run again with --verbose if you need details.") {
		t.Fatalf("unexpected-exit message missing: %q", errb.String())
	}
	if strings.Contains(errb.String(), "exit:") || strings.Contains(errb.String(), "run:") {
		t.Fatalf("default error leaked internal details: %q", errb.String())
	}
}

func TestCLIRunClubFAQScopedSecrets(t *testing.T) {
	home, stub := registerTree(t, "club-system")
	restore := cli.SetCheckHostsForTest(committed.New(
		func(string) (string, error) { return stub, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()
	t.Setenv("CLUB_DB_TOKEN", "token-value-must-not-be-recorded")
	t.Setenv("AUDIT_TOKEN", "audit-value-must-not-be-recorded")
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "--json", "--accept-warnings", "run", "club-system/club-faq", "--host", "claude-code"}, &out, &errb)
	if code != 0 {
		t.Fatalf("scoped MCP/hook secrets must run, got %d:\n%s\n%s", code, errb.String(), out.String())
	}
	if strings.Contains(out.String(), "token-value-must-not") || strings.Contains(out.String(), "audit-value-must-not") {
		t.Fatal("secret values leaked")
	}
	var doc struct {
		Report struct {
			Decision string `json:"decision"`
		} `json:"report"`
		Outcome struct {
			Class string `json:"class"`
			RunID string `json:"run_id"`
		} `json:"outcome"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Report.Decision == "refused" || doc.Outcome.Class != "host_process" {
		t.Fatalf("%+v", doc)
	}
	if _, err := os.Stat(filepath.Join(home, "runs", doc.Outcome.RunID)); err == nil {
		t.Fatal("successful run must not leave a temporary run workspace")
	}
	rec, err := os.ReadFile(filepath.Join(home, "records", doc.Outcome.RunID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(rec, []byte("token-value")) || bytes.Contains(rec, []byte("audit-value")) {
		t.Fatalf("record leaked secrets: %s", rec)
	}
}

func TestCLIRunNativeArgsRejected(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", t.TempDir(), "run", "s/a", "--host", "claude-code", "--", "--danger"}, &out, &errb)
	if code != cli.ExitUsage || !strings.Contains(errb.String(), "native args") {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
}

func TestCLIRunVerboseShowsDetailsAfterUnexpectedExit(t *testing.T) {
	home, _ := registerTree(t, "markdown-leading-dashes")
	failingHost := filepath.Join(t.TempDir(), "host-fails")
	if err := os.WriteFile(failingHost, []byte("#!/bin/sh\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := cli.SetCheckHostsForTest(committed.New(
		func(string) (string, error) { return failingHost, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()

	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "--verbose", "run", "fm/demo", "--host", "claude-code"}, &out, &errb)
	if code != cli.ExitHostProcess {
		t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, cli.ExitHostProcess, out.String(), errb.String())
	}
	for _, want := range []string{"Session ended unexpectedly.", "Details:", "exit: 2"} {
		if !strings.Contains(errb.String(), want) {
			t.Fatalf("verbose unexpected-exit missing %q: %q", want, errb.String())
		}
	}
	if strings.Contains(out.String(), "Details:") {
		t.Fatalf("details must stay on stderr: %q", out.String())
	}
}

func TestCLIRunUnverifiedHostVersionHumanMessage(t *testing.T) {
	home, stub := registerTree(t, "club-system")
	restore := cli.SetCheckHostsForTest(committed.New(
		func(string) (string, error) { return stub, nil },
		func(string) (string, error) { return "9.9.9-unverified", nil },
	))
	defer restore()
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "run", "club-system/club-faq", "--host", "claude-code"}, &out, &errb)
	if code != cli.ExitRefused {
		t.Fatalf("unverified host version must refuse run, got %d stdout=%s stderr=%s", code, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "Not started. This Host cannot meet this Agent's requirements.") {
		t.Fatalf("human refuse message missing: stdout=%q stderr=%q", out.String(), errb.String())
	}
	if strings.Contains(out.String(), "Starting") || strings.Contains(errb.String(), "Starting") {
		t.Fatalf("refused run must not start a Host: stdout=%q stderr=%q", out.String(), errb.String())
	}
	assertNoRunDirs(t, home)
}

func TestCLIRunUnverifiedHostVersionRefuses(t *testing.T) {
	home, stub := registerTree(t, "club-system")
	restore := cli.SetCheckHostsForTest(committed.New(
		func(string) (string, error) { return stub, nil },
		func(string) (string, error) { return "9.9.9-unverified", nil },
	))
	defer restore()
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "--json", "--accept-warnings", "run", "club-system/club-faq", "--host", "claude-code"}, &out, &errb)
	if code != cli.ExitRefused {
		t.Fatalf("unverified host version must refuse run, got %d stdout=%s stderr=%s", code, out.String(), errb.String())
	}
	assertNoRunDirs(t, home)
}

func TestCLIRunRefusedNoLaunch(t *testing.T) {
	home, _ := registerTree(t, "club-system")
	restore := cli.SetCheckHostsForTest(committed.New(
		func(string) (string, error) { return "", os.ErrNotExist },
		func(string) (string, error) { return "", os.ErrNotExist },
	))
	defer restore()
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "--json", "run", "club-system/club-faq", "--host", "claude-code"}, &out, &errb)
	if code != cli.ExitRefused {
		t.Fatalf("code %d %s", code, out.String())
	}
	if strings.Contains(out.String(), `"kind": "run"`) && strings.Contains(out.String(), `"decision": "refused"`) {
		return
	}
	if !strings.Contains(out.String(), "refused") && !strings.Contains(errb.String(), "refused") && !strings.Contains(errb.String(), "unsupported") {
		t.Fatalf("stdout=%s stderr=%s", out.String(), errb.String())
	}
	assertNoRunDirs(t, home)
}

func TestCLIRunWarningsNotAcceptedLeavesNoRunDir(t *testing.T) {
	home, stub := registerTree(t, "club-system")
	restore := cli.SetCheckHostsForTest(committed.New(
		func(string) (string, error) { return stub, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "--json", "run", "club-system/club-faq", "--host", "codex"}, &out, &errb)
	if code == 0 {
		t.Fatalf("expected a refused or unaccepted warning run, got success: %s", out.String())
	}
	assertNoRunDirs(t, home)
}

func TestCLICheckLeavesNoRunDir(t *testing.T) {
	home, stub := registerTree(t, "club-system")
	restore := cli.SetCheckHostsForTest(committed.New(
		func(string) (string, error) { return stub, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	))
	defer restore()
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", home, "--json", "check", "club-system/club-faq", "--host", "claude-code"}, &out, &errb)
	if code != 0 && code != cli.ExitRefused {
		t.Fatalf("check %d: %s %s", code, out.String(), errb.String())
	}
	assertNoRunDirs(t, home)
}

func assertNoRunDirs(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, "runs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("runs/ must be empty, found %d entries", len(entries))
	}
}

func registerTree(t *testing.T, tree string) (home, stub string) {
	t.Helper()
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", tree)
	home = t.TempDir()
	var out, errb bytes.Buffer
	if cli.Main([]string{"a2h", "--home", home, "register", src}, &out, &errb) != 0 {
		t.Fatalf("register %s", errb.String())
	}
	stub = filepath.Join(t.TempDir(), "host")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho host-ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return home, stub
}
