package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutOutsideHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(os.TempDir(), "a2h-outside-home-test")
	if PathUnderHome(outside, home) {
		t.Skip("temp dir unexpectedly under $HOME")
	}
	if !layoutOutsideHome(outside, outside+"/private", home) {
		t.Fatal("expected outside-home layout")
	}
	if layoutOutsideHome(filepath.Join(home, "proj"), outside, home) {
		t.Fatal("wd under home must not qualify")
	}
}
