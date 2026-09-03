package store

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/artifact"
	"github.com/agent2host/agent2host/internal/source/decode"
)

func TestWriteArtifactRefusesEscapeAndLeavesSentinel(t *testing.T) {
	home := t.TempDir()
	spaceDir := filepath.Join(home, "space")
	if err := os.MkdirAll(filepath.Join(spaceDir, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(spaceDir, "registry.json")
	if err := os.WriteFile(sentinel, []byte("KEEP\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(spaceDir, "artifacts", ".tmp-escape")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}

	sys := []byte(`{"ok":true}`)
	attack := []byte("PWNED\n")
	cases := []string{
		"../../../registry.json",
		"../registry.json",
		"/etc/passwd",
		"agents/./demo.agent.json",
		"agents//demo.agent.json",
		`agents\demo.agent.json`,
	}
	for _, p := range cases {
		art := craftedArtifact(sys, p, attack)
		if err := writeArtifact(tmp, art); err == nil {
			t.Fatalf("path %q: write must refuse", p)
		}
		got, err := os.ReadFile(sentinel)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "KEEP\n" {
			t.Fatalf("path %q overwrote sentinel: %q", p, got)
		}
	}
}

func TestPublishRefusesEscapeArtifact(t *testing.T) {
	home := t.TempDir()
	st, err := New(home)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(home, "space", "registry.json")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("KEEP\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	art := craftedArtifact([]byte(`{"ok":true}`), "../../../registry.json", []byte("PWNED\n"))
	if err := st.Publish(art); err == nil {
		t.Fatal("Publish must refuse escaped member path")
	}
	got, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "KEEP\n" {
		t.Fatalf("sentinel overwritten: %q", got)
	}
}

func craftedArtifact(sys []byte, escapePath string, body []byte) *artifact.Artifact {
	sysHash := sha256.Sum256(sys)
	escHash := sha256.Sum256(body)
	files := []decode.ArtifactMember{
		{Path: escapePath, SHA256: hex.EncodeToString(escHash[:]), Type: "file"},
		{Path: "system.json", SHA256: hex.EncodeToString(sysHash[:]), Type: "file"},
	}
	if escapePath > "system.json" {
		files[0], files[1] = files[1], files[0]
	}
	return &artifact.Artifact{
		Manifest: decode.ArtifactManifest{DigestFormat: "agent2host-system-v1", Files: files},
		Revision: "sha256:" + strings.Repeat("0", 64),
		Payload: map[string][]byte{
			"system.json": sys,
			escapePath:    body,
		},
	}
}
