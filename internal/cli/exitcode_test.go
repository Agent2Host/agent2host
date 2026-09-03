package cli_test

import (
	"bytes"
	"testing"

	"github.com/agent2host/agent2host/internal/cli"
)

func TestCLIExitCodesUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cli.Main([]string{"a2h", "nope"}, &out, &errb); code != cli.ExitUsage {
		t.Fatalf("unknown command: got %d", code)
	}
}

func TestCLIExitCodesPrecondition(t *testing.T) {
	var out, errb bytes.Buffer
	code := cli.Main([]string{"a2h", "--home", t.TempDir(), "inspect", "missing/agent"}, &out, &errb)
	if code != cli.ExitPrecondition {
		t.Fatalf("unknown agent: got %d", code)
	}
}
