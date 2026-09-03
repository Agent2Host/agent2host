package compatibility

import "encoding/json"

// Assessment is adapter-reported Host facts. It must not carry Evaluate fields.
type Assessment struct {
	Activation       *ActivationAssess `json:"activation,omitempty"`
	SOP              *SOPAssess        `json:"sop,omitempty"`
	Skills           []SkillAssess     `json:"skills,omitempty"`
	Contexts         []ContextAssess   `json:"contexts,omitempty"`
	ContextIsolation []IsolationAssess `json:"context_isolation,omitempty"`
	MCP              []MCPAssess       `json:"mcp,omitempty"`
	MCPToolIsolation []MCPIsoAssess    `json:"mcp_tool_isolation,omitempty"`
	Hooks            []HookAssess      `json:"hooks,omitempty"`
	OutputSchema     *OrdinaryAssess   `json:"output_schema,omitempty"`
	SecretIsolation  []SecretAssess    `json:"secret_isolation,omitempty"`
	Security         *SecurityAssess   `json:"security,omitempty"`
}

type ActivationAssess struct {
	Mode       string `json:"mode"`
	Confidence string `json:"confidence"`
}

type OrdinaryAssess struct {
	Support    string `json:"support"`
	Scope      string `json:"scope,omitempty"`
	Confidence string `json:"confidence"`
}

type SOPAssess struct {
	Support            string          `json:"support"`
	Scope              string          `json:"scope,omitempty"`
	Confidence         string          `json:"confidence"`
	AppliesFromTurnOne json.RawMessage `json:"applies_from_turn_one,omitempty"`
}

type SkillAssess struct {
	ID         string `json:"id"`
	Support    string `json:"support"`
	Scope      string `json:"scope,omitempty"`
	Confidence string `json:"confidence"`
}

type ContextAssess struct {
	Path       string `json:"path"`
	Required   *bool  `json:"required,omitempty"`
	Loading    string `json:"loading,omitempty"`
	Isolation  string `json:"isolation,omitempty"`
	Support    string `json:"support"`
	Scope      string `json:"scope,omitempty"`
	Confidence string `json:"confidence"`
}

type IsolationAssess struct {
	Path        string `json:"path"`
	Required    bool   `json:"required"`
	Support     string `json:"support"`
	Scope       string `json:"scope"`
	Enforcement string `json:"enforcement"`
	Confidence  string `json:"confidence"`
}

type MCPAssess struct {
	ServerID        string `json:"server_id"`
	Name            string `json:"name"`
	Required        *bool  `json:"required,omitempty"`
	Support         string `json:"support"`
	Scope           string `json:"scope,omitempty"`
	Confidence      string `json:"confidence"`
	Invocable       *bool  `json:"invocable,omitempty"`
	ServerConnected *bool  `json:"server_connected,omitempty"`
}

type MCPIsoAssess struct {
	ServerID    string `json:"server_id"`
	Support     string `json:"support"`
	Scope       string `json:"scope"`
	Enforcement string `json:"enforcement"`
	Confidence  string `json:"confidence"`
}

type HookAssess struct {
	Ref        string `json:"ref"`
	Required   *bool  `json:"required,omitempty"`
	Support    string `json:"support"`
	Scope      string `json:"scope,omitempty"`
	Confidence string `json:"confidence"`
}

type SecretAssess struct {
	Consumer     string `json:"consumer"`
	Target       string `json:"target"`
	ConsumerKind string `json:"consumer_kind,omitempty"`
	ServerID     string `json:"server_id,omitempty"`
	Required     bool   `json:"required"`
	Support      string `json:"support"`
	Scope        string `json:"scope"`
	Enforcement  string `json:"enforcement"`
	Confidence   string `json:"confidence"`
}

type PolicyAssess struct {
	Support               string `json:"support"`
	Scope                 string `json:"scope"`
	Enforcement           string `json:"enforcement"`
	Confidence            string `json:"confidence"`
	GrantSubseteqDeclared *bool  `json:"grant_subseteq_declared,omitempty"`
	GateVsDeclared        string `json:"gate_vs_declared,omitempty"`
	ModeVsDeclared        string `json:"mode_vs_declared,omitempty"`
}

type SecurityAssess struct {
	Permissions      *PolicyAssess `json:"permissions,omitempty"`
	Approvals        *PolicyAssess `json:"approvals,omitempty"`
	Sandbox          *PolicyAssess `json:"sandbox,omitempty"`
	OutputValidation *PolicyAssess `json:"output_validation,omitempty"`
}
