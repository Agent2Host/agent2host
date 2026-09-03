package cli

import (
	"io"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/runtime"
	"github.com/agent2host/agent2host/internal/space"
)

// AuthorizeOpts is the Execute-bound path through warning policy and Project.
type AuthorizeOpts struct {
	Registry          *adapter.Registry
	Host              string
	Run               *space.ResolvedAgentRun
	Pctx              adapter.ProjectionContext
	Agent2HostVersion string
	AcceptWarnings    bool
	RequireStrictRead bool
	TTY               bool
	Stdin             io.Reader
	Stderr            io.Writer
	ShowStartNotice   bool
}

// AuthorizeExecute evaluates, applies warning policy, then Projects and mints.
// Decline / refuse must not Project. check must not call this.
func AuthorizeExecute(opts AuthorizeOpts) (*runtime.Authorization, compatibility.Report, error) {
	ev, err := adapter.EvaluatePipeline(opts.Registry, opts.Host, opts.Run, opts.Pctx, opts.Agent2HostVersion, adapter.RunPolicy{
		RequireStrictRead: opts.RequireStrictRead,
		ForExecute:        true,
	})
	if err != nil {
		return nil, compatibility.Report{}, err
	}
	if opts.ShowStartNotice && ev.Report.Decision != "refused" {
		printRunReadNotice(opts.Stderr, opts.RequireStrictRead, opts.Host, ev.Report, opts.Pctx)
	}
	acc, err := ConfirmWarnings(ev.Report, opts.AcceptWarnings, opts.TTY, opts.Stdin, opts.Stderr)
	if err != nil {
		return nil, ev.Report, err
	}
	plans, err := ev.Project(opts.Run, opts.Pctx)
	if err != nil {
		return nil, ev.Report, err
	}
	auth, err := MintAuthorization(ev.Report, plans, acc, opts.Host)
	if err != nil {
		return nil, ev.Report, err
	}
	return auth, ev.Report, nil
}

// MintAuthorization binds this Report and these exact Plans. Runtime does
// not mint. A mismatched acceptance must not authorize Execute.
func MintAuthorization(r compatibility.Report, plans adapter.Plans, acc WarningAcceptance, hostID string) (*runtime.Authorization, error) {
	if r.Decision == "refused" {
		return nil, ErrExecuteRefused
	}
	if !acc.Matches(r) {
		return nil, ErrAcceptanceMismatch
	}
	digest, err := adapter.DigestPlans(plans)
	if err != nil {
		return nil, err
	}
	return &runtime.Authorization{
		Binding:          compatibility.BindingOf(r),
		Decision:         r.Decision,
		WarningSet:       compatibility.WarningSet(r),
		HostID:           hostID,
		Plans:            plans,
		PlansDigest:      digest,
		AcceptedWarnings: r.Decision == "allowed_with_warnings",
		Executable:       plans.Launch.Executable,
	}, nil
}
