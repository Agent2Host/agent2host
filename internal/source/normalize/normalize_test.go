package normalize_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/source/jsonreader"
	"github.com/agent2host/agent2host/internal/source/normalize"
)

func TestSecurityBaseline(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	want := loadJSON(t, filepath.Join(root, "normalize", "expected-baseline-security.json"))
	for _, name := range []string{"omit-all.agent.json", "empty-objects.agent.json", "partial-fields.agent.json"} {
		name := name
		t.Run(name, func(t *testing.T) {
			a := decodeAgent(t, filepath.Join(root, "normalize", name))
			normalize.Agent(a)
			got := map[string]any{
				"permissions": mustJSON(t, a.Permissions),
				"approvals":   mustJSON(t, a.Approvals),
				"sandbox":     mustJSON(t, a.Sandbox),
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("security fill mismatch\ngot  %s\nwant %s", mustRaw(t, got), mustRaw(t, want))
			}
		})
	}
}

func TestEffectiveWorkRootV1IsInvocation(t *testing.T) {
	got := normalize.EffectiveWorkRoot(&decode.SystemSource{SchemaVersion: decode.SchemaVersionV1})
	if got.Mode != decode.WorkRootInvocation || got.PathFromHome != "" {
		t.Fatalf("%+v", got)
	}
	got = normalize.EffectiveWorkRoot(&decode.SystemSource{
		SchemaVersion: decode.SchemaVersionV2,
		WorkRoot:      &decode.WorkRoot{Mode: decode.WorkRootFixed, PathFromHome: "Desktop/X"},
	})
	if got.Mode != decode.WorkRootFixed || got.PathFromHome != "Desktop/X" {
		t.Fatalf("%+v", got)
	}
}

func TestSystemDefaultsBaseline(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	want := loadJSON(t, filepath.Join(root, "system", "normalize", "expected-baseline-defaults.json"))
	for _, name := range []string{"omit-defaults.system.json", "empty-defaults.system.json", "partial-defaults.system.json"} {
		name := name
		t.Run(name, func(t *testing.T) {
			s := decodeSystem(t, filepath.Join(root, "system", "normalize", name))
			normalize.System(s)
			got := mustJSON(t, s.Defaults)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("defaults fill mismatch\ngot  %s\nwant %s", mustRaw(t, got), mustRaw(t, want))
			}
		})
	}
}

func TestIdempotent(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	t.Run("agent", func(t *testing.T) {
		a := decodeAgent(t, filepath.Join(root, "valid", "hooks-closed-events.agent.json"))
		normalize.Agent(a)
		first, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		normalize.Agent(a)
		second, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(second) {
			t.Fatalf("not idempotent\n1 %s\n2 %s", first, second)
		}
	})
	t.Run("system", func(t *testing.T) {
		s := decodeSystem(t, filepath.Join(root, "system", "normalize", "omit-defaults.system.json"))
		normalize.System(s)
		first, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		normalize.System(s)
		second, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(second) {
			t.Fatalf("not idempotent\n1 %s\n2 %s", first, second)
		}
	})
}

func TestShortSkillBecomesLongRequired(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	a := decodeAgent(t, filepath.Join(root, "valid", "skill-short-and-long.agent.json"))
	normalize.Agent(a)
	s0 := (*a.Skills)[0]
	if s0.IsShort() || s0.Required == nil || !*s0.Required {
		t.Fatalf("short form after normalize: %#v", s0)
	}
	s1 := (*a.Skills)[1]
	if s1.Required == nil || *s1.Required {
		t.Fatalf("explicit required:false must stay false: %#v", s1)
	}
}

func TestOmittedExtensionsStayOmitted(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	a := decodeAgent(t, filepath.Join(root, "normalize", "omit-all.agent.json"))
	normalize.Agent(a)
	if a.Extensions != nil {
		t.Fatalf("SRC-DEFAULT-EXT: invented extensions %#v", a.Extensions)
	}
	s := decodeSystem(t, filepath.Join(root, "system", "normalize", "omit-defaults.system.json"))
	normalize.System(s)
	if s.Extensions != nil {
		t.Fatalf("SRC-DEFAULT-EXT: invented system extensions %#v", s.Extensions)
	}
}

func TestOmittedOutputStaysOmitted(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	a := decodeAgent(t, filepath.Join(root, "normalize", "omit-all.agent.json"))
	normalize.Agent(a)
	if a.Output != nil {
		t.Fatal("omitted output must stay omitted")
	}
}

func TestOutputEnforcementDefault(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	a := decodeAgent(t, filepath.Join(root, "valid", "output-with-schema.agent.json"))
	normalize.Agent(a)
	if a.Output == nil || a.Output.Enforcement == nil || *a.Output.Enforcement != "best_effort" {
		t.Fatalf("enforcement default: %#v", a.Output)
	}
}

func TestContextLoadingDefault(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	// loading-on-demand already writes the value; use a decoded clone with loading stripped via roundtrip after mutate.
	a := decodeAgent(t, filepath.Join(root, "valid", "loading-on-demand.agent.json"))
	(*a.Contexts)[0].Loading = nil
	normalize.Agent(a)
	c := (*a.Contexts)[0]
	if c.Loading == nil || *c.Loading != "on_demand" {
		t.Fatalf("loading: %#v", c)
	}
	if c.Required == nil || !*c.Required {
		t.Fatalf("context required default: %#v", c)
	}
}

func decodeAgent(t *testing.T, path string) *decode.AgentSource {
	t.Helper()
	doc := read(t, path)
	a, err := decode.Agent(doc.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func decodeSystem(t *testing.T, path string) *decode.SystemSource {
	t.Helper()
	doc := read(t, path)
	s, err := decode.System(doc.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func read(t *testing.T, path string) *jsonreader.Document {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonreader.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func loadJSON(t *testing.T, path string) any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func mustJSON(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func mustRaw(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
