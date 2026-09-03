package adapter

import (
	"errors"
	"fmt"
	"sort"

	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/space"
)

// ErrUnsupportedHost is Select refusal: host_id is not in this release’s registry.
var ErrUnsupportedHost = errors.New("adapter: unsupported host")

// ErrProjectRefused is Project refusal when Report.decision is refused.
var ErrProjectRefused = errors.New("adapter: project refused")

// Descriptor is static Adapter identity.
// It must not contain capability profiles or Probe/Assess judgments.
type Descriptor struct {
	AdapterID      string
	HostID         string
	AdapterVersion string
}

// HostAdapter is one committed host adapter, built into this release.
// It is not a runtime plugin.
type HostAdapter interface {
	Descriptor() Descriptor
	Probe() (ProbeResult, error)
	Assess(run *space.ResolvedAgentRun, probe ProbeResult) (compatibility.Assessment, ControlIntent, error)
	Project(run *space.ResolvedAgentRun, probe ProbeResult, report compatibility.Report, pctx ProjectionContext) (NativeProjectionPlan, LaunchPlan, error)
	HostState() HostStateBinder
}

// Registry is SupportedHostRegistry for this release.
type Registry struct {
	byHost map[string]HostAdapter
}

// NewRegistry builds a registry from adapters. Duplicate host_id is refused.
func NewRegistry(adapters ...HostAdapter) (*Registry, error) {
	r := &Registry{byHost: map[string]HostAdapter{}}
	for _, a := range adapters {
		id := a.Descriptor().HostID
		if id == "" {
			return nil, fmt.Errorf("adapter: empty host_id")
		}
		if _, ok := r.byHost[id]; ok {
			return nil, fmt.Errorf("adapter: duplicate host_id %s", id)
		}
		r.byHost[id] = a
	}
	return r, nil
}

// Select returns the Adapter for host_id or ErrUnsupportedHost.
func (r *Registry) Select(hostID string) (HostAdapter, error) {
	if r == nil {
		return nil, ErrUnsupportedHost
	}
	a, ok := r.byHost[hostID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedHost, hostID)
	}
	return a, nil
}

// HostIDs returns sorted registered host ids.
func (r *Registry) HostIDs() []string {
	if r == nil {
		return nil
	}
	ids := make([]string, 0, len(r.byHost))
	for id := range r.byHost {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

const AdapterVersion = "0.1.0"
