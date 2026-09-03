package fixtures

import (
	"fmt"
	"os"
	"path/filepath"
)

// Root returns spec/schemas/source/v1alpha1/fixtures relative to the module root.
// Tests only: walk from the process working directory. Do not use runtime.Caller
// (broken under go test -trimpath and in shipped binaries).
func Root() (string, error) {
	mod, err := moduleRoot()
	if err != nil {
		return "", err
	}
	root := filepath.Join(mod, "spec", "schemas", "source", "v1alpha1", "fixtures")
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("fixtures: frozen corpus missing at %s", root)
	}
	return root, nil
}

// SystemsRoot returns the official acceptance Agent Systems
// (dev-studio, ops-desk, research-lab).
func SystemsRoot() (string, error) {
	mod, err := moduleRoot()
	if err != nil {
		return "", err
	}
	root := filepath.Join(mod, "test", "systems")
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("fixtures: official systems missing at %s", root)
	}
	return root, nil
}

// OfficialSystem is the path of one committed acceptance system.
func OfficialSystem(name string) (string, error) {
	root, err := SystemsRoot()
	if err != nil {
		return "", err
	}
	p := filepath.Join(root, name)
	if _, err := os.Stat(filepath.Join(p, "system.json")); err != nil {
		return "", fmt.Errorf("fixtures: official system %s: %w", name, err)
	}
	return p, nil
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("fixtures: getwd: %w", err)
	}
	start := dir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("fixtures: go.mod not found walking from %s", start)
		}
		dir = parent
	}
}
