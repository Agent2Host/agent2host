package runtime

// Record is durable redacted run metadata. It must not contain secret values.
type Record struct {
	RunID             string   `json:"run_id"`
	SystemID          string   `json:"system_id"`
	AgentID           string   `json:"agent_id"`
	Revision          string   `json:"revision"`
	HostID            string   `json:"host_id"`
	HostVersion       string   `json:"host_version"`
	Decision          string   `json:"decision"`
	Fingerprint       string   `json:"probe_fingerprint"`
	AdapterVersion    string   `json:"adapter_version"`
	Agent2HostVersion string   `json:"agent2host_version"`
	Class             string   `json:"class"`
	ExitCode          int      `json:"exit_code"`
	Stage             string   `json:"stage,omitempty"`
	OmittedSecrets    []string `json:"omitted_secret_names,omitempty"`
	Error             string   `json:"error,omitempty"`
	RecordedAt        string   `json:"recorded_at"`
}
