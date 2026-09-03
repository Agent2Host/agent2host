package kiro

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/space"
)

func reconcileSemanticKiro(run *space.ResolvedAgentRun, intent adapter.ControlIntent, report compatibility.Report, np adapter.NativeProjectionPlan, lp adapter.LaunchPlan) error {
	if run == nil {
		return fmt.Errorf("%w: nil run", adapter.ErrSemanticMismatch)
	}
	if intent.Has(adapter.ControlIsolation) {
		home, ok := lp.Env[EnvHome]
		if !ok || !strings.Contains(home, HomeDirRel) {
			return fmt.Errorf("%w: %s must point at isolated kiro-home", adapter.ErrSemanticMismatch, EnvHome)
		}
	}
	if adapter.LaunchArgPresent(lp.Args, "--agent") {
		if agent, ok := adapter.LaunchArgValue(lp.Args, "--agent"); !ok || agent != run.AgentID {
			return fmt.Errorf("%w: --agent must select %q", adapter.ErrSemanticMismatch, run.AgentID)
		}
	}
	agentRel := AgentsDirRel + "/" + run.AgentID + ".json"
	if intent.Has(adapter.ControlPermissions) || intent.Has(adapter.ControlApprovals) || intent.Has(adapter.ControlMCP) {
		raw, ok := adapter.ProjectionContent(np, agentRel)
		if !ok {
			return fmt.Errorf("%w: missing %s", adapter.ErrSemanticMismatch, agentRel)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("%w: invalid %s: %v", adapter.ErrSemanticMismatch, agentRel, err)
		}
		if intent.Has(adapter.ControlPermissions) {
			if err := reconcileKiroTools(run, report, doc); err != nil {
				return err
			}
		}
		if intent.Has(adapter.ControlApprovals) {
			if err := reconcileKiroToolsSettings(run, doc); err != nil {
				return err
			}
		}
	}
	return nil
}

func reconcileKiroTools(run *space.ResolvedAgentRun, report compatibility.Report, doc map[string]any) error {
	want := kiroTools(run, report)
	got := stringSliceField(doc, "tools")
	if !adapter.StringSetEqual(got, want) {
		return fmt.Errorf("%w: kiro tools list does not match permission synthesis", adapter.ErrSemanticMismatch)
	}
	return nil
}

func reconcileKiroToolsSettings(run *space.ResolvedAgentRun, doc map[string]any) error {
	want, err := json.Marshal(kiroToolsSettings(run))
	if err != nil {
		return err
	}
	gotRaw, err := json.Marshal(doc["toolsSettings"])
	if err != nil {
		return fmt.Errorf("%w: toolsSettings not serializable", adapter.ErrSemanticMismatch)
	}
	if string(gotRaw) != string(want) {
		return fmt.Errorf("%w: kiro toolsSettings do not match approval synthesis", adapter.ErrSemanticMismatch)
	}
	return nil
}

func stringSliceField(doc map[string]any, key string) []string {
	raw, ok := doc[key]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
