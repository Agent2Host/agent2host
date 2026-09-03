package schema

import (
	"bytes"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/agent2host/agent2host/internal/source/jsonreader"
	schemafs "github.com/agent2host/agent2host/spec/schemas"
)

// Kind selects a published schema.
type Kind string

const (
	KindAgent    Kind = "agent"
	KindSystem   Kind = "system"
	KindArtifact Kind = "artifact"
)

const (
	idCommon   = "urn:agent2host:schema:source:v1alpha1:common"
	idAgent    = "urn:agent2host:schema:source:v1alpha1:agent"
	idSystem   = "urn:agent2host:schema:source:v1alpha1:system"
	idSystemV2 = "urn:agent2host:schema:source:v1alpha2:system"
	idArtifact = "urn:agent2host:schema:artifact:agent2host-system-v1"
	srcV1      = "agent2host/v1alpha1"
	srcV2      = "agent2host/v1alpha2"
)

// Validator compiles published Source schemas from the embedded snapshot.
// It never loads draft-v1alpha1/ or the network, and does not search the source tree.
type Validator struct {
	agent    *jsonschema.Schema
	system   *jsonschema.Schema
	systemV2 *jsonschema.Schema
	artifact *jsonschema.Schema
}

type denyRemote struct{}

func (denyRemote) Load(url string) (any, error) {
	return nil, fmt.Errorf("remote schema retrieval disabled: %s", url)
}

// Load compiles Agent, System, and Artifact schemas from the embedded FS.
func Load() (*Validator, error) {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.UseLoader(denyRemote{})

	type pair struct {
		id   string
		name string
	}
	for _, p := range []pair{
		{idCommon, schemafs.Common},
		{idAgent, schemafs.Agent},
		{idSystem, schemafs.System},
		{idSystemV2, schemafs.SystemV2},
		{idArtifact, schemafs.Artifact},
	} {
		doc, err := loadEmbedded(p.name)
		if err != nil {
			return nil, err
		}
		if err := c.AddResource(p.id, doc); err != nil {
			return nil, fmt.Errorf("schema: add %s: %w", p.id, err)
		}
	}

	agent, err := c.Compile(idAgent)
	if err != nil {
		return nil, fmt.Errorf("schema: compile agent: %w", err)
	}
	system, err := c.Compile(idSystem)
	if err != nil {
		return nil, fmt.Errorf("schema: compile system: %w", err)
	}
	systemV2, err := c.Compile(idSystemV2)
	if err != nil {
		return nil, fmt.Errorf("schema: compile system v1alpha2: %w", err)
	}
	artifact, err := c.Compile(idArtifact)
	if err != nil {
		return nil, fmt.Errorf("schema: compile artifact: %w", err)
	}
	return &Validator{agent: agent, system: system, systemV2: systemV2, artifact: artifact}, nil
}

func loadEmbedded(name string) (any, error) {
	f, err := schemafs.Files.Open(name)
	if err != nil {
		return nil, fmt.Errorf("schema: open embedded %s: %w", name, err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		return nil, fmt.Errorf("schema: parse embedded %s: %w", name, err)
	}
	return doc, nil
}

// UnmarshalInstance decodes JSON the same way the Schema compiler does
// (json.Number, not encoding/json float64). SRC-VAL-LAYERS tests must use this.
func UnmarshalInstance(raw []byte) (any, error) {
	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

// ValidateBytes runs jsonreader.Read (SRC-JSON-UTF8/SYNTAX/DUP) then Schema.
func (v *Validator) ValidateBytes(kind Kind, raw []byte) error {
	doc, err := jsonreader.Read(raw)
	if err != nil {
		return err
	}
	inst, err := UnmarshalInstance(doc.Bytes)
	if err != nil {
		return err
	}
	return v.Validate(kind, inst)
}

// Validate checks a decoded instance against the published schema for kind.
func (v *Validator) Validate(kind Kind, instance any) error {
	var sch *jsonschema.Schema
	switch kind {
	case KindAgent:
		sch = v.agent
	case KindSystem:
		switch instanceSchemaVersion(instance) {
		case srcV2:
			sch = v.systemV2
		case srcV1, "":
			sch = v.system
		default:
			return fmt.Errorf("schema: unsupported system schema_version %q", instanceSchemaVersion(instance))
		}
	case KindArtifact:
		sch = v.artifact
	default:
		return fmt.Errorf("schema: unknown kind %q", kind)
	}
	if err := sch.Validate(instance); err != nil {
		return err
	}
	return nil
}

func instanceSchemaVersion(instance any) string {
	m, ok := instance.(map[string]any)
	if !ok {
		return ""
	}
	s, _ := m["schema_version"].(string)
	return s
}
