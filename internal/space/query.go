package space

import (
	"sort"
	"strings"

	"github.com/agent2host/agent2host/internal/source/decode"
)

// ParseTarget splits <system-id>/<agent-id>.
func ParseTarget(s string) (systemID, agentID string, err error) {
	systemID, agentID, ok := strings.Cut(s, "/")
	if !ok || systemID == "" || agentID == "" || strings.Contains(agentID, "/") {
		return "", "", fail(KindBadTarget, s)
	}
	return systemID, agentID, nil
}

// ListedSystem is one registry row for list (--json).
type ListedSystem struct {
	ID             string   `json:"id"`
	Version        string   `json:"version"`
	ActiveRevision string   `json:"active_revision"`
	AgentCount     int      `json:"agent_count"`
	Agents         []string `json:"agents"`
	Source         string   `json:"source"`
}

// Inspection is Space-side facts for inspect without --host (--json).
type Inspection struct {
	SystemID         string          `json:"system_id"`
	AgentID          string          `json:"agent_id"`
	Version          string          `json:"version"`
	ArtifactRevision string          `json:"artifact_revision"`
	Source           string          `json:"source"`
	Agents           []string        `json:"agents"`
	Skills           []string        `json:"skills"`
	WorkRoot         decode.WorkRoot `json:"work_root"`
}

// List returns registered systems sorted by id.
func (s *Space) List() ([]ListedSystem, error) {
	doc, err := s.registry.Load()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(doc.Systems))
	for id := range doc.Systems {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]ListedSystem, 0, len(ids))
	for _, id := range ids {
		rec := doc.Systems[id]
		agents := append([]string(nil), rec.Agents...)
		out = append(out, ListedSystem{
			ID:             id,
			Version:        rec.Version,
			ActiveRevision: rec.ActiveRevision,
			AgentCount:     len(agents),
			Agents:         agents,
			Source:         rec.Source,
		})
	}
	return out, nil
}

// Inspect reports registry facts plus the Named Agent’s resolved Skill ids.
// Agent existence and artifact integrity come from Resolve (the bound
// Artifact), not from the registry agent index. Registry Agents remain
// Space-side facts on the result.
func (s *Space) Inspect(systemID, agentID string) (*Inspection, error) {
	rec, err := s.registry.Get(systemID)
	if err != nil {
		return nil, err
	}
	run, err := s.Resolve(systemID, agentID, rec.ActiveRevision)
	if err != nil {
		return nil, err
	}
	var skills []string
	for _, sk := range run.Skills {
		skills = append(skills, sk.ID)
	}
	return &Inspection{
		SystemID:         systemID,
		AgentID:          agentID,
		Version:          rec.Version,
		ArtifactRevision: rec.ActiveRevision,
		Source:           rec.Source,
		Agents:           append([]string(nil), rec.Agents...),
		Skills:           skills,
		WorkRoot:         run.WorkRoot,
	}, nil
}
