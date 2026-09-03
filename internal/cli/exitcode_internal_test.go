package cli

import (
	"testing"

	"github.com/agent2host/agent2host/internal/runtime"
)

func TestExitExecuteHostFailure(t *testing.T) {
	if exitExecute(runtime.Outcome{Class: runtime.ClassHostProcess, ExitCode: 2}, nil) != ExitHostProcess {
		t.Fatal("host non-zero must exit 4")
	}
	if exitExecute(runtime.Outcome{Class: runtime.ClassHostProcess, ExitCode: 0}, nil) != ExitOK {
		t.Fatal("host zero must exit 0")
	}
	if exitExecute(runtime.Outcome{Class: runtime.ClassPreLaunch}, errExecuteSample()) != ExitPrecondition {
		t.Fatal("pre-launch must exit 3")
	}
}

func errExecuteSample() error { return errSample }

var errSample = &sampleErr{}

type sampleErr struct{}

func (e *sampleErr) Error() string { return "sample" }
