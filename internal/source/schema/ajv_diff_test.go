package schema_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/source/jsonreader"
	"github.com/agent2host/agent2host/internal/source/schema"
)

type ajvRow struct {
	Rel    string `json:"rel"`
	Kind   string `json:"kind"`
	Accept bool   `json:"accept"`
}

func TestAjvGoFrozenInstanceDiff(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		skipUnlessAjv(t, "node not on PATH")
	}
	cmd := exec.Command("node", "ajv-verdicts.mjs")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 2 {
			skipUnlessAjv(t, "ajv not installed; run npm ci in fixtures")
		}
		t.Fatalf("ajv-verdicts: %v\n%s", err, out)
	}
	var rows []ajvRow
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("empty Ajv verdicts")
	}
	v, err := schema.Load()
	if err != nil {
		t.Fatal(err)
	}
	kindOf := func(s string) schema.Kind {
		switch s {
		case "agent":
			return schema.KindAgent
		case "system":
			return schema.KindSystem
		case "artifact":
			return schema.KindArtifact
		default:
			t.Fatalf("kind %s", s)
			return ""
		}
	}
	for _, row := range rows {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(row.Rel)))
		if err != nil {
			t.Fatal(err)
		}
		goErr := v.ValidateBytes(kindOf(row.Kind), raw)
		goAccept := goErr == nil
		if goAccept != row.Accept {
			t.Errorf("%s: Ajv accept=%v Go accept=%v go_err=%v", row.Rel, row.Accept, goAccept, goErr)
		}
	}
}

func TestCriticalDepthParity(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		skipUnlessAjv(t, "node not on PATH")
	}
	v, err := schema.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []int{32, 256, 1000} {
		raw := deepAgent(depth)
		_, scanErr := jsonreader.Read(raw)
		if re, ok := scanErr.(*jsonreader.Error); ok && re.RuleID == "SRC-JSON-SYNTAX" {
			t.Fatalf("depth %d scanner SRC-JSON-SYNTAX: %v", depth, scanErr)
		}
		goErr := v.ValidateBytes(schema.KindAgent, raw)
		script := `const Ajv2020=(await import("ajv/dist/2020.js")).default;
const fs=await import("node:fs");
const path=process.argv[1];
const schemaDir=process.argv[2];
const ajv=new Ajv2020({strict:true,allErrors:true,validateFormats:false});
ajv.addSchema(JSON.parse(fs.readFileSync(schemaDir+"/common.schema.json","utf8")));
ajv.addSchema(JSON.parse(fs.readFileSync(schemaDir+"/system.schema.json","utf8")));
ajv.addSchema(JSON.parse(fs.readFileSync(schemaDir+"/agent.schema.json","utf8")));
const validate=ajv.getSchema("urn:agent2host:schema:source:v1alpha1:agent");
const inst=JSON.parse(fs.readFileSync(path,"utf8"));
process.stdout.write(validate(inst)?"1":"0");`
		tmp := filepath.Join(t.TempDir(), "deep.agent.json")
		if err := os.WriteFile(tmp, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("node", "--input-type=module", "-e", script, tmp, filepath.Join(root, ".."))
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			msg := err.Error()
			if ee, ok := err.(*exec.ExitError); ok {
				msg = string(ee.Stderr)
			}
			if strings.Contains(msg, "ERR_MODULE_NOT_FOUND") {
				skipUnlessAjv(t, "ajv not installed; run npm ci in fixtures")
			}
			t.Fatalf("node depth %d: %v\n%s", depth, err, msg)
		}
		ajvAccept := strings.TrimSpace(string(out)) == "1"
		goAccept := goErr == nil
		if scanErr != nil {
			if _, ok := scanErr.(*jsonreader.LimitError); ok {
				t.Logf("depth %d: Go decoder resource limit (not SRC-JSON-SYNTAX)", depth)
				continue
			}
			t.Fatalf("depth %d: unexpected reader error %v", depth, scanErr)
		}
		if ajvAccept != goAccept {
			t.Fatalf("depth %d: Ajv accept=%v Go accept=%v go_err=%v", depth, ajvAccept, goAccept, goErr)
		}
	}
}

func skipUnlessAjv(t *testing.T, reason string) {
	t.Helper()
	if os.Getenv("A2H_REQUIRE_AJV") != "" {
		t.Fatalf("Ajv required (A2H_REQUIRE_AJV): %s", reason)
	}
	t.Skip(reason)
}

func deepAgent(depth int) []byte {
	var b strings.Builder
	b.WriteString(`{"schema_version":"agent2host/v1alpha1","kind":"Agent","id":"demo","sop":"./sops/demo.sop.md","extensions":{"x/d":`)
	for i := 0; i < depth; i++ {
		b.WriteByte('[')
	}
	b.WriteByte('1')
	for i := 0; i < depth; i++ {
		b.WriteByte(']')
	}
	b.WriteString(`}}`)
	return []byte(b.String())
}
