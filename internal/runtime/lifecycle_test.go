package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent2host/agent2host/internal/adapter"
)

func TestCleanDefaultSkipsRegistryHostStateQuarantine(t *testing.T) {
	home := t.TempDir()
	mustWriteFile(t, filepath.Join(home, "space", "registry.json"), []byte(`{"systems":{}}`))
	mustWriteFile(t, filepath.Join(home, adapter.AuthProfilesDirName, "claude-code", "keep"), []byte("login"))
	q := filepath.Join(home, quarantineDirName, "bad-run")
	mustWriteFile(t, filepath.Join(q, "secret"), []byte("x"))
	left := filepath.Join(home, runsDirName, "old")
	if err := os.MkdirAll(left, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := markRecovered(left); err != nil {
		t.Fatal(err)
	}
	unsafe := filepath.Join(home, runsDirName, "unsafe")
	if err := os.MkdirAll(unsafe, 0o750); err != nil {
		t.Fatal(err)
	}
	res, err := Clean(home, CleanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) != 1 || res.Paths[0] != left {
		t.Fatalf("paths %v", res.Paths)
	}
	if _, err := os.Stat(filepath.Join(home, "space", "registry.json")); err != nil {
		t.Fatal("must keep registry")
	}
	if _, err := os.Stat(filepath.Join(home, adapter.AuthProfilesDirName, "claude-code", "keep")); err != nil {
		t.Fatal("must keep host state")
	}
	if _, err := os.Stat(filepath.Join(q, "secret")); err != nil {
		t.Fatal("default clean must keep quarantine")
	}
	if _, err := os.Stat(left); err == nil {
		t.Fatal("default clean must delete leftover run")
	}
	if _, err := os.Stat(unsafe); err != nil {
		t.Fatal("default clean must keep unrecovered leftover")
	}
}

func TestCleanDryRunDoesNotMutate(t *testing.T) {
	home := t.TempDir()
	left := filepath.Join(home, runsDirName, "old")
	if err := os.MkdirAll(left, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := markRecovered(left); err != nil {
		t.Fatal(err)
	}
	res, err := Clean(home, CleanOpts{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) != 1 {
		t.Fatalf("paths %v", res.Paths)
	}
	if _, err := os.Stat(left); err != nil {
		t.Fatal("dry-run must leave leftover run")
	}
}

func TestCleanQuarantineOnly(t *testing.T) {
	home := t.TempDir()
	q := filepath.Join(home, quarantineDirName, "bad-run")
	mustWriteFile(t, filepath.Join(q, "secret"), []byte("x"))
	left := filepath.Join(home, runsDirName, "old")
	if err := os.MkdirAll(left, 0o750); err != nil {
		t.Fatal(err)
	}
	res, err := Clean(home, CleanOpts{Quarantine: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) != 1 || res.Paths[0] != q {
		t.Fatalf("paths %v", res.Paths)
	}
	if _, err := os.Stat(q); err == nil {
		t.Fatal("quarantine should be gone")
	}
	if _, err := os.Stat(left); err != nil {
		t.Fatal("quarantine-only must not delete leftover runs")
	}
}

func TestCleanLiveSkip(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLiveLock(p); err != nil {
		t.Fatal(err)
	}
	res, err := Clean(home, CleanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range res.Paths {
		if path == p.Root {
			t.Fatal("must not delete a live run")
		}
	}
	if _, err := os.Stat(p.Root); err != nil {
		t.Fatal("live run must remain")
	}
}

func TestExecuteDeletesWorkspaceKeepsRecord(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(t.TempDir(), "host")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{Launch: adapter.LaunchPlan{Executable: stub}})
	out, err := Execute(p, auth, r, ExecOpts{Getenv: func(string) string { return "" }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.Root); err == nil {
		t.Fatal("workspace must be gone")
	}
	if _, err := os.Stat(recordPath(home, out.RunID)); err != nil {
		t.Fatal(err)
	}
}

func TestCleanHostStateRequiresHost(t *testing.T) {
	_, err := Clean(t.TempDir(), CleanOpts{HostState: true})
	if err != ErrHostStateNeedsHost {
		t.Fatalf("got %v", err)
	}
}

func TestCleanHostStateDeletesOnlyThatHostCopy(t *testing.T) {
	home := t.TempDir()
	keep := filepath.Join(home, adapter.AuthProfilesDirName, "claude-code", "keep")
	other := filepath.Join(home, adapter.AuthProfilesDirName, "codex", "keep")
	mustWriteFile(t, keep, []byte("claude"))
	mustWriteFile(t, other, []byte("codex"))
	res, err := Clean(home, CleanOpts{HostState: true, Host: "claude-code"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) != 1 {
		t.Fatalf("paths %v", res.Paths)
	}
	if _, err := os.Stat(keep); err == nil {
		t.Fatal("named Host state copy should be gone")
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatal("other Host state must stay")
	}
}

func TestRecoverLeftoversHandlesPreparedWithoutBaseline(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "secret-baseline")); err == nil {
		t.Fatal("fixture must not have a secret baseline")
	}
	simulateCrashedLock(t, p)
	rep, err := RecoverLeftovers(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Recovered) != 1 || rep.Recovered[0] != p.RunID {
		t.Fatalf("recovered %v quarantined %v", rep.Recovered, rep.Quarantined)
	}
	if _, err := os.Stat(p.Root); err == nil {
		t.Fatal("prepared run without secrets must be deleted")
	}
}

func TestRecoverLeftoversSkipsLiveAndUsesBaselineWhenPresent(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := persistSecretBaselines(p, adapter.NativeProjectionPlan{
		Files: []adapter.ProjectionFile{{
			RelPath: "host-config/mcp.json",
			Class:   adapter.DestHostPrivate,
			Content: []byte(`{"T":"` + adapter.SecretPlaceholder("T") + `"}`),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeLiveLock(p); err != nil {
		t.Fatal(err)
	}
	rep, err := RecoverLeftovers(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Recovered)+len(rep.Quarantined) != 0 {
		t.Fatalf("live run must be skipped: %+v", rep)
	}
	clearLiveLock(p)
	rep, err = RecoverLeftovers(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Recovered) != 1 || rep.Recovered[0] != p.RunID {
		t.Fatalf("after unlock, baseline run should restore: %+v", rep)
	}
	if _, err := os.Stat(p.Root); err != nil {
		t.Fatal("restored secret-bearing run stays until clean")
	}
	if !runHasMark(p.Root, recoveredMarkName) {
		t.Fatal("restored run must be marked recovered")
	}
}

func TestRecoverLeftoversQuarantinesWhenBaselineUnreadable(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	canary := filepath.Join(p.Root, "keep-me")
	if err := os.WriteFile(canary, []byte("maybe-secrets"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(p.Root, "secret-baseline")
	if err := os.Symlink("secret-baseline", base); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(base); err == nil || os.IsNotExist(err) {
		t.Fatalf("symlink cycle must be a Stat error other than not-exist, got %v", err)
	}
	simulateCrashedLock(t, p)

	rep, err := RecoverLeftovers(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Quarantined) != 1 || rep.Quarantined[0] != p.RunID {
		t.Fatalf("unreadable baseline must be quarantined, not deleted: %+v", rep)
	}
	if len(rep.Recovered) != 0 {
		t.Fatalf("must not report recovered: %+v", rep)
	}
	if _, err := os.Stat(p.Root); err == nil {
		t.Fatal("run must leave runs/")
	}
	kept := filepath.Join(home, quarantineDirName, p.RunID, "keep-me")
	if _, err := os.Stat(kept); err != nil {
		t.Fatal("canary must survive in quarantine, not be deleted")
	}
}

func TestRecoverLeftoversQuarantinesFailedRestore(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rel := "host-config/mcp.json"
	if err := persistSecretBaselines(p, adapter.NativeProjectionPlan{
		Files: []adapter.ProjectionFile{{
			RelPath: rel, Class: adapter.DestHostPrivate, Content: []byte(`{"T":"` + adapter.SecretPlaceholder("T") + `"}`),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(p.Private); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Private, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	simulateCrashedLock(t, p)
	rep, err := RecoverLeftovers(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Quarantined) != 1 || rep.Quarantined[0] != p.RunID {
		t.Fatalf("quarantined %v", rep.Quarantined)
	}
	if _, err := os.Stat(p.Root); err == nil {
		t.Fatal("failed recovery must move the run dir out of runs/")
	}
	if _, err := os.Stat(filepath.Join(home, quarantineDirName, p.RunID)); err != nil {
		t.Fatal("failed recovery must move to quarantine")
	}
}

func TestCleanDefaultSkipsUnrecoveredRun(t *testing.T) {
	home := t.TempDir()
	unsafe := filepath.Join(home, runsDirName, "leaked")
	mustWriteFile(t, filepath.Join(unsafe, "secret"), []byte("still-here"))
	res, err := Clean(home, CleanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) != 0 {
		t.Fatalf("unrecovered leftover must not be deleted, paths %v", res.Paths)
	}
	if _, err := os.Stat(filepath.Join(unsafe, "secret")); err != nil {
		t.Fatal(err)
	}
}

func TestHostStateDirRejectsUnknownAndEscape(t *testing.T) {
	home := t.TempDir()
	mustWriteFile(t, filepath.Join(home, "outside", "keep"), []byte("no"))
	for _, host := range []string{"../outside", "claude-code/../outside", "not-a-host", "claude-code/extra"} {
		if _, err := hostStateDir(home, host); !errors.Is(err, ErrUnknownHost) {
			t.Fatalf("host %q: got %v", host, err)
		}
	}
	if _, err := Clean(home, CleanOpts{HostState: true, Host: "../outside"}); !errors.Is(err, ErrUnknownHost) {
		t.Fatalf("clean: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "outside", "keep")); err != nil {
		t.Fatal("traversal must not delete outside Host state root")
	}
}

func TestHostStateDirRejectsSymlinkEscape(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, adapter.AuthProfilesDirName)
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, "outside")
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, adapter.HostClaudeCode)); err != nil {
		t.Fatal(err)
	}
	if _, err := hostStateDir(home, adapter.HostClaudeCode); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("got %v", err)
	}
}

func TestPathInsideRejectsParent(t *testing.T) {
	root := t.TempDir()
	if err := pathInside(root, filepath.Join(root, "..", "sibling")); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("got %v", err)
	}
	if err := pathInside(root, root); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("root itself: %v", err)
	}
}

func TestExecuteWipeFailureIsNotSuccess(t *testing.T) {
	t.Cleanup(func() {
		wipeSecretsAfterRun = wipeSecrets
		quarantineAfterRun = quarantineRun
	})
	wipeSecretsAfterRun = func(Prepared, adapter.NativeProjectionPlan, map[string]string) error {
		return fmt.Errorf("wipe boom")
	}
	quarantineAfterRun = func(Prepared) error { return fmt.Errorf("q boom") }

	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(t.TempDir(), "host")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{Launch: adapter.LaunchPlan{Executable: stub}})
	_, err = Execute(p, auth, r, ExecOpts{Getenv: func(string) string { return "" }})
	if err == nil || !errors.Is(err, ErrSecretWipe) {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(p.Root); err != nil {
		t.Fatal("wipe+quarantine failure must keep the run dir")
	}
	res, err := Clean(home, CleanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range res.Paths {
		if path == p.Root {
			t.Fatal("default clean must not delete unrecovered secret debris")
		}
	}
}

func TestExecuteDeleteFailureIsNotSuccess(t *testing.T) {
	t.Cleanup(func() {
		deleteWorkspaceAfterRun = deleteRunWorkspace
		quarantineAfterRun = quarantineRun
	})
	deleteWorkspaceAfterRun = func(Prepared) error { return fmt.Errorf("rm boom") }
	quarantineAfterRun = func(Prepared) error { return fmt.Errorf("q boom") }

	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(t.TempDir(), "host")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{Launch: adapter.LaunchPlan{Executable: stub}})
	_, err = Execute(p, auth, r, ExecOpts{Getenv: func(string) string { return "" }})
	if err == nil || !errors.Is(err, ErrWorkspaceCleanup) {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(p.Root); err != nil {
		t.Fatal("delete+quarantine failure must keep the run dir")
	}
	if runHasMark(p.Root, finalizedMarkName) {
		t.Fatal("kept debris must not be marked finalized")
	}
	res, err := Clean(home, CleanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range res.Paths {
		if path == p.Root {
			t.Fatal("default clean must not delete a run that failed to finalize")
		}
	}
}

func TestExecuteOverlayWipeFailureKeepsDebris(t *testing.T) {
	t.Cleanup(func() {
		wipeSecretsAfterRun = wipeSecrets
		quarantineAfterRun = quarantineRun
	})
	wipeSecretsAfterRun = func(Prepared, adapter.NativeProjectionPlan, map[string]string) error {
		return fmt.Errorf("wipe boom")
	}
	quarantineAfterRun = func(Prepared) error { return fmt.Errorf("q boom") }

	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	relOK := "host-config/a.json"
	relBad := "host-config/b.json"
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Projection: adapter.NativeProjectionPlan{
			HostID: "claude-code",
			Files: []adapter.ProjectionFile{
				{RelPath: relOK, Class: adapter.DestHostPrivate, Content: []byte(`{"T":"` + adapter.SecretPlaceholder("T") + `"}`)},
				{RelPath: relBad, Class: adapter.DestHostPrivate, Content: []byte(`{"T":"` + adapter.SecretPlaceholder("T"))},
			},
		},
		Launch: adapter.LaunchPlan{
			Executable: "/bin/true",
			Secrets:    []adapter.SecretRef{{Name: "T", Consumer: "/mcp_servers/db", Required: true}},
		},
	})
	_, err = Execute(p, auth, r, ExecOpts{Getenv: func(k string) string {
		if k == "T" {
			return "secret-xyz"
		}
		return ""
	}})
	if err == nil || !errors.Is(err, ErrSecretWipe) {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(p.Root); err != nil {
		t.Fatal("overlay wipe+quarantine failure must keep the run dir")
	}
	if runHasMark(p.Root, finalizedMarkName) {
		t.Fatal("unrecovered secret debris must not be marked finalized")
	}
	body, err := os.ReadFile(filepath.Join(p.Private, relOK))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("secret-xyz")) {
		t.Fatalf("first overlay must have written the secret before the later file failed: %s", body)
	}
	res, err := Clean(home, CleanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range res.Paths {
		if path == p.Root {
			t.Fatal("default clean must not delete unrecovered secret debris")
		}
	}
}

func TestExpiredRecordsAreCleaned(t *testing.T) {
	home := t.TempDir()
	old := Record{RunID: "old", SystemID: "s", RecordedAt: time.Now().UTC().Add(-40 * 24 * time.Hour).Format(time.RFC3339)}
	fresh := Record{RunID: "new", SystemID: "s", RecordedAt: time.Now().UTC().Format(time.RFC3339)}
	mustWriteJSON(t, recordPath(home, "old"), old)
	mustWriteJSON(t, recordPath(home, "new"), fresh)
	res, err := Clean(home, CleanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(recordPath(home, "old")); err == nil {
		t.Fatal("expired record must go")
	}
	if _, err := os.Stat(recordPath(home, "new")); err != nil {
		t.Fatal("fresh record must stay")
	}
	if len(res.Paths) != 1 {
		t.Fatalf("paths %v", res.Paths)
	}
}

func simulateCrashedLock(t *testing.T, p Prepared) {
	t.Helper()
	if err := os.WriteFile(liveLockPath(p.Root), []byte("999999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if runIsLive(p.Root) {
		t.Fatal("crash fixture pid must not be live")
	}
}

func mustWriteFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustWriteJSON(t *testing.T, path string, rec Record) {
	t.Helper()
	body, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, path, body)
}
