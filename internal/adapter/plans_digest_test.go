package adapter_test

import (
	"testing"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/committed"
	"github.com/agent2host/agent2host/internal/space"
)

func TestProjectDigestStableAcrossGenerations(t *testing.T) {
	reg := committed.New(foundLook(), stubVersion)
	req := true
	run := sampleRun(false, false)
	run.Contexts = []space.ResolvedContext{
		{Path: "contexts/z-notes.md", Required: &req},
		{Path: "contexts/a-handbook.md", Required: &req},
	}
	run.Content["contexts/z-notes.md"] = []byte("z\n")
	run.Content["contexts/a-handbook.md"] = []byte("a\n")

	hosts := []string{adapter.HostClaudeCode, adapter.HostKiro, adapter.HostCodex}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			var first string
			var files []string
			for i := 0; i < 2; i++ {
				out, err := adapter.RunPipeline(reg, host, run, adapter.ProjectionContext{}, "0.0.0-test", adapter.RunPolicy{})
				if err != nil {
					t.Fatal(err)
				}
				if out.Report.Decision == "refused" || out.Plans == nil {
					t.Fatalf("decision %s plans %v", out.Report.Decision, out.Plans)
				}
				got, err := adapter.DigestPlans(*out.Plans)
				if err != nil {
					t.Fatal(err)
				}
				if i == 0 {
					first = got
					for _, f := range out.Plans.Projection.Files {
						files = append(files, f.RelPath)
					}
					continue
				}
				if got != first {
					t.Fatalf("digest changed across generations: %s vs %s", first, got)
				}
			}
			ia, iz := -1, -1
			for i, p := range files {
				if p == "contexts/a-handbook.md" {
					ia = i
				}
				if p == "contexts/z-notes.md" {
					iz = i
				}
			}
			if ia < 0 || iz < 0 {
				t.Fatalf("missing context files in %v", files)
			}
			if ia > iz {
				t.Fatalf("context files not sorted: %v", files)
			}
		})
	}
}
