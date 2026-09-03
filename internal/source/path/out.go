package path

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/agent2host/agent2host/internal/source/jsonreader"
	"github.com/agent2host/agent2host/internal/source/rule"
)

// dialect2020 is the only 2020-12 URI listed by freeze fixtures (SRC-OUT-DIALECT).
const dialect2020 = "https://json-schema.org/draft/2020-12/schema"

type denyRemote struct{}

func (denyRemote) Load(url string) (any, error) {
	return nil, fmt.Errorf("remote schema retrieval disabled: %s", url)
}

// CheckOutputSchema applies SRC-OUT-* to a declared output.schema file.
func CheckOutputSchema(raw []byte) error {
	doc, err := jsonreader.Read(raw)
	if err != nil {
		if id := rule.ID(err); id == "SRC-JSON-UTF8" {
			return rule.Fail("SRC-OUT-UTF8", err.Error())
		}
		return err
	}
	switch v := doc.Value.(type) {
	case bool:
		return nil
	case map[string]any:
		if s, ok := v["$schema"]; ok {
			uri, ok := s.(string)
			if !ok {
				return rule.Fail("SRC-OUT-DIALECT", "non-string $schema")
			}
			if uri != dialect2020 {
				return rule.Fail("SRC-OUT-DIALECT", uri)
			}
		}
		inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc.Bytes))
		if err != nil {
			return rule.Fail("SRC-OUT-META", err.Error())
		}
		c := jsonschema.NewCompiler()
		c.DefaultDraft(jsonschema.Draft2020)
		c.UseLoader(denyRemote{})
		const id = "urn:agent2host:output-schema-check"
		if err := c.AddResource(id, inst); err != nil {
			return mapOutErr(err)
		}
		if _, err := c.Compile(id); err != nil {
			return mapOutErr(err)
		}
		return nil
	default:
		return rule.Fail("SRC-OUT-ROOT", "root must be an object or boolean schema")
	}
}

func mapOutErr(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "remote schema") || strings.Contains(msg, "http://") || strings.Contains(msg, "https://") {
		return rule.Fail("SRC-OUT-REF", msg)
	}
	return rule.Fail("SRC-OUT-META", msg)
}
