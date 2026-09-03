package adapter

import (
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/space"
)

// ProbeResult is observed Host-install facts. It must not carry Evaluate fields.
type ProbeResult struct {
	HostID      string
	HostVersion string
	Fingerprint string
	Executable  string
	Found       bool
}

// FixtureDriver returns canned Probe/Assessment facts. It is a test driver,
// not a committed Host adapter.
type FixtureDriver struct {
	Observed   ProbeResult
	Assessment compatibility.Assessment
	AdapterID  string
	AdapterVer string
}

func (d FixtureDriver) Descriptor() Descriptor {
	id := d.AdapterID
	if id == "" {
		id = d.Observed.HostID
	}
	return Descriptor{AdapterID: id, HostID: d.Observed.HostID, AdapterVersion: d.AdapterVer}
}

func (d FixtureDriver) Probe() (ProbeResult, error) {
	return d.Observed, nil
}

func (d FixtureDriver) Assess(_ *space.ResolvedAgentRun, _ ProbeResult) (compatibility.Assessment, ControlIntent, error) {
	return d.Assessment, ControlIntent{}, nil
}

// Project emits a canned plan. It does not reconcile — Evaluation.Project
// runs ReconcilePlansCommon once after this returns. Do not add reconcile here.
func (d FixtureDriver) Project(run *space.ResolvedAgentRun, probe ProbeResult, report compatibility.Report, _ ProjectionContext) (NativeProjectionPlan, LaunchPlan, error) {
	if err := RefuseIfNeeded(report); err != nil {
		return NativeProjectionPlan{}, LaunchPlan{}, err
	}
	var files []ProjectionFile
	if run != nil && SOPIncluded(report) && run.SOP != "" {
		files = append(files, File("SOP.md", run, run.SOP, []byte("# SOP\n")))
	}
	lp := Launch(probe, "fixture")
	if run != nil {
		lp.Secrets = SecretRefs(run)
	}
	return NativeProjectionPlan{HostID: d.Observed.HostID, Files: files}, lp, nil
}

func (d FixtureDriver) HostState() HostStateBinder {
	host := d.Observed.HostID
	if host == "" {
		host = "fixture"
	}
	return NoopHostState{Desc: AuthDescription{
		Profile:     AuthProfileKey{Host: host, Provider: "fixture", NativeAuthNamespace: "default"},
		Topology:    AuthTopologyExternal,
		Concurrency: AuthConcurrencySafe,
	}}
}

func (d FixtureDriver) Envelope(subject compatibility.Subject, a2hVersion string) compatibility.Envelope {
	return compatibility.Envelope{
		SchemaVersion:     "agent2host/compatibility-report/v1alpha1",
		Agent2HostVersion: a2hVersion,
		Subject:           subject,
		Host:              compatibility.HostRef{ID: d.Observed.HostID, Version: d.Observed.HostVersion},
		Adapter:           compatibility.AdapterRef{ID: d.AdapterID, Version: d.AdapterVer},
		Probe:             compatibility.Probe{Fingerprint: d.Observed.Fingerprint},
	}
}
