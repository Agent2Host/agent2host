package path

import (
	"strings"

	"github.com/agent2host/agent2host/internal/source/rule"
)

// Canonicalize applies SRC-PATH-REL/SEP/ESCAPE/DOT/ASCII/WINDOWS-RESERVED
// in the locked order and returns the canonical member path (no leading ./).
func Canonicalize(authoring string) (string, error) {
	if strings.Contains(authoring, `\`) {
		return "", rule.Fail("SRC-PATH-SEP", authoring)
	}
	if strings.HasPrefix(authoring, "/") || strings.Contains(authoring, "://") {
		return "", rule.Fail("SRC-PATH-REL", authoring)
	}

	s := authoring
	if strings.HasPrefix(s, "./") {
		s = s[2:]
	}
	if s == ".." || strings.HasPrefix(s, "../") {
		return "", rule.Fail("SRC-PATH-ESCAPE", authoring)
	}
	if s == "" {
		return "", rule.Fail("SRC-PATH-DOT", authoring)
	}

	parts := strings.Split(s, "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return "", rule.Fail("SRC-PATH-DOT", authoring)
		}
	}

	canonical := strings.Join(parts, "/")
	if !portableASCII(canonical) {
		return "", rule.Fail("SRC-PATH-ASCII", authoring)
	}
	for _, p := range parts {
		if windowsReserved(p) {
			return "", rule.Fail("SRC-PATH-WINDOWS-RESERVED", authoring)
		}
	}
	return canonical, nil
}

// IsCanonicalMember reports whether s is already SRC-PATH-CANON form
// (no leading ./, no . / .. / empty segments). Artifact member paths
// must satisfy this; authoring strings that Canonicalize rewrites do not.
func IsCanonicalMember(s string) bool {
	c, err := Canonicalize(s)
	return err == nil && c == s
}

// CollisionKey is ASCII lower-case of a distinct canonical path (SRC-PATH-COLLIDE).
func CollisionKey(canonical string) string {
	return strings.ToLower(canonical)
}

func portableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '/' {
			continue
		}
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}

func windowsReserved(seg string) bool {
	if seg != "." && seg != ".." && strings.HasSuffix(seg, ".") {
		return true
	}
	base := seg
	if i := strings.IndexByte(seg, '.'); i >= 0 {
		base = seg[:i]
	}
	u := strings.ToUpper(base)
	switch u {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(u) == 4 && u[3] >= '1' && u[3] <= '9' {
		if strings.HasPrefix(u, "COM") || strings.HasPrefix(u, "LPT") {
			return true
		}
	}
	return false
}

// HardDeny reports SRC-PATH-HARD-DENY on a canonical path string
// (basename secret set + any `.git` segment). Store-internal layout is FS-only.
func HardDeny(canonical string) error {
	parts := strings.Split(canonical, "/")
	for _, seg := range parts {
		if strings.EqualFold(seg, ".git") {
			return rule.Fail("SRC-PATH-HARD-DENY", canonical)
		}
	}
	base := parts[len(parts)-1]
	switch strings.ToLower(base) {
	case ".env.example", ".env.template":
		return nil
	case ".env", ".env.local", ".env.production", ".env.development", ".env.test", "id_rsa", "id_ed25519":
		return rule.Fail("SRC-PATH-HARD-DENY", canonical)
	}
	return nil
}

// Collide reports SRC-PATH-COLLIDE among distinct canonical paths.
func Collide(canonicals []string) error {
	seen := map[string]string{} // collision key → first canonical
	for _, c := range canonicals {
		k := CollisionKey(c)
		if prev, ok := seen[k]; ok && prev != c {
			return rule.Fail("SRC-PATH-COLLIDE", c)
		}
		if _, ok := seen[k]; !ok {
			seen[k] = c
		}
	}
	return nil
}

// UniqueCanonical returns distinct canonical paths in first-seen order.
func UniqueCanonical(canonicals []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, c := range canonicals {
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}
