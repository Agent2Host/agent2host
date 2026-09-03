package adapter_test

import (
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/committed"
)

func TestCommittedAuthTopologies(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	cases := []struct {
		host     string
		topology string
		copies   int
		envKey   string
	}{
		{adapter.HostClaudeCode, adapter.AuthTopologyBoundRoot, 0, "CLAUDE_CONFIG_DIR"},
		{adapter.HostCodex, adapter.AuthTopologySeparated, 1, "CODEX_HOME"},
		{adapter.HostKiro, adapter.AuthTopologyExternal, 0, ""},
	}
	for _, tc := range cases {
		a, err := reg.Select(tc.host)
		if err != nil {
			t.Fatal(err)
		}
		d := a.HostState().DescribeAuth()
		if err := adapter.ValidateAuthDescription(d); err != nil {
			t.Fatalf("%s: %v", tc.host, err)
		}
		if d.Topology != tc.topology {
			t.Fatalf("%s topology %s want %s", tc.host, d.Topology, tc.topology)
		}
		if d.Profile.Host != tc.host || d.Profile.Provider == "" || d.Profile.NativeAuthNamespace == "" {
			t.Fatalf("%s incomplete profile %+v", tc.host, d.Profile)
		}
		if len(d.Materials) != tc.copies {
			t.Fatalf("%s materials %d want %d", tc.host, len(d.Materials), tc.copies)
		}
		bind, err := a.HostState().BindForRun(adapter.AuthBindRequest{})
		if err != nil {
			t.Fatalf("%s bind: %v", tc.host, err)
		}
		if len(bind.Copies) != tc.copies {
			t.Fatalf("%s bind copies %d want %d", tc.host, len(bind.Copies), tc.copies)
		}
		if tc.envKey != "" {
			if bind.Env[tc.envKey] == "" {
				t.Fatalf("%s bind must declare %s", tc.host, tc.envKey)
			}
		} else if len(bind.Env) != 0 {
			t.Fatalf("%s bind must not set env, got %v", tc.host, bind.Env)
		}
	}
}

func TestKiroAuthDeclaresExternalNoMaterials(t *testing.T) {
	a, err := committed.New(foundLook(), stubVersion).Select(adapter.HostKiro)
	if err != nil {
		t.Fatal(err)
	}
	d := a.HostState().DescribeAuth()
	if d.Topology != adapter.AuthTopologyExternal || len(d.Materials) != 0 {
		t.Fatalf("%+v", d)
	}
	bind, err := a.HostState().BindForRun(adapter.AuthBindRequest{})
	if err != nil || len(bind.Copies) != 0 {
		t.Fatalf("Kiro must not copy host state: %+v %v", bind, err)
	}
}

func TestCodexAuthDeclaresOpaqueAuthJSON(t *testing.T) {
	a, err := committed.New(foundLook(), stubVersion).Select(adapter.HostCodex)
	if err != nil {
		t.Fatal(err)
	}
	d := a.HostState().DescribeAuth()
	if len(d.Materials) != 1 || d.Materials[0].StoreRel != "auth.json" || !d.Materials[0].Lock {
		t.Fatalf("%+v", d.Materials)
	}
	if d.Concurrency != adapter.AuthConcurrencyUnverified {
		t.Fatalf("dual-session refresh is unverified, got %s", d.Concurrency)
	}
}
