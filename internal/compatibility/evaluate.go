package compatibility

import (
	"bytes"
	"encoding/json"
	"strings"
)

const (
	schemaVersion = "agent2host/compatibility-report/v1alpha1"

	resultSatisfied   = "satisfied"
	resultDegraded    = "degraded"
	resultUnsatisfied = "unsatisfied"
	resultUnknown     = "unknown"

	dispInclude = "include"
	dispOmit    = "omit"

	decisionAllowed  = "allowed"
	decisionWarnings = "allowed_with_warnings"
	decisionRefused  = "refused"
)

// Evaluate applies the frozen Compatibility algebra. It writes only Evaluate
// fields; Assess facts are copied onto the Report unchanged.
func Evaluate(env Envelope, req Requirement, assess Assessment) Report {
	r := defaultReport(env)

	if assess.Activation != nil {
		r.Activation = evalActivation(*assess.Activation)
	}
	if assess.SOP != nil {
		r.Capabilities.SOP = evalSOP(*assess.SOP)
	}
	if len(req.Skills) > 0 || len(assess.Skills) > 0 {
		r.Capabilities.Skills = evalSkills(req.Skills, assess.Skills)
	}
	if len(req.Contexts) > 0 || len(assess.Contexts) > 0 {
		r.Capabilities.Context = evalContexts(req.Contexts, assess.Contexts)
	}
	if len(req.MCP) > 0 || len(assess.MCP) > 0 {
		r.Capabilities.MCP = evalMCP(req.MCP, assess.MCP)
	}
	if len(req.Hooks) > 0 || len(assess.Hooks) > 0 {
		r.Capabilities.Hooks = evalHooks(req.Hooks, assess.Hooks)
	}
	if assess.OutputSchema != nil {
		r.Capabilities.OutputSchema = evalOrdinary(*assess.OutputSchema)
	}
	if assess.Security != nil {
		if assess.Security.Permissions != nil {
			r.Security.Permissions = evalPolicy("permissions", true, *assess.Security.Permissions)
		}
		if assess.Security.Approvals != nil {
			r.Security.Approvals = evalPolicy("approvals", true, *assess.Security.Approvals)
		}
		if assess.Security.Sandbox != nil {
			r.Security.Sandbox = evalPolicy("sandbox", req.sandboxRequired(), *assess.Security.Sandbox)
		}
		if assess.Security.OutputValidation != nil {
			r.Security.OutputValidation = evalPolicy("output_validation", req.outputValidationRequired(), *assess.Security.OutputValidation)
		}
	}
	if len(assess.ContextIsolation) > 0 {
		r.Security.ContextIsolation.Items = evalContextIsolation(assess.ContextIsolation)
		omitIsolatedContexts(&r, r.Security.ContextIsolation.Items)
	}
	if len(assess.MCPToolIsolation) > 0 {
		r.Security.MCPToolIsolation.Items = evalMCPIsolation(assess.MCPToolIsolation)
	}
	if len(assess.SecretIsolation) > 0 || len(req.Secrets) > 0 {
		r.Security.SecretIsolation.Items = evalSecrets(req.Secrets, assess.SecretIsolation)
	}

	r.Decision = aggregate(r, req)
	return r
}

func defaultReport(env Envelope) Report {
	if env.SchemaVersion == "" {
		env.SchemaVersion = schemaVersion
	}
	sat := OrdinaryRow{
		Support: "native", Scope: "agent", Confidence: "documented",
		RequirementResult: resultSatisfied, Disposition: dispInclude,
	}
	// Missing Assess facts must not look enforced. Permissions/approvals
	// are hard gates: unknown here refuses in aggregate.
	unk := PolicyRow{
		Support: "unknown", Scope: "unknown", Enforcement: "unknown", Confidence: "unknown",
		RequirementResult: resultUnknown, ReasonCode: "insufficient_evidence",
	}
	empty := CollectionSummary{DecisionImpact: "pass"}
	return Report{
		SchemaVersion:     env.SchemaVersion,
		Agent2HostVersion: env.Agent2HostVersion,
		Subject:           env.Subject,
		Host:              env.Host,
		Adapter:           env.Adapter,
		Probe:             env.Probe,
		Decision:          decisionAllowed,
		Activation: Activation{
			Mode: "primary_native", Confidence: "documented", RequirementResult: resultSatisfied,
		},
		Capabilities: Capabilities{
			SOP:          sat,
			Skills:       SkillCollection{Items: []SkillItem{}, Summary: empty},
			Context:      ContextCollection{Items: []ContextItem{}, Summary: empty},
			MCP:          MCPCollection{Items: []MCPItem{}, Summary: empty},
			Hooks:        HookCollection{Items: []HookItem{}, Summary: empty},
			OutputSchema: sat,
		},
		Security: Security{
			Permissions:      unk,
			Approvals:        unk,
			Sandbox:          unk,
			ContextIsolation: IsolationCollection{Items: []ContextIsolationItem{}},
			MCPToolIsolation: MCPIsolationCollection{Items: []MCPIsolationItem{}},
			OutputValidation: unk,
			SecretIsolation:  SecretIsolationCollection{Items: []SecretIsolationItem{}},
		},
	}
}

func evalActivation(a ActivationAssess) Activation {
	out := Activation{Mode: a.Mode, Confidence: a.Confidence}
	switch a.Mode {
	case "primary_native":
		out.RequirementResult = resultSatisfied
	case "primary_mapped":
		out.RequirementResult = resultDegraded
		out.ReasonCode = "mapped_activation"
	case "subagent_only", "unsupported":
		out.RequirementResult = resultUnsatisfied
		out.ReasonCode = "activation_not_primary"
	default:
		out.RequirementResult = resultUnknown
		out.ReasonCode = "activation_not_primary"
		if out.Mode == "" {
			out.Mode = "unknown"
		}
		if out.Confidence == "" {
			out.Confidence = "unknown"
		}
	}
	return out
}

func evalSOP(a SOPAssess) OrdinaryRow {
	scope := a.Scope
	if scope == "" {
		scope = "agent"
	}
	row := OrdinaryRow{Support: a.Support, Scope: scope, Confidence: a.Confidence}
	known, turnOne, unknown := parseTernary(a.AppliesFromTurnOne)
	if unknown || (known && !turnOne && a.Support != "unknown") {
		if unknown {
			row.RequirementResult = resultUnknown
			row.Disposition = dispOmit
			row.ReasonCode = "insufficient_evidence"
			return row
		}
		row.RequirementResult = resultUnsatisfied
		row.Disposition = dispOmit
		row.ReasonCode = "sop_not_primary"
		return row
	}
	if a.Support == "unknown" || a.Confidence == "unknown" {
		row.RequirementResult = resultUnknown
		row.Disposition = dispOmit
		row.ReasonCode = "insufficient_evidence"
		return row
	}
	return evalOrdinary(OrdinaryAssess{Support: a.Support, Scope: scope, Confidence: a.Confidence})
}

func defaultConfidence(support, confidence string) string {
	if confidence != "" {
		return confidence
	}
	if support == "unknown" || support == "" {
		return "unknown"
	}
	return "documented"
}

func evalOrdinary(a OrdinaryAssess) OrdinaryRow {
	a.Confidence = defaultConfidence(a.Support, a.Confidence)
	scope := a.Scope
	if scope == "" {
		if a.Support == "unknown" {
			scope = "unknown"
		} else {
			scope = "agent"
		}
	}
	row := OrdinaryRow{Support: a.Support, Scope: scope, Confidence: a.Confidence}
	rr, disp, code := projectable(a.Support, a.Confidence, "", false)
	row.RequirementResult = rr
	row.Disposition = disp
	row.ReasonCode = code
	return row
}

func projectable(support, confidence, specificFail string, specific bool) (rr, disp, code string) {
	if confidence == "unknown" || support == "unknown" || support == "" {
		return resultUnknown, dispOmit, "insufficient_evidence"
	}
	if specific {
		return resultUnsatisfied, dispOmit, specificFail
	}
	if support == "unsupported" {
		return resultUnsatisfied, dispOmit, "acceptance_failed"
	}
	rr, disp, code = resultSatisfied, dispInclude, ""
	if support == "approximate" {
		rr, code = resultDegraded, "visible_loss"
	}
	if confidence == "inferred" {
		rr, code = resultDegraded, "confidence_inferred"
	}
	return
}

func evalSkills(reqs []SkillReq, facts []SkillAssess) SkillCollection {
	byID := map[string]SkillAssess{}
	for _, f := range facts {
		byID[f.ID] = f
	}
	reqByID := map[string]SkillReq{}
	order := make([]string, 0, len(reqs))
	for _, r := range reqs {
		reqByID[r.ID] = r
		order = append(order, r.ID)
	}
	for _, f := range facts {
		if _, ok := reqByID[f.ID]; !ok {
			order = append(order, f.ID)
		}
	}
	items := make([]SkillItem, 0, len(order))
	for _, id := range order {
		r := reqByID[id]
		if r.ID == "" {
			r.ID = id
			r.Required = true
		}
		f, ok := byID[id]
		it := SkillItem{ID: id, Required: r.Required}
		if !ok {
			it.Support, it.Scope, it.Confidence = "unknown", "unknown", "unknown"
			it.RequirementResult, it.Disposition, it.ReasonCode = resultUnknown, dispOmit, "insufficient_evidence"
			items = append(items, it)
			continue
		}
		ord := evalOrdinary(OrdinaryAssess{Support: f.Support, Scope: f.Scope, Confidence: f.Confidence})
		it.Support, it.Scope, it.Confidence = ord.Support, ord.Scope, ord.Confidence
		it.RequirementResult, it.Disposition, it.ReasonCode = ord.RequirementResult, ord.Disposition, ord.ReasonCode
		items = append(items, it)
	}
	omitted := map[string]bool{}
	for _, it := range items {
		if it.Disposition == dispOmit {
			omitted[it.ID] = true
		}
	}
	for i, it := range items {
		r := reqByID[it.ID]
		if r.ExclusiveOf != "" && omitted[r.ExclusiveOf] {
			items[i].RequirementResult = resultUnsatisfied
			items[i].Disposition = dispOmit
			items[i].ReasonCode = "parent_omitted"
		}
	}
	return SkillCollection{Items: items, Summary: summarizeSkills(items)}
}

func evalContexts(reqs []ContextReq, facts []ContextAssess) ContextCollection {
	byPath := map[string]ContextAssess{}
	for _, f := range facts {
		byPath[f.Path] = f
	}
	reqBy := map[string]ContextReq{}
	order := make([]string, 0, len(reqs))
	for _, r := range reqs {
		reqBy[r.Path] = r
		order = append(order, r.Path)
	}
	for _, f := range facts {
		if _, ok := reqBy[f.Path]; !ok {
			order = append(order, f.Path)
		}
	}
	items := make([]ContextItem, 0, len(order))
	for _, path := range order {
		r := reqBy[path]
		f := byPath[path]
		required := r.Required
		if r.Path == "" {
			if f.Required != nil {
				required = *f.Required
			} else {
				required = true
			}
		}
		loading := r.Loading
		if loading == "" {
			loading = f.Loading
		}
		if loading == "" {
			loading = "on_demand"
		}
		iso := r.Isolation
		if iso == "" {
			iso = f.Isolation
		}
		if iso == "" {
			iso = "required"
		}
		ord := evalOrdinary(OrdinaryAssess{Support: f.Support, Scope: f.Scope, Confidence: f.Confidence})
		items = append(items, ContextItem{
			Path: path, Required: required, Loading: loading, Isolation: iso,
			Support: ord.Support, Scope: ord.Scope, Confidence: ord.Confidence,
			RequirementResult: ord.RequirementResult, Disposition: ord.Disposition, ReasonCode: ord.ReasonCode,
		})
	}
	return ContextCollection{Items: items, Summary: summarizeContexts(items)}
}

func evalMCP(reqs []MCPReq, facts []MCPAssess) MCPCollection {
	type key struct{ s, n string }
	by := map[key]MCPAssess{}
	for _, f := range facts {
		by[key{f.ServerID, f.Name}] = f
	}
	reqBy := map[key]MCPReq{}
	order := make([]key, 0, len(reqs))
	for _, r := range reqs {
		k := key{r.ServerID, r.Name}
		reqBy[k] = r
		order = append(order, k)
	}
	for _, f := range facts {
		k := key{f.ServerID, f.Name}
		if _, ok := reqBy[k]; !ok {
			order = append(order, k)
		}
	}
	items := make([]MCPItem, 0, len(order))
	for _, k := range order {
		r := reqBy[k]
		f := by[k]
		required := r.Required
		if r.ServerID == "" {
			if f.Required != nil {
				required = *f.Required
			} else {
				required = true
			}
		}
		specific, code := false, ""
		if f.ServerConnected != nil && !*f.ServerConnected {
			specific, code = true, "server_not_connected"
		} else if f.Invocable != nil && !*f.Invocable {
			specific, code = true, "acceptance_failed"
		}
		rr, disp, reason := projectable(f.Support, f.Confidence, code, specific)
		scope := f.Scope
		if scope == "" {
			scope = "agent"
		}
		items = append(items, MCPItem{
			ServerID: k.s, Name: k.n, Required: required,
			Support: f.Support, Scope: scope, Confidence: f.Confidence,
			RequirementResult: rr, Disposition: disp, ReasonCode: reason,
		})
	}
	return MCPCollection{Items: items, Summary: summarizeMCP(items)}
}

func evalHooks(reqs []HookReq, facts []HookAssess) HookCollection {
	by := map[string]HookAssess{}
	for _, f := range facts {
		by[f.Ref] = f
	}
	reqBy := map[string]HookReq{}
	order := make([]string, 0, len(reqs))
	for _, r := range reqs {
		reqBy[r.Ref] = r
		order = append(order, r.Ref)
	}
	for _, f := range facts {
		if _, ok := reqBy[f.Ref]; !ok {
			order = append(order, f.Ref)
		}
	}
	items := make([]HookItem, 0, len(order))
	for _, ref := range order {
		r := reqBy[ref]
		f := by[ref]
		required := r.Required
		if r.Ref == "" {
			if f.Required != nil {
				required = *f.Required
			} else {
				required = true
			}
		}
		ord := evalOrdinary(OrdinaryAssess{Support: f.Support, Scope: f.Scope, Confidence: f.Confidence})
		items = append(items, HookItem{
			Ref: ref, Required: required,
			Support: ord.Support, Scope: ord.Scope, Confidence: ord.Confidence,
			RequirementResult: ord.RequirementResult, Disposition: ord.Disposition, ReasonCode: ord.ReasonCode,
		})
	}
	return HookCollection{Items: items, Summary: summarizeHooks(items)}
}

func evalPolicy(kind string, required bool, a PolicyAssess) PolicyRow {
	row := PolicyRow{
		Support: a.Support, Scope: a.Scope, Enforcement: a.Enforcement, Confidence: a.Confidence,
	}
	if policyFactMissing(a) {
		row.Support = emptyAsUnknown(row.Support)
		row.Scope = emptyAsUnknown(row.Scope)
		row.Enforcement = emptyAsUnknown(row.Enforcement)
		row.Confidence = emptyAsUnknown(row.Confidence)
		row.RequirementResult = resultUnknown
		row.ReasonCode = "insufficient_evidence"
		return row
	}
	if (a.Enforcement == "unknown" || a.Enforcement == "") &&
		(kind == "permissions" || kind == "approvals" || (kind == "sandbox" && required)) {
		row.RequirementResult = resultUnknown
		row.ReasonCode = "insufficient_evidence"
		return row
	}
	switch kind {
	case "permissions":
		if a.GrantSubseteqDeclared != nil && !*a.GrantSubseteqDeclared {
			row.RequirementResult = resultUnsatisfied
			row.ReasonCode = "permission_overgrant"
			return row
		}
	case "approvals":
		if a.GateVsDeclared == "stricter" {
			row.RequirementResult = resultDegraded
			row.ReasonCode = "approval_stricter"
			return row
		}
		if a.GateVsDeclared == "weaker" || a.Enforcement == "prompt_only" || a.Enforcement == "none" {
			row.RequirementResult = resultUnsatisfied
			row.ReasonCode = "approval_weaker"
			return row
		}
	case "sandbox":
		if !required {
			row.RequirementResult = resultSatisfied
			return row
		}
		if a.Enforcement == "unknown" || a.Enforcement == "" {
			row.RequirementResult = resultUnknown
			row.ReasonCode = "insufficient_evidence"
			return row
		}
		if a.Enforcement == "prompt_only" || a.Enforcement == "none" || a.ModeVsDeclared == "looser" {
			row.RequirementResult = resultUnsatisfied
			row.ReasonCode = "acceptance_failed"
			return row
		}
		if a.ModeVsDeclared == "stricter" {
			row.RequirementResult = resultDegraded
			row.ReasonCode = "sandbox_stricter"
			return row
		}
	case "output_validation":
		if !required {
			row.RequirementResult = resultSatisfied
			return row
		}
		if a.Enforcement != "host_enforced" && a.Enforcement != "agent2host_enforced" {
			row.RequirementResult = resultUnsatisfied
			row.ReasonCode = "acceptance_failed"
			return row
		}
	}
	if required && a.Confidence == "inferred" {
		row.RequirementResult = resultUnsatisfied
		row.ReasonCode = "insufficient_evidence"
		return row
	}
	row.RequirementResult = resultSatisfied
	return row
}

func evalContextIsolation(facts []IsolationAssess) []ContextIsolationItem {
	items := make([]ContextIsolationItem, 0, len(facts))
	for _, f := range facts {
		it := ContextIsolationItem{
			Path: f.Path, Required: f.Required, Support: f.Support,
			Scope: f.Scope, Enforcement: f.Enforcement, Confidence: f.Confidence,
		}
		if tooBroad(f.Scope) {
			it.RequirementResult = resultUnsatisfied
			it.ReasonCode = "isolation_too_broad"
		} else {
			it.RequirementResult = resultSatisfied
		}
		items = append(items, it)
	}
	return items
}

func evalMCPIsolation(facts []MCPIsoAssess) []MCPIsolationItem {
	items := make([]MCPIsolationItem, 0, len(facts))
	for _, f := range facts {
		it := MCPIsolationItem{
			ServerID: f.ServerID, Support: f.Support, Scope: f.Scope,
			Enforcement: f.Enforcement, Confidence: f.Confidence,
		}
		if tooBroad(f.Scope) {
			it.RequirementResult = resultUnsatisfied
			it.ReasonCode = "isolation_too_broad"
		} else if f.Enforcement == "unknown" || f.Enforcement == "" {
			it.RequirementResult = resultUnknown
			it.ReasonCode = "insufficient_evidence"
		} else if f.Enforcement == "none" || f.Enforcement == "prompt_only" {
			it.RequirementResult = resultUnsatisfied
			it.ReasonCode = "acceptance_failed"
		} else {
			it.RequirementResult = resultSatisfied
		}
		items = append(items, it)
	}
	return items
}

func evalSecrets(reqs []SecretReq, facts []SecretAssess) []SecretIsolationItem {
	type key struct{ c, t string }
	by := map[key]SecretAssess{}
	for _, f := range facts {
		by[key{f.Consumer, f.Target}] = f
	}
	order := make([]key, 0, len(facts))
	seen := map[key]bool{}
	for _, r := range reqs {
		k := key{r.Consumer, r.Target}
		if !seen[k] {
			order = append(order, k)
			seen[k] = true
		}
	}
	for _, f := range facts {
		k := key{f.Consumer, f.Target}
		if !seen[k] {
			order = append(order, k)
			seen[k] = true
		}
	}
	items := make([]SecretIsolationItem, 0, len(order))
	reqBy := map[key]SecretReq{}
	for _, r := range reqs {
		reqBy[key{r.Consumer, r.Target}] = r
	}
	for _, k := range order {
		f, ok := by[k]
		r := reqBy[k]
		required := r.Required
		if r.Consumer == "" {
			required = f.Required
		}
		it := SecretIsolationItem{
			Consumer: k.c, Target: k.t, Required: required,
		}
		if ok {
			it.ConsumerKind = f.ConsumerKind
			it.ServerID = f.ServerID
			it.Support, it.Scope, it.Enforcement, it.Confidence = f.Support, f.Scope, f.Enforcement, f.Confidence
			if tooBroad(f.Scope) {
				it.RequirementResult = resultUnsatisfied
				it.ReasonCode = "secret_scope_too_broad"
			} else {
				it.RequirementResult = resultSatisfied
			}
		} else {
			it.Support, it.Scope, it.Enforcement, it.Confidence = "unknown", "unknown", "unknown", "unknown"
			it.RequirementResult, it.ReasonCode = resultUnknown, "insufficient_evidence"
		}
		items = append(items, it)
	}
	return items
}

func omitIsolatedContexts(r *Report, isol []ContextIsolationItem) {
	fail := map[string]bool{}
	for _, it := range isol {
		if it.RequirementResult != resultSatisfied {
			fail[it.Path] = true
		}
	}
	if len(fail) == 0 {
		return
	}
	changed := false
	for i, it := range r.Capabilities.Context.Items {
		if fail[it.Path] {
			r.Capabilities.Context.Items[i].RequirementResult = resultUnsatisfied
			r.Capabilities.Context.Items[i].Disposition = dispOmit
			r.Capabilities.Context.Items[i].ReasonCode = "isolation_too_broad"
			changed = true
		}
	}
	if changed {
		r.Capabilities.Context.Summary = summarizeContexts(r.Capabilities.Context.Items)
	}
}

func policyFactMissing(a PolicyAssess) bool {
	return a.Support == "unknown" || a.Support == "" ||
		a.Confidence == "unknown" || a.Confidence == ""
}

func emptyAsUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func tooBroad(scope string) bool {
	return scope != "" && scope != "agent"
}

func parseTernary(raw json.RawMessage) (known, value, unknown bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return false, false, false
	}
	s := strings.TrimSpace(string(raw))
	if s == "null" || s == `"unknown"` {
		return true, false, true
	}
	if s == "true" {
		return true, true, false
	}
	if s == "false" {
		return true, false, false
	}
	return false, false, false
}

func summarizeSkills(items []SkillItem) CollectionSummary {
	s := CollectionSummary{DecisionImpact: "pass"}
	var reqFail, soft bool
	for _, it := range items {
		tally(&s, it.RequirementResult, it.Disposition, it.Required, &reqFail, &soft)
	}
	s.DecisionImpact = impact(reqFail, soft)
	return s
}

func summarizeContexts(items []ContextItem) CollectionSummary {
	s := CollectionSummary{DecisionImpact: "pass"}
	var reqFail, soft bool
	for _, it := range items {
		tally(&s, it.RequirementResult, it.Disposition, it.Required, &reqFail, &soft)
	}
	s.DecisionImpact = impact(reqFail, soft)
	return s
}

func summarizeMCP(items []MCPItem) CollectionSummary {
	s := CollectionSummary{DecisionImpact: "pass"}
	var reqFail, soft bool
	for _, it := range items {
		tally(&s, it.RequirementResult, it.Disposition, it.Required, &reqFail, &soft)
	}
	s.DecisionImpact = impact(reqFail, soft)
	return s
}

func summarizeHooks(items []HookItem) CollectionSummary {
	s := CollectionSummary{DecisionImpact: "pass"}
	var reqFail, soft bool
	for _, it := range items {
		tally(&s, it.RequirementResult, it.Disposition, it.Required, &reqFail, &soft)
	}
	s.DecisionImpact = impact(reqFail, soft)
	return s
}

func tally(s *CollectionSummary, rr, disp string, required bool, reqFail, soft *bool) {
	switch rr {
	case resultSatisfied:
		s.Satisfied++
	case resultDegraded:
		s.Degraded++
		*soft = true
	case resultUnsatisfied:
		s.Unsatisfied++
		if required {
			*reqFail = true
		} else {
			*soft = true
		}
	case resultUnknown:
		s.Unknown++
		if required {
			*reqFail = true
		} else {
			*soft = true
		}
	}
	if disp == dispInclude {
		s.Included++
	} else if disp == dispOmit {
		s.Omitted++
	}
}

func impact(reqFail, soft bool) string {
	if reqFail {
		return "refuse"
	}
	if soft {
		return "warning"
	}
	return "pass"
}

func aggregate(r Report, req Requirement) string {
	var refuse, warn bool
	check := func(rr string, required bool, disp string, projectable bool) {
		if required && (rr == resultUnsatisfied || rr == resultUnknown) {
			refuse = true
		}
		if rr == resultDegraded {
			warn = true
		}
		if !required && (rr == resultUnsatisfied || (projectable && (rr == resultUnknown || (disp == dispOmit && rr != resultSatisfied)))) {
			warn = true
		}
	}
	check(r.Activation.RequirementResult, true, "", false)
	check(r.Capabilities.SOP.RequirementResult, true, r.Capabilities.SOP.Disposition, true)
	check(r.Capabilities.OutputSchema.RequirementResult, req.OutputSchema != nil && req.OutputSchema.Required, r.Capabilities.OutputSchema.Disposition, true)
	for _, it := range r.Capabilities.Skills.Items {
		check(it.RequirementResult, it.Required, it.Disposition, true)
	}
	for _, it := range r.Capabilities.Context.Items {
		check(it.RequirementResult, it.Required, it.Disposition, true)
	}
	for _, it := range r.Capabilities.MCP.Items {
		check(it.RequirementResult, it.Required, it.Disposition, true)
	}
	for _, it := range r.Capabilities.Hooks.Items {
		check(it.RequirementResult, it.Required, it.Disposition, true)
	}
	check(r.Security.Permissions.RequirementResult, true, "", false)
	check(r.Security.Approvals.RequirementResult, true, "", false)
	check(r.Security.Sandbox.RequirementResult, req.sandboxRequired(), "", false)
	check(r.Security.OutputValidation.RequirementResult, req.outputValidationRequired(), "", false)
	for _, it := range r.Security.ContextIsolation.Items {
		check(it.RequirementResult, it.Required, "", false)
	}
	for _, it := range r.Security.MCPToolIsolation.Items {
		check(it.RequirementResult, true, "", false)
	}
	for _, it := range r.Security.SecretIsolation.Items {
		check(it.RequirementResult, it.Required, "", false)
	}
	if refuse {
		return decisionRefused
	}
	if warn {
		return decisionWarnings
	}
	return decisionAllowed
}
