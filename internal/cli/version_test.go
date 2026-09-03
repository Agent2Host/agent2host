package cli_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/cli"
)

func TestVersionCommandDefaultDev(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "version"}, &out, &errb)
	if code != 0 {
		t.Fatalf("version exit %d: %s", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "a2h 0.0.0-dev") {
		t.Fatalf("default version: %q", got)
	}
	if !strings.Contains(got, "commit unknown") || !strings.Contains(got, "built unknown") {
		t.Fatalf("default identity: %q", got)
	}
}

func TestVersionFlagAndJSON(t *testing.T) {
	prevV, prevC, prevT := cli.Version, cli.Commit, cli.BuildTime
	cli.Version, cli.Commit, cli.BuildTime = "0.1.0-alpha.1", "abc1234", "2026-09-02T00:00:00Z"
	t.Cleanup(func() {
		cli.Version, cli.Commit, cli.BuildTime = prevV, prevC, prevT
	})

	var out, errb bytes.Buffer
	if code := cli.Main([]string{"a2h", "--version"}, &out, &errb); code != 0 {
		t.Fatalf("--version exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "a2h 0.1.0-alpha.1") {
		t.Fatalf("--version: %q", out.String())
	}

	out.Reset()
	errb.Reset()
	if code := cli.Main([]string{"a2h", "version", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("version --json exit %d: %s", code, errb.String())
	}
	var ident map[string]string
	if err := json.Unmarshal(out.Bytes(), &ident); err != nil {
		t.Fatal(err)
	}
	if ident["version"] != "0.1.0-alpha.1" || ident["commit"] != "abc1234" || ident["build_time"] != "2026-09-02T00:00:00Z" {
		t.Fatalf("json %+v", ident)
	}
}

func TestVersionRejectsExtraArgs(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cli.Main([]string{"a2h", "version", "extra"}, &out, &errb); code != cli.ExitUsage {
		t.Fatalf("got %d stderr=%s", code, errb.String())
	}
}

func TestVersionLdflagsInjection(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "a2h")
	ldflags := strings.Join([]string{
		"-X", "github.com/agent2host/agent2host/internal/cli.Version=0.1.0-alpha.1",
		"-X", "github.com/agent2host/agent2host/internal/cli.Commit=deadbeef",
		"-X", "github.com/agent2host/agent2host/internal/cli.BuildTime=2026-09-02T12:00:00Z",
	}, " ")
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", bin, "github.com/agent2host/agent2host/cmd/a2h")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	got, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, got)
	}
	text := string(got)
	for _, want := range []string{"a2h 0.1.0-alpha.1", "commit deadbeef", "built 2026-09-02T12:00:00Z"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
}
