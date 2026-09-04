package adapter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/agent2host/agent2host/internal/compatibility"
)

// Control kinds carried by ControlIntent.
const (
	ControlSandbox     = "sandbox"
	ControlPermissions = "permissions"
	ControlApprovals   = "approvals"
	ControlIsolation   = "isolation"
	ControlMCP         = "mcp"
	ControlHook        = "hook"
)

const (
	ViaProjectionFile = "projection_file"
	ViaLaunchArg      = "launch_arg"
	ViaNamedConfig    = "named_config"
)

// SecretPlaceholderPrefix marks a late-resolve slot in projected Host config.
// Plans and cacheable projection bytes carry the placeholder, never the value.
const SecretPlaceholderPrefix = "__A2H_SECRET__:"

// WorkspaceToken is replaced by Runtime with the run workspace path at Execute.
// This is the model-readable projection surface (skills, MCP scripts, contexts).
const WorkspaceToken = "$A2H_WORKSPACE"

// PrivateToken is replaced by Runtime with the run-private Host home path.
// Secret-bearing Host config lives here; Adapters must not --add-dir it.
const PrivateToken = "$A2H_PRIVATE"

// HomeToken is replaced by Runtime with the Agent2Host home (--home / A2H_HOME).
const HomeToken = "$A2H_HOME"

// HostAuthDirName is the legacy durable Host dir under HomeToken.
const HostAuthDirName = "host-auth"

// HostAuthDir is the stable Host config directory for this committed Host.
func HostAuthDir(hostID string) string {
	return HomeToken + "/" + HostAuthDirName + "/" + hostID
}

// ErrIntentMismatch is Project failure: ControlIntent was not realized.
var ErrIntentMismatch = errors.New("adapter: control intent not realized in plans")

// ErrIncludeOrphan is Project failure: an include has no Plan member.
var ErrIncludeOrphan = errors.New("adapter: include without plan member")

// PlannedControl is one this-run control Assess promised.
type PlannedControl struct {
	Kind       string `json:"kind"`
	ID         string `json:"id,omitempty"`
	Via        string `json:"via"`
	Rel        string `json:"rel,omitempty"`
	Derivation string `json:"derivation,omitempty"`
}

// PlannedSecret is consumer-scoped delivery Assess promised.
type PlannedSecret struct {
	Consumer    string `json:"consumer"`
	Target      string `json:"target"`
	Scope       string `json:"scope"`
	Enforcement string `json:"enforcement,omitempty"`
}

// ControlIntent is Assess-time effective policy. It is not a Report field.
type ControlIntent struct {
	Controls []PlannedControl `json:"controls,omitempty"`
	Secrets  []PlannedSecret  `json:"secrets,omitempty"`
}

// SecretPlaceholder is the projected stand-in for name.
func SecretPlaceholder(name string) string {
	return SecretPlaceholderPrefix + name
}

// Has reports whether a control of this kind is planned.
func (c ControlIntent) Has(kind string) bool {
	for _, x := range c.Controls {
		if x.Kind == kind {
			return true
		}
	}
	return false
}

// SecretScope returns the planned delivery scope for this binding.
func (c ControlIntent) SecretScope(consumer, target string) (string, bool) {
	s, ok := c.secretBinding(consumer, target)
	if !ok {
		return "", false
	}
	return s.Scope, true
}

func (c ControlIntent) secretBinding(consumer, target string) (PlannedSecret, bool) {
	for _, s := range c.Secrets {
		if s.Consumer == consumer && s.Target == target {
			return s, true
		}
	}
	return PlannedSecret{}, false
}

// MarkUnverifiedProcessEnvSecrets records that MCP/hook values delivered on
// the Host process environment are not proven isolated from that session.
func MarkUnverifiedProcessEnvSecrets(c *ControlIntent) {
	if c == nil {
		return
	}
	for i := range c.Secrets {
		if HostProcessConsumer(c.Secrets[i].Consumer) {
			continue
		}
		c.Secrets[i].Enforcement = "unknown"
	}
}

// DigestIntent is the in-process identity of this ControlIntent.
func DigestIntent(c ControlIntent) (string, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func reconcileIntent(intent ControlIntent, report compatibility.Report, np NativeProjectionPlan, lp LaunchPlan) error {
	files := map[string]bool{}
	for _, f := range np.Files {
		files[f.RelPath] = true
	}
	args := strings.Join(lp.Args, " ")
	for _, c := range intent.Controls {
		switch c.Via {
		case ViaProjectionFile:
			if c.Rel != "" && !files[c.Rel] {
				return fmt.Errorf("%w: %s missing file %s", ErrIntentMismatch, c.Kind, c.Rel)
			}
		case ViaLaunchArg:
			if c.Rel != "" && !strings.Contains(args, c.Rel) {
				return fmt.Errorf("%w: %s missing arg %s", ErrIntentMismatch, c.Kind, c.Rel)
			}
		case ViaNamedConfig:
			if c.Rel != "" {
				if _, ok := lp.Env[c.Rel]; !ok {
					return fmt.Errorf("%w: %s missing env %s", ErrIntentMismatch, c.Kind, c.Rel)
				}
			}
		}
	}
	if SOPIncluded(report) && len(np.Files) == 0 {
		return fmt.Errorf("%w: sop", ErrIncludeOrphan)
	}
	for id := range IncludedSkills(report) {
		found := false
		for rel := range files {
			if strings.Contains(rel, id) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: skill %s", ErrIncludeOrphan, id)
		}
	}
	for path := range IncludedContexts(report) {
		if !files[path] {
			return fmt.Errorf("%w: context %s", ErrIncludeOrphan, path)
		}
	}
	if len(IncludedMCPServers(report)) > 0 {
		found := false
		for _, c := range intent.Controls {
			if c.Kind == ControlMCP && c.Rel != "" && files[c.Rel] {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: mcp", ErrIncludeOrphan)
		}
	}
	if len(IncludedHooks(report)) > 0 {
		found := false
		for _, c := range intent.Controls {
			if c.Kind == ControlHook && c.Rel != "" && files[c.Rel] {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: hooks", ErrIncludeOrphan)
		}
	}
	return nil
}

func IncludedMCPServers(report compatibility.Report) map[string]bool {
	out := map[string]bool{}
	for _, it := range report.Capabilities.MCP.Items {
		if it.Disposition == "include" {
			out[it.ServerID] = true
		}
	}
	return out
}

func IncludedHooks(report compatibility.Report) map[string]bool {
	out := map[string]bool{}
	for _, it := range report.Capabilities.Hooks.Items {
		if it.Disposition == "include" {
			out[it.Ref] = true
		}
	}
	return out
}
