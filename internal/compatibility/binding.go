package compatibility

import (
	"sort"
	"strings"
)

// Binding is the Report reuse identity.
type Binding struct {
	SystemID          string
	AgentID           string
	Revision          string
	Fingerprint       string
	AdapterVersion    string
	Agent2HostVersion string
}

// OfficialReuse is lawful only when every binding field matches and the
// existing Report carried a fingerprint.
func OfficialReuse(existing, current Binding) bool {
	if existing.Fingerprint == "" || current.Fingerprint == "" {
		return false
	}
	return existing.SystemID == current.SystemID &&
		existing.AgentID == current.AgentID &&
		existing.Revision == current.Revision &&
		existing.Fingerprint == current.Fingerprint &&
		existing.AdapterVersion == current.AdapterVersion &&
		existing.Agent2HostVersion == current.Agent2HostVersion
}

// MayProject is false when the official Report must not authorize Project.
func MayProject(existing, current Binding) bool {
	return OfficialReuse(existing, current)
}

// BindingOf is the official identity of an evaluated Report.
func BindingOf(r Report) Binding {
	return Binding{
		SystemID:          r.Subject.SystemID,
		AgentID:           r.Subject.AgentID,
		Revision:          r.Subject.Revision,
		Fingerprint:       r.Probe.Fingerprint,
		AdapterVersion:    r.Adapter.Version,
		Agent2HostVersion: r.Agent2HostVersion,
	}
}

// WarningSet is the decision plus sorted reason_codes. A changed set
// invalidates a prior warning acceptance.
func WarningSet(r Report) string {
	codes := collectReasonCodes(r)
	sort.Strings(codes)
	return r.Decision + "\n" + strings.Join(codes, "\n")
}

func collectReasonCodes(r Report) []string {
	var out []string
	add := func(c string) {
		if c != "" {
			out = append(out, c)
		}
	}
	add(r.Activation.ReasonCode)
	add(r.Capabilities.SOP.ReasonCode)
	add(r.Capabilities.OutputSchema.ReasonCode)
	for _, it := range r.Capabilities.Skills.Items {
		add(it.ReasonCode)
	}
	for _, it := range r.Capabilities.Context.Items {
		add(it.ReasonCode)
	}
	for _, it := range r.Capabilities.MCP.Items {
		add(it.ReasonCode)
	}
	for _, it := range r.Capabilities.Hooks.Items {
		add(it.ReasonCode)
	}
	add(r.Security.Permissions.ReasonCode)
	add(r.Security.Approvals.ReasonCode)
	add(r.Security.Sandbox.ReasonCode)
	add(r.Security.OutputValidation.ReasonCode)
	for _, it := range r.Security.ContextIsolation.Items {
		add(it.ReasonCode)
	}
	for _, it := range r.Security.MCPToolIsolation.Items {
		add(it.ReasonCode)
	}
	for _, it := range r.Security.SecretIsolation.Items {
		add(it.ReasonCode)
	}
	return out
}
