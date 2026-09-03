package adapter

import (
	"testing"
)

func TestParseTOMLMapIgnoresCommentsAndWhitespace(t *testing.T) {
	raw := []byte(`
# comment
features.apps = false

default_permissions = "a2h"
approval_policy = "on-request"

[permissions.a2h.filesystem]
":root" = "deny"
":workspace_roots" = "write"

[permissions.a2h.network]
enabled = false
`)
	doc, err := ParseTOMLMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := TOMLString(doc, "default_permissions"); !ok || got != "a2h" {
		t.Fatalf("default_permissions %q %v", got, ok)
	}
	features, ok := TOMLTable(doc, "features")
	if !ok {
		t.Fatal("features table")
	}
	if apps, ok := TOMLBool(features, "apps"); !ok || apps {
		t.Fatalf("features.apps %v %v", apps, ok)
	}
	fs, ok := TOMLTable(mustTable(t, mustTable(t, doc, "permissions"), "a2h"), "filesystem")
	if !ok {
		t.Fatal("filesystem")
	}
	if root, ok := TOMLString(fs, ":root"); !ok || root != "deny" {
		t.Fatalf(":root %q %v", root, ok)
	}
}

func mustTable(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	got, ok := TOMLTable(m, key)
	if !ok {
		t.Fatalf("missing table %s", key)
	}
	return got
}
