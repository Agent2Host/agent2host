package space

import (
	"encoding/json"
	"testing"
)

func TestV1Alpha1WorkRootDefaultsToInvocation(t *testing.T) {
	sys := []byte(`{"schema_version":"agent2host/v1alpha1","kind":"AgentSystem","id":"sys","version":"1.0.0","agents":["agents/a.agent.json"]}`)
	agent := []byte(`{"schema_version":"agent2host/v1alpha1","kind":"Agent","id":"demo","sop":"sops/a.sop.md"}`)
	payload := map[string][]byte{
		"system.json":         sys,
		"agents/a.agent.json": agent,
		"sops/a.sop.md":       []byte("# sop\n"),
	}
	run, err := closure("sys", "demo", "sha256:test", payload)
	if err != nil {
		t.Fatal(err)
	}
	if run.WorkRoot.Mode != "invocation" || run.WorkRoot.PathFromHome != "" {
		t.Fatalf("%+v", run.WorkRoot)
	}
}

func TestV1Alpha2FixedWorkRootSurvivesResolve(t *testing.T) {
	sys := []byte(`{"schema_version":"agent2host/v1alpha2","kind":"AgentSystem","id":"sys","version":"1.0.0","agents":["agents/a.agent.json"],"work_root":{"mode":"fixed","path_from_home":"Desktop/Events"}}`)
	agent := []byte(`{"schema_version":"agent2host/v1alpha1","kind":"Agent","id":"demo","sop":"sops/a.sop.md"}`)
	payload := map[string][]byte{
		"system.json":         sys,
		"agents/a.agent.json": agent,
		"sops/a.sop.md":       []byte("# sop\n"),
	}
	run, err := closure("sys", "demo", "sha256:test", payload)
	if err != nil {
		t.Fatal(err)
	}
	if run.WorkRoot.Mode != "fixed" || run.WorkRoot.PathFromHome != "Desktop/Events" {
		t.Fatalf("%+v", run.WorkRoot)
	}
}

func TestScriptsOrderDeterministic(t *testing.T) {
	sys := []byte(`{"schema_version":"agent2host/v1alpha1","kind":"AgentSystem","id":"sys","version":"1.0.0","agents":["agents/a.agent.json"]}`)
	agent := []byte(`{"schema_version":"agent2host/v1alpha1","kind":"Agent","id":"demo","sop":"sops/a.sop.md","mcp_servers":{"z-server":{"transport":"stdio","command":"python","args":["./mcp/z.py"],"files":["./mcp/z.py"],"tools":["tz"]},"a-server":{"transport":"stdio","command":"python","args":["./mcp/a.py"],"files":["./mcp/a.py"],"tools":["ta"]}}}`)
	payload := map[string][]byte{
		"system.json":         sys,
		"agents/a.agent.json": agent,
		"sops/a.sop.md":       []byte("# sop\n"),
		"mcp/z.py":            []byte("z"),
		"mcp/a.py":            []byte("a"),
	}
	var prev []byte
	for i := 0; i < 30; i++ {
		run, err := closure("sys", "demo", "sha256:test", payload)
		if err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(run.Scripts)
		if err != nil {
			t.Fatal(err)
		}
		if prev != nil && string(prev) != string(got) {
			t.Fatalf("scripts unstable: %s vs %s", prev, got)
		}
		prev = got
	}
	run, err := closure("sys", "demo", "sha256:test", payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Scripts) != 2 || run.Scripts[0] != "mcp/a.py" || run.Scripts[1] != "mcp/z.py" {
		t.Fatalf("want MCP ids sorted into scripts, got %v", run.Scripts)
	}
}

func TestClosureCommandMatchesContentKeys(t *testing.T) {
	sys := []byte(`{"schema_version":"agent2host/v1alpha1","kind":"AgentSystem","id":"sys","version":"1.0.0","agents":["agents/a.agent.json"]}`)
	agent := []byte(`{"schema_version":"agent2host/v1alpha1","kind":"Agent","id":"demo","sop":"sops/a.sop.md","hooks":{"session_start":[{"command":"./hooks/start.py","files":["./hooks/start.py"]}]},"mcp_servers":{"db":{"transport":"stdio","command":"./mcp/a.py","args":["./mcp/a.py"],"files":["./mcp/a.py"],"tools":["t"]}}}`)
	payload := map[string][]byte{
		"system.json":         sys,
		"agents/a.agent.json": agent,
		"sops/a.sop.md":       []byte("# sop\n"),
		"hooks/start.py":      []byte("h"),
		"mcp/a.py":            []byte("m"),
	}
	run, err := closure("sys", "demo", "sha256:test", payload)
	if err != nil {
		t.Fatal(err)
	}
	if run.Hooks == nil || run.Hooks.SessionStart == nil || (*run.Hooks.SessionStart)[0].Command != "hooks/start.py" {
		t.Fatalf("hook command: %+v", run.Hooks)
	}
	mcp := run.MCPServers["db"]
	if mcp.Command != "mcp/a.py" {
		t.Fatalf("mcp command %q", mcp.Command)
	}
	if _, ok := run.Content["hooks/start.py"]; !ok {
		t.Fatal("hook file missing from Content")
	}
	if _, ok := run.Content["mcp/a.py"]; !ok {
		t.Fatal("mcp file missing from Content")
	}
}

func TestClosureDedupsScriptsAndAssets(t *testing.T) {
	sys := []byte(`{"schema_version":"agent2host/v1alpha1","kind":"AgentSystem","id":"sys","version":"1.0.0","agents":["agents/a.agent.json"],"skills":{"s":{"name":"S","description":"d","document":"skills/s.skill.md","scripts":["tools/a.py","tools/a.py"],"assets":["assets/x.bin","assets/x.bin"]}}}`)
	agent := []byte(`{"schema_version":"agent2host/v1alpha1","kind":"Agent","id":"demo","name":"Demo","description":"A demo agent.","sop":"sops/a.sop.md","skills":["s"],"hooks":{"agent_stop":[{"command":"python","files":["tools/a.py"]}]}}`)
	payload := map[string][]byte{
		"system.json":         sys,
		"agents/a.agent.json": agent,
		"sops/a.sop.md":       []byte("# sop\n"),
		"skills/s.skill.md":   []byte("# s\n"),
		"tools/a.py":          []byte("t"),
		"assets/x.bin":        []byte("x"),
	}
	run, err := closure("sys", "demo", "sha256:test", payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Scripts) != 1 || run.Scripts[0] != "tools/a.py" {
		t.Fatalf("scripts %v", run.Scripts)
	}
	if len(run.Assets) != 1 || run.Assets[0] != "assets/x.bin" {
		t.Fatalf("assets %v", run.Assets)
	}
	if run.Skills[0].Name != "S" || run.Skills[0].Description != "d" {
		t.Fatalf("skill fields %+v", run.Skills[0])
	}
	if run.Name != "Demo" || run.Description != "A demo agent." {
		t.Fatalf("agent identity %q %q", run.Name, run.Description)
	}
}

func TestClosureRejectsSystemIDMismatch(t *testing.T) {
	payload := map[string][]byte{
		"system.json":         []byte(`{"schema_version":"agent2host/v1alpha1","kind":"AgentSystem","id":"other","version":"1.0.0","agents":["agents/a.agent.json"]}`),
		"agents/a.agent.json": []byte(`{"schema_version":"agent2host/v1alpha1","kind":"Agent","id":"demo","sop":"sops/a.sop.md"}`),
		"sops/a.sop.md":       []byte("#\n"),
	}
	_, err := closure("sys", "demo", "sha256:test", payload)
	if err == nil {
		t.Fatal("expected mismatch")
	}
	se, ok := err.(*Error)
	if !ok || se.Kind != KindMismatch {
		t.Fatalf("got %v", err)
	}
}
