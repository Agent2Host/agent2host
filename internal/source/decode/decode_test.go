package decode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/source/jsonreader"
	"github.com/agent2host/agent2host/internal/source/schema"
)

func TestSchemaValidAgentAndSystemDecode(t *testing.T) {
	v, err := schema.Load()
	if err != nil {
		t.Fatal(err)
	}
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}

	decodeAgents(t, v, []string{
		filepath.Join(root, "valid"),
		filepath.Join(root, "normalize"),
		filepath.Join(root, "semantic-reject"),
	}, ".agent.json")

	decodeSystems(t, v, []string{
		filepath.Join(root, "system", "valid"),
		filepath.Join(root, "system", "normalize"),
		filepath.Join(root, "semantic-reject"),
	}, ".system.json")

	t.Run("unknown-core-field-stays-schema-rejected", func(t *testing.T) {
		b, err := os.ReadFile(filepath.Join(root, "invalid", "unknown-field.agent.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := v.ValidateBytes(schema.KindAgent, b); err == nil {
			t.Fatal("unknown core field must remain Schema-rejected")
		}
	})
}

func decodeAgents(t *testing.T, v *schema.Validator, dirs []string, suffix string) {
	t.Helper()
	for _, dir := range dirs {
		for _, name := range listJSON(t, dir) {
			if !strings.HasSuffix(name, suffix) {
				continue
			}
			path := filepath.Join(dir, name)
			name := name
			t.Run("agent/"+name, func(t *testing.T) {
				doc := readOK(t, path)
				if err := v.ValidateBytes(schema.KindAgent, doc.Bytes); err != nil {
					t.Skip("not Schema-valid agent")
				}
				a, err := decode.Agent(doc.Bytes)
				if err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(a)
				if err != nil {
					t.Fatal(err)
				}
				assertSpecifiedFieldsKept(t, doc.Bytes, encoded)
			})
		}
	}
}

func decodeSystems(t *testing.T, v *schema.Validator, dirs []string, suffix string) {
	t.Helper()
	for _, dir := range dirs {
		for _, name := range listJSON(t, dir) {
			if !strings.HasSuffix(name, suffix) {
				continue
			}
			path := filepath.Join(dir, name)
			name := name
			t.Run("system/"+name, func(t *testing.T) {
				doc := readOK(t, path)
				if err := v.ValidateBytes(schema.KindSystem, doc.Bytes); err != nil {
					t.Skip("not Schema-valid system")
				}
				s, err := decode.System(doc.Bytes)
				if err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(s)
				if err != nil {
					t.Fatal(err)
				}
				assertSpecifiedFieldsKept(t, doc.Bytes, encoded)
			})
		}
	}
}

func TestArtifactDecode(t *testing.T) {
	v, err := schema.Load()
	if err != nil {
		t.Fatal(err)
	}
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "artifact", "valid")
	for _, name := range listJSON(t, dir) {
		path := filepath.Join(dir, name)
		t.Run(name, func(t *testing.T) {
			doc := readOK(t, path)
			if err := v.ValidateBytes(schema.KindArtifact, doc.Bytes); err != nil {
				t.Fatal(err)
			}
			if _, err := decode.Artifact(doc.Bytes); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func readOK(t *testing.T, path string) *jsonreader.Document {
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

func listJSON(t *testing.T, dir string) []string {
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
		if strings.HasSuffix(n, ".json") && !strings.Contains(n, "expected") {
			names = append(names, n)
		}
	}
	return names
}

func assertSpecifiedFieldsKept(t *testing.T, original, encoded []byte) {
	t.Helper()
	var o, e any
	if err := json.Unmarshal(original, &o); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &e); err != nil {
		t.Fatal(err)
	}
	keepSpecified(t, "", o, e)
}

func keepSpecified(t *testing.T, path string, orig, enc any) {
	t.Helper()
	switch o := orig.(type) {
	case map[string]any:
		em, ok := enc.(map[string]any)
		if !ok {
			t.Fatalf("%s: encoded type %T, want object", path, enc)
		}
		for k, v := range o {
			ev, ok := em[k]
			if !ok {
				t.Fatalf("dropped specified field %s", joinPath(path, k))
			}
			keepSpecified(t, joinPath(path, k), v, ev)
		}
	case []any:
		ea, ok := enc.([]any)
		if !ok {
			t.Fatalf("%s: encoded type %T, want array", path, enc)
		}
		if len(ea) != len(o) {
			t.Fatalf("%s: array length %d → %d", path, len(o), len(ea))
		}
		for i := range o {
			keepSpecified(t, joinPath(path, itoa(i)), o[i], ea[i])
		}
	}
}

func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}

func TestShortAndLongSkillForms(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	doc := readOK(t, filepath.Join(root, "valid", "skill-short-and-long.agent.json"))
	a, err := decode.Agent(doc.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if a.Skills == nil || len(*a.Skills) != 2 {
		t.Fatalf("skills = %#v", a.Skills)
	}
	s0 := (*a.Skills)[0]
	if !s0.IsShort() || s0.ID != "search-policy" {
		t.Fatalf("short form: %#v", s0)
	}
	s1 := (*a.Skills)[1]
	if s1.IsShort() || s1.ID != "verify-member" || s1.Required == nil || *s1.Required {
		t.Fatalf("long form: %#v", s1)
	}
}
