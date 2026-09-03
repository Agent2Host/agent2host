package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
)

func TestPrepareContextCheckCreatesNoDirs(t *testing.T) {
	home := t.TempDir()
	pctx, res, err := PrepareContext(home, t.TempDir(), ContextCheck)
	if err != nil {
		t.Fatal(err)
	}
	if pctx.ApprovedWorkingDirectory != "" || pctx.RunPrivateDirectory != "" {
		t.Fatalf("check context must use empty paths, got %+v", pctx)
	}
	if res.prepared.RunID != "" {
		t.Fatal("check must not reserve a durable run")
	}
	if entries, err := os.ReadDir(filepath.Join(home, runsDirName)); err == nil && len(entries) != 0 {
		t.Fatalf("check must not create runs/: %v", names(entries))
	}
}

func TestPrepareContextRunReservesPathsWithoutCreating(t *testing.T) {
	home := t.TempDir()
	wd := t.TempDir()
	pctx, res, err := PrepareContext(home, wd, ContextRun)
	if err != nil {
		t.Fatal(err)
	}
	if pctx.ApprovedWorkingDirectory != wd || pctx.RunPrivateDirectory == "" {
		t.Fatalf("run context must plan paths, got %+v", pctx)
	}
	if res.prepared.RunID == "" || res.prepared.Private != pctx.RunPrivateDirectory {
		t.Fatalf("reservation %+v", res.prepared)
	}
	if _, err := os.Stat(res.prepared.Root); err == nil {
		t.Fatal("PrepareContext must not create the run directory")
	}
}

func TestBeginRunWritesStateAndFinalizeAbandonedDeletes(t *testing.T) {
	home := t.TempDir()
	_, res, err := PrepareContext(home, t.TempDir(), ContextRun)
	if err != nil {
		t.Fatal(err)
	}
	p, err := BeginRun(res)
	if err != nil {
		t.Fatal(err)
	}
	if !runHasMark(p.Root, begunMarkName) {
		t.Fatal("BeginRun must write recovery state")
	}
	if !runIsLive(p.Root) {
		t.Fatal("BeginRun must write live.lock before Execute")
	}
	if err := FinalizeAbandoned(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.Root); err == nil {
		t.Fatal("abandoned run must be deleted")
	}
}

func TestBeginRunLockStopsForeignRecover(t *testing.T) {
	home := t.TempDir()
	p, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !runIsLive(p.Root) {
		t.Fatal("BeginRun must write live.lock")
	}
	rep, err := RecoverLeftovers(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Recovered)+len(rep.Quarantined) != 0 {
		t.Fatalf("another recover must skip the live run: %+v", rep)
	}
	if _, err := os.Stat(p.Root); err != nil {
		t.Fatal("live run directory must remain")
	}
}

func TestBeginRunCleansPartialCreate(t *testing.T) {
	home := t.TempDir()
	_, res, err := PrepareContext(home, t.TempDir(), ContextRun)
	if err != nil {
		t.Fatal(err)
	}
	p := res.prepared
	if err := os.MkdirAll(p.Root, runtimeDirMode); err != nil {
		t.Fatal(err)
	}
	// Survive RecoverLeftovers so the workspace-as-file can fail the next mkdir.
	if err := os.WriteFile(filepath.Join(p.Root, liveLockName), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Workspace, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := BeginRun(res); err == nil {
		t.Fatal("expected BeginRun to fail when workspace cannot be created")
	}
	if _, err := os.Stat(p.Root); err == nil {
		t.Fatal("partial BeginRun must remove the run directory")
	}
}

func names(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestValidForRequiresAcceptedWarnings(t *testing.T) {
	r := sampleReport()
	r.Decision = "allowed_with_warnings"
	plans := adapter.Plans{Launch: adapter.LaunchPlan{Executable: "/bin/true"}}
	auth := mustAuth(r, plans)
	auth.AcceptedWarnings = false
	if err := auth.ValidFor(r, plans); !errors.Is(err, ErrAcceptanceRequired) {
		t.Fatalf("got %v", err)
	}
	auth.AcceptedWarnings = true
	if err := auth.ValidFor(r, plans); err != nil {
		t.Fatal(err)
	}
}
