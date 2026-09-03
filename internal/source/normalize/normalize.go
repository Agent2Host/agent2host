package normalize

import "github.com/agent2host/agent2host/internal/source/decode"

func strPtr(s string) *string { return &s }

func boolPtr(b bool) *bool { return &b }

func stringsPtr(ss ...string) *[]string {
	out := append([]string(nil), ss...)
	return &out
}

func emptyStrings() *[]string {
	out := []string{}
	return &out
}

// Agent applies SRC-DEFAULT-* and SRC-SEC-OMIT. It does not invent omitted extensions.
func Agent(a *decode.AgentSource) {
	if a.Skills != nil {
		for i := range *a.Skills {
			s := &(*a.Skills)[i]
			s.ExpandShort()
			if s.Required == nil {
				s.Required = boolPtr(true)
			}
		}
	}
	if a.Contexts != nil {
		for i := range *a.Contexts {
			c := &(*a.Contexts)[i]
			if c.Loading == nil {
				c.Loading = strPtr("on_demand")
			}
			if c.Required == nil {
				c.Required = boolPtr(true)
			}
		}
	}
	if a.MCPServers != nil {
		m := *a.MCPServers
		for id, srv := range m {
			fillMCP(&srv)
			m[id] = srv
		}
	}
	if a.Hooks != nil {
		fillHookList(a.Hooks.SessionStart)
		fillHookList(a.Hooks.BeforeToolCall)
		fillHookList(a.Hooks.AfterToolCall)
		fillHookList(a.Hooks.AgentStop)
	}
	fillBindings(a.Environment)
	a.Permissions = fillPermissions(a.Permissions)
	a.Approvals = fillApprovals(a.Approvals)
	a.Sandbox = fillSandbox(a.Sandbox)
	if a.Output != nil && a.Output.Enforcement == nil {
		a.Output.Enforcement = strPtr("best_effort")
	}
}

// System applies SRC-DEFAULT-SYSTEM. Omitted extensions stay omitted (SRC-DEFAULT-EXT).
// v1alpha1 systems have no work_root; EffectiveWorkRoot treats that as invocation.
func System(s *decode.SystemSource) {
	if s.Defaults == nil {
		s.Defaults = &decode.SystemDefaults{}
	}
	if s.Defaults.ContextAccess == nil {
		s.Defaults.ContextAccess = strPtr("explicit")
	}
	if s.Defaults.CrossSystemAccess == nil {
		s.Defaults.CrossSystemAccess = strPtr("deny")
	}
}

// EffectiveWorkRoot is the declared work-root mode. v1alpha1 (nil) is invocation.
func EffectiveWorkRoot(s *decode.SystemSource) decode.WorkRoot {
	if s != nil && s.WorkRoot != nil && s.WorkRoot.Mode != "" {
		return *s.WorkRoot
	}
	return decode.WorkRoot{Mode: decode.WorkRootInvocation}
}

func fillMCP(srv *decode.MCPServer) {
	if srv.Args == nil {
		srv.Args = emptyStrings()
	}
	for i := range srv.Tools {
		t := &srv.Tools[i]
		t.ExpandShort()
		if t.Required == nil {
			t.Required = boolPtr(true)
		}
	}
	fillBindings(srv.Environment)
}

func fillHookList(list *[]decode.HookEntry) {
	if list == nil {
		return
	}
	for i := range *list {
		h := &(*list)[i]
		if h.Args == nil {
			h.Args = emptyStrings()
		}
		if h.Required == nil {
			h.Required = boolPtr(true)
		}
		fillBindings(h.Environment)
	}
}

func fillBindings(list *[]decode.EnvironmentBinding) {
	if list == nil {
		return
	}
	for i := range *list {
		if (*list)[i].Required == nil {
			(*list)[i].Required = boolPtr(true)
		}
	}
}

func fillPermissions(p *decode.Permissions) *decode.Permissions {
	if p == nil {
		p = &decode.Permissions{}
	}
	if p.Filesystem == nil {
		p.Filesystem = &decode.FilesystemPermissions{}
	}
	if p.Filesystem.Read == nil {
		p.Filesystem.Read = stringsPtr("working_directory")
	}
	if p.Filesystem.Write == nil {
		p.Filesystem.Write = stringsPtr("working_directory")
	}
	if p.Network == nil {
		p.Network = &decode.NetworkPermissions{}
	}
	if p.Network.Default == nil {
		p.Network.Default = strPtr("deny")
	}
	return p
}

func fillApprovals(a *decode.Approvals) *decode.Approvals {
	if a == nil {
		a = &decode.Approvals{}
	}
	if a.ShellExecute == nil {
		a.ShellExecute = strPtr("on_boundary")
	}
	return a
}

func fillSandbox(s *decode.Sandbox) *decode.Sandbox {
	if s == nil {
		s = &decode.Sandbox{}
	}
	if s.Required == nil {
		s.Required = boolPtr(false)
	}
	if s.Mode == nil {
		s.Mode = strPtr("workspace_write")
	}
	return s
}
