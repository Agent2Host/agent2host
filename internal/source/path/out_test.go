package path_test

import (
	"testing"

	"github.com/agent2host/agent2host/internal/source/path"
	"github.com/agent2host/agent2host/internal/source/rule"
)

func TestOutputDialectExactURI(t *testing.T) {
	if err := path.CheckOutputSchema([]byte(`true`)); err != nil {
		t.Fatalf("boolean true: %v", err)
	}
	if err := path.CheckOutputSchema([]byte(`{"type":"object"}`)); err != nil {
		t.Fatalf("omitted $schema: %v", err)
	}
	ok := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`)
	if err := path.CheckOutputSchema(ok); err != nil {
		t.Fatalf("exact URI: %v", err)
	}
	rejects := []string{
		`{"$schema":"http://json-schema.org/draft/2020-12/schema","type":"object"}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema#","type":"object"}`,
		`{"$schema":"http://json-schema.org/draft/2020-12/schema#","type":"object"}`,
		`{"$schema":"http://json-schema.org/draft-07/schema#","type":"object"}`,
	}
	for _, raw := range rejects {
		err := path.CheckOutputSchema([]byte(raw))
		if rule.ID(err) != "SRC-OUT-DIALECT" {
			t.Errorf("%s: rule %q want SRC-OUT-DIALECT (%v)", raw, rule.ID(err), err)
		}
	}
}

func TestOutputSchemaRefs(t *testing.T) {
	local := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$defs": { "t": { "type": "string" } },
		"$ref": "#/$defs/t"
	}`)
	if err := path.CheckOutputSchema(local); err != nil {
		t.Fatalf("local fragment $ref: %v", err)
	}

	remote := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$ref":"https://example.com/remote.json"}`)
	if rule.ID(path.CheckOutputSchema(remote)) != "SRC-OUT-REF" {
		t.Fatalf("remote $ref: %v", path.CheckOutputSchema(remote))
	}

	dynRemote := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$dynamicRef":"https://example.com/dyn.json"}`)
	if rule.ID(path.CheckOutputSchema(dynRemote)) != "SRC-OUT-REF" {
		t.Fatalf("remote $dynamicRef: %v", path.CheckOutputSchema(dynRemote))
	}

	dynLocal := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id": "urn:agent2host:output-dyn-local",
		"$dynamicAnchor": "node",
		"type": "object",
		"$dynamicRef": "#node"
	}`)
	if err := path.CheckOutputSchema(dynLocal); err != nil {
		t.Fatalf("local $dynamicRef: %v", err)
	}

	for name, raw := range map[string]string{
		"const":    `{"const":{"$ref":"https://example.com/literal"}}`,
		"enum":     `{"enum":[{"$ref":"https://example.com/literal"}]}`,
		"examples": `{"examples":[{"$ref":"https://example.com/literal"}]}`,
	} {
		if err := path.CheckOutputSchema([]byte(raw)); err != nil {
			t.Errorf("literal $ref in %s must be accepted: %v", name, err)
		}
	}
}
