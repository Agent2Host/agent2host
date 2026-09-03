package compatibility

// Report is the frozen Compatibility Report (v1alpha1). Evaluate writes
// requirement_result, disposition, reason_code, and decision only.
type Report struct {
	SchemaVersion     string       `json:"schema_version"`
	Agent2HostVersion string       `json:"agent2host_version"`
	Subject           Subject      `json:"subject"`
	Host              HostRef      `json:"host"`
	Adapter           AdapterRef   `json:"adapter"`
	Probe             Probe        `json:"probe"`
	Decision          string       `json:"decision"`
	Activation        Activation   `json:"activation"`
	Capabilities      Capabilities `json:"capabilities"`
	Security          Security     `json:"security"`
}

// Envelope is the Report identity Evaluate copies, not evaluates.
type Envelope struct {
	SchemaVersion     string
	Agent2HostVersion string
	Subject           Subject
	Host              HostRef
	Adapter           AdapterRef
	Probe             Probe
}

type Subject struct {
	SystemID string `json:"system_id"`
	AgentID  string `json:"agent_id"`
	Revision string `json:"revision"`
}

type HostRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type AdapterRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type Probe struct {
	Fingerprint string `json:"fingerprint"`
}

type Activation struct {
	Mode              string `json:"mode"`
	Confidence        string `json:"confidence"`
	RequirementResult string `json:"requirement_result"`
	ReasonCode        string `json:"reason_code,omitempty"`
}

type OrdinaryRow struct {
	Support           string `json:"support"`
	Scope             string `json:"scope"`
	Confidence        string `json:"confidence"`
	RequirementResult string `json:"requirement_result"`
	Disposition       string `json:"disposition"`
	ReasonCode        string `json:"reason_code,omitempty"`
}

type PolicyRow struct {
	Support           string `json:"support"`
	Scope             string `json:"scope"`
	Enforcement       string `json:"enforcement"`
	Confidence        string `json:"confidence"`
	RequirementResult string `json:"requirement_result"`
	ReasonCode        string `json:"reason_code,omitempty"`
}

type CollectionSummary struct {
	Satisfied      int    `json:"satisfied"`
	Degraded       int    `json:"degraded"`
	Unsatisfied    int    `json:"unsatisfied"`
	Unknown        int    `json:"unknown"`
	Included       int    `json:"included"`
	Omitted        int    `json:"omitted"`
	DecisionImpact string `json:"decision_impact"`
}

type SkillItem struct {
	ID                string `json:"id"`
	Required          bool   `json:"required"`
	Support           string `json:"support"`
	Scope             string `json:"scope"`
	Confidence        string `json:"confidence"`
	RequirementResult string `json:"requirement_result"`
	Disposition       string `json:"disposition"`
	ReasonCode        string `json:"reason_code,omitempty"`
}

type ContextItem struct {
	Path              string `json:"path"`
	Required          bool   `json:"required"`
	Loading           string `json:"loading"`
	Isolation         string `json:"isolation"`
	Support           string `json:"support"`
	Scope             string `json:"scope"`
	Confidence        string `json:"confidence"`
	RequirementResult string `json:"requirement_result"`
	Disposition       string `json:"disposition"`
	ReasonCode        string `json:"reason_code,omitempty"`
}

type MCPItem struct {
	ServerID          string `json:"server_id"`
	Name              string `json:"name"`
	Required          bool   `json:"required"`
	Support           string `json:"support"`
	Scope             string `json:"scope"`
	Confidence        string `json:"confidence"`
	RequirementResult string `json:"requirement_result"`
	Disposition       string `json:"disposition"`
	ReasonCode        string `json:"reason_code,omitempty"`
}

type HookItem struct {
	Ref               string `json:"ref"`
	Required          bool   `json:"required"`
	Support           string `json:"support"`
	Scope             string `json:"scope"`
	Confidence        string `json:"confidence"`
	RequirementResult string `json:"requirement_result"`
	Disposition       string `json:"disposition"`
	ReasonCode        string `json:"reason_code,omitempty"`
}

type ContextIsolationItem struct {
	Path              string `json:"path"`
	Required          bool   `json:"required"`
	Support           string `json:"support"`
	Scope             string `json:"scope"`
	Enforcement       string `json:"enforcement"`
	Confidence        string `json:"confidence"`
	RequirementResult string `json:"requirement_result"`
	ReasonCode        string `json:"reason_code,omitempty"`
}

type MCPIsolationItem struct {
	ServerID          string `json:"server_id"`
	Support           string `json:"support"`
	Scope             string `json:"scope"`
	Enforcement       string `json:"enforcement"`
	Confidence        string `json:"confidence"`
	RequirementResult string `json:"requirement_result"`
	ReasonCode        string `json:"reason_code,omitempty"`
}

type SecretIsolationItem struct {
	Consumer          string `json:"consumer"`
	Target            string `json:"target"`
	ConsumerKind      string `json:"consumer_kind,omitempty"`
	ServerID          string `json:"server_id,omitempty"`
	Required          bool   `json:"required"`
	Support           string `json:"support"`
	Scope             string `json:"scope"`
	Enforcement       string `json:"enforcement"`
	Confidence        string `json:"confidence"`
	RequirementResult string `json:"requirement_result"`
	ReasonCode        string `json:"reason_code,omitempty"`
}

type SkillCollection struct {
	Items   []SkillItem       `json:"items"`
	Summary CollectionSummary `json:"summary"`
}

type ContextCollection struct {
	Items   []ContextItem     `json:"items"`
	Summary CollectionSummary `json:"summary"`
}

type MCPCollection struct {
	Items   []MCPItem         `json:"items"`
	Summary CollectionSummary `json:"summary"`
}

type HookCollection struct {
	Items   []HookItem        `json:"items"`
	Summary CollectionSummary `json:"summary"`
}

type Capabilities struct {
	SOP          OrdinaryRow       `json:"sop"`
	Skills       SkillCollection   `json:"skills"`
	Context      ContextCollection `json:"context"`
	MCP          MCPCollection     `json:"mcp"`
	Hooks        HookCollection    `json:"hooks"`
	OutputSchema OrdinaryRow       `json:"output_schema"`
}

type IsolationCollection struct {
	Items []ContextIsolationItem `json:"items"`
}

type MCPIsolationCollection struct {
	Items []MCPIsolationItem `json:"items"`
}

type SecretIsolationCollection struct {
	Items []SecretIsolationItem `json:"items"`
}

type Security struct {
	Permissions      PolicyRow                 `json:"permissions"`
	Approvals        PolicyRow                 `json:"approvals"`
	Sandbox          PolicyRow                 `json:"sandbox"`
	ContextIsolation IsolationCollection       `json:"context_isolation"`
	MCPToolIsolation MCPIsolationCollection    `json:"mcp_tool_isolation"`
	OutputValidation PolicyRow                 `json:"output_validation"`
	SecretIsolation  SecretIsolationCollection `json:"secret_isolation"`
}
