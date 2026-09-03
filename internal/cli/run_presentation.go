package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/runtime"
)

func hostDisplayName(hostID string) string {
	switch hostID {
	case adapter.HostClaudeCode:
		return "Claude Code"
	case adapter.HostKiro:
		return "Kiro"
	case adapter.HostCodex:
		return "Codex"
	default:
		return hostID
	}
}

func printRunReadNotice(w io.Writer, requireStrict bool, hostID string, report compatibility.Report, pctx adapter.ProjectionContext) {
	if w == nil || requireStrict || adapter.StrictReadEnforced(hostID, pctx) || report.Decision == "refused" {
		return
	}
	fmt.Fprintln(w, "This session can read files outside your project folder.")
}

func renderRunAuthorizationFailure(stdout, stderr io.Writer, report compatibility.Report, err error, verbose bool) {
	switch {
	case errors.Is(err, ErrWarningsNotAccepted):
		if stdinIsTTY() {
			fmt.Fprintln(stdout, "Canceled. No session was started.")
		} else {
			fmt.Fprintln(stderr, "Not started. This run needs an explicit warning confirmation.")
		}
	case errors.Is(err, ErrExecuteRefused):
		fmt.Fprintln(stderr, "Not started. This Host cannot meet this Agent's requirements.")
	default:
		fmt.Fprintln(stderr, "Not started. Agent2Host could not prepare this session.")
	}
	renderRunDetails(stderr, report, runtime.Outcome{}, err, verbose)
}

func renderRunOutcome(stdout, stderr io.Writer, report compatibility.Report, out runtime.Outcome, err error, verbose bool) {
	switch {
	case out.Stage == "wipe_secrets":
		fmt.Fprintln(stderr, "Session ended, but Agent2Host could not clean up all temporary run files.")
	case out.Class == runtime.ClassHostProcess:
		if out.Interrupted || out.ExitCode == 0 {
			fmt.Fprintln(stdout, "Session ended.")
		} else {
			fmt.Fprintln(stderr, "Session ended unexpectedly. Run again with --verbose if you need details.")
		}
	case errors.Is(err, runtime.ErrMissingSecret):
		if name := missingCredentialName(err); name != "" {
			fmt.Fprintf(stderr, "Not started. A required credential is missing: %s.\n", name)
		} else {
			fmt.Fprintln(stderr, "Not started. A required credential is missing.")
		}
	case out.Stage == "launch":
		fmt.Fprintf(stderr, "Could not start %s. Make sure it can start normally, then try again.\n", hostDisplayName(report.Host.ID))
	default:
		fmt.Fprintln(stderr, "Not started. Agent2Host could not prepare this session. Run again with --verbose for details.")
	}
	if out.HostStateSaveFailed {
		fmt.Fprintln(stderr, "Host sign-in updates from this session might not have been saved.")
	}
	renderRunDetails(stderr, report, out, err, verbose)
}

func missingCredentialName(err error) string {
	if err == nil {
		return ""
	}
	prefix := runtime.ErrMissingSecret.Error() + ": "
	if name, ok := strings.CutPrefix(err.Error(), prefix); ok {
		return name
	}
	return ""
}

func renderRunDetails(w io.Writer, report compatibility.Report, out runtime.Outcome, runErr error, verbose bool) {
	if !verbose || w == nil {
		return
	}
	fmt.Fprintln(w, "Details:")
	if report.Decision != "" {
		fmt.Fprintf(w, "  decision: %s\n", report.Decision)
	}
	if out.RunID != "" {
		fmt.Fprintf(w, "  run: %s\n", out.RunID)
	}
	if out.Class != "" {
		fmt.Fprintf(w, "  class: %s\n", out.Class)
	}
	if out.Stage != "" {
		fmt.Fprintf(w, "  stage: %s\n", out.Stage)
	}
	if out.Class == runtime.ClassHostProcess {
		fmt.Fprintf(w, "  exit: %d\n", out.ExitCode)
	}
	if out.Interrupted {
		fmt.Fprintln(w, "  interrupted: true")
	}
	if runErr != nil {
		fmt.Fprintf(w, "  error: %v\n", runErr)
	}
}
