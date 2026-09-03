// Package adapter is the Host Compatibility layer: probe a Host, assess a
// Named Agent against it, project files and a launch plan, then bind the Host
// to a stable Agent2Host-owned state directory.
//
// Layout
//
//	this package     contract, pipeline, shared helpers (used by every Host)
//	hoststate/       stable Host home bind (not an Agent2Host login API)
//	claude/          Claude Code adapter
//	codex/           Codex adapter
//	kiro/            Kiro adapter
//	committed/       wires the Hosts this release ships
//
// Adding a Host
//
//  1. Create internal/adapter/<name>/ with New(look, version) HostAdapter.
//  2. Implement Probe, Assess, Project, and HostState (DescribeAuth /
//     BindForRun / FinalizeRun). HostState points the real Host at a stable
//     directory — it is not a login API.
//  3. Add the host id to hosts.go. Add a version prefix in
//     verified_versions.go only after you have verified that version.
//  4. Register the constructor in committed.New.
//  5. Project computes intent once and runs semantic reconcile only.
//     Do not call ReconcilePlansCommon from the Host package — Evaluation.Project
//     owns that. FixtureDriver.Project also must not reconcile.
//  6. Secret walks (environment, MCP, hooks) use AppendEnvironmentSecrets,
//     AppendMCPSecrets, and AppendHookSecrets. Do not copy those loops.
package adapter
