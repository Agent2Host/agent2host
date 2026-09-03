package main

import (
	"os"

	"github.com/agent2host/agent2host/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args, os.Stdout, os.Stderr))
}
