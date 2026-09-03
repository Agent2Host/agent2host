package runtime

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
)

var (
	wipeSecretsAfterRun     = wipeSecrets
	deleteWorkspaceAfterRun = deleteRunWorkspace
	quarantineAfterRun      = quarantineRun
)

// Outcome is this invocation’s result.
type Outcome struct {
	Class               string   `json:"class"`
	ExitCode            int      `json:"exit_code"`
	Stage               string   `json:"stage,omitempty"`
	RunID               string   `json:"run_id"`
	Omitted             []string `json:"omitted_secrets,omitempty"`
	Interrupted         bool     `json:"interrupted,omitempty"`
	HostStateSaveFailed bool     `json:"-"`
}

const (
	ClassHostProcess = "host_process"
	ClassPreLaunch   = "pre_launch_rejection"
	ClassInfra       = "runtime_infrastructure_failure"
)

// ExecOpts is Execute I/O and secret lookup. Tests inject getenv.
type ExecOpts struct {
	Stdin                  io.Reader
	Stdout                 io.Writer
	Stderr                 io.Writer
	Getenv                 func(string) string
	HostState              adapter.HostStateBinder
	OnHostStarting         func()
	OnHostStateBindFailure func()
	OnHostStateSaveFailure func()
}

// Execute materializes run-locally, resolves secrets, launches, records.
// Agent2Host does not reuse a projection cache for execution.
func Execute(p Prepared, auth *Authorization, report compatibility.Report, opts ExecOpts) (out Outcome, err error) {
	out = Outcome{RunID: p.RunID, Class: ClassPreLaunch, Stage: "authorize"}
	if auth == nil {
		return failExecute(p, report, out, ClassPreLaunch, "authorize", ErrUnauthorized, true)
	}
	if err := auth.ValidFor(report, auth.Plans); err != nil {
		return failExecute(p, report, out, ClassPreLaunch, "authorize", err, true)
	}
	plans := auth.Plans
	p, err = attachAuthProfile(p, opts.HostState)
	if err != nil {
		return failExecute(p, report, out, ClassInfra, "auth_profile", err, true)
	}
	if err := materialize(p, plans.Projection); err != nil {
		return failExecute(p, report, out, ClassInfra, "materialize", err, true)
	}
	if err := expandWorkspaceInFiles(p, plans.Projection); err != nil {
		return failExecute(p, report, out, ClassInfra, "materialize", err, true)
	}
	sec, err := resolveSecrets(plans.Launch.Secrets, opts.Getenv)
	if err != nil {
		return failExecute(p, report, out, ClassPreLaunch, "secrets", err, true)
	}
	out.Omitted = sec.omitted
	if err := persistSecretBaselines(p, plans.Projection); err != nil {
		return failExecute(p, report, out, ClassInfra, "secrets", err, true)
	}
	if err := overlaySecrets(p, plans.Projection, sec.values, sec.omitted); err != nil {
		return abortAfterSecretWrite(p, report, out, plans.Projection, sec.values, err, "secrets")
	}
	if err := writeLiveLock(p); err != nil {
		return abortAfterSecretWrite(p, report, out, plans.Projection, sec.values, err, "live_lock")
	}
	overlaid := true
	defer func() {
		if perr := finalizeHostAuth(p, opts.HostState); perr != nil {
			out.HostStateSaveFailed = true
			if opts.OnHostStateSaveFailure != nil {
				opts.OnHostStateSaveFailure()
			} else {
				hostAuthNote(opts.Stderr, "save", perr)
			}
		}
		if overlaid {
			if werr := wipeSecretsAfterRun(p, plans.Projection, sec.values); werr != nil {
				out.Class = ClassInfra
				out.Stage = "wipe_secrets"
				wipeErr := fmt.Errorf("%w: %v", ErrSecretWipe, werr)
				_ = writeRecord(p, report, out, wipeErr)
				clearLiveLock(p)
				if qerr := quarantineAfterRun(p); qerr != nil {
					if err == nil {
						err = fmt.Errorf("%w: %v (quarantine: %v)", ErrSecretWipe, werr, qerr)
					}
					return
				}
				if err == nil {
					err = wipeErr
				}
				return
			}
		}
		clearLiveLock(p)
		if rerr := writeRecord(p, report, out, err); rerr != nil && err == nil {
			out.Class = ClassInfra
			out.Stage = "record"
			err = rerr
		}
		if derr := deleteWorkspaceAfterRun(p); derr != nil {
			out.Class = ClassInfra
			out.Stage = "cleanup"
			_ = writeRecord(p, report, out, derr)
			if qerr := quarantineAfterRun(p); qerr != nil {
				if err == nil {
					err = fmt.Errorf("%w: %v (quarantine: %v)", ErrWorkspaceCleanup, derr, qerr)
				}
				return
			}
			if err == nil {
				err = fmt.Errorf("%w: %v", ErrWorkspaceCleanup, derr)
			}
		}
	}()
	if err := adapter.VerifyLaunchPlan(plans.Launch); err != nil {
		return failExecute(p, report, out, ClassPreLaunch, "launch_reconcile", err, false)
	}
	bind, err := bindHostAuth(p, opts.HostState)
	if err != nil {
		if opts.OnHostStateBindFailure != nil {
			opts.OnHostStateBindFailure()
		} else {
			hostAuthNote(opts.Stderr, "bind", err)
		}
	}
	stopPersist := watchHostAuth(p, opts.HostState)
	defer stopPersist()
	exe := plans.Launch.Executable
	cmd := exec.Command(exe, expandLaunchArgs(p, append(append([]string{}, plans.Launch.Args...), bind.Args...))...)
	cmd.Dir = launchCWD(p, plans.Launch.WorkingDirClass)
	// Minimal parent env + Plan env + Host-state bind env + authorized secrets.
	// Do not inherit the full parent process environment.
	cmd.Env = append(baseLaunchEnv(opts.Getenv), expandLaunchEnv(p, mergeLaunchEnv(plans.Launch.Env, bind.Env))...)
	cmd.Env = append(cmd.Env, sec.env...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	if opts.OnHostStarting != nil {
		opts.OnHostStarting()
	}
	process, runErr := runHostProcessFn(cmd)
	out.Class = ClassHostProcess
	out.Stage = "host"
	out.Interrupted = process.Interrupted
	if cmd.ProcessState != nil {
		out.ExitCode = cmd.ProcessState.ExitCode()
	}
	if runErr != nil {
		if _, ok := runErr.(*exec.ExitError); ok {
			return out, nil
		}
		return failExecute(p, report, out, ClassInfra, "launch", runErr, false)
	}
	return out, nil
}

// failExecute records a pre-host or infrastructure failure. dropWorkspace is
// for returns before the Execute defer; after the defer, wipe then delete.
func failExecute(p Prepared, report compatibility.Report, out Outcome, class, stage string, err error, dropWorkspace bool) (Outcome, error) {
	out.Class = class
	out.Stage = stage
	_ = writeRecord(p, report, out, err)
	if dropWorkspace {
		if derr := deleteWorkspaceAfterRun(p); derr != nil {
			if qerr := quarantineAfterRun(p); qerr != nil {
				return out, fmt.Errorf("%w (cleanup: %v; quarantine: %v)", err, derr, qerr)
			}
		}
	}
	return out, err
}

// abortAfterSecretWrite restores placeholders after a pre-launch failure that
// may have already written secret values. Wipe failure must not fall through
// to delete: isolate or keep the debris.
func abortAfterSecretWrite(p Prepared, report compatibility.Report, out Outcome, plan adapter.NativeProjectionPlan, values map[string]string, cause error, stage string) (Outcome, error) {
	if werr := wipeSecretsAfterRun(p, plan, values); werr != nil {
		out.Class = ClassInfra
		out.Stage = "wipe_secrets"
		err := fmt.Errorf("%w: %v", ErrSecretWipe, werr)
		if cause != nil {
			err = fmt.Errorf("%w (%v)", err, cause)
		}
		_ = writeRecord(p, report, out, err)
		if qerr := quarantineAfterRun(p); qerr != nil {
			return out, fmt.Errorf("%w (quarantine: %v)", err, qerr)
		}
		return out, err
	}
	return failExecute(p, report, out, ClassInfra, stage, cause, true)
}

// RejectNativeArgs is the V0 fail-closed allowlist (empty).
func RejectNativeArgs(args []string) error {
	if len(args) == 0 {
		return nil
	}
	return ErrNativeArgs
}
