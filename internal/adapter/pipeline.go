package adapter

import (
	"errors"
	"fmt"

	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/space"
)

var ErrNilRun = errors.New("adapter: nil ResolvedAgentRun")

// Outcome is Select → Probe → Assess → Evaluate → (optional) Project.
type Outcome struct {
	Report compatibility.Report
	Plans  *Plans
}

// Evaluation is the pipeline stopped before Project. Execute-bound
// callers apply warning policy, then Project.
type Evaluation struct {
	Adapter HostAdapter
	Probe   ProbeResult
	Report  compatibility.Report
	Intent  ControlIntent
}

// EvaluatePipeline runs Select → Probe → Assess → Evaluate. It does not Project.
func EvaluatePipeline(reg *Registry, hostID string, run *space.ResolvedAgentRun, pctx ProjectionContext, a2hVersion string, policy RunPolicy) (Evaluation, error) {
	a, err := reg.Select(hostID)
	if err != nil {
		return Evaluation{}, err
	}
	if run == nil {
		return Evaluation{}, ErrNilRun
	}
	probe, err := a.Probe()
	if err != nil {
		return Evaluation{}, err
	}
	assess, intent, err := a.Assess(run, probe)
	if err != nil {
		return Evaluation{}, err
	}
	req, err := compatibility.BuildRequirement(run)
	if err != nil {
		return Evaluation{}, err
	}
	env := compatibility.Envelope{
		SchemaVersion:     "agent2host/compatibility-report/v1alpha1",
		Agent2HostVersion: a2hVersion,
		Subject: compatibility.Subject{
			SystemID: run.SystemID, AgentID: run.AgentID, Revision: run.ArtifactRevision,
		},
		Host:    compatibility.HostRef{ID: probe.HostID, Version: probe.HostVersion},
		Adapter: compatibility.AdapterRef{ID: a.Descriptor().AdapterID, Version: a.Descriptor().AdapterVersion},
		Probe:   compatibility.Probe{Fingerprint: probe.Fingerprint},
	}
	if env.Subject.Revision == "" {
		env.Subject.Revision = "sha256:" + fmt.Sprintf("%064d", 0)
	}
	if env.Host.Version == "" {
		env.Host.Version = "unknown"
	}
	report := compatibility.Evaluate(env, req, assess)
	report = ApplyRunPolicy(report, probe, hostID, pctx, policy)
	return Evaluation{
		Adapter: a,
		Probe:   probe,
		Report:  report,
		Intent:  intent,
	}, nil
}

// Project emits plans for this Evaluation. Refused reports must not Project.
func (e Evaluation) Project(run *space.ResolvedAgentRun, pctx ProjectionContext) (Plans, error) {
	if e.Adapter == nil {
		return Plans{}, errors.New("adapter: empty evaluation")
	}
	np, lp, err := e.Adapter.Project(run, e.Probe, e.Report, pctx)
	if err != nil {
		return Plans{}, err
	}
	// Sole common reconcile (Evaluate intent). Adapters and FixtureDriver must not call it.
	if err := ReconcilePlansCommon(e.Intent, e.Report, np, lp); err != nil {
		return Plans{}, err
	}
	return Plans{Projection: np, Launch: lp}, nil
}

// RunPipeline is the check path: Evaluate, then pure-Project when not refused.
// Execute-bound callers must use EvaluatePipeline + warning policy instead.
func RunPipeline(reg *Registry, hostID string, run *space.ResolvedAgentRun, pctx ProjectionContext, a2hVersion string, policy RunPolicy) (Outcome, error) {
	ev, err := EvaluatePipeline(reg, hostID, run, pctx, a2hVersion, policy)
	if err != nil {
		return Outcome{}, err
	}
	out := Outcome{Report: ev.Report}
	if ev.Report.Decision == "refused" {
		return out, nil
	}
	plans, err := ev.Project(run, pctx)
	if err != nil {
		return out, err
	}
	out.Plans = &plans
	return out, nil
}
