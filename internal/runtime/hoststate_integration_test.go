//go:build integration

package runtime

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/committed"
)

func TestIntegrationHostAuthReuse(t *testing.T) {
	if os.Getenv("A2H_REQUIRE_INTEGRATION") == "" {
		t.Skip("set A2H_REQUIRE_INTEGRATION=1 to run host login-reuse probes")
	}
	allow := integrationHostSet()
	if allow["codex"] {
		t.Run("codex", testCodexAuthProfileBind)
	}
	if allow["claude-code"] {
		t.Run("claude-code", testClaudeAuthProfileStable)
	}
	if allow["kiro"] {
		t.Run("kiro", testKiroExternalSession)
	}
}

func integrationHostSet() map[string]bool {
	raw := os.Getenv("A2H_PROBE_HOSTS")
	out := map[string]bool{}
	if raw == "" {
		out["codex"] = true
		out["claude-code"] = true
		out["kiro"] = true
		return out
	}
	for _, h := range strings.Split(raw, ",") {
		h = strings.TrimSpace(h)
		if h != "" {
			out[h] = true
		}
	}
	return out
}

func testCodexAuthProfileBind(t *testing.T) {
	a, err := committed.Default().Select(adapter.HostCodex)
	if err != nil {
		t.Fatal(err)
	}
	authn := a.HostState()
	home := t.TempDir()
	p1, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p1, err = attachAuthProfile(p1, authn)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p2, err = attachAuthProfile(p2, authn)
	if err != nil {
		t.Fatal(err)
	}
	if p1.AuthProfile != p2.AuthProfile {
		t.Fatal("Codex Auth Profile must be stable across runs")
	}
	t.Log("codex login status / TUI second-run is not claimed by this probe")
}

func testClaudeAuthProfileStable(t *testing.T) {
	a, err := committed.Default().Select(adapter.HostClaudeCode)
	if err != nil {
		t.Fatal(err)
	}
	authn := a.HostState()
	home := t.TempDir()
	p1, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p1, err = attachAuthProfile(p1, authn)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := Prepare(home, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p2, err = attachAuthProfile(p2, authn)
	if err != nil {
		t.Fatal(err)
	}
	if p1.AuthProfile != p2.AuthProfile {
		t.Fatal("Claude Auth Profile must be stable across runs")
	}
	if strings.Contains(p1.AuthProfile, p1.Private) || strings.Contains(p1.AuthProfile, "host-config") {
		t.Fatal("per-run host-config must not be the Claude login namespace")
	}
	if _, err := bindHostAuth(p1, authn); err != nil {
		t.Fatal(err)
	}
	t.Log("claude auth status is not TUI-login evidence; second-run TUI was operator-confirmed 2026-09-01")
}

func testKiroExternalSession(t *testing.T) {
	if _, err := exec.LookPath("kiro-cli"); err != nil {
		t.Skip("kiro-cli not on PATH")
	}
	home := t.TempDir()
	cmd := exec.Command("kiro-cli", "whoami")
	cmd.Env = append(os.Environ(), "KIRO_HOME="+home)
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		t.Fatalf("kiro-cli whoami: %v\n%s", err, text)
	}
	if !strings.Contains(strings.ToLower(text), "logged in") {
		t.Fatalf("isolated KIRO_HOME should keep login (auth is outside KIRO_HOME), got:\n%s", text)
	}
}
