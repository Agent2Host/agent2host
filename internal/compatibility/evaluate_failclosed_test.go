package compatibility

import "testing"

func testEnv() Envelope {
	return Envelope{
		SchemaVersion:     schemaVersion,
		Agent2HostVersion: "0.0.0-test",
		Subject:           Subject{SystemID: "club-system", AgentID: "club-faq", Revision: "sha256:" + zeros(64)},
		Host:              HostRef{ID: "codex", Version: "1"},
		Adapter:           AdapterRef{ID: "codex", Version: "0.1.0"},
		Probe:             Probe{Fingerprint: "sha256:" + zeros(64)},
	}
}

func TestEvaluateMissingSecurityFactsRefuse(t *testing.T) {
	req := Requirement{
		Security: &SecurityReq{
			Permissions: &FlagReq{Required: true},
			Approvals:   &FlagReq{Required: true},
		},
	}
	got := Evaluate(testEnv(), req, Assessment{})
	if got.Decision != decisionRefused {
		t.Fatalf("decision %s", got.Decision)
	}
	if got.Security.Permissions.RequirementResult != resultUnknown ||
		got.Security.Permissions.ReasonCode != "insufficient_evidence" {
		t.Fatalf("permissions %+v", got.Security.Permissions)
	}
	if got.Security.Approvals.RequirementResult != resultUnknown {
		t.Fatalf("approvals %+v", got.Security.Approvals)
	}
}

func TestEvaluateEmptyPolicyFactsUnknown(t *testing.T) {
	got := Evaluate(testEnv(), Requirement{}, Assessment{
		Security: &SecurityAssess{
			Permissions: &PolicyAssess{},
			Approvals:   &PolicyAssess{},
		},
	})
	if got.Decision != decisionRefused {
		t.Fatalf("decision %s", got.Decision)
	}
	if got.Security.Permissions.Support != "unknown" ||
		got.Security.Permissions.RequirementResult != resultUnknown ||
		got.Security.Permissions.ReasonCode != "insufficient_evidence" {
		t.Fatalf("empty permissions %+v", got.Security.Permissions)
	}
}

func TestEvaluateOptionalUnknownPolicyDoesNotWarn(t *testing.T) {
	req := Requirement{
		Security: &SecurityReq{
			Permissions: &FlagReq{Required: true},
			Approvals:   &FlagReq{Required: true},
		},
	}
	got := Evaluate(testEnv(), req, Assessment{
		Security: &SecurityAssess{
			Permissions: &PolicyAssess{
				Support: "mapped", Scope: "agent", Enforcement: "host_enforced", Confidence: "documented",
				GrantSubseteqDeclared: boolPtr(true),
			},
			Approvals: &PolicyAssess{
				Support: "mapped", Scope: "agent", Enforcement: "host_enforced", Confidence: "documented",
				GateVsDeclared: "equal",
			},
		},
	})
	if got.Decision != decisionAllowed {
		t.Fatalf("untriggered sandbox/output_validation unknown must not warn, got %s", got.Decision)
	}
}

func TestEvaluateUnknownEnforcementRefusesRequiredPermissions(t *testing.T) {
	req := Requirement{
		Security: &SecurityReq{
			Permissions: &FlagReq{Required: true},
			Approvals:   &FlagReq{Required: true},
		},
	}
	got := Evaluate(testEnv(), req, Assessment{
		Security: &SecurityAssess{
			Permissions: &PolicyAssess{
				Support: "mapped", Scope: "agent", Enforcement: "unknown", Confidence: "documented",
				GrantSubseteqDeclared: boolPtr(true),
			},
			Approvals: &PolicyAssess{
				Support: "mapped", Scope: "agent", Enforcement: "unknown", Confidence: "documented",
				GateVsDeclared: "equal",
			},
		},
	})
	if got.Decision != decisionRefused {
		t.Fatalf("decision %s", got.Decision)
	}
	if got.Security.Permissions.RequirementResult != resultUnknown ||
		got.Security.Permissions.ReasonCode != "insufficient_evidence" {
		t.Fatalf("permissions %+v", got.Security.Permissions)
	}
}

func boolPtr(v bool) *bool { return &v }

func TestEvaluateSecretRequiredNotOverriddenByAssess(t *testing.T) {
	req := Requirement{
		Secrets: []SecretReq{{
			Consumer: "/mcp_servers/club-database",
			Target:   "CLUB_DB_TOKEN",
			Required: true,
		}},
	}
	got := Evaluate(testEnv(), req, Assessment{
		SecretIsolation: []SecretAssess{{
			Consumer: "/mcp_servers/club-database", Target: "CLUB_DB_TOKEN",
			Required: false, Support: "mapped", Scope: "host",
			Enforcement: "host_enforced", Confidence: "documented",
		}},
	})
	if len(got.Security.SecretIsolation.Items) != 1 {
		t.Fatalf("items %d", len(got.Security.SecretIsolation.Items))
	}
	it := got.Security.SecretIsolation.Items[0]
	if !it.Required {
		t.Fatal("Assessment must not clear Source required")
	}
	if it.RequirementResult != resultUnsatisfied || it.ReasonCode != "secret_scope_too_broad" {
		t.Fatalf("secret row %+v", it)
	}
	if got.Decision != decisionRefused {
		t.Fatalf("required wide secret must refuse, got %s", got.Decision)
	}
}
