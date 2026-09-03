package runtime

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// HostProcessResult records facts observed while Agent2Host waited for the Host.
// It records Ctrl-C itself rather than inferring it from a Host exit code:
// Hosts are free to use any exit code after an interrupt.
type HostProcessResult struct {
	Interrupted bool
}

// runHostProcess keeps Agent2Host alive when the terminal interrupts it, so it
// can wait for the Host to exit and run finalization (including host-auth
// persistence). The Host remains in the foreground process group: an
// interactive Host must be able to read the terminal.
func runHostProcess(cmd *exec.Cmd) (HostProcessResult, error) {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	return runHostProcessWithSignalsAndStarted(cmd, signals, nil)
}

// runHostProcessWithSignals exists so the signal-to-Host path can be verified
// without delivering an interrupt to the test process.
func runHostProcessWithSignals(cmd *exec.Cmd, signals <-chan os.Signal) (HostProcessResult, error) {
	return runHostProcessWithSignalsAndStarted(cmd, signals, nil)
}

func runHostProcessWithSignalsAndStarted(cmd *exec.Cmd, signals <-chan os.Signal, started chan<- *os.Process) (HostProcessResult, error) {
	if err := cmd.Start(); err != nil {
		return HostProcessResult{}, err
	}
	if started != nil {
		started <- cmd.Process
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	result := HostProcessResult{}
	for {
		select {
		case err := <-done:
			return result, err
		case sig := <-signals:
			// Ctrl-C is delivered by the terminal to Agent2Host and the Host
			// together. Do not send it a second time: Claude uses the first
			// Ctrl-C to cancel work and the second to exit.
			if sig == os.Interrupt {
				result.Interrupted = true
			} else {
				if err := cmd.Process.Signal(sig); err != nil && !errors.Is(err, os.ErrProcessDone) {
					return result, err
				}
			}
		}
	}
}

var runHostProcessFn = runHostProcess
