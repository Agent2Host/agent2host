package path_test

import (
	"testing"

	"github.com/agent2host/agent2host/internal/source/path"
)

func TestHasPEMAllOccurrences(t *testing.T) {
	if !path.HasPEM([]byte("-----BEGIN PRIVATE KEY-----\nbody\n")) {
		t.Fatal("real header plus newline must match")
	}
	if !path.HasPEM([]byte("prefix-----BEGIN PRIVATE KEY-----")) {
		t.Fatal("header at end of input must match")
	}
	if path.HasPEM([]byte("-----BEGIN PRIVATE KEY-----xxx")) {
		t.Fatal("header without boundary must not match")
	}
	decoy := []byte("-----BEGIN PRIVATE KEY-----xxx\n-----BEGIN PRIVATE KEY-----\nreal\n")
	if !path.HasPEM(decoy) {
		t.Fatal("decoy first occurrence must not hide a later real header")
	}
	if path.HasPEM([]byte("-----BEGIN CERTIFICATE-----\n")) {
		t.Fatal("certificate header is not in the closed set")
	}
}
