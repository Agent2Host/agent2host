package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agent2host/agent2host/internal/adapter"
)

func TestExecuteInterruptPersistsOpaqueAuth(t *testing.T) {
	p, _ := prepAuth(t)
	a := fileAuth()
	privateAuth := filepath.Join(p.Private, "host-home", "token.bin")
	ready := filepath.Join(t.TempDir(), "ready")
	stub := filepath.Join(t.TempDir(), "host")
	script := "#!/bin/sh\n" +
		"mkdir -p \"$(dirname '" + privateAuth + "')\"\n" +
		"trap 'echo refreshed > \"" + privateAuth + "\"; exit 130' INT\n" +
		"touch '" + ready + "'\n" +
		"while :; do :; done\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	signals := make(chan os.Signal, 1)
	started := make(chan *os.Process, 1)
	oldRunner := runHostProcessFn
	runHostProcessFn = func(cmd *exec.Cmd) (HostProcessResult, error) {
		return runHostProcessWithSignalsAndStarted(cmd, signals, started)
	}
	t.Cleanup(func() { runHostProcessFn = oldRunner })

	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Launch: adapter.LaunchPlan{Executable: stub},
	})
	type result struct {
		out Outcome
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := Execute(p, auth, r, ExecOpts{Getenv: func(string) string { return "" }, HostState: a})
		done <- result{out: out, err: err}
	}()

	deadline := time.After(2 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case got := <-done:
			t.Fatalf("host exited before interrupt: out=%+v err=%v", got.out, got.err)
		case <-deadline:
			t.Fatal("host did not start")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	host := <-started
	signals <- os.Interrupt
	if err := host.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-done:
		if got.err != nil || got.out.Class != ClassHostProcess || !got.out.Interrupted {
			t.Fatalf("out=%+v err=%v", got.out, got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Execute did not finish after host interrupt")
	}
	got := strings.TrimSpace(string(mustRead(t, filepath.Join(p.Home, adapter.AuthProfilesDirName, "fake-file", a.desc.Profile.DirName(), "token.bin"))))
	if got != "refreshed" {
		t.Fatalf("persisted %q", got)
	}
}
