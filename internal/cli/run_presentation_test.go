package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/runtime"
)

func TestRenderRunOutcomeDefault(t *testing.T) {
	report := compatibility.Report{Decision: "allowed", Host: compatibility.HostRef{ID: adapter.HostClaudeCode}}
	tests := []struct {
		name       string
		out        runtime.Outcome
		err        error
		wantStdout string
		wantStderr string
	}{
		{
			name:       "clean host exit",
			out:        runtime.Outcome{Class: runtime.ClassHostProcess, Stage: "host"},
			wantStdout: "Session ended.\n",
		},
		{
			name:       "terminal interrupt",
			out:        runtime.Outcome{Class: runtime.ClassHostProcess, Stage: "host", ExitCode: 130, Interrupted: true},
			wantStdout: "Session ended.\n",
		},
		{
			name:       "unexpected host exit",
			out:        runtime.Outcome{Class: runtime.ClassHostProcess, Stage: "host", ExitCode: 2},
			wantStderr: "Session ended unexpectedly. Run again with --verbose if you need details.\n",
		},
		{
			name:       "missing credential before launch",
			out:        runtime.Outcome{Class: runtime.ClassPreLaunch, Stage: "secrets"},
			err:        fmt.Errorf("%w: MUST_HAVE", runtime.ErrMissingSecret),
			wantStderr: "Not started. A required credential is missing: MUST_HAVE.\n",
		},
		{
			name:       "cleanup failure after host exit",
			out:        runtime.Outcome{Class: runtime.ClassInfra, Stage: "wipe_secrets"},
			err:        errors.New("wipe failed"),
			wantStderr: "Session ended, but Agent2Host could not clean up all temporary run files.\n",
		},
		{
			name: "host-state save failed after clean exit",
			out: runtime.Outcome{
				Class:               runtime.ClassHostProcess,
				Stage:               "host",
				HostStateSaveFailed: true,
			},
			wantStdout: "Session ended.\n",
			wantStderr: "Host sign-in updates from this session might not have been saved.\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			renderRunOutcome(&stdout, &stderr, report, tt.out, tt.err, false)
			if got := stdout.String(); got != tt.wantStdout {
				t.Fatalf("stdout = %q, want %q", got, tt.wantStdout)
			}
			if got := stderr.String(); got != tt.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}

func TestRenderRunOutcomeVerboseShowsDetailsOnlyOnRequest(t *testing.T) {
	report := compatibility.Report{Decision: "allowed", Host: compatibility.HostRef{ID: adapter.HostCodex}}
	outcome := runtime.Outcome{
		Class:       runtime.ClassHostProcess,
		Stage:       "host",
		RunID:       "run-123",
		ExitCode:    2,
		Interrupted: true,
	}
	var stdout, stderr bytes.Buffer
	renderRunOutcome(&stdout, &stderr, report, outcome, errors.New("host failed"), true)
	got := stderr.String()
	for _, want := range []string{"Details:", "decision: allowed", "run: run-123", "class: host_process", "stage: host", "exit: 2", "interrupted: true", "error: host failed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("verbose output missing %q: %q", want, got)
		}
	}
	if strings.Contains(stdout.String(), "decision:") {
		t.Fatalf("default session line must remain separate from details: %q", stdout.String())
	}
}

func TestRenderRunAuthorizationFailure(t *testing.T) {
	report := compatibility.Report{Decision: "refused", Host: compatibility.HostRef{ID: adapter.HostKiro}}
	var stdout, stderr bytes.Buffer
	renderRunAuthorizationFailure(&stdout, &stderr, report, ErrExecuteRefused, false)
	if stdout.Len() != 0 {
		t.Fatalf("refuse must not write a session line: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Not started. This Host cannot meet this Agent's requirements.") {
		t.Fatalf("refuse message missing: %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "Details:") {
		t.Fatalf("default refuse must not dump details: %q", stderr.String())
	}
}

func TestPrintRunReadNotice(t *testing.T) {
	var ordinary bytes.Buffer
	printRunReadNotice(&ordinary, false, adapter.HostClaudeCode, compatibility.Report{Decision: "allowed"}, adapter.ProjectionContext{})
	if got, want := ordinary.String(), "This session can read files outside your project folder.\n"; got != want {
		t.Fatalf("ordinary read notice = %q, want %q", got, want)
	}
	var refused bytes.Buffer
	printRunReadNotice(&refused, false, adapter.HostClaudeCode, compatibility.Report{Decision: "refused"}, adapter.ProjectionContext{})
	if refused.Len() != 0 {
		t.Fatalf("refused run must not show start notice: %q", refused.String())
	}
}

func TestRunJSONAddsInterruptedOnlyWhenObserved(t *testing.T) {
	report := compatibility.Report{Decision: "allowed"}
	for _, tt := range []struct {
		name        string
		interrupted bool
		wantField   bool
	}{
		{name: "not interrupted", interrupted: false, wantField: false},
		{name: "interrupted", interrupted: true, wantField: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			code := writeJSON(&out, &errb, runJSON{Outcome: &runtime.Outcome{Interrupted: tt.interrupted}, Report: report})
			if code != ExitOK || errb.Len() != 0 {
				t.Fatalf("code=%d stderr=%q", code, errb.String())
			}
			var doc map[string]any
			if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			outcome := doc["outcome"].(map[string]any)
			_, gotField := outcome["interrupted"]
			if gotField != tt.wantField {
				t.Fatalf("interrupted field present=%t, want %t: %s", gotField, tt.wantField, out.String())
			}
		})
	}
}
