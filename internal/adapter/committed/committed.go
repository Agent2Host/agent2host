package committed

import (
	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/claude"
	"github.com/agent2host/agent2host/internal/adapter/codex"
	"github.com/agent2host/agent2host/internal/adapter/kiro"
)

// Default is this release’s committed Hosts: claude-code, kiro, codex.
func Default() *adapter.Registry {
	return New(nil, nil)
}

// New builds the committed adapters. Tests inject Probe look/version.
// Add a Host: implement adapter.HostAdapter in its own package, then list it here.
func New(look adapter.LookPathFunc, version adapter.VersionFunc) *adapter.Registry {
	r, err := adapter.NewRegistry(claude.New(look, version), kiro.New(look, version), codex.New(look, version))
	if err != nil {
		panic(err)
	}
	return r
}
