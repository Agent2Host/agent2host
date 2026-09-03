package adapter

import (
	"testing"

	"github.com/agent2host/agent2host/internal/compatibility"
)

func TestHostVersionVerified(t *testing.T) {
	cases := []struct {
		host, ver string
		want      bool
	}{
		{HostClaudeCode, "2.1.252 (Claude Code)", true},
		{HostClaudeCode, "2.0.9", false},
		{HostKiro, "2.20.1", true},
		{HostKiro, "kiro-cli 2.20.2", true},
		{HostKiro, "kiro-cli 2.21.0", true},
		{HostKiro, "2.19.0", false},
		{HostCodex, "0.149.1", true},
		{HostCodex, "codex-cli 0.149.1", true},
		{HostCodex, "0.148.0", false},
		{HostClaudeCode, "1.0.0-test", true},
		{HostClaudeCode, "unknown", false},
		{HostClaudeCode, "", false},
	}
	for _, tc := range cases {
		if got := HostVersionVerified(tc.host, tc.ver); got != tc.want {
			t.Fatalf("%s %q: got %v want %v", tc.host, tc.ver, got, tc.want)
		}
	}
}

func TestApplyRunPolicyStrictReadRefuses(t *testing.T) {
	report := ApplyRunPolicy(
		compatibilityReportAllowed(),
		ProbeResult{HostID: HostClaudeCode, Found: true, HostVersion: "2.1.0"},
		HostClaudeCode,
		ProjectionContext{},
		RunPolicy{RequireStrictRead: true},
	)
	if report.Decision != "refused" {
		t.Fatalf("decision %q", report.Decision)
	}
}

func TestApplyRunPolicyUnverifiedVersionCheckWarns(t *testing.T) {
	report := ApplyRunPolicy(
		compatibilityReportAllowed(),
		ProbeResult{HostID: HostClaudeCode, Found: true, HostVersion: "9.9.9"},
		HostClaudeCode,
		ProjectionContext{},
		RunPolicy{},
	)
	if report.Decision != "allowed_with_warnings" {
		t.Fatalf("decision %q", report.Decision)
	}
}

func TestApplyRunPolicyUnverifiedVersionRunRefuses(t *testing.T) {
	report := ApplyRunPolicy(
		compatibilityReportAllowed(),
		ProbeResult{HostID: HostClaudeCode, Found: true, HostVersion: "9.9.9"},
		HostClaudeCode,
		ProjectionContext{},
		RunPolicy{ForExecute: true},
	)
	if report.Decision != "refused" {
		t.Fatalf("decision %q", report.Decision)
	}
}

func compatibilityReportAllowed() compatibility.Report {
	return compatibility.Report{Decision: "allowed"}
}
