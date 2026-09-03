package path_test

import (
	"testing"

	"github.com/agent2host/agent2host/internal/source/path"
	"github.com/agent2host/agent2host/internal/source/rule"
)

func TestCanonicalizeOrder(t *testing.T) {
	cases := []struct {
		in, id, canon string
	}{
		{"../other/demo.sop.md", "SRC-PATH-ESCAPE", ""},
		{"./../other/demo.sop.md", "SRC-PATH-ESCAPE", ""},
		{"./sops/foo/./demo.sop.md", "SRC-PATH-DOT", ""},
		{"./sops/foo/../demo.sop.md", "SRC-PATH-DOT", ""},
		{"foo//bar.md", "SRC-PATH-DOT", ""},
		{"./sops/CON.sop.md", "SRC-PATH-WINDOWS-RESERVED", ""},
		{"./sops/demo.sop.md.", "SRC-PATH-WINDOWS-RESERVED", ""},
		{"./sops/nul.md", "SRC-PATH-WINDOWS-RESERVED", ""},
		{"contexts\\a.md", "SRC-PATH-SEP", ""},
		{"/etc/passwd", "SRC-PATH-REL", ""},
		{"./sops/demo.sop.md", "", "sops/demo.sop.md"},
		{"./contexts/shared.md", "", "contexts/shared.md"},
		{"contexts/shared.md", "", "contexts/shared.md"},
		{"./.env", "", ".env"},
	}
	for _, tc := range cases {
		got, err := path.Canonicalize(tc.in)
		if tc.id == "" {
			if err != nil {
				t.Errorf("Canonicalize(%q): %v", tc.in, err)
				continue
			}
			if got != tc.canon {
				t.Errorf("Canonicalize(%q)=%q want %q", tc.in, got, tc.canon)
			}
			continue
		}
		if rule.ID(err) != tc.id {
			t.Errorf("Canonicalize(%q) rule %q want %q (err=%v)", tc.in, rule.ID(err), tc.id, err)
		}
	}
}

func TestIsCanonicalMember(t *testing.T) {
	if !path.IsCanonicalMember("sops/demo.sop.md") || !path.IsCanonicalMember("system.json") {
		t.Fatal("legal members")
	}
	for _, p := range []string{"../x", "./sops/a.sop.md", "a/./b.md", "a//b.md", "/abs", `a\b.md`} {
		if path.IsCanonicalMember(p) {
			t.Fatalf("must reject %q", p)
		}
	}
}

func TestHardDeny(t *testing.T) {
	if err := path.HardDeny(".env"); rule.ID(err) != "SRC-PATH-HARD-DENY" {
		t.Fatalf("got %v", err)
	}
	if err := path.HardDeny(".env.example"); err != nil {
		t.Fatalf(".env.example: %v", err)
	}
	if err := path.HardDeny(".git/config"); rule.ID(err) != "SRC-PATH-HARD-DENY" {
		t.Fatalf("git: %v", err)
	}
	if err := path.HardDeny("secrets/id_rsa"); rule.ID(err) != "SRC-PATH-HARD-DENY" {
		t.Fatalf("id_rsa: %v", err)
	}
}

func TestCollide(t *testing.T) {
	if err := path.Collide([]string{"contexts/Foo.md", "contexts/foo.md"}); rule.ID(err) != "SRC-PATH-COLLIDE" {
		t.Fatalf("got %v", err)
	}
	if err := path.Collide([]string{"contexts/shared.md", "contexts/shared.md"}); err != nil {
		t.Fatalf("identical canonical is not a collision: %v", err)
	}
}

func TestArgv(t *testing.T) {
	if err := path.CheckProcess("./hooks/start.py", nil, nil); rule.ID(err) != "SRC-ARGV-COMMAND" {
		t.Fatalf("missing files: %v", err)
	}
	if err := path.CheckProcess("C:foo", nil, nil); rule.ID(err) != "SRC-ARGV-COMMAND" {
		t.Fatalf("drive-relative: %v", err)
	}
	if err := path.CheckProcess("python", []string{"hooks/a.py"}, nil); err != nil {
		t.Fatalf("opaque args: %v", err)
	}
	if err := path.CheckProcess("C:/Python/python.exe", nil, nil); err != nil {
		t.Fatalf("drive-absolute: %v", err)
	}
	if err := path.CheckProcess("python", []string{"./hooks/a.py"}, nil); rule.ID(err) != "SRC-ARGV-ARGS" {
		t.Fatalf("local arg not in files: %v", err)
	}
	if err := path.CheckProcess("python", []string{"./hooks/a.py"}, []string{"./hooks/a.py"}); err != nil {
		t.Fatalf("local arg listed: %v", err)
	}
	if err := path.CheckProcess("py\x00thon", nil, nil); rule.ID(err) != "SRC-ID-NUL" {
		t.Fatalf("nul: %v", err)
	}
}
