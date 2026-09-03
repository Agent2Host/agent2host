package adapter

import (
	"errors"
	"fmt"
	"strings"

	"github.com/agent2host/agent2host/internal/compatibility"
)

// ErrSemanticMismatch is Project failure: Plans do not semantically match ControlIntent.
var ErrSemanticMismatch = errors.New("adapter: semantic plan mismatch")

var forbiddenLaunchArgs = []string{
	"--dangerously-skip-permissions",
	"--dangerously-bypass-approvals-and-sandbox",
	"--dangerously-bypass-hook-trust",
}

// ReconcilePlansCommon runs structural include/control reconcile and launch safety.
// Call only from Evaluation.Project. Host ReconcilePlans run semantic only —
// do not call this from those helpers, or common runs twice.
func ReconcilePlansCommon(intent ControlIntent, report compatibility.Report, np NativeProjectionPlan, lp LaunchPlan) error {
	if err := reconcileIntent(intent, report, np, lp); err != nil {
		return err
	}
	return VerifyLaunchPlan(lp)
}

// VerifyLaunchPlan is Runtime pre-launch safety (SEC-RUN-FAILCLOSED / diagnostics pre-spawn).
func VerifyLaunchPlan(lp LaunchPlan) error {
	for _, a := range lp.Args {
		for _, forbidden := range forbiddenLaunchArgs {
			if a == forbidden || strings.HasPrefix(a, forbidden+"=") {
				return fmt.Errorf("%w: forbidden launch arg %s", ErrSemanticMismatch, forbidden)
			}
		}
	}
	return nil
}

func ProjectionContent(np NativeProjectionPlan, rel string) ([]byte, bool) {
	for _, f := range np.Files {
		if f.RelPath == rel {
			return f.Content, true
		}
	}
	return nil, false
}

func LaunchArgValue(args []string, flag string) (string, bool) {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1], true
		}
		if strings.HasPrefix(a, flag+"=") {
			return strings.TrimPrefix(a, flag+"="), true
		}
	}
	return "", false
}

func LaunchArgPresent(args []string, flag string) bool {
	for _, a := range args {
		if a == flag || strings.HasPrefix(a, flag+"=") {
			return true
		}
	}
	return false
}

func StringSetContains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func StringSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ma := map[string]int{}
	for _, s := range a {
		ma[s]++
	}
	for _, s := range b {
		ma[s]--
		if ma[s] < 0 {
			return false
		}
	}
	for _, n := range ma {
		if n != 0 {
			return false
		}
	}
	return true
}
