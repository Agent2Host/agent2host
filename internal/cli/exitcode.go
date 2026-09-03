package cli

import (
	"errors"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/runtime"
	"github.com/agent2host/agent2host/internal/space"
	"github.com/agent2host/agent2host/internal/space/registry"
)

// Exit codes are the stable CLI contract.
const (
	ExitOK           = 0
	ExitRefused      = 1
	ExitUsage        = 2
	ExitPrecondition = 3
	ExitHostProcess  = 4
	ExitInternal     = 70
)

func exitParseTarget(err error) int {
	var se *space.Error
	if errors.As(err, &se) && se.Kind == space.KindBadTarget {
		return ExitUsage
	}
	return ExitPrecondition
}

func exitSpaceOrRegistry(err error) int {
	if err == nil {
		return ExitPrecondition
	}
	var se *space.Error
	if errors.As(err, &se) {
		return ExitPrecondition
	}
	var re *registry.Error
	if errors.As(err, &re) {
		return ExitPrecondition
	}
	return ExitPrecondition
}

func exitAuthorize(err error) int {
	if errors.Is(err, ErrExecuteRefused) ||
		errors.Is(err, ErrWarningsNotAccepted) ||
		errors.Is(err, ErrAcceptanceMismatch) {
		return ExitRefused
	}
	return exitProject(err)
}

func exitProject(err error) int {
	if errors.Is(err, adapter.ErrIntentMismatch) || errors.Is(err, adapter.ErrSemanticMismatch) {
		return ExitInternal
	}
	return ExitPrecondition
}

func exitExecute(out runtime.Outcome, err error) int {
	if out.Class == runtime.ClassHostProcess {
		if err == nil && out.ExitCode == 0 {
			return ExitOK
		}
		return ExitHostProcess
	}
	if out.Class == runtime.ClassInfra {
		return ExitInternal
	}
	return ExitPrecondition
}
