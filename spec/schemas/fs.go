package schemafs

import "embed"

// Files holds the published Schema documents. Production Load() must
// use this FS, not a path walked from runtime.Caller.
//
//go:embed source/v1alpha1/common.schema.json
//go:embed source/v1alpha1/agent.schema.json
//go:embed source/v1alpha1/system.schema.json
//go:embed source/v1alpha2/system.schema.json
//go:embed artifact/agent2host-system-v1.schema.json
var Files embed.FS

const (
	Common   = "source/v1alpha1/common.schema.json"
	Agent    = "source/v1alpha1/agent.schema.json"
	System   = "source/v1alpha1/system.schema.json"
	SystemV2 = "source/v1alpha2/system.schema.json"
	Artifact = "artifact/agent2host-system-v1.schema.json"
)
