package cli

import (
	"fmt"
	"io"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
)

func printReadProtection(w io.Writer, requireStrict bool, hostID string, report compatibility.Report, pctx adapter.ProjectionContext) {
	fmt.Fprintln(w, formatReadProtection(requireStrict, hostID, report, pctx))
}

func formatReadProtection(requireStrict bool, hostID string, report compatibility.Report, pctx adapter.ProjectionContext) string {
	if requireStrict {
		// "enforced" is allowed only when the Host probe path actually confines
		// reads. StrictReadEnforced is false for all committed Hosts today.
		if adapter.StrictReadEnforced(hostID, pctx) && report.Decision != "refused" {
			return "Read protection: strict workspace read (enforced)"
		}
		return "Read protection: strict workspace read required; this host cannot enforce it (use ordinary run without --require-strict-read)"
	}
	if hostID == adapter.HostClaudeCode && adapter.ClaudePartialReadProtection(pctx) {
		return "Read protection: partial privacy (home-tree reads denied when run layout is outside $HOME); not strictly confined to workspace"
	}
	_ = report
	return "Read protection: ordinary run; reads are not strictly confined to the workspace"
}
