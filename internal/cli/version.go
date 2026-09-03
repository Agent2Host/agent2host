package cli

import (
	"fmt"
	"io"
)

// Link-time identity. Public builds overwrite these with -X.
var (
	Version   = "0.0.0-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func cmdVersion(jsonOut bool, stdout, stderr io.Writer) int {
	if jsonOut {
		return writeJSON(stdout, stderr, map[string]string{
			"version":    Version,
			"commit":     Commit,
			"build_time": BuildTime,
		})
	}
	fmt.Fprintf(stdout, "a2h %s\ncommit %s\nbuilt %s\n", Version, Commit, BuildTime)
	return ExitOK
}
