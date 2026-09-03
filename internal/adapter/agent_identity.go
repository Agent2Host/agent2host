package adapter

import (
	"strings"

	"github.com/agent2host/agent2host/internal/space"
)

// namedAgentInstructionPrefix prepends identity the Host must honor in chat.
// Used by mapped-activation Hosts (Kiro prompt, Codex developer_instructions).
func namedAgentInstructionPrefix(run *space.ResolvedAgentRun) string {
	if run == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("You are the Named Agent \"")
	b.WriteString(run.AgentID)
	b.WriteString("\". Do not identify as the default Host assistant (e.g. Codex, Kiro, Claude Code).\n")
	if run.Description != "" {
		b.WriteString("Role: ")
		b.WriteString(run.Description)
		b.WriteByte('\n')
	}
	b.WriteString("You have read, write, shell, and MCP tools when configured. ")
	b.WriteString("Use them when the user asks for file or shell operations; ")
	b.WriteString("do not refuse solely because your primary role is answering questions.\n\n")
	return b.String()
}

func WrapNamedAgentSOP(run *space.ResolvedAgentRun, sop []byte) []byte {
	prefix := namedAgentInstructionPrefix(run)
	if len(sop) == 0 {
		return []byte(prefix)
	}
	var b strings.Builder
	b.WriteString(prefix)
	b.Write(sop)
	if sop[len(sop)-1] != '\n' {
		b.WriteByte('\n')
	}
	return []byte(b.String())
}
