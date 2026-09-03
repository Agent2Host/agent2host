package adapter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/source/decode"
	"github.com/agent2host/agent2host/internal/space"
)

func RefuseIfNeeded(report compatibility.Report) error {
	if report.Decision == "refused" {
		return ErrProjectRefused
	}
	return nil
}

func IncludedSkills(report compatibility.Report) map[string]bool {
	out := map[string]bool{}
	for _, it := range report.Capabilities.Skills.Items {
		if it.Disposition == "include" {
			out[it.ID] = true
		}
	}
	return out
}

func IncludedContexts(report compatibility.Report) map[string]bool {
	out := map[string]bool{}
	for _, it := range report.Capabilities.Context.Items {
		if it.Disposition == "include" {
			out[it.Path] = true
		}
	}
	return out
}

func SOPIncluded(report compatibility.Report) bool {
	return report.Capabilities.SOP.Disposition == "include"
}

func CopyContent(run *space.ResolvedAgentRun, key string) []byte {
	if run.Content == nil {
		return nil
	}
	return append([]byte(nil), run.Content[key]...)
}

func SecretRefs(run *space.ResolvedAgentRun) []SecretRef {
	var out []SecretRef
	for _, b := range run.Environment {
		out = append(out, SecretRef{Name: b.ValueFrom.Environment, Consumer: "/environment", Required: reqBool(b.Required, true)})
	}
	for _, sid := range SortedMCPServerIDs(run) {
		for _, b := range run.MCPServers[sid].Environment {
			out = append(out, SecretRef{Name: b.ValueFrom.Environment, Consumer: "/mcp_servers/" + sid, Required: reqBool(b.Required, true)})
		}
	}
	if run.Hooks != nil {
		add := func(prefix string, entries *[]decode.HookEntry) {
			if entries == nil {
				return
			}
			for i, h := range *entries {
				if h.Environment == nil {
					continue
				}
				for _, b := range *h.Environment {
					out = append(out, SecretRef{Name: b.ValueFrom.Environment, Consumer: fmt.Sprintf("%s/%d", prefix, i), Required: reqBool(b.Required, true)})
				}
			}
		}
		add("/hooks/session_start", run.Hooks.SessionStart)
		add("/hooks/before_tool_call", run.Hooks.BeforeToolCall)
		add("/hooks/after_tool_call", run.Hooks.AfterToolCall)
		add("/hooks/agent_stop", run.Hooks.AgentStop)
	}
	return out
}

func Launch(probe ProbeResult, selectHow string) LaunchPlan {
	return LaunchPlan{
		Executable:      probe.Executable,
		Args:            nil,
		WorkingDirClass: DestWorkingDir,
		Env:             map[string]string{},
		Secrets:         nil,
		AgentSelect:     selectHow,
	}
}

func File(rel string, run *space.ResolvedAgentRun, irKey string, fallback []byte) ProjectionFile {
	body := CopyContent(run, irKey)
	from := irKey
	if len(body) == 0 {
		body = fallback
		from = ""
	}
	return ProjectionFile{RelPath: rel, Class: DestProjection, Content: body, FromContent: from}
}

func skillDoc(run *space.ResolvedAgentRun, id string) []byte {
	for _, sk := range run.Skills {
		if sk.ID == id {
			body := CopyContent(run, sk.Document)
			if len(body) == 0 {
				body = []byte("# " + sk.Name + "\n\n" + sk.Description + "\n")
			}
			return withYAMLFrontmatter(sk.ID, sk.Description, body)
		}
	}
	return withYAMLFrontmatter(id, "", []byte("# "+id+"\n"))
}

func DisplayName(run *space.ResolvedAgentRun) string {
	if run != nil && run.Name != "" {
		return run.Name
	}
	if run != nil && run.AgentID != "" {
		return run.AgentID
	}
	return "agent"
}

func AgentCard(run *space.ResolvedAgentRun, includeSOP bool) []byte {
	if run == nil {
		return nil
	}
	var b strings.Builder
	b.WriteString("---\nname: ")
	b.WriteString(YAMLScalar(run.AgentID))
	b.WriteString("\ndescription: ")
	b.WriteString(YAMLScalar(run.Description))
	b.WriteString("\n---\n\n")
	if includeSOP {
		if body := CopyContent(run, run.SOP); len(body) > 0 {
			b.Write(body)
			if body[len(body)-1] != '\n' {
				b.WriteByte('\n')
			}
			return []byte(b.String())
		}
	}
	b.WriteString("# ")
	b.WriteString(DisplayName(run))
	b.WriteByte('\n')
	if run.Description != "" {
		b.WriteString("\n")
		b.WriteString(run.Description)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func BannerSOPFiles(files []ProjectionFile, sopRel string, run *space.ResolvedAgentRun) []ProjectionFile {
	for i, f := range files {
		if f.RelPath != sopRel {
			continue
		}
		files[i].Content = withBanner(run, f.Content)
		break
	}
	return files
}

func withBanner(run *space.ResolvedAgentRun, sop []byte) []byte {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(DisplayName(run))
	b.WriteByte('\n')
	if run != nil && run.Description != "" {
		b.WriteString("\n")
		b.WriteString(run.Description)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.Write(sop)
	return []byte(b.String())
}

func withYAMLFrontmatter(name, description string, body []byte) []byte {
	if hasYAMLFrontmatter(body) {
		return body
	}
	var b strings.Builder
	b.WriteString("---\nname: ")
	b.WriteString(YAMLScalar(name))
	b.WriteString("\ndescription: ")
	b.WriteString(YAMLScalar(description))
	b.WriteString("\n---\n\n")
	b.Write(body)
	return []byte(b.String())
}

func hasYAMLFrontmatter(body []byte) bool {
	s := strings.TrimPrefix(string(body), "\ufeff")
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return false
	}
	rest := s[3:]
	if i := strings.Index(rest, "\n---\n"); i >= 0 {
		return true
	}
	if i := strings.Index(rest, "\r\n---\r\n"); i >= 0 {
		return true
	}
	return strings.Contains(rest, "\n---")
}

func YAMLScalar(s string) string {
	if s == "" {
		return `""`
	}
	need := strings.ContainsAny(s, ":#{}[]&*!|>'\"%@`,") ||
		strings.ContainsAny(s, "\n\r\t") ||
		strings.HasPrefix(s, " ") ||
		strings.HasSuffix(s, " ") ||
		s == "true" || s == "false" || s == "null" || s == "yes" || s == "no"
	if !need {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			// drop CR in folded scalars
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func ProjectSOPAndSkills(run *space.ResolvedAgentRun, report compatibility.Report, sopRel, skillRel func(id string) string, skillClass DestinationClass) []ProjectionFile {
	var files []ProjectionFile
	if SOPIncluded(report) && run.SOP != "" {
		f := File(sopRel(""), run, run.SOP, []byte("# SOP\n"))
		files = append(files, f)
	}
	if skillClass == "" {
		skillClass = DestProjection
	}
	keep := IncludedSkills(report)
	for _, sk := range run.Skills {
		if !keep[sk.ID] {
			continue
		}
		files = append(files, ProjectionFile{
			RelPath:     skillRel(sk.ID),
			Class:       skillClass,
			Content:     skillDoc(run, sk.ID),
			FromContent: sk.Document,
		})
	}
	ctxKeep := IncludedContexts(report)
	ctxPaths := make([]string, 0, len(ctxKeep))
	for path := range ctxKeep {
		ctxPaths = append(ctxPaths, path)
	}
	sort.Strings(ctxPaths)
	for _, path := range ctxPaths {
		files = append(files, File(path, run, path, nil))
	}
	return files
}

func PrefixedSOP(rel string) func(string) string {
	return func(string) string { return rel }
}

func SkillDir(dir string) func(string) string {
	return func(id string) string {
		return fmt.Sprintf("%s/%s/SKILL.md", dir, id)
	}
}
