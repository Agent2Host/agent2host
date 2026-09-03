package adapter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// DestinationClass is an Adapter-facing namespace class.
// Physical roots are Runtime-defined; adapters must not invent Store paths.
type DestinationClass string

const (
	DestProjection DestinationClass = "immutable_projection"
	DestWorkspace  DestinationClass = "run_workspace"
	// DestHostPrivate is run-local Host home / secret-bearing config.
	// Runtime keeps it outside any Host --add-dir / model-readable surface.
	DestHostPrivate DestinationClass = "host_private"
	// DestHostAuth is the legacy durable dir under $A2H_HOME/host-auth/.
	// New login material uses DestAuthProfile.
	DestHostAuth DestinationClass = "host_auth"
	// DestAuthProfile is the stable Auth Profile directory for this Host identity.
	DestAuthProfile DestinationClass = "auth_profile"
	DestWorkingDir  DestinationClass = "approved_working_directory"
)

// ProjectionContext is the Runtime-supplied mapping environment.
type ProjectionContext struct {
	// ApprovedWorkingDirectory is the Host process cwd (absolute). Empty on check.
	ApprovedWorkingDirectory string
	// RunPrivateDirectory is the run-local Host home partition (absolute). Empty on check.
	RunPrivateDirectory string
}

// ProjectionFile is one planned Host-native file. Content is inline bytes
// from ResolvedAgentRun; Destination is a class, not an absolute path.
//
// Executable marks a System-local ProcessSpec command member. Runtime sets the
// execute bit from this flag, never from Source-tree mode bits
// Runtime sets this from the resolved Source member, not file mode bits.
type ProjectionFile struct {
	RelPath     string           `json:"rel_path"`
	Class       DestinationClass `json:"class"`
	Content     []byte           `json:"content,omitempty"`
	FromContent string           `json:"from_content,omitempty"`
	Executable  bool             `json:"executable,omitempty"`
}

// NativeProjectionPlan is which files to generate. It is not files on disk.
type NativeProjectionPlan struct {
	HostID string           `json:"host_id"`
	Files  []ProjectionFile `json:"files"`
}

// SecretRef is a late-resolve binding. Plans must not carry secret values.
type SecretRef struct {
	Name     string `json:"name"`
	Consumer string `json:"consumer"`
	Required bool   `json:"required"`
}

// HostProcessConsumer is true when V0 may inject this binding into the Host env.
// Empty consumer is treated as /environment for in-process test plans.
func HostProcessConsumer(consumer string) bool {
	return consumer == "" || consumer == "/environment"
}

// LaunchPlan is how to start the probed Host. It is not a running process.
type LaunchPlan struct {
	Executable      string            `json:"executable"`
	Args            []string          `json:"args,omitempty"`
	WorkingDirClass DestinationClass  `json:"working_dir_class"`
	Env             map[string]string `json:"env,omitempty"`
	Secrets         []SecretRef       `json:"secrets,omitempty"`
	AgentSelect     string            `json:"agent_select,omitempty"`
}

// Plans is the Project pair.
type Plans struct {
	Projection NativeProjectionPlan `json:"projection"`
	Launch     LaunchPlan           `json:"launch"`
}

// DigestPlans is the in-process identity of these exact Plans
// Encoding is not a published cache key.
func DigestPlans(p Plans) (string, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
