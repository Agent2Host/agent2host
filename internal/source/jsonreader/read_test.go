package jsonreader_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/source/jsonreader"
)

type catalogRow struct {
	File   string `json:"file"`
	RuleID string `json:"rule_id"`
	Expect string `json:"expect"`
	Layer  string `json:"validation_layer"`
}

func TestCatalog(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "json_reader")
	raw, err := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []catalogRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("empty json_reader catalog")
	}
	for _, row := range rows {
		row := row
		t.Run(row.File, func(t *testing.T) {
			if row.Layer != "json_reader" || row.RuleID == "" {
				t.Fatalf("incomplete catalog row %+v", row)
			}
			b, err := os.ReadFile(filepath.Join(dir, row.File))
			if err != nil {
				t.Fatal(err)
			}
			doc, err := jsonreader.Read(b)
			wantInvalid := row.Expect == "invalid"
			if wantInvalid {
				if err == nil {
					t.Fatalf("expected %s, got valid document", row.RuleID)
				}
				re, ok := err.(*jsonreader.Error)
				if !ok {
					t.Fatalf("error type %T: %v", err, err)
				}
				if re.RuleID != row.RuleID {
					t.Fatalf("rule_id: got %s want %s", re.RuleID, row.RuleID)
				}
				if re.Offset < 0 {
					t.Fatalf("offset must be a byte position, got %d", re.Offset)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if doc == nil || doc.Value == nil {
				t.Fatal("nil document")
			}
		})
	}
}

func TestBigJSONNumber(t *testing.T) {
	doc, err := jsonreader.Read([]byte(`{"n":1e400}`))
	if err != nil {
		t.Fatalf("legal JSON number must not be SRC-JSON-SYNTAX: %v", err)
	}
	m, ok := doc.Value.(map[string]any)
	if !ok {
		t.Fatalf("value = %#v", doc.Value)
	}
	n, ok := m["n"].(json.Number)
	if !ok || n.String() != "1e400" {
		t.Fatalf("want json.Number 1e400, got %#v", m["n"])
	}
}

func TestValidObject(t *testing.T) {
	doc, err := jsonreader.Read([]byte(`{"id":"demo"}`))
	if err != nil {
		t.Fatal(err)
	}
	m, ok := doc.Value.(map[string]any)
	if !ok || m["id"] != "demo" {
		t.Fatalf("value = %#v", doc.Value)
	}
}

func TestDeepNestingIsNotSyntax(t *testing.T) {
	nested := func(n int) []byte {
		b := make([]byte, 0, n*2)
		for i := 0; i < n; i++ {
			b = append(b, '[')
		}
		for i := 0; i < n; i++ {
			b = append(b, ']')
		}
		return b
	}
	for _, n := range []int{32, 256, 1000, 1001} {
		_, err := jsonreader.Read(nested(n))
		if err == nil {
			continue
		}
		if re, ok := err.(*jsonreader.Error); ok && re.RuleID == "SRC-JSON-SYNTAX" {
			t.Fatalf("depth %d must not be SRC-JSON-SYNTAX: %v", n, err)
		}
		if _, ok := err.(*jsonreader.LimitError); !ok {
			t.Fatalf("depth %d: want success or LimitError, got %T %v", n, err, err)
		}
	}
}

func TestTrailingCommaRejected(t *testing.T) {
	for _, raw := range []string{`{"a":1,}`, `[1,]`} {
		_, err := jsonreader.Read([]byte(raw))
		re, ok := err.(*jsonreader.Error)
		if !ok || re.RuleID != "SRC-JSON-SYNTAX" {
			t.Fatalf("%s: got %v", raw, err)
		}
	}
}
