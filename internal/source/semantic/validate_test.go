package semantic_test

import (
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/source/fixtures"
	"github.com/agent2host/agent2host/internal/source/path"
	"github.com/agent2host/agent2host/internal/source/rule"
	"github.com/agent2host/agent2host/internal/source/semantic"
)

type expected struct {
	Register string `json:"register"`
	RuleID   string `json:"rule_id"`
	Warning  bool   `json:"warning"`
	Secrets  []struct {
		Consumer string `json:"consumer"`
		Target   string `json:"target"`
	} `json:"secret_isolation_items"`
	Assertions *struct {
		MemberOccurrences   map[string]int      `json:"member_occurrences"`
		ArtifactMemberPaths []string            `json:"artifact_member_paths"`
		ResolvedSkillIDs    map[string][]string `json:"resolved_skill_ids"`
		NotPackedPaths      []string            `json:"not_packed_paths"`
	} `json:"assertions"`
}

func TestSemanticRejectDocuments(t *testing.T) {
	e, err := semantic.New()
	if err != nil {
		t.Fatal(err)
	}
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "semantic-reject")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range ents {
		name := ent.Name()
		switch {
		case strings.HasSuffix(name, ".agent.json"):
			expName := strings.TrimSuffix(name, ".agent.json") + ".agent.expected.json"
			raw := read(t, filepath.Join(dir, name))
			exp := loadExpected(t, filepath.Join(dir, expName))
			t.Run(name, func(t *testing.T) {
				_, err := e.AgentBytes(raw)
				if exp.Register != "fail" {
					t.Fatalf("fixture expected fail")
				}
				if rule.ID(err) != exp.RuleID {
					t.Fatalf("rule %q want %q (err=%v)", rule.ID(err), exp.RuleID, err)
				}
			})
		case strings.HasSuffix(name, ".system.json"):
			expName := strings.TrimSuffix(name, ".system.json") + ".system.expected.json"
			raw := read(t, filepath.Join(dir, name))
			exp := loadExpected(t, filepath.Join(dir, expName))
			t.Run(name, func(t *testing.T) {
				err := e.SystemBytes(raw)
				if rule.ID(err) != exp.RuleID {
					t.Fatalf("rule %q want %q (err=%v)", rule.ID(err), exp.RuleID, err)
				}
			})
		}
	}
}

func TestSourceTrees(t *testing.T) {
	e, err := semantic.New()
	if err != nil {
		t.Fatal(err)
	}
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	for _, bucket := range []string{"valid", "invalid"} {
		base := filepath.Join(root, "trees", bucket)
		ents, err := os.ReadDir(base)
		if err != nil {
			t.Fatal(err)
		}
		for _, ent := range ents {
			if !ent.IsDir() {
				continue
			}
			dir := filepath.Join(base, ent.Name())
			expPath := filepath.Join(dir, "expected.json")
			if _, err := os.Stat(expPath); err != nil {
				continue
			}
			exp := loadExpected(t, expPath)
			t.Run(bucket+"/"+ent.Name(), func(t *testing.T) {
				res, err := e.Tree(dir)
				if exp.Register == "fail" {
					if rule.ID(err) != exp.RuleID {
						t.Fatalf("rule %q want %q (err=%v)", rule.ID(err), exp.RuleID, err)
					}
					return
				}
				if err != nil {
					t.Fatalf("register: %v", err)
				}
				if exp.Warning {
					if !hasWarning(res.Warnings, exp.RuleID) {
						t.Fatalf("missing warning %s in %+v", exp.RuleID, res.Warnings)
					}
				}
				assertResult(t, res, exp)
				res2, err := e.Tree(dir)
				if err != nil {
					t.Fatal(err)
				}
				if res.Revision != res2.Revision {
					t.Fatalf("revision not stable: %s vs %s", res.Revision, res2.Revision)
				}
			})
		}
	}
}

func TestTreeReadsSystemJSONThroughFS(t *testing.T) {
	e, err := semantic.New()
	if err != nil {
		t.Fatal(err)
	}
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "trees", "valid", "markdown-leading-dashes")
	fsys, err := path.NewFS(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	real := fsys.ReadFile
	var names []string
	fsys.ReadFile = func(name string) ([]byte, error) {
		names = append(names, name)
		return real(name)
	}
	if _, err := e.TreeWithFS(dir, fsys); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range names {
		if filepath.Base(n) == "system.json" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("system.json must be read through injected FS, got %v", names)
	}
}

func TestTreeUsesSingleByteSnapshot(t *testing.T) {
	e, err := semantic.New()
	if err != nil {
		t.Fatal(err)
	}
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "trees", "valid", "markdown-leading-dashes")
	fsys, err := path.NewFS(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	real := fsys.ReadFile
	first := map[string][]byte{}
	counts := map[string]int{}
	fsys.ReadFile = func(name string) ([]byte, error) {
		counts[name]++
		if counts[name] > 1 {
			return []byte("TAMPERED-SECOND-READ"), nil
		}
		b, err := real(name)
		if err != nil {
			return nil, err
		}
		first[name] = append([]byte(nil), b...)
		return b, nil
	}
	res, err := e.TreeWithFS(dir, fsys)
	if err != nil {
		t.Fatal(err)
	}
	for name, n := range counts {
		if n != 1 {
			t.Errorf("ReadFile(%s) called %d times", name, n)
		}
	}
	for p, raw := range res.Payload {
		abs := filepath.Join(fsys.Root, filepath.FromSlash(p))
		want, ok := first[abs]
		if !ok {
			t.Fatalf("payload %s was not read through injected FS", p)
		}
		if string(raw) != string(want) {
			t.Fatalf("payload %s mixed a later filesystem version", p)
		}
	}
}

func TestPathTypeRecipe(t *testing.T) {
	e, err := semantic.New()
	if err != nil {
		t.Fatal(err)
	}
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	template := filepath.Join(root, "trees", "generated", "path-type", "source-template")

	t.Run("symlink-leaf", func(t *testing.T) {
		dir := copyTemplate(t, template)
		if err := os.Symlink("demo.sop.md", filepath.Join(dir, "sops", "link.sop.md")); err != nil {
			t.Fatal(err)
		}
		rewriteSOP(t, dir, "./sops/link.sop.md")
		_, err := e.Tree(dir)
		if rule.ID(err) != "SRC-PATH-TYPE" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("symlink-component", func(t *testing.T) {
		dir := copyTemplate(t, template)
		if err := os.Symlink("sops", filepath.Join(dir, "sops-link")); err != nil {
			t.Fatal(err)
		}
		rewriteSOP(t, dir, "./sops-link/demo.sop.md")
		_, err := e.Tree(dir)
		if rule.ID(err) != "SRC-PATH-TYPE" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("directory", func(t *testing.T) {
		dir := copyTemplate(t, template)
		if err := os.Mkdir(filepath.Join(dir, "sops", "not-a-file.sop.md"), 0o755); err != nil {
			t.Fatal(err)
		}
		rewriteSOP(t, dir, "./sops/not-a-file.sop.md")
		_, err := e.Tree(dir)
		if rule.ID(err) != "SRC-PATH-TYPE" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("fifo", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("fifo not on windows")
		}
		dir := copyTemplate(t, template)
		p := filepath.Join(dir, "sops", "fifo.sop.md")
		if err := mkfifo(p); err != nil {
			t.Fatal(err)
		}
		rewriteSOP(t, dir, "./sops/fifo.sop.md")
		_, err := e.Tree(dir)
		if rule.ID(err) != "SRC-PATH-TYPE" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("unix-socket", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("unix socket not on windows")
		}
		dir := copyTemplate(t, template)
		p := filepath.Join(dir, "sops", "sock.sop.md")
		ln, err := listenUnix(p)
		if err != nil {
			t.Skipf("listen_failed: %v", err)
		}
		t.Cleanup(func() { ln.Close() })
		rewriteSOP(t, dir, "./sops/sock.sop.md")
		_, err = e.Tree(dir)
		if rule.ID(err) != "SRC-PATH-TYPE" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("device", func(t *testing.T) {
		dir := copyTemplate(t, template)
		fsys, err := path.NewFS(dir, "")
		if err != nil {
			t.Fatal(err)
		}
		real := fsys.Lstat
		fsys.Lstat = func(name string) (os.FileInfo, error) {
			fi, err := real(name)
			if err != nil {
				return nil, err
			}
			if filepath.Base(name) == "demo.sop.md" && fi.Mode().IsRegular() {
				return deviceFile{FileInfo: fi}, nil
			}
			return fi, nil
		}
		_, err = e.TreeWithFS(dir, fsys)
		if rule.ID(err) != "SRC-PATH-TYPE" {
			t.Fatalf("got %v", err)
		}
	})
}

type deviceFile struct{ os.FileInfo }

func (d deviceFile) Mode() os.FileMode { return os.ModeDevice }

func assertResult(t *testing.T, res *semantic.Result, exp expected) {
	t.Helper()
	if exp.Assertions == nil && len(exp.Secrets) == 0 {
		return
	}
	set := map[string]int{}
	for _, p := range res.Members {
		set[p]++
	}
	if exp.Assertions != nil {
		for p, n := range exp.Assertions.MemberOccurrences {
			if set[p] != n {
				t.Fatalf("member_occurrences[%s]=%d want %d (members=%v)", p, set[p], n, res.Members)
			}
		}
		if len(exp.Assertions.ArtifactMemberPaths) > 0 {
			want := map[string]struct{}{}
			for _, p := range exp.Assertions.ArtifactMemberPaths {
				want[p] = struct{}{}
			}
			got := map[string]struct{}{}
			for _, p := range res.Members {
				got[p] = struct{}{}
			}
			if len(got) != len(want) {
				t.Fatalf("member count %d want %d\ngot %v\nwant %v", len(got), len(want), res.Members, exp.Assertions.ArtifactMemberPaths)
			}
			for p := range want {
				if _, ok := got[p]; !ok {
					t.Fatalf("missing member %s in %v", p, res.Members)
				}
			}
		}
		for _, p := range exp.Assertions.NotPackedPaths {
			if set[p] != 0 {
				t.Fatalf("%s must not be packed", p)
			}
		}
		for id, skills := range exp.Assertions.ResolvedSkillIDs {
			got := res.SkillIDs[id]
			if strings.Join(got, ",") != strings.Join(skills, ",") {
				t.Fatalf("skills[%s]=%v want %v", id, got, skills)
			}
		}
	}
	if len(exp.Secrets) > 0 {
		if len(res.Secrets) != len(exp.Secrets) {
			t.Fatalf("secrets %d want %d: %+v", len(res.Secrets), len(exp.Secrets), res.Secrets)
		}
		for i, s := range exp.Secrets {
			if res.Secrets[i].Consumer != s.Consumer || res.Secrets[i].Target != s.Target {
				t.Fatalf("secret[%d]=%+v want %+v", i, res.Secrets[i], s)
			}
		}
	}
}

func hasWarning(ws []rule.Warning, id string) bool {
	for _, w := range ws {
		if w.ID == id {
			return true
		}
	}
	return false
}

func loadExpected(t *testing.T, p string) expected {
	t.Helper()
	var exp expected
	if err := json.Unmarshal(read(t, p), &exp); err != nil {
		t.Fatal(err)
	}
	return exp
}

func read(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func copyTemplate(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func rewriteSOP(t *testing.T, root, sop string) {
	t.Helper()
	p := filepath.Join(root, "agents", "demo.agent.json")
	var obj map[string]any
	if err := json.Unmarshal(read(t, p), &obj); err != nil {
		t.Fatal(err)
	}
	obj["sop"] = sop
	raw, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
