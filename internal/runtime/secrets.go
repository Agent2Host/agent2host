package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agent2host/agent2host/internal/adapter"
)

type resolvedSecrets struct {
	env     []string
	omitted []string
	values  map[string]string
}

func resolveSecrets(refs []adapter.SecretRef, getenv func(string) string) (resolvedSecrets, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	out := resolvedSecrets{values: map[string]string{}}
	for _, ref := range refs {
		if ref.Name == "" {
			continue
		}
		v := getenv(ref.Name)
		if v == "" {
			if ref.Required {
				return resolvedSecrets{}, fmt.Errorf("%w: %s", ErrMissingSecret, ref.Name)
			}
			out.omitted = append(out.omitted, ref.Name)
			continue
		}
		out.values[ref.Name] = v
		if adapter.HostProcessConsumer(ref.Consumer) || ref.DeliverProcessEnv {
			out.env = append(out.env, ref.Name+"="+v)
		}
	}
	return out, nil
}

// overlaySecrets fills secret slots by walking JSON string fields that hold
// placeholders. It does not whole-file string-replace. Omitted optional names
// remove the env entry that still holds their placeholder.
func overlaySecrets(p Prepared, plan adapter.NativeProjectionPlan, values map[string]string, omitted []string) error {
	omit := map[string]bool{}
	for _, n := range omitted {
		omit[n] = true
	}
	for _, f := range plan.Files {
		if !adapter.SecretOverlayAllowed(f.Class) {
			continue
		}
		if !bytes.Contains(f.Content, []byte(adapter.SecretPlaceholderPrefix)) {
			continue
		}
		root, err := rootFor(p, f.Class)
		if err != nil {
			return err
		}
		dest, err := joinUnder(root, f.RelPath)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(dest)
		if err != nil {
			return err
		}
		var out []byte
		if bytes.Contains(body, []byte("{")) && (bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) || bytes.HasPrefix(bytes.TrimSpace(body), []byte("["))) {
			out, err = overlayJSONSecretSlots(body, values, omit)
			if err != nil {
				return fmt.Errorf("overlay %s: %w", f.RelPath, err)
			}
		} else if strings.HasSuffix(strings.ToLower(f.RelPath), ".toml") {
			out = overlayTOMLSecretSlots(body, values, omit)
		} else {
			out = overlayPlainSecretSlots(body, values, omit)
		}
		if err := os.WriteFile(dest, out, secretFileMode(f)); err != nil {
			return err
		}
	}
	return nil
}

// secretFileMode keeps a System-local command runnable across overlay/wipe
// rewrites while staying owner-only for secret-bearing files.
func secretFileMode(f adapter.ProjectionFile) os.FileMode {
	if f.Executable {
		return 0o700
	}
	return 0o600
}

func overlayJSONSecretSlots(body []byte, values map[string]string, omit map[string]bool) ([]byte, error) {
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	if err := walkSecretSlots(doc, values, omit); err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// overlayTOMLSecretSlots replaces whole quoted placeholders with QuoteTOMLString(value).
// Plain byte-replace of the placeholder alone would let " or newlines break/inject config.toml.
func overlayTOMLSecretSlots(body []byte, values map[string]string, omit map[string]bool) []byte {
	out := body
	replace := func(name, val string, drop bool) {
		quotedPh := []byte(adapter.QuoteTOMLString(adapter.SecretPlaceholder(name)))
		if bytes.Contains(out, quotedPh) {
			if drop {
				out = bytes.ReplaceAll(out, quotedPh, []byte(`""`))
				return
			}
			out = bytes.ReplaceAll(out, quotedPh, []byte(adapter.QuoteTOMLString(val)))
			return
		}
		// Legacy: placeholder already inside quotes without re-quoting the slot.
		ph := []byte(adapter.SecretPlaceholder(name))
		if !bytes.Contains(out, ph) {
			return
		}
		if drop {
			out = bytes.ReplaceAll(out, ph, nil)
			return
		}
		inner := adapter.QuoteTOMLString(val)
		// QuoteTOMLString includes surrounding quotes; strip them for in-quote replace.
		if len(inner) >= 2 && inner[0] == '"' {
			inner = inner[1 : len(inner)-1]
		}
		out = bytes.ReplaceAll(out, ph, []byte(inner))
	}
	for name, val := range values {
		replace(name, val, omit[name])
	}
	for name := range omit {
		if _, ok := values[name]; ok {
			continue
		}
		replace(name, "", true)
	}
	return out
}

func overlayPlainSecretSlots(body []byte, values map[string]string, omit map[string]bool) []byte {
	out := body
	for name, val := range values {
		ph := []byte(adapter.SecretPlaceholder(name))
		if !bytes.Contains(out, ph) {
			continue
		}
		if omit[name] {
			out = bytes.ReplaceAll(out, ph, nil)
			continue
		}
		out = bytes.ReplaceAll(out, ph, []byte(val))
	}
	// Drop omitted placeholders that were never in values.
	for name := range omit {
		ph := []byte(adapter.SecretPlaceholder(name))
		out = bytes.ReplaceAll(out, ph, nil)
	}
	return out
}

func walkSecretSlots(v any, values map[string]string, omit map[string]bool) error {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			s, ok := val.(string)
			if !ok {
				if err := walkSecretSlots(val, values, omit); err != nil {
					return err
				}
				continue
			}
			if !strings.HasPrefix(s, adapter.SecretPlaceholderPrefix) {
				continue
			}
			name := strings.TrimPrefix(s, adapter.SecretPlaceholderPrefix)
			if omit[name] {
				delete(x, k)
				continue
			}
			real, ok := values[name]
			if !ok {
				delete(x, k)
				continue
			}
			x[k] = real
		}
	case []any:
		for _, el := range x {
			if err := walkSecretSlots(el, values, omit); err != nil {
				return err
			}
		}
	}
	return nil
}

// wipeSecrets restores Plan bytes for secret-bearing files, then removes any
// remaining secret values from those planned destinations only.
func wipeSecrets(p Prepared, plan adapter.NativeProjectionPlan, values map[string]string) error {
	for _, f := range plan.Files {
		root, err := rootFor(p, f.Class)
		if err != nil {
			return err
		}
		dest, err := joinUnder(root, f.RelPath)
		if err != nil {
			return err
		}
		if bytes.Contains(f.Content, []byte(adapter.SecretPlaceholderPrefix)) {
			if err := os.WriteFile(dest, f.Content, secretFileMode(f)); err != nil {
				return err
			}
			continue
		}
		if err := scrubSecretValuesInFile(dest, values, secretFileMode(f)); err != nil {
			return err
		}
	}
	return nil
}

// minSecretScrubLen skips short values so wipe does not delete common
// substrings such as "dev" or "test" from unrelated planned files.
const minSecretScrubLen = 8

func scrubSecretValuesInFile(path string, values map[string]string, mode os.FileMode) error {
	if len(values) == 0 {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if mode == 0 {
		mode = filePermOr(path, 0o600)
	}
	changed := false
	for _, val := range values {
		if len(val) < minSecretScrubLen || !bytes.Contains(body, []byte(val)) {
			continue
		}
		body = bytes.ReplaceAll(body, []byte(val), nil)
		changed = true
	}
	if !changed {
		return nil
	}
	return os.WriteFile(path, body, mode)
}

func filePermOr(path string, fallback os.FileMode) os.FileMode {
	fi, err := os.Stat(path)
	if err != nil {
		return fallback
	}
	return fi.Mode().Perm()
}

// persistSecretBaselines stores placeholder Plan bytes so a later Prepare can
// restore after crash/kill before wipeSecrets runs. Restore skips runs that
// still hold a live.lock (concurrent Execute).
func persistSecretBaselines(p Prepared, plan adapter.NativeProjectionPlan) error {
	for _, f := range plan.Files {
		if !bytes.Contains(f.Content, []byte(adapter.SecretPlaceholderPrefix)) {
			continue
		}
		dest := filepath.Join(p.Root, "secret-baseline", string(f.Class), f.RelPath)
		if err := os.MkdirAll(filepath.Dir(dest), runtimeDirMode); err != nil {
			return err
		}
		if err := os.WriteFile(dest, f.Content, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func scrubSecretValues(dir string, values map[string]string) error {
	return scrubSecretValuesExcept(dir, values, nil)
}

func scrubSecretValuesExcept(dir string, values map[string]string, skip map[string]bool) error {
	if len(values) == 0 {
		return nil
	}
	fi, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !fi.IsDir() {
		return nil
	}
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if skip[info.Name()] {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		changed := false
		for _, val := range values {
			if len(val) < minSecretScrubLen || !bytes.Contains(body, []byte(val)) {
				continue
			}
			body = bytes.ReplaceAll(body, []byte(val), nil)
			changed = true
		}
		if !changed {
			return nil
		}
		return os.WriteFile(path, body, info.Mode().Perm())
	})
}

func mergeLaunchEnv(launch, bind map[string]string) map[string]string {
	if len(launch) == 0 && len(bind) == 0 {
		return nil
	}
	out := make(map[string]string, len(launch)+len(bind))
	for k, v := range launch {
		out[k] = v
	}
	for k, v := range bind {
		out[k] = v
	}
	return out
}

func expandLaunchEnv(p Prepared, env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		if k == "" {
			continue
		}
		out = append(out, k+"="+expandTokens(p, v))
	}
	return out
}

func expandLaunchArgs(p Prepared, args []string) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = expandTokens(p, a)
	}
	return out
}

func expandTokens(p Prepared, s string) string {
	s = strings.ReplaceAll(s, adapter.WorkspaceToken, p.Workspace)
	s = strings.ReplaceAll(s, adapter.PrivateToken, p.Private)
	s = strings.ReplaceAll(s, adapter.HomeToken, p.Home)
	s = strings.ReplaceAll(s, adapter.AuthProfileToken, p.AuthProfile)
	return s
}

// expandWorkspaceInFiles rewrites path tokens in Plan files after materialize
// so MCP/Hook local paths resolve under RunWorkspace while Host cwd stays
// ApprovedWorkingDirectory. Plans keep the token; disk gets absolute paths.
func expandWorkspaceInFiles(p Prepared, plan adapter.NativeProjectionPlan) error {
	for _, f := range plan.Files {
		if !bytes.Contains(f.Content, []byte(adapter.WorkspaceToken)) &&
			!bytes.Contains(f.Content, []byte(adapter.PrivateToken)) &&
			!bytes.Contains(f.Content, []byte(adapter.HomeToken)) &&
			!bytes.Contains(f.Content, []byte(adapter.AuthProfileToken)) {
			continue
		}
		root, err := rootFor(p, f.Class)
		if err != nil {
			return err
		}
		dest, err := joinUnder(root, f.RelPath)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(dest)
		if err != nil {
			return err
		}
		body = bytes.ReplaceAll(body, []byte(adapter.WorkspaceToken), []byte(p.Workspace))
		body = bytes.ReplaceAll(body, []byte(adapter.PrivateToken), []byte(p.Private))
		body = bytes.ReplaceAll(body, []byte(adapter.HomeToken), []byte(p.Home))
		body = bytes.ReplaceAll(body, []byte(adapter.AuthProfileToken), []byte(p.AuthProfile))
		if err := os.WriteFile(dest, body, secretFileMode(f)); err != nil {
			return err
		}
	}
	return nil
}
