package schema_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/source/jsonreader"
	"github.com/agent2host/agent2host/internal/source/schema"
)

// TestFrozenSchemaVerdicts checks Go Schema accept/reject against the frozen
// fixture directories. Ajv 8 parity on the same instance set is
// TestAjvGoFrozenInstanceDiff (ajv-verdicts.mjs); fixtures `npm test` remains
// the Freeze-time Node harness (SP-G2 / status.json).
func TestFrozenSchemaVerdicts(t *testing.T) {
	v, err := schema.Load()
	if err != nil {
		t.Fatal(err)
	}
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}

	mustPass(t, v, schema.KindAgent, join(root, "valid"), ".agent.json")
	mustFail(t, v, schema.KindAgent, join(root, "invalid"), ".agent.json")
	mustPass(t, v, schema.KindSystem, join(root, "system", "valid"), ".system.json")
	mustFail(t, v, schema.KindSystem, join(root, "system", "invalid"), ".system.json")
	mustPass(t, v, schema.KindArtifact, join(root, "artifact", "valid"), ".json")
	mustFail(t, v, schema.KindArtifact, join(root, "artifact", "invalid"), ".json")
	mustPass(t, v, schema.KindAgent, join(root, "normalize"), ".agent.json")
	mustPass(t, v, schema.KindSystem, join(root, "system", "normalize"), ".system.json")
	mustPass(t, v, schema.KindAgent, join(root, "semantic-reject"), ".agent.json")
	mustPass(t, v, schema.KindSystem, join(root, "semantic-reject"), ".system.json")
}

func TestV1Alpha2WorkRoot(t *testing.T) {
	v, err := schema.Load()
	if err != nil {
		t.Fatal(err)
	}
	pass := []string{
		`{"schema_version":"agent2host/v1alpha2","kind":"AgentSystem","id":"demo","version":"0.1.0","agents":["./agents/a.agent.json"],"work_root":{"mode":"invocation"}}`,
		`{"schema_version":"agent2host/v1alpha2","kind":"AgentSystem","id":"demo","version":"0.1.0","agents":["./agents/a.agent.json"],"work_root":{"mode":"fixed","path_from_home":"Desktop/Crossroads/Events"}}`,
		// macOS allows @ in directory names
		`{"schema_version":"agent2host/v1alpha2","kind":"AgentSystem","id":"demo","version":"0.1.0","agents":["./agents/a.agent.json"],"work_root":{"mode":"fixed","path_from_home":"Desktop/Tech@Crossroads/Events"}}`,
		// macOS allows spaces in directory names
		`{"schema_version":"agent2host/v1alpha2","kind":"AgentSystem","id":"demo","version":"0.1.0","agents":["./agents/a.agent.json"],"work_root":{"mode":"fixed","path_from_home":"Desktop/My Events/Mid-Autumn"}}`,
		// macOS allows & ( ) # ! + in directory names
		`{"schema_version":"agent2host/v1alpha2","kind":"AgentSystem","id":"demo","version":"0.1.0","agents":["./agents/a.agent.json"],"work_root":{"mode":"fixed","path_from_home":"Desktop/Arts & Culture/Events (2026)"}}`,
	}
	for _, raw := range pass {
		if err := v.ValidateBytes(schema.KindSystem, []byte(raw)); err != nil {
			t.Fatalf("accept: %v\n%s", err, raw)
		}
	}
	fail := []string{
		`{"schema_version":"agent2host/v1alpha2","kind":"AgentSystem","id":"demo","version":"0.1.0","agents":["./agents/a.agent.json"]}`,
		`{"schema_version":"agent2host/v1alpha2","kind":"AgentSystem","id":"demo","version":"0.1.0","agents":["./agents/a.agent.json"],"work_root":{"mode":"fixed"}}`,
		`{"schema_version":"agent2host/v1alpha2","kind":"AgentSystem","id":"demo","version":"0.1.0","agents":["./agents/a.agent.json"],"work_root":{"mode":"invocation","path_from_home":"Desktop/X"}}`,
		`{"schema_version":"agent2host/v1alpha2","kind":"AgentSystem","id":"demo","version":"0.1.0","agents":["./agents/a.agent.json"],"work_root":{"mode":"fixed","path_from_home":"../etc"}}`,
		`{"schema_version":"agent2host/v1alpha2","kind":"AgentSystem","id":"demo","version":"0.1.0","agents":["./agents/a.agent.json"],"work_root":{"mode":"fixed","path_from_home":"Desktop/foo:bar"}}`,
		`{"schema_version":"agent2host/v1alpha2","kind":"AgentSystem","id":"demo","version":"0.1.0","agents":["./agents/a.agent.json"],"work_root":{"mode":"fixed","path_from_home":"."}}`,
		`{"schema_version":"agent2host/v1alpha1","kind":"AgentSystem","id":"demo","version":"0.1.0","agents":["./agents/a.agent.json"],"work_root":{"mode":"invocation"}}`,
	}
	for _, raw := range fail {
		if err := v.ValidateBytes(schema.KindSystem, []byte(raw)); err == nil {
			t.Fatalf("must reject %s", raw)
		}
	}
}

func TestValidateBytesRejectsDuplicateKeys(t *testing.T) {
	v, err := schema.Load()
	if err != nil {
		t.Fatal(err)
	}
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "json_reader", "duplicate-json-key.raw"))
	if err != nil {
		t.Fatal(err)
	}
	err = v.ValidateBytes(schema.KindAgent, raw)
	if err == nil {
		t.Fatal("ValidateBytes must not skip SRC-JSON-DUP")
	}
	re, ok := err.(*jsonreader.Error)
	if !ok || re.RuleID != "SRC-JSON-DUP" {
		t.Fatalf("got %v", err)
	}
}

func join(elem ...string) string { return filepath.Join(elem...) }

func mustPass(t *testing.T, v *schema.Validator, kind schema.Kind, dir, suffix string) {
	t.Helper()
	for _, name := range listInstances(t, dir, suffix) {
		name := name
		t.Run(filepath.ToSlash(filepath.Join(relLabel(dir), name))+":pass", func(t *testing.T) {
			raw := readFile(t, filepath.Join(dir, name))
			if err := v.ValidateBytes(kind, raw); err != nil {
				t.Fatalf("schema rejected (frozen directory expects accept): %v", err)
			}
		})
	}
}

func mustFail(t *testing.T, v *schema.Validator, kind schema.Kind, dir, suffix string) {
	t.Helper()
	for _, name := range listInstances(t, dir, suffix) {
		name := name
		t.Run(filepath.ToSlash(filepath.Join(relLabel(dir), name))+":fail", func(t *testing.T) {
			raw := readFile(t, filepath.Join(dir, name))
			if err := v.ValidateBytes(kind, raw); err == nil {
				t.Fatal("schema accepted (frozen directory expects reject)")
			}
		})
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func listInstances(t *testing.T, dir, suffix string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasSuffix(n, suffix) {
			continue
		}
		if strings.Contains(n, "expected") {
			continue
		}
		switch n {
		case "catalog.json", "package.json", "status.json", "package-lock.json":
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatalf("no instances matching %q in %s", suffix, dir)
	}
	return names
}

func relLabel(dir string) string {
	root, err := fixtures.Root()
	if err != nil {
		return dir
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return dir
	}
	return rel
}
