package runtime

// Execute authorization (minted by the CLI, checked before spawn).
// Not Host login. Host-state bind is hoststate.go.

import (
	"errors"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
)

var (
	ErrUnauthorized       = errors.New("runtime: missing execution authorization")
	ErrRefused            = errors.New("runtime: compatibility refused")
	ErrBindingMismatch    = errors.New("runtime: authorization binding mismatch")
	ErrExecutableMismatch = errors.New("runtime: launch executable does not match authorization")
	ErrMissingSecret      = errors.New("runtime: required secret missing")
	ErrSecretWipe         = errors.New("runtime: secret wipe failed")
	ErrSecretWidened      = errors.New("runtime: refusing to inject secret outside authorized consumer")
	ErrPathEscape         = errors.New("runtime: projection path escapes namespace")
	ErrNativeArgs         = errors.New("runtime: unknown native args rejected")
	ErrHostBusy           = errors.New("runtime: this host is already running; wait for the other session to exit")
	ErrHostStateNeedsHost = errors.New("clean --host-state requires --host")
	ErrUnknownHost        = errors.New("clean --host-state --host must be a supported Host id")
	ErrWorkspaceCleanup   = errors.New("runtime: run workspace could not be removed")
	ErrQuarantine         = errors.New("runtime: leftover could not be quarantined")
	ErrAcceptanceRequired = errors.New("runtime: allowed_with_warnings requires accepted warnings")
)

// Authorization is in-process Execute proof.
// Orchestrator mints it; Runtime and Adapters must not.
type Authorization struct {
	Binding          compatibility.Binding
	Decision         string
	WarningSet       string
	HostID           string
	Plans            adapter.Plans
	PlansDigest      string
	AcceptedWarnings bool
	Executable       string
}

// ValidFor reports whether this token authorizes Execute of this Report and these Plans.
func (a *Authorization) ValidFor(r compatibility.Report, plans adapter.Plans) error {
	if a == nil {
		return ErrUnauthorized
	}
	if r.Decision == "refused" || a.Decision == "refused" {
		return ErrRefused
	}
	if r.Decision == "allowed_with_warnings" || a.Decision == "allowed_with_warnings" {
		if !a.AcceptedWarnings {
			return ErrAcceptanceRequired
		}
	}
	if a.Binding != compatibility.BindingOf(r) || a.WarningSet != compatibility.WarningSet(r) || a.Decision != r.Decision {
		return ErrBindingMismatch
	}
	got, err := adapter.DigestPlans(plans)
	if err != nil || a.PlansDigest == "" || got != a.PlansDigest {
		return ErrBindingMismatch
	}
	if plans.Launch.Executable == "" {
		return ErrExecutableMismatch
	}
	if a.Executable != "" && a.Executable != plans.Launch.Executable {
		return ErrExecutableMismatch
	}
	return nil
}
