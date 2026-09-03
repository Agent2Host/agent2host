package adapter

import "strings"

// ShellJoin preserves argv boundaries for Hosts that only accept a command string.
func ShellJoin(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, a := range argv {
		parts = append(parts, ShellQuote(a))
	}
	return strings.Join(parts, " ")
}

func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func DerefArgs(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}

func DerefFiles(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}
