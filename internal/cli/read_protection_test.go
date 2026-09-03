package cli

import (
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
)

func TestFormatReadProtectionStrictNeverClaimsEnforcedToday(t *testing.T) {
	msg := formatReadProtection(true, adapter.HostClaudeCode, compatibility.Report{Decision: "allowed"}, adapter.ProjectionContext{})
	if strings.Contains(msg, "(enforced)") {
		t.Fatalf("no committed Host has StrictReadEnforced; got %q", msg)
	}
	if !strings.Contains(msg, "cannot enforce") {
		t.Fatalf("want cannot-enforce message, got %q", msg)
	}
}

func TestFormatReadProtectionOrdinary(t *testing.T) {
	msg := formatReadProtection(false, adapter.HostCodex, compatibility.Report{Decision: "allowed"}, adapter.ProjectionContext{})
	if !strings.Contains(msg, "not strictly confined") {
		t.Fatalf("got %q", msg)
	}
}
