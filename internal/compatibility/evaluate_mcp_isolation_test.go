package compatibility

import "testing"

func TestEvalMCPIsolationUnknownRefuses(t *testing.T) {
	items := evalMCPIsolation([]MCPIsoAssess{{
		ServerID: "db", Support: "approximate", Scope: "agent",
		Enforcement: "unknown", Confidence: "documented",
	}})
	if len(items) != 1 {
		t.Fatal(items)
	}
	if items[0].RequirementResult != resultUnknown || items[0].ReasonCode != "insufficient_evidence" {
		t.Fatalf("%+v", items[0])
	}
}

func TestEvalMCPIsolationAgent2HostEnforcedSatisfied(t *testing.T) {
	items := evalMCPIsolation([]MCPIsoAssess{{
		ServerID: "db", Support: "approximate", Scope: "agent",
		Enforcement: "agent2host_enforced", Confidence: "documented",
	}})
	if items[0].RequirementResult != resultSatisfied {
		t.Fatalf("%+v", items[0])
	}
}
