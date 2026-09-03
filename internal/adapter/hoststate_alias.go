package adapter

import "github.com/agent2host/agent2host/internal/adapter/hoststate"

// Host-state types live in package hoststate. Aliases keep existing
// adapter.Auth* names so Host packages and Runtime do not churn.

const (
	AuthTopologySeparated     = hoststate.AuthTopologySeparated
	AuthTopologyBoundRoot     = hoststate.AuthTopologyBoundRoot
	AuthTopologyExternal      = hoststate.AuthTopologyExternal
	AuthTopologyExecSecret    = hoststate.AuthTopologyExecSecret
	AuthConcurrencySafe       = hoststate.AuthConcurrencySafe
	AuthConcurrencySerialize  = hoststate.AuthConcurrencySerialize
	AuthConcurrencyUnverified = hoststate.AuthConcurrencyUnverified
	AuthProfileToken          = hoststate.AuthProfileToken
	AuthProfilesDirName       = hoststate.AuthProfilesDirName
	AuthRootProfile           = hoststate.AuthRootProfile
	AuthRootPrivate           = hoststate.AuthRootPrivate
)

type (
	AuthRoot              = hoststate.AuthRoot
	AuthProfileKey        = hoststate.AuthProfileKey
	AuthDescription       = hoststate.AuthDescription
	AuthMaterial          = hoststate.AuthMaterial
	AuthBindRequest       = hoststate.AuthBindRequest
	AuthBindDirective     = hoststate.AuthBindDirective
	AuthFinalizeRequest   = hoststate.AuthFinalizeRequest
	AuthFinalizeDirective = hoststate.AuthFinalizeDirective
	HostStateBinder       = hoststate.Binder
	NoopHostState         = hoststate.Noop
)

// ValidateAuthDescription rejects empty identity or unknown topology values.
func ValidateAuthDescription(d AuthDescription) error {
	return hoststate.Validate(d)
}
