// Package hoststate is how Agent2Host points a Host at a stable home so the
// Host can reuse its own login. It is not an Agent2Host login, status, or
// logout API. Type names still say Auth because the on-disk directory is
// auth-profiles/; do not add login methods here.
package hoststate

import (
	"fmt"
	"strings"
	"unicode"
)

// Topology: how this Host finds the same native state on the next run.
// Runtime must not infer these from host_id.
const (
	AuthTopologySeparated  = "separated"   // native auth material independent of this-run config
	AuthTopologyBoundRoot  = "bound_root"  // native auth namespace is the Host config root
	AuthTopologyExternal   = "external"    // browser / SSO outside Agent2Host
	AuthTopologyExecSecret = "exec_secret" // API key injected only at Execute
)

const (
	AuthConcurrencySafe       = "safe"
	AuthConcurrencySerialize  = "serialize"
	AuthConcurrencyUnverified = "unverified"
)

const (
	AuthRootProfile AuthRoot = "profile"
	AuthRootPrivate AuthRoot = "private"
)

// AuthRoot is where an opaque Host-owned blob is placed for this run.
type AuthRoot string

// AuthProfileToken is replaced by Runtime with the stable Host-state directory.
const AuthProfileToken = "$A2H_AUTH_PROFILE"

// AuthProfilesDirName is the durable Host-state root under Agent2Host home.
// The on-disk name is historical; the directory is not a user-facing login product.
const AuthProfilesDirName = "auth-profiles"

// AuthProfileKey identifies one stable Host-state location. It is not only
// host_id: provider and native namespace distinguish multi-provider Hosts and
// Claude-style config-dir Keychain spaces.
type AuthProfileKey struct {
	Host                string
	Provider            string
	AccountOrWorkspace  string
	NativeAuthNamespace string
}

// DirName is the path-safe folder under auth-profiles/<host>/.
func (k AuthProfileKey) DirName() string {
	parts := []string{authPathPart(k.Provider), authPathPart(k.NativeAuthNamespace)}
	if strings.TrimSpace(k.AccountOrWorkspace) != "" {
		parts = append(parts, authPathPart(k.AccountOrWorkspace))
	}
	return strings.Join(parts, "--")
}

func authPathPart(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "default"
	}
	return b.String()
}

// AuthDescription is the adapter's declaration of Host-state topology.
// Runtime copies only declared opaque blobs and applies declared env; it does
// not parse credentials or decide whether the Host is logged in.
type AuthDescription struct {
	Profile        AuthProfileKey
	Topology       string
	Concurrency    string
	Materials      []AuthMaterial
	ExecSecretEnvs []string // informational; delivery stays on LaunchPlan.Secrets
}

// AuthMaterial is one opaque Host-owned blob the Runtime may move.
type AuthMaterial struct {
	StoreRel string
	BindRoot AuthRoot
	BindRel  string
	Lock     bool
}

// AuthBindRequest is what Runtime knows when pointing a run at stable Host state.
type AuthBindRequest struct {
	ProfileDir string
	PrivateDir string
}

// AuthBindDirective tells Runtime which opaque copies, env, and extra args
// to apply so the Host finds the same native state location.
type AuthBindDirective struct {
	Copies []AuthMaterial
	Env    map[string]string
	Args   []string
}

// AuthFinalizeRequest is the post-Host view of the same bind roots.
type AuthFinalizeRequest struct {
	ProfileDir string
	PrivateDir string
}

// AuthFinalizeDirective is which opaque blobs the Host itself updated and
// which may be kept for the next run.
type AuthFinalizeDirective struct {
	Copies []AuthMaterial
}

// Binder is the Host-specific half of "put this process back in the same
// native home." It is not a login, status, or logout API.
type Binder interface {
	DescribeAuth() AuthDescription
	BindForRun(AuthBindRequest) (AuthBindDirective, error)
	FinalizeRun(AuthFinalizeRequest) (AuthFinalizeDirective, error)
}

// Validate rejects empty identity or unknown topology values.
func Validate(d AuthDescription) error {
	if d.Profile.Host == "" {
		return fmt.Errorf("adapter: host-state profile missing host")
	}
	switch d.Topology {
	case AuthTopologySeparated, AuthTopologyBoundRoot, AuthTopologyExternal, AuthTopologyExecSecret:
	default:
		return fmt.Errorf("adapter: unknown host-state topology %q", d.Topology)
	}
	switch d.Concurrency {
	case AuthConcurrencySafe, AuthConcurrencySerialize, AuthConcurrencyUnverified:
	default:
		return fmt.Errorf("adapter: unknown host-state concurrency %q", d.Concurrency)
	}
	for _, m := range d.Materials {
		if m.StoreRel == "" || m.BindRel == "" {
			return fmt.Errorf("adapter: host-state material missing store or bind path")
		}
		if m.BindRoot != AuthRootProfile && m.BindRoot != AuthRootPrivate {
			return fmt.Errorf("adapter: host-state material bind root %q", m.BindRoot)
		}
	}
	return nil
}

// Noop is for fixtures and Hosts that declare no Agent2Host-owned materials.
type Noop struct {
	Desc AuthDescription
}

func (n Noop) DescribeAuth() AuthDescription { return n.Desc }

func (n Noop) BindForRun(AuthBindRequest) (AuthBindDirective, error) {
	return AuthBindDirective{Copies: n.Desc.Materials}, nil
}

func (n Noop) FinalizeRun(AuthFinalizeRequest) (AuthFinalizeDirective, error) {
	return AuthFinalizeDirective{Copies: n.Desc.Materials}, nil
}
