package compatibility

import (
	"path/filepath"
	"testing"
)

func TestOfficialReuseProtocolFixtures(t *testing.T) {
	root, err := fixtureRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"row-10-revision-mismatch.json", "row-10b-fingerprint-mismatch.json"} {
		t.Run(name, func(t *testing.T) {
			var doc struct {
				Existing ReportBindingJSON `json:"existing_report"`
				Current  ReportBindingJSON `json:"current_binding"`
				Expected struct {
					Reuse   bool `json:"reuse"`
					Project bool `json:"project"`
				} `json:"expected"`
			}
			mustJSON(t, filepath.Join(root, "protocol", name), &doc)
			reuse := OfficialReuse(doc.Existing.binding(), doc.Current.binding())
			if reuse != doc.Expected.Reuse {
				t.Fatalf("reuse got %v want %v", reuse, doc.Expected.Reuse)
			}
			if MayProject(doc.Existing.binding(), doc.Current.binding()) != doc.Expected.Project {
				t.Fatalf("project got %v want %v", !doc.Expected.Project, doc.Expected.Project)
			}
		})
	}
}

func TestBindingOfAndWarningSet(t *testing.T) {
	r := Report{
		Decision:          "allowed_with_warnings",
		Agent2HostVersion: "0.0.0-dev",
		Subject:           Subject{SystemID: "s", AgentID: "a", Revision: "sha256:1"},
		Adapter:           AdapterRef{Version: "0.1.0"},
		Probe:             Probe{Fingerprint: "sha256:2"},
		Activation:        Activation{ReasonCode: "mapped_activation"},
	}
	b := BindingOf(r)
	if b.SystemID != "s" || b.Fingerprint != "sha256:2" || b.AdapterVersion != "0.1.0" {
		t.Fatalf("%+v", b)
	}
	if WarningSet(r) == WarningSet(Report{Decision: "allowed"}) {
		t.Fatal("warning set must include decision and reason codes")
	}
	other := r
	other.Activation.ReasonCode = "confidence_inferred"
	if WarningSet(r) == WarningSet(other) {
		t.Fatal("reason_code change must change warning set")
	}
}

func TestOfficialReuseMatch(t *testing.T) {
	b := Binding{
		SystemID: "example-system", AgentID: "example-agent", Revision: "sha256:1",
		Fingerprint: "sha256:2", AdapterVersion: "0.1.0", Agent2HostVersion: "0.0.0-dev",
	}
	if !OfficialReuse(b, b) {
		t.Fatal("identical binding must reuse")
	}
	other := b
	other.SystemID = "other-system"
	if OfficialReuse(b, other) {
		t.Fatal("system_id mismatch must not reuse")
	}
}

type ReportBindingJSON struct {
	Subject struct {
		SystemID string `json:"system_id"`
		AgentID  string `json:"agent_id"`
		Revision string `json:"revision"`
	} `json:"subject"`
	Probe struct {
		Fingerprint string `json:"fingerprint"`
	} `json:"probe"`
	Adapter struct {
		Version string `json:"version"`
	} `json:"adapter"`
	Agent2HostVersion string `json:"agent2host_version"`
}

func (r ReportBindingJSON) binding() Binding {
	return Binding{
		SystemID: r.Subject.SystemID, AgentID: r.Subject.AgentID, Revision: r.Subject.Revision,
		Fingerprint: r.Probe.Fingerprint, AdapterVersion: r.Adapter.Version, Agent2HostVersion: r.Agent2HostVersion,
	}
}
