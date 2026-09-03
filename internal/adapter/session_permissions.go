package adapter

import "github.com/agent2host/agent2host/internal/space"

// SessionEffect is what one Host does for one capability class, from that
// Host's documented knobs or a probe — not from Source defaults.
//
//	silent   Host performs the action without a qualifying ask
//	ask      documented ask-path for that class of action
//	deny     documented refusal for that class
//	unknown  this Host cannot prove the effect
type SessionEffect string

const (
	EffectSilent  SessionEffect = "silent"
	EffectAsk     SessionEffect = "ask"
	EffectDeny    SessionEffect = "deny"
	EffectUnknown SessionEffect = "unknown"
)

// SessionFacts are Host-translated effects. Adapters fill this; they do
// not decide pass/fail.
type SessionFacts struct {
	Network SessionEffect
}

// CeilingResult is the shared permission compare (ADR-0021).
type CeilingResult string

const (
	CeilingWithin     CeilingResult = "within"
	CeilingOvergrant  CeilingResult = "overgrant"
	CeilingUnverified CeilingResult = "unverified"
)

// ComparePermissions is declaration × Host effect. Host packages must not
// reimplement this.
//
// Undeclared + silent → overgrant. Undeclared + deny or ask → within.
// Undeclared + unknown → unverified (start allowed; no security claim).
// Declared + silent or ask → within. Declared + deny or unknown → overgrant
// (the requested grant is not proven usable).
func ComparePermissions(run *space.ResolvedAgentRun, facts SessionFacts) CeilingResult {
	if !FSCeilingWorkingDirectoryOnly(run) {
		return CeilingOvergrant
	}
	return compareAxis(!NetworkDenied(run), facts.Network)
}

func compareAxis(declared bool, effect SessionEffect) CeilingResult {
	if declared {
		switch effect {
		case EffectSilent, EffectAsk:
			return CeilingWithin
		default:
			return CeilingOvergrant
		}
	}
	switch effect {
	case EffectSilent:
		return CeilingOvergrant
	case EffectDeny, EffectAsk:
		return CeilingWithin
	default:
		return CeilingUnverified
	}
}

// PermissionPolicyFields maps a ceiling result onto Assess facts.
// GrantSubseteqDeclared is true unless a silent over-grant (or unusable
// requested grant) was proven. Unverified is not over-grant.
func PermissionPolicyFields(ceiling CeilingResult) (grant bool, vsDeclared string) {
	switch ceiling {
	case CeilingOvergrant:
		return false, string(CeilingOvergrant)
	case CeilingUnverified:
		return true, string(CeilingUnverified)
	default:
		return true, string(CeilingWithin)
	}
}
