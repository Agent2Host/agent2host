package cli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/committed"
	"github.com/agent2host/agent2host/internal/cli"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/space"
	"path/filepath"
)

func TestConfirmWarningsTable(t *testing.T) {
	r := compatibility.Report{
		Decision:          "allowed_with_warnings",
		Agent2HostVersion: "0.0.0-dev",
		Subject:           compatibility.Subject{SystemID: "s", AgentID: "a", Revision: "sha256:1"},
		Adapter:           compatibility.AdapterRef{Version: "0.1.0"},
		Probe:             compatibility.Probe{Fingerprint: "sha256:" + strings.Repeat("a", 64)},
		Activation:        compatibility.Activation{ReasonCode: "mapped_activation"},
	}
	allowed := r
	allowed.Decision = "allowed"
	allowed.Activation.ReasonCode = ""
	refused := r
	refused.Decision = "refused"

	t.Run("allowed no prompt", func(t *testing.T) {
		var errb bytes.Buffer
		acc, err := cli.ConfirmWarnings(allowed, false, false, strings.NewReader(""), &errb)
		if err != nil || !acc.Matches(allowed) || errb.Len() != 0 {
			t.Fatalf("allowed: acc=%+v err=%v stderr=%q", acc, err, errb.String())
		}
	})
	t.Run("refused ignores flag", func(t *testing.T) {
		_, err := cli.ConfirmWarnings(refused, true, true, strings.NewReader("y\n"), ioDiscard{})
		if !errors.Is(err, cli.ErrExecuteRefused) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("warnings non-TTY no flag", func(t *testing.T) {
		var errb bytes.Buffer
		_, err := cli.ConfirmWarnings(r, false, false, strings.NewReader(""), &errb)
		if !errors.Is(err, cli.ErrWarningsNotAccepted) || !strings.Contains(errb.String(), "--accept-warnings") {
			t.Fatalf("err=%v stderr=%q", err, errb.String())
		}
	})
	t.Run("warnings flag", func(t *testing.T) {
		acc, err := cli.ConfirmWarnings(r, true, false, strings.NewReader(""), ioDiscard{})
		if err != nil || !acc.Matches(r) {
			t.Fatalf("acc=%+v err=%v", acc, err)
		}
	})
	t.Run("warnings TTY yes", func(t *testing.T) {
		var errb bytes.Buffer
		acc, err := cli.ConfirmWarnings(r, false, true, strings.NewReader("yes\n"), &errb)
		if err != nil || !acc.Matches(r) || !strings.Contains(errb.String(), "standard interface") || !strings.Contains(errb.String(), "Start this session?") {
			t.Fatalf("acc=%+v err=%v stderr=%q", acc, err, errb.String())
		}
	})
	t.Run("warnings TTY no", func(t *testing.T) {
		_, err := cli.ConfirmWarnings(r, false, true, strings.NewReader("n\n"), ioDiscard{})
		if !errors.Is(err, cli.ErrWarningsNotAccepted) {
			t.Fatalf("got %v", err)
		}
	})
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func TestAcceptanceMismatchInvalidates(t *testing.T) {
	r := compatibility.Report{
		Decision:          "allowed_with_warnings",
		Agent2HostVersion: "0.0.0-dev",
		Subject:           compatibility.Subject{SystemID: "s", AgentID: "a", Revision: "sha256:1"},
		Adapter:           compatibility.AdapterRef{Version: "0.1.0"},
		Probe:             compatibility.Probe{Fingerprint: "sha256:" + strings.Repeat("a", 64)},
		Activation:        compatibility.Activation{ReasonCode: "mapped_activation"},
	}
	acc, err := cli.ConfirmWarnings(r, true, false, nil, ioDiscard{})
	if err != nil {
		t.Fatal(err)
	}
	changed := r
	changed.Probe.Fingerprint = "sha256:" + strings.Repeat("b", 64)
	if acc.Matches(changed) {
		t.Fatal("changed fingerprint must invalidate acceptance")
	}
	sameObject := r
	if !acc.Matches(sameObject) {
		t.Fatal("new object with same binding must still match")
	}
	_, err = cli.MintAuthorization(changed, adapter.Plans{}, acc, "codex")
	if !errors.Is(err, cli.ErrAcceptanceMismatch) {
		t.Fatalf("got %v", err)
	}
}

func TestAuthorizeExecuteCodexAllowedBaseline(t *testing.T) {
	reg, run := authorizeFixture(t)
	auth, report, err := cli.AuthorizeExecute(cli.AuthorizeOpts{
		Registry:          reg,
		Host:              adapter.HostCodex,
		Run:               run,
		Agent2HostVersion: "0.0.0-dev",
		AcceptWarnings:    true,
		Stderr:            ioDiscard{},
	})
	if err != nil || auth == nil || report.Decision == "refused" {
		t.Fatalf("codex baseline should authorize, auth=%+v err=%v decision=%s", auth, err, report.Decision)
	}
}

func TestAuthorizeExecuteCodexNonTTYRequiresAcceptWarnings(t *testing.T) {
	reg, run := authorizeFixture(t)
	auth, report, err := cli.AuthorizeExecute(cli.AuthorizeOpts{
		Registry:          reg,
		Host:              adapter.HostCodex,
		Run:               run,
		Agent2HostVersion: "0.0.0-dev",
		Stderr:            ioDiscard{},
	})
	if report.Decision == "allowed_with_warnings" {
		if auth != nil || err == nil {
			t.Fatalf("non-TTY must not authorize warnings without --accept-warnings, auth=%v err=%v", auth, err)
		}
		return
	}
	if err != nil || auth == nil || report.Decision == "refused" {
		t.Fatalf("codex baseline should allow or warn, auth=%v err=%v decision=%s", auth, err, report.Decision)
	}
}

func TestAuthorizeExecuteRefusedNoAuth(t *testing.T) {
	reg, run := authorizeFixture(t)
	req := true
	run.Sandbox = &decode.Sandbox{Required: &req}
	auth, report, err := cli.AuthorizeExecute(cli.AuthorizeOpts{
		Registry:          reg,
		Host:              adapter.HostKiro,
		Run:               run,
		Agent2HostVersion: "0.0.0-dev",
		AcceptWarnings:    true,
		TTY:               true,
		Stdin:             strings.NewReader("y\n"),
		Stderr:            ioDiscard{},
	})
	if !errors.Is(err, cli.ErrExecuteRefused) || auth != nil || report.Decision != "refused" {
		t.Fatalf("auth=%v err=%v decision=%s", auth, err, report.Decision)
	}
}

func TestAuthorizeExecuteAllowedNoFakeConfirm(t *testing.T) {
	reg, run := authorizeFixture(t)
	auth, report, err := cli.AuthorizeExecute(cli.AuthorizeOpts{
		Registry:          reg,
		Host:              adapter.HostClaudeCode,
		Run:               run,
		Agent2HostVersion: "0.0.0-dev",
	})
	if err != nil || auth == nil || auth.AcceptedWarnings || report.Decision == "refused" {
		t.Fatalf("auth=%+v err=%v decision=%s", auth, err, report.Decision)
	}
}

func TestCLIForceAndYesRejected(t *testing.T) {
	var out, errb bytes.Buffer
	for _, flag := range []string{"--force", "--yes"} {
		code := cli.Main([]string{"a2h", flag, "check", "s/a", "--host", "codex"}, &out, &errb)
		if code != cli.ExitUsage || !strings.Contains(errb.String(), "--accept-warnings") {
			t.Fatalf("%s: code=%d stderr=%q", flag, code, errb.String())
		}
	}
}

func TestCLICheckRejectsAcceptWarnings(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", t.TempDir(), "--json", "--accept-warnings", "check", "fm/demo", "--host", "claude-code"}, &out, &errb)
	if code != cli.ExitUsage || out.Len() != 0 || !strings.Contains(errb.String(), "--accept-warnings is not valid with check") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func authorizeFixture(t *testing.T) (*adapter.Registry, *space.ResolvedAgentRun) {
	t.Helper()
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "trees", "valid", "markdown-leading-dashes")
	home := t.TempDir()
	sp, err := space.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sp.Register(src); err != nil {
		t.Fatal(err)
	}
	run, err := sp.Resolve("fm", "demo", "")
	if err != nil {
		t.Fatal(err)
	}
	reg := committed.New(
		func(file string) (string, error) { return "/opt/" + file, nil },
		func(string) (string, error) { return "1.0.0-test", nil },
	)
	return reg, run
}
