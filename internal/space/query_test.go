package space_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent2host/agent2host/internal/space"
	"github.com/agent2host/agent2host/internal/space/registry"
	"github.com/agent2host/agent2host/internal/space/store"
)

func TestListInspectResolve(t *testing.T) {
	sp, err := space.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	listed, err := sp.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("empty home: %+v", listed)
	}

	src := fixtureTree(t, "valid", "markdown-leading-dashes")
	rep, err := sp.Register(src)
	if err != nil {
		t.Fatal(err)
	}
	listed, err = sp.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != "fm" || listed[0].ActiveRevision != rep.Revision {
		t.Fatalf("list %+v", listed)
	}

	ins, err := sp.Inspect("fm", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if ins.AgentID != "demo" || ins.ArtifactRevision != rep.Revision {
		t.Fatalf("inspect %+v", ins)
	}

	_, err = sp.Inspect("fm", "nope")
	var se *space.Error
	if !errors.As(err, &se) || se.Kind != space.KindUnknownAgent {
		t.Fatalf("unknown agent: %v", err)
	}
	_, err = sp.Inspect("missing", "demo")
	var re *registry.Error
	if !errors.As(err, &re) || re.Kind != registry.KindUnknown {
		t.Fatalf("unknown system: %v", err)
	}

	run, err := sp.Resolve("fm", "demo", "")
	if err != nil {
		t.Fatal(err)
	}
	if run.SOP != "sops/demo.sop.md" || run.Content["sops/demo.sop.md"] == nil {
		t.Fatalf("resolve %+v content %v", run, run.Content)
	}
	if _, ok := run.Content["system.json"]; ok {
		t.Fatal("Content must not expose full system.json")
	}
}

func TestParseTarget(t *testing.T) {
	sys, ag, err := space.ParseTarget("club-system/club-faq")
	if err != nil || sys != "club-system" || ag != "club-faq" {
		t.Fatalf("%s %s %v", sys, ag, err)
	}
	if _, _, err := space.ParseTarget("noshift"); err == nil {
		t.Fatal("expected bad target")
	}
	if _, _, err := space.ParseTarget("a/b/c"); err == nil {
		t.Fatal("expected extra slash")
	}
}

func TestInspectCorruptFailsClosed(t *testing.T) {
	home := t.TempDir()
	sp, err := space.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := sp.Register(fixtureTree(t, "valid", "markdown-leading-dashes"))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "space", "artifacts", "sha256-"+rep.Revision[len("sha256:"):], "content")
	if err := os.WriteFile(filepath.Join(dir, "system.json"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := sp.Inspect("fm", "demo"); err == nil {
		t.Fatal("corrupt must fail")
	}
	if _, err := sp.Resolve("fm", "demo", ""); err == nil {
		t.Fatal("corrupt resolve must fail")
	}
}

func TestClubSystemClosureOnly(t *testing.T) {
	sp, err := space.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := fixtureTree(t, "valid", "club-system")
	rep, err := sp.Register(src)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := sp.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != "club-system" || listed[0].AgentCount != 2 {
		t.Fatalf("list %+v", listed)
	}

	faq, err := sp.Resolve("club-system", "club-faq", "")
	if err != nil {
		t.Fatal(err)
	}
	if faq.ArtifactRevision != rep.Revision {
		t.Fatalf("revision %s", faq.ArtifactRevision)
	}
	got := []string{}
	for _, sk := range faq.Skills {
		got = append(got, sk.ID)
	}
	if len(got) != 2 || got[0] != "search-policy" || got[1] != "verify-member" {
		t.Fatalf("faq skills %v", got)
	}
	if faq.Skills[1].Required {
		t.Fatal("verify-member is optional")
	}
	if faq.Skills[0].Name != "Search policy" || faq.Skills[0].Description == "" {
		t.Fatalf("skill name/description %+v", faq.Skills[0])
	}
	if len(faq.Skills[0].MCPTools) == 0 {
		t.Fatal("skill mcp_tools must be in IR")
	}
	if len(faq.Contexts) != 1 || faq.Contexts[0].Path != "contexts/club-handbook.md" {
		t.Fatalf("contexts %+v", faq.Contexts)
	}
	if faq.Contexts[0].Loading == nil || *faq.Contexts[0].Loading != "on_demand" {
		t.Fatalf("context loading %+v", faq.Contexts[0])
	}
	if faq.Contexts[0].Isolation == nil || *faq.Contexts[0].Isolation != "required" {
		t.Fatalf("context isolation %+v", faq.Contexts[0])
	}
	mcp, ok := faq.MCPServers["club-database"]
	if !ok || len(mcp.Environment) == 0 {
		t.Fatalf("mcp environment %+v", faq.MCPServers)
	}
	if len(mcp.Tools) == 0 || mcp.Tools[0].Required == nil {
		t.Fatal("mcp tool required flags must be preserved")
	}
	if _, ok := faq.Content["system.json"]; ok {
		t.Fatal("Content must not expose full system.json")
	}
	if _, ok := faq.Content["skills/unused-declared.skill.md"]; ok {
		t.Fatal("unreferenced skill must not be in resolve closure")
	}
	if _, ok := faq.Content["agents/club-admin.agent.json"]; ok {
		t.Fatal("other Named Agent spec must not be in resolve Content")
	}
	if faq.Content["agents/club-faq.agent.json"] == nil {
		t.Fatal("selected Agent spec must be in resolve Content")
	}
	if faq.Content["skills/search-policy.skill.md"] == nil || faq.Content["mcp/club-database.py"] == nil {
		t.Fatal("referenced files must be inlined")
	}

	admin, err := sp.Resolve("club-system", "club-admin", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(admin.Skills) != 1 || admin.Skills[0].ID != "search-policy" {
		t.Fatalf("admin skills %+v", admin.Skills)
	}

	ins, err := sp.Inspect("club-system", "club-faq")
	if err != nil {
		t.Fatal(err)
	}
	if len(ins.Skills) != 2 {
		t.Fatalf("inspect skills %v", ins.Skills)
	}

	// Artifact still packs the unreferenced skill.
	st, err := store.New(sp.Home)
	if err != nil {
		t.Fatal(err)
	}
	art, err := st.Load(rep.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := art.Payload["skills/unused-declared.skill.md"]; !ok {
		t.Fatal("artifact must pack declared unused skill")
	}
}

func TestResolveRevisionBoundToSystem(t *testing.T) {
	sp, err := space.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sp.Register(fixtureTree(t, "valid", "markdown-leading-dashes")); err != nil {
		t.Fatal(err)
	}
	repB, err := sp.Register(fixtureTree(t, "valid", "env-example"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = sp.Resolve("fm", "demo", repB.Revision)
	var se *space.Error
	if !errors.As(err, &se) || se.Kind != space.KindMismatch {
		t.Fatalf("cross-system revision: %v", err)
	}
}

func TestAgentExistenceFromArtifactNotRegistryIndex(t *testing.T) {
	home := t.TempDir()
	sp, err := space.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sp.Register(fixtureTree(t, "valid", "club-system")); err != nil {
		t.Fatal(err)
	}

	rewriteAgents(t, home, "club-system", []string{"club-faq"})
	if _, err := sp.Resolve("club-system", "club-admin", ""); err != nil {
		t.Fatalf("artifact still has club-admin: %v", err)
	}
	ins, err := sp.Inspect("club-system", "club-admin")
	if err != nil {
		t.Fatalf("inspect must follow Artifact, not registry index: %v", err)
	}
	if len(ins.Agents) != 1 || ins.Agents[0] != "club-faq" {
		t.Fatalf("inspect Agents are registry facts: %+v", ins.Agents)
	}

	rewriteAgents(t, home, "club-system", []string{"club-faq", "ghost"})
	var se *space.Error
	if _, err := sp.Resolve("club-system", "ghost", ""); !errors.As(err, &se) || se.Kind != space.KindUnknownAgent {
		t.Fatalf("ghost resolve: %v", err)
	}
	if _, err := sp.Inspect("club-system", "ghost"); !errors.As(err, &se) || se.Kind != space.KindUnknownAgent {
		t.Fatalf("ghost inspect: %v", err)
	}
}

func rewriteAgents(t *testing.T, home, systemID string, agents []string) {
	t.Helper()
	reg, err := registry.New(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.WithWrite(func(doc *registry.Document) error {
		rec, ok := doc.Systems[systemID]
		if !ok {
			t.Fatalf("missing system %s", systemID)
		}
		rec.Agents = append([]string(nil), agents...)
		doc.Systems[systemID] = rec
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
