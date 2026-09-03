package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/committed"
)

type fakeAuth struct {
	desc adapter.AuthDescription
	env  map[string]string
	args []string
}

func (f fakeAuth) DescribeAuth() adapter.AuthDescription { return f.desc }
func (f fakeAuth) BindForRun(adapter.AuthBindRequest) (adapter.AuthBindDirective, error) {
	return adapter.AuthBindDirective{Copies: f.desc.Materials, Env: f.env, Args: f.args}, nil
}
func (f fakeAuth) FinalizeRun(adapter.AuthFinalizeRequest) (adapter.AuthFinalizeDirective, error) {
	return adapter.AuthFinalizeDirective{Copies: f.desc.Materials}, nil
}

func fileAuth() fakeAuth {
	return fakeAuth{desc: adapter.AuthDescription{
		Profile:     adapter.AuthProfileKey{Host: "fake-file", Provider: "p", NativeAuthNamespace: "ns"},
		Topology:    adapter.AuthTopologySeparated,
		Concurrency: adapter.AuthConcurrencySerialize,
		Materials: []adapter.AuthMaterial{{
			StoreRel: "token.bin",
			BindRoot: adapter.AuthRootPrivate,
			BindRel:  "host-home/token.bin",
			Lock:     true,
		}},
	}}
}

func externalAuth() fakeAuth {
	return fakeAuth{desc: adapter.AuthDescription{
		Profile:     adapter.AuthProfileKey{Host: "fake-sso", Provider: "idp", NativeAuthNamespace: "browser"},
		Topology:    adapter.AuthTopologyExternal,
		Concurrency: adapter.AuthConcurrencySafe,
	}}
}

func secretAuth() fakeAuth {
	return fakeAuth{desc: adapter.AuthDescription{
		Profile:        adapter.AuthProfileKey{Host: "fake-key", Provider: "api", NativeAuthNamespace: "env"},
		Topology:       adapter.AuthTopologyExecSecret,
		Concurrency:    adapter.AuthConcurrencySafe,
		ExecSecretEnvs: []string{"FAKE_API_KEY"},
	}}
}

func boundRootAuth() fakeAuth {
	return fakeAuth{
		desc: adapter.AuthDescription{
			Profile:     adapter.AuthProfileKey{Host: "fake-root", Provider: "p", NativeAuthNamespace: "config-dir"},
			Topology:    adapter.AuthTopologyBoundRoot,
			Concurrency: adapter.AuthConcurrencyUnverified,
		},
		env: map[string]string{"HOST_STATE_HOME": adapter.AuthProfileToken},
	}
}

func TestRuntimeHostAuthSourceHasNoHostSpecialCases(t *testing.T) {
	body, err := os.ReadFile("hoststate.go")
	if err != nil {
		t.Fatal(err)
	}
	src := strings.ToLower(string(body))
	for _, needle := range []string{
		"claude-code", ".credentials.json", "keychain",
		"auth.json", "codex", "kiro",
	} {
		if strings.Contains(src, needle) {
			t.Fatalf("runtime/hoststate.go must not mention %q", needle)
		}
	}
}

func TestBindOpaqueFileDoesNotImportUserHome(t *testing.T) {
	p, user := prepAuth(t)
	mustWrite(t, filepath.Join(user, ".host", "token.bin"), []byte("user-secret"))
	a := fileAuth()
	p, err := attachAuthProfile(p, a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bindHostAuth(p, a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.Private, "host-home", "token.bin")); !os.IsNotExist(err) {
		t.Fatal("must not import user-home login material")
	}
}

func TestBindOpaqueFileCopiesStoreOnly(t *testing.T) {
	p, _ := prepAuth(t)
	a := fileAuth()
	p, err := attachAuthProfile(p, a)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(p.AuthProfile, "token.bin"), []byte("from-profile"))
	if _, err := bindHostAuth(p, a); err != nil {
		t.Fatal(err)
	}
	got := mustRead(t, filepath.Join(p.Private, "host-home", "token.bin"))
	if string(got) != "from-profile" {
		t.Fatalf("got %s", got)
	}
}

func TestFinalizeOpaqueFilePersistsRefresh(t *testing.T) {
	p, _ := prepAuth(t)
	a := fileAuth()
	p, err := attachAuthProfile(p, a)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(p.Private, "host-home", "token.bin"), []byte("refreshed"))
	if err := finalizeHostAuth(p, a); err != nil {
		t.Fatal(err)
	}
	got := mustRead(t, filepath.Join(p.AuthProfile, "token.bin"))
	if string(got) != "refreshed" {
		t.Fatalf("got %s", got)
	}
}

func TestExternalSessionCopiesNothing(t *testing.T) {
	p, user := prepAuth(t)
	mustWrite(t, filepath.Join(user, "Library", "session.db"), []byte("sso"))
	a := externalAuth()
	p, err := attachAuthProfile(p, a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bindHostAuth(p, a); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(p.AuthProfile)
	if err == nil && len(ents) > 0 {
		t.Fatalf("external session must not populate profile, got %v", ents)
	}
}

func TestBoundRootBindSetsEnvCopiesNothing(t *testing.T) {
	p, user := prepAuth(t)
	mustWrite(t, filepath.Join(user, ".host", "token.bin"), []byte("user-secret"))
	a := boundRootAuth()
	p, err := attachAuthProfile(p, a)
	if err != nil {
		t.Fatal(err)
	}
	bind, err := bindHostAuth(p, a)
	if err != nil {
		t.Fatal(err)
	}
	if bind.Env["HOST_STATE_HOME"] != adapter.AuthProfileToken {
		t.Fatalf("bound-root bind env %v", bind.Env)
	}
	if _, err := os.Stat(filepath.Join(p.Private, "host-home", "token.bin")); !os.IsNotExist(err) {
		t.Fatal("bound-root must not import user-home material")
	}
	ents, err := os.ReadDir(p.AuthProfile)
	if err == nil && len(ents) > 0 {
		t.Fatalf("bound-root must not seed opaque blobs, got %v", ents)
	}
}

func TestExecuteAppliesBindEnvAndArgs(t *testing.T) {
	p, _ := prepAuth(t)
	a := boundRootAuth()
	a.args = []string{"--state-flag"}
	dump := filepath.Join(t.TempDir(), "env")
	stub := filepath.Join(t.TempDir(), "host")
	script := "#!/bin/sh\nprintf '%s %s\\n' \"$HOST_STATE_HOME\" \"$1\" > '" + dump + "'\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Launch: adapter.LaunchPlan{
			Executable: stub,
			Env:        map[string]string{"HOST_STATE_HOME": "from-launch-plan"},
		},
	})
	if _, err := Execute(p, auth, r, ExecOpts{Getenv: func(string) string { return "" }, HostState: a}); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(mustRead(t, dump)))
	want := filepath.Join(p.Home, adapter.AuthProfilesDirName, a.desc.Profile.Host, a.desc.Profile.DirName()) + " --state-flag"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestExecSecretBindCopiesNothing(t *testing.T) {
	p, _ := prepAuth(t)
	a := secretAuth()
	p, err := attachAuthProfile(p, a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bindHostAuth(p, a); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(p.AuthProfile)
	if err == nil && len(ents) > 0 {
		t.Fatalf("exec secret must not write profile blobs, got %v", ents)
	}
}

func TestNilHostStateIsNoop(t *testing.T) {
	p, _ := prepAuth(t)
	if _, err := bindHostAuth(p, nil); err != nil {
		t.Fatal(err)
	}
	if err := finalizeHostAuth(p, nil); err != nil {
		t.Fatal(err)
	}
}

func TestKiroCommittedAuthDoesNotCopy(t *testing.T) {
	p, user := prepAuth(t)
	mustWrite(t, filepath.Join(user, ".kiro", "settings", "cli.json"), []byte(`{"secret":1}`))
	a, err := committed.Default().Select(adapter.HostKiro)
	if err != nil {
		t.Fatal(err)
	}
	authn := a.HostState()
	p, err = attachAuthProfile(p, authn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bindHostAuth(p, authn); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.Workspace, "kiro-home", "settings", "cli.json")); err == nil {
		t.Fatal("must not copy Kiro user files")
	}
	ents, err := os.ReadDir(p.AuthProfile)
	if err == nil && len(ents) > 0 {
		t.Fatalf("kiro profile must stay empty, got %v", ents)
	}
}

func TestCodexOpaquePersistAndLock(t *testing.T) {
	p, _ := prepAuth(t)
	a, err := committed.Default().Select(adapter.HostCodex)
	if err != nil {
		t.Fatal(err)
	}
	authn := a.HostState()
	p, err = attachAuthProfile(p, authn)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(p.AuthProfile, "auth.json"), []byte(`{"from":"store"}`))
	if _, err := bindHostAuth(p, authn); err != nil {
		t.Fatal(err)
	}
	got := mustRead(t, filepath.Join(p.Private, "codex-home", "auth.json"))
	if string(got) != `{"from":"store"}` {
		t.Fatalf("got %s", got)
	}
	mustWrite(t, filepath.Join(p.Private, "codex-home", "auth.json"), []byte(`{"from":"refresh"}`))
	if err := finalizeHostAuth(p, authn); err != nil {
		t.Fatal(err)
	}
	got = mustRead(t, filepath.Join(p.AuthProfile, "auth.json"))
	if string(got) != `{"from":"refresh"}` {
		t.Fatalf("persist %s", got)
	}
}

func TestAuthStoreFileLockSerializes(t *testing.T) {
	p, _ := prepAuth(t)
	store := filepath.Join(p.Home, "lock-target", "token.bin")
	mustWrite(t, store, []byte(`{"v":0}`))
	holding := make(chan struct{})
	release := make(chan struct{})
	writerDone := make(chan struct{})
	readerDone := make(chan error, 1)
	go func() {
		defer close(writerDone)
		_ = withAuthStoreFileLock(store, func() error {
			close(holding)
			<-release
			return writeAuthFile(store, []byte(`{"v":1}`))
		})
	}()
	<-holding
	if got := string(mustRead(t, store)); got != `{"v":0}` {
		t.Fatalf("while writer holds lock, store should stay v0, got %s", got)
	}
	go func() {
		readerDone <- withAuthStoreFileLock(store, func() error {
			body, err := os.ReadFile(store)
			if err != nil {
				return err
			}
			if string(body) != `{"v":1}` {
				return fmt.Errorf("reader want v1 after writer release, got %s", body)
			}
			return nil
		})
	}()
	close(release)
	<-writerDone
	if err := <-readerDone; err != nil {
		t.Fatal(err)
	}
}

func TestBindOpaqueFileConcurrentStore(t *testing.T) {
	p, _ := prepAuth(t)
	a := fileAuth()
	p, err := attachAuthProfile(p, a)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(p.AuthProfile, "token.bin"), []byte("shared"))
	p2, err := Prepare(p.Home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p2, err = attachAuthProfile(p2, a)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, px := range []Prepared{p, p2} {
		wg.Add(1)
		go func(px Prepared) {
			defer wg.Done()
			if _, err := bindHostAuth(px, a); err != nil {
				t.Error(err)
			}
		}(px)
	}
	wg.Wait()
	for _, dest := range []string{
		filepath.Join(p.Private, "host-home", "token.bin"),
		filepath.Join(p2.Private, "host-home", "token.bin"),
	} {
		if string(mustRead(t, dest)) != "shared" {
			t.Fatalf("dest %s", dest)
		}
	}
}

func TestBindRejectsSymlinkStore(t *testing.T) {
	p, _ := prepAuth(t)
	a := fileAuth()
	p, err := attachAuthProfile(p, a)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "secret")
	mustWrite(t, target, []byte("nope"))
	if err := os.Symlink(target, filepath.Join(p.AuthProfile, "token.bin")); err != nil {
		t.Fatal(err)
	}
	if _, err := bindHostAuth(p, a); err == nil {
		t.Fatal("expected symlink reject")
	}
}

func TestWatchPersistsOpaqueAuthWhileHostRuns(t *testing.T) {
	p, _ := prepAuth(t)
	a := fileAuth()
	p, err := attachAuthProfile(p, a)
	if err != nil {
		t.Fatal(err)
	}
	oldInterval := authPersistInterval
	authPersistInterval = 30 * time.Millisecond
	t.Cleanup(func() { authPersistInterval = oldInterval })

	privateAuth := filepath.Join(p.Private, "host-home", "token.bin")
	ready := filepath.Join(t.TempDir(), "ready")
	stub := filepath.Join(t.TempDir(), "host")
	script := "#!/bin/sh\n" +
		"mkdir -p \"$(dirname '" + privateAuth + "')\"\n" +
		"echo refreshed > '" + privateAuth + "'\n" +
		"touch '" + ready + "'\n" +
		"trap 'exit 130' INT\n" +
		"while :; do sleep 1; done\n"
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
		err error
	}
	done := make(chan result, 1)
	go func() {
		_, err := Execute(p, auth, r, ExecOpts{Getenv: func(string) string { return "" }, HostState: a})
		done <- result{err: err}
	}()

	deadline := time.After(2 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case got := <-done:
			t.Fatalf("host exited before writing login material: %v", got.err)
		case <-deadline:
			t.Fatal("host did not write login material")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	store := filepath.Join(p.Home, adapter.AuthProfilesDirName, "fake-file", a.desc.Profile.DirName(), "token.bin")
	persistDeadline := time.After(2 * time.Second)
	for {
		if body, err := os.ReadFile(store); err == nil && strings.TrimSpace(string(body)) == "refreshed" {
			select {
			case <-done:
				t.Fatal("host exited before mid-run persist was observed")
			default:
			}
			break
		}
		select {
		case got := <-done:
			t.Fatalf("host exited before mid-run persist: %v", got.err)
		case <-persistDeadline:
			t.Fatal("declared login material was not persisted while the host still ran")
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
		if got.err != nil {
			t.Fatal(got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Execute did not finish after host interrupt")
	}
}

func TestExecutePersistsOpaqueAuthAfterHostExit(t *testing.T) {
	p, _ := prepAuth(t)
	a := fileAuth()
	privateAuth := filepath.Join(p.Private, "host-home", "token.bin")
	stub := filepath.Join(t.TempDir(), "host")
	script := "#!/bin/sh\nmkdir -p \"$(dirname '" + privateAuth + "')\"\necho refreshed > '" + privateAuth + "'\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	auth := mustAuth(r, adapter.Plans{
		Launch: adapter.LaunchPlan{Executable: stub},
	})
	if _, err := Execute(p, auth, r, ExecOpts{Getenv: func(string) string { return "" }, HostState: a}); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(mustRead(t, filepath.Join(p.Home, adapter.AuthProfilesDirName, "fake-file", a.desc.Profile.DirName(), "token.bin"))))
	if got != "refreshed" {
		t.Fatalf("persisted %q", got)
	}
}

func TestClaudeBoundRootDoesNotSeedPerRunCredentials(t *testing.T) {
	p, _ := prepAuth(t)
	a, err := committed.Default().Select(adapter.HostClaudeCode)
	if err != nil {
		t.Fatal(err)
	}
	authn := a.HostState()
	p, err = attachAuthProfile(p, authn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bindHostAuth(p, authn); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.Private, "host-config", ".credentials.json")); !os.IsNotExist(err) {
		t.Fatal("Claude must not treat a per-run config dir as the login store")
	}
}

func prepAuth(t *testing.T) (Prepared, string) {
	t.Helper()
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return p, t.TempDir()
}

func mustWrite(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
