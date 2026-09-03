package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agent2host/agent2host/internal/adapter"
)

// ContextKind selects check vs run ProjectionContext semantics.
type ContextKind int

const (
	// ContextCheck is Evaluate/Project for check: empty path fields.
	ContextCheck ContextKind = iota
	// ContextRun reserves planned run paths without creating the directory.
	ContextRun
)

// Prepared is one run’s tool-owned namespaces.
// V0 uses the non-cacheable path: material lives only under Root.
//
// Private is a Host-home partition (secret-bearing config). Adapters must not
// expose it via --add-dir; Workspace is the model-readable projection surface.
type Prepared struct {
	RunID       string
	Home        string
	Root        string
	Projection  string
	Workspace   string
	Private     string
	WorkingDir  string
	AuthProfile string // stable Auth Profile dir; set at Execute from adapter declaration
	Recover     RecoverReport
}

// RunReservation is a planned run that has not been written to disk.
type RunReservation struct {
	prepared Prepared
}

// PrepareContext builds the Adapter-facing environment. It does not create a
// durable run directory. Check uses empty paths. Run reserves an id and
// planned paths so Evaluate/Project see the same layout Execute will use.
func PrepareContext(home, workingDir string, kind ContextKind) (adapter.ProjectionContext, RunReservation, error) {
	if kind == ContextCheck {
		return adapter.ProjectionContext{}, RunReservation{}, nil
	}
	id, err := newRunID()
	if err != nil {
		return adapter.ProjectionContext{}, RunReservation{}, err
	}
	if workingDir == "" {
		workingDir, err = os.Getwd()
		if err != nil {
			return adapter.ProjectionContext{}, RunReservation{}, err
		}
	}
	p := plannedRun(home, id, workingDir)
	return p.ProjectionContext(), RunReservation{prepared: p}, nil
}

// BeginRun creates the reserved run directory, writes recovery state, and
// takes the live lock immediately so a concurrent RecoverLeftovers will not
// treat this run as leftover. The caller must Finalize or quarantine this run.
func BeginRun(res RunReservation) (Prepared, error) {
	p := res.prepared
	if p.RunID == "" || p.Root == "" {
		return Prepared{}, fmt.Errorf("runtime: BeginRun requires a run reservation")
	}
	rep, err := RecoverLeftovers(p.Home)
	if err != nil {
		return Prepared{}, err
	}
	p.Recover = rep
	created := false
	defer func() {
		if !created && p.Root != "" {
			_ = os.RemoveAll(p.Root)
		}
	}()
	if err := os.MkdirAll(p.Projection, runtimeDirMode); err != nil {
		return Prepared{}, err
	}
	if err := os.MkdirAll(p.Workspace, runtimeDirMode); err != nil {
		return Prepared{}, err
	}
	if err := os.MkdirAll(p.Private, runtimeDirMode); err != nil {
		return Prepared{}, err
	}
	if err := markBegun(p.Root); err != nil {
		return Prepared{}, err
	}
	if err := writeLiveLock(p); err != nil {
		return Prepared{}, err
	}
	created = true
	return p, nil
}

// Prepare allocates a durable run directory. Tests and Execute helpers use
// this; the CLI run path uses PrepareContext then BeginRun after authorize.
func Prepare(home, workingDir string) (Prepared, error) {
	_, res, err := PrepareContext(home, workingDir, ContextRun)
	if err != nil {
		return Prepared{}, err
	}
	return BeginRun(res)
}

// FinalizeAbandoned deletes a run that never reached Execute. No official
// run record is written. Delete failure isolates the directory.
func FinalizeAbandoned(p Prepared) error {
	if p.Root == "" {
		return nil
	}
	clearLiveLock(p)
	if runIsLive(p.Root) {
		return nil
	}
	if err := deleteRunWorkspace(p); err != nil {
		if qerr := quarantineAfterRun(p); qerr != nil {
			return fmt.Errorf("%w: %v (quarantine: %v)", ErrWorkspaceCleanup, err, qerr)
		}
	}
	return nil
}

// ProjectionContext is the Adapter-facing environment for Project.
func (p Prepared) ProjectionContext() adapter.ProjectionContext {
	return adapter.ProjectionContext{
		ApprovedWorkingDirectory: p.WorkingDir,
		RunPrivateDirectory:      p.Private,
	}
}

func plannedRun(home, id, workingDir string) Prepared {
	root := filepath.Join(home, runsDirName, id)
	return Prepared{
		RunID:      id,
		Home:       home,
		Root:       root,
		Projection: filepath.Join(root, "projection"),
		Workspace:  filepath.Join(root, "workspace"),
		Private:    filepath.Join(root, "private"),
		WorkingDir: workingDir,
	}
}

func newRunID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
