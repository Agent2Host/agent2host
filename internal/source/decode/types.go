package decode

import "encoding/json"

// AgentSource is the frozen Agent Spec object. Optional members use pointers so
// omitted JSON keys stay omitted on encode (decode does not fill defaults).
type AgentSource struct {
	SchemaVersion string                      `json:"schema_version"`
	Kind          string                      `json:"kind"`
	ID            string                      `json:"id"`
	Name          *string                     `json:"name,omitempty"`
	Description   *string                     `json:"description,omitempty"`
	SOP           string                      `json:"sop"`
	Skills        *[]SkillRef                 `json:"skills,omitempty"`
	Contexts      *[]ContextEntry             `json:"contexts,omitempty"`
	MCPServers    *map[string]MCPServer       `json:"mcp_servers,omitempty"`
	Hooks         *Hooks                      `json:"hooks,omitempty"`
	Environment   *[]EnvironmentBinding       `json:"environment,omitempty"`
	Permissions   *Permissions                `json:"permissions,omitempty"`
	Approvals     *Approvals                  `json:"approvals,omitempty"`
	Sandbox       *Sandbox                    `json:"sandbox,omitempty"`
	Output        *Output                     `json:"output,omitempty"`
	Extensions    *map[string]json.RawMessage `json:"extensions,omitempty"`
}

// SystemSource is the frozen AgentSystem object.
type SystemSource struct {
	SchemaVersion string                      `json:"schema_version"`
	Kind          string                      `json:"kind"`
	ID            string                      `json:"id"`
	Name          *string                     `json:"name,omitempty"`
	Description   *string                     `json:"description,omitempty"`
	Version       string                      `json:"version"`
	Agents        []string                    `json:"agents"`
	Skills        *map[string]SkillEntry      `json:"skills,omitempty"`
	Defaults      *SystemDefaults             `json:"defaults,omitempty"`
	Extensions    *map[string]json.RawMessage `json:"extensions,omitempty"`
}

// ArtifactManifest is the Canonical System Artifact object. Separate from Source types.
type ArtifactManifest struct {
	DigestFormat string           `json:"digest_format"`
	Files        []ArtifactMember `json:"files"`
}

// ArtifactMember is one Artifact payload member.
type ArtifactMember struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Type   string `json:"type"`
}

// SkillEntry is system.json.skills.<id>.
type SkillEntry struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Document    string        `json:"document"`
	Scripts     *[]string     `json:"scripts,omitempty"`
	Contexts    *[]string     `json:"contexts,omitempty"`
	Assets      *[]string     `json:"assets,omitempty"`
	MCPTools    *[]MCPToolRef `json:"mcp_tools,omitempty"`
}

// MCPToolRef is Skill mcp_tools[] item.
type MCPToolRef struct {
	ServerID string `json:"server_id"`
	ToolName string `json:"tool_name"`
}

// SystemDefaults is system.json.defaults.
type SystemDefaults struct {
	ContextAccess     *string `json:"context_access,omitempty"`
	CrossSystemAccess *string `json:"cross_system_access,omitempty"`
}

// ContextEntry is Agent.contexts[] item.
type ContextEntry struct {
	Path      string  `json:"path"`
	Loading   *string `json:"loading,omitempty"`
	Isolation *string `json:"isolation,omitempty"`
	Required  *bool   `json:"required,omitempty"`
}

// MCPServer is mcp_servers.<id>.
type MCPServer struct {
	Transport   string                `json:"transport"`
	Command     string                `json:"command"`
	Args        *[]string             `json:"args,omitempty"`
	Files       *[]string             `json:"files,omitempty"`
	Tools       []ToolAllowlistEntry  `json:"tools"`
	Environment *[]EnvironmentBinding `json:"environment,omitempty"`
}

// Hooks is Agent.hooks (closed event set).
type Hooks struct {
	SessionStart   *[]HookEntry `json:"session_start,omitempty"`
	BeforeToolCall *[]HookEntry `json:"before_tool_call,omitempty"`
	AfterToolCall  *[]HookEntry `json:"after_tool_call,omitempty"`
	AgentStop      *[]HookEntry `json:"agent_stop,omitempty"`
}

// HookEntry is one Hook process object.
type HookEntry struct {
	Command     string                `json:"command"`
	Args        *[]string             `json:"args,omitempty"`
	Files       *[]string             `json:"files,omitempty"`
	Required    *bool                 `json:"required,omitempty"`
	Environment *[]EnvironmentBinding `json:"environment,omitempty"`
}

// EnvironmentBinding is the V0 secret binding object.
type EnvironmentBinding struct {
	ValueFrom ValueFrom `json:"value_from"`
	Required  *bool     `json:"required,omitempty"`
}

// ValueFrom is EnvironmentBinding.value_from.
type ValueFrom struct {
	Environment string `json:"environment"`
}

// Permissions is the closed Source permissions object.
type Permissions struct {
	Filesystem *FilesystemPermissions `json:"filesystem,omitempty"`
	Network    *NetworkPermissions    `json:"network,omitempty"`
}

// FilesystemPermissions is permissions.filesystem.
type FilesystemPermissions struct {
	Read  *[]string `json:"read,omitempty"`
	Write *[]string `json:"write,omitempty"`
}

// NetworkPermissions is permissions.network.
type NetworkPermissions struct {
	Default *string `json:"default,omitempty"`
}

// Approvals is the closed Source approvals object.
type Approvals struct {
	ShellExecute *string `json:"shell_execute,omitempty"`
}

// Sandbox is the closed Source sandbox object.
type Sandbox struct {
	Required *bool   `json:"required,omitempty"`
	Mode     *string `json:"mode,omitempty"`
}

// Output is Agent.output.
type Output struct {
	Schema      string  `json:"schema"`
	Enforcement *string `json:"enforcement,omitempty"`
}
