package compatibility

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEvaluateCorpus(t *testing.T) {
	root, err := fixtureRoot()
	if err != nil {
		t.Fatal(err)
	}
	evalDir := filepath.Join(root, "evaluate")
	entries, err := os.ReadDir(evalDir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n++
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(evalDir, name)
			var req Requirement
			var assess Assessment
			var want Report
			mustJSON(t, filepath.Join(dir, "requirement.json"), &req)
			mustJSON(t, filepath.Join(dir, "assessment.json"), &assess)
			mustJSON(t, filepath.Join(dir, "expected-report.json"), &want)
			got := Evaluate(envelopeOf(want), req, assess)
			if diff := reportDiff(got, want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
	if n != 29 {
		t.Fatalf("expected 29 evaluate packs, got %d", n)
	}
}

func TestBestEffortOutputValidationSatisfied(t *testing.T) {
	env := Envelope{
		SchemaVersion:     schemaVersion,
		Agent2HostVersion: "0.0.0-test",
		Subject:           Subject{SystemID: "club-system", AgentID: "club-faq", Revision: "sha256:" + zeros(64)},
		Host:              HostRef{ID: "codex", Version: "1"},
		Adapter:           AdapterRef{ID: "codex", Version: "0.1.0"},
		Probe:             Probe{Fingerprint: "sha256:" + zeros(64)},
	}
	req := Requirement{
		Security: &SecurityReq{OutputValidation: &FlagReq{Required: false}},
	}
	assess := Assessment{
		Security: &SecurityAssess{
			OutputValidation: &PolicyAssess{
				Support: "mapped", Scope: "agent", Enforcement: "none", Confidence: "documented",
			},
		},
	}
	got := Evaluate(env, req, assess)
	if got.Security.OutputValidation.RequirementResult != resultSatisfied {
		t.Fatalf("best_effort: got %s %s", got.Security.OutputValidation.RequirementResult, got.Security.OutputValidation.ReasonCode)
	}
	if got.Security.OutputValidation.ReasonCode != "" {
		t.Fatalf("best_effort must not emit acceptance_failed, got %s", got.Security.OutputValidation.ReasonCode)
	}
}

func envelopeOf(r Report) Envelope {
	return Envelope{
		SchemaVersion:     r.SchemaVersion,
		Agent2HostVersion: r.Agent2HostVersion,
		Subject:           r.Subject,
		Host:              r.Host,
		Adapter:           r.Adapter,
		Probe:             r.Probe,
	}
}

func mustJSON(t *testing.T, path string, dest any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, dest); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func reportDiff(got, want Report) string {
	gb, err := json.Marshal(got)
	if err != nil {
		return err.Error()
	}
	wb, err := json.Marshal(want)
	if err != nil {
		return err.Error()
	}
	var g, w any
	if err := json.Unmarshal(gb, &g); err != nil {
		return err.Error()
	}
	if err := json.Unmarshal(wb, &w); err != nil {
		return err.Error()
	}
	if reflect.DeepEqual(g, w) {
		return ""
	}
	prettyG, _ := json.MarshalIndent(g, "", "  ")
	prettyW, _ := json.MarshalIndent(w, "", "  ")
	return "got:\n" + string(prettyG) + "\nwant:\n" + string(prettyW)
}

func fixtureRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			root := filepath.Join(dir, "spec", "schemas", "compatibility", "v1alpha1", "fixtures")
			if st, err := os.Stat(root); err == nil && st.IsDir() {
				return root, nil
			}
			return "", os.ErrNotExist
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func zeros(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '0'
	}
	return string(b)
}
