package runtime

import (
	"os"
	"strings"
)

// baseEnvAllow is the closed set of parent-process keys copied into the Host
// launch environment. Full os.Environ() inheritance would leak undeclared
// credentials into the Host, model tools, and subprocesses.
//
// Agent2Host Agent secrets are layered separately via LaunchPlan secret refs.
// Host product auth keys (API tokens) are allowlisted so isolated Host homes
// can still authenticate without inheriting the whole shell.
var baseEnvAllow = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "SHELL": true,
	"TERM": true, "TERMINFO": true, "COLORTERM": true,
	"NO_COLOR": true, "FORCE_COLOR": true,
	"LANG": true, "LC_ALL": true, "LC_CTYPE": true, "LC_MESSAGES": true,
	"TMPDIR": true, "TMP": true, "TEMP": true,
	"TZ": true, "XDG_RUNTIME_DIR": true,
	"HTTP_PROXY": true, "HTTPS_PROXY": true, "ALL_PROXY": true, "NO_PROXY": true,
	"http_proxy": true, "https_proxy": true, "all_proxy": true, "no_proxy": true,
	// Host product auth (not Agent2Host Agent EnvironmentBindings).
	"ANTHROPIC_API_KEY": true, "ANTHROPIC_AUTH_TOKEN": true,
	"CLAUDE_CODE_OAUTH_TOKEN": true,
	"OPENAI_API_KEY":          true, "OPENAI_BASE_URL": true,
	"CODEX_API_KEY": true, "CODEX_ACCESS_TOKEN": true,
}

// baseLaunchEnv builds a minimal Host process environment from parent.
// getenv is for tests; nil uses os.Environ + os.Getenv.
func baseLaunchEnv(getenv func(string) string) []string {
	if getenv == nil {
		return filterParentEnv(os.Environ())
	}
	out := make([]string, 0, len(baseEnvAllow))
	seen := map[string]bool{}
	for k := range baseEnvAllow {
		if seen[k] {
			continue
		}
		seen[k] = true
		if v := getenv(k); v != "" {
			out = append(out, k+"="+v)
		}
	}
	// LC_* beyond the fixed allow entries (locale).
	for _, k := range []string{
		"LC_COLLATE", "LC_MONETARY", "LC_NUMERIC", "LC_TIME", "LC_IDENTIFICATION",
	} {
		if v := getenv(k); v != "" {
			out = append(out, k+"="+v)
		}
	}
	return out
}

func filterParentEnv(parent []string) []string {
	out := make([]string, 0, len(baseEnvAllow)+8)
	for _, kv := range parent {
		k, _, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			continue
		}
		if baseEnvAllow[k] || strings.HasPrefix(k, "LC_") {
			out = append(out, kv)
		}
	}
	return out
}
