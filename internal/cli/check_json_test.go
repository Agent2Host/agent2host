package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/source/schema"
)

const reportSchemaVersion = "agent2host/compatibility-report/v1alpha1"

var (
	reportSchemaOnce sync.Once
	reportSchema     *jsonschema.Schema
	reportSchemaErr  error
)

type denyRemote struct{}

func (denyRemote) Load(url string) (any, error) {
	return nil, fmt.Errorf("remote schema retrieval disabled: %s", url)
}

func loadReportSchema() (*jsonschema.Schema, error) {
	reportSchemaOnce.Do(func() {
		root, err := fixturesModuleRoot()
		if err != nil {
			reportSchemaErr = err
			return
		}
		path := filepath.Join(root, "spec", "schemas", "compatibility", "v1alpha1", "report.schema.json")
		f, err := os.Open(path)
		if err != nil {
			reportSchemaErr = err
			return
		}
		defer f.Close()
		doc, err := jsonschema.UnmarshalJSON(f)
		if err != nil {
			reportSchemaErr = err
			return
		}
		c := jsonschema.NewCompiler()
		c.DefaultDraft(jsonschema.Draft2020)
		c.UseLoader(denyRemote{})
		id := "urn:agent2host:schema:compatibility-report:v1alpha1:report"
		if err := c.AddResource(id, doc); err != nil {
			reportSchemaErr = err
			return
		}
		reportSchema, reportSchemaErr = c.Compile(id)
	})
	return reportSchema, reportSchemaErr
}

func fixturesModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	start := dir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found walking from %s", start)
		}
		dir = parent
	}
}

func decodeCheckReport(t *testing.T, raw []byte) compatibility.Report {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("check --json: %v\n%s", err, raw)
	}
	for _, forbidden := range []string{"kind", "plans", "report", "outcome"} {
		if _, ok := top[forbidden]; ok {
			t.Fatalf("check --json must not wrap the Report with %q\n%s", forbidden, raw)
		}
	}
	sch, err := loadReportSchema()
	if err != nil {
		t.Fatal(err)
	}
	inst, err := schema.UnmarshalInstance(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Fatalf("check --json is not a valid Compatibility Report: %v\n%s", err, raw)
	}
	var report compatibility.Report
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != reportSchemaVersion {
		t.Fatalf("schema_version %q want %q", report.SchemaVersion, reportSchemaVersion)
	}
	if bytes.Contains(raw, []byte("agent2host/execution-contract/v1alpha1")) {
		t.Fatal("check --json must not emit the execution-contract envelope")
	}
	return report
}
