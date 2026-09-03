package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent2host/agent2host/internal/compatibility"
)

func TestFixtureDriverEvaluatePack(t *testing.T) {
	root, err := fixtureRoot(t)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "evaluate", "act-mapped")
	var req compatibility.Requirement
	var assess compatibility.Assessment
	var want compatibility.Report
	load(t, filepath.Join(dir, "requirement.json"), &req)
	load(t, filepath.Join(dir, "assessment.json"), &assess)
	load(t, filepath.Join(dir, "expected-report.json"), &want)

	d := FixtureDriver{
		Observed: ProbeResult{
			HostID: want.Host.ID, HostVersion: want.Host.Version, Fingerprint: want.Probe.Fingerprint,
		},
		Assessment: assess,
		AdapterID:  want.Adapter.ID,
		AdapterVer: want.Adapter.Version,
	}
	gotAssess, _, err := d.Assess(nil, d.Observed)
	if err != nil {
		t.Fatal(err)
	}
	env := d.Envelope(want.Subject, want.Agent2HostVersion)
	got := compatibility.Evaluate(env, req, gotAssess)
	gb, _ := json.Marshal(got)
	wb, _ := json.Marshal(want)
	if string(gb) != string(wb) {
		t.Fatalf("driver evaluate mismatch\ngot %s\nwant %s", gb, wb)
	}
}

func load(t *testing.T, path string, dest any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, dest); err != nil {
		t.Fatal(err)
	}
}

func fixtureRoot(t *testing.T) (string, error) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "spec", "schemas", "compatibility", "v1alpha1", "fixtures"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
