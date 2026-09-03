package adapter

import (
	"os"
	"path/filepath"
	"strings"
)

// Claude permission globs expand ~, not $HOME. The old "Read(//$HOME/**)"
// form is silently ignored, so it never denied home-tree Read.
const ClaudeHomeReadDenyRule = "Read(~/**)"

// ClaudePartialReadProtection is true when Claude home-tree deny is projected.
func ClaudePartialReadProtection(pctx ProjectionContext) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	return layoutOutsideHome(pctx.ApprovedWorkingDirectory, pctx.RunPrivateDirectory, home)
}

func ClaudeShouldDenyHomeReads(pctx ProjectionContext) bool {
	return ClaudePartialReadProtection(pctx)
}

func layoutOutsideHome(approvedWD, runPrivate, home string) bool {
	if approvedWD == "" || runPrivate == "" || home == "" {
		return false
	}
	return !PathUnderHome(approvedWD, home) && !PathUnderHome(runPrivate, home)
}

func PathUnderHome(path, home string) bool {
	path = filepath.Clean(path)
	home = filepath.Clean(home)
	if path == home {
		return true
	}
	return strings.HasPrefix(path, home+string(filepath.Separator))
}
