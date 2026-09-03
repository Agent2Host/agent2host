package path

import (
	"strings"

	"github.com/agent2host/agent2host/internal/source/rule"
)

const (
	kindHostAbs = iota
	kindLocal
	kindPATH
)

// CheckProcess applies SRC-ARGV-* and SRC-ID-NUL to one Hook/MCP ProcessSpec.
func CheckProcess(command string, args []string, files []string) error {
	canonFiles := map[string]struct{}{}
	var fileCanon []string
	for _, f := range files {
		c, err := Canonicalize(f)
		if err != nil {
			return err
		}
		if err := HardDeny(c); err != nil {
			return err
		}
		if _, ok := canonFiles[c]; ok {
			return rule.Fail("SRC-REF-DUP-DECL", c)
		}
		canonFiles[c] = struct{}{}
		fileCanon = append(fileCanon, c)
	}
	if err := Collide(fileCanon); err != nil {
		return err
	}

	if strings.ContainsRune(command, 0) {
		return rule.Fail("SRC-ID-NUL", "U+0000 in command")
	}
	kind, local, err := classifyCommand(command)
	if err != nil {
		return err
	}
	if kind == kindLocal {
		if _, ok := canonFiles[local]; !ok {
			return rule.Fail("SRC-ARGV-COMMAND", command)
		}
	}

	for _, a := range args {
		if strings.ContainsRune(a, 0) {
			return rule.Fail("SRC-ID-NUL", "U+0000 in args")
		}
		local, err := classifyArg(a)
		if err != nil {
			return err
		}
		if local != "" {
			if _, ok := canonFiles[local]; !ok {
				return rule.Fail("SRC-ARGV-ARGS", a)
			}
		}
	}
	return nil
}

func classifyCommand(command string) (kind int, localPath string, err error) {
	if strings.HasPrefix(command, "/") {
		return kindHostAbs, "", nil
	}
	if windowsDriveAbsolute(command) || windowsUNC(command) {
		return kindHostAbs, "", nil
	}
	if windowsDriveRelative(command) {
		return 0, "", rule.Fail("SRC-ARGV-COMMAND", command)
	}
	if strings.HasPrefix(command, "./") {
		c, err := Canonicalize(command)
		if err != nil {
			return 0, "", err
		}
		if err := HardDeny(c); err != nil {
			return 0, "", err
		}
		return kindLocal, c, nil
	}
	if strings.HasPrefix(command, "../") {
		return 0, "", rule.Fail("SRC-PATH-ESCAPE", command)
	}
	if strings.ContainsAny(command, `/\`) {
		return 0, "", rule.Fail("SRC-ARGV-COMMAND", command)
	}
	return kindPATH, "", nil
}

func classifyArg(token string) (localPath string, err error) {
	if strings.HasPrefix(token, "./") {
		c, err := Canonicalize(token)
		if err != nil {
			return "", err
		}
		if err := HardDeny(c); err != nil {
			return "", err
		}
		return c, nil
	}
	if strings.HasPrefix(token, "../") {
		return "", rule.Fail("SRC-PATH-ESCAPE", token)
	}
	return "", nil
}

func windowsDriveAbsolute(s string) bool {
	if len(s) < 3 {
		return false
	}
	if !isLetter(s[0]) || s[1] != ':' {
		return false
	}
	return s[2] == '/' || s[2] == '\\'
}

func windowsUNC(s string) bool {
	return strings.HasPrefix(s, `\\`)
}

func windowsDriveRelative(s string) bool {
	if len(s) < 2 || !isLetter(s[0]) || s[1] != ':' {
		return false
	}
	return !windowsDriveAbsolute(s)
}

func isLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}
