package space

import (
	"os"
	"testing"
	"time"

	"github.com/agent2host/agent2host/internal/source/path"
)

func TestCheckInclusionSizeFileLimit(t *testing.T) {
	err := checkInclusionSize(map[string][]byte{"assets/big.bin": make([]byte, maxMemberBytes+1)})
	if err == nil {
		t.Fatal("expected refuse")
	}
	se, ok := err.(*Error)
	if !ok || se.Kind != KindTooLarge {
		t.Fatalf("got %v", err)
	}
}

func TestCheckInclusionSizeSystemLimit(t *testing.T) {
	payload := map[string][]byte{}
	chunk := maxMemberBytes
	// 5 × 16 MiB = 80 MiB > 64 MiB, each file legal.
	for i := 0; i < 5; i++ {
		payload[string(rune('a'+i))] = make([]byte, chunk)
	}
	err := checkInclusionSize(payload)
	if err == nil {
		t.Fatal("expected refuse")
	}
	if err.(*Error).Kind != KindTooLarge {
		t.Fatalf("got %v", err)
	}
}

func TestCheckInclusionSizeAllowsUnderLimit(t *testing.T) {
	if err := checkInclusionSize(map[string][]byte{"ok.md": []byte("hi")}); err != nil {
		t.Fatal(err)
	}
}

type fakeInfo struct{ size int64 }

func (f fakeInfo) Name() string       { return "f" }
func (f fakeInfo) Size() int64        { return f.size }
func (f fakeInfo) Mode() os.FileMode  { return 0o600 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return nil }

func TestSizeGuardDoesNotReadOversizedFile(t *testing.T) {
	fs := &path.FS{
		Lstat: func(string) (os.FileInfo, error) {
			return fakeInfo{size: maxMemberBytes + 1}, nil
		},
		ReadFile: func(string) ([]byte, error) {
			t.Fatal("must not read a file larger than 16 MiB")
			return nil, nil
		},
	}
	bindSizeGuard(fs)
	_, err := fs.ReadFile("assets/big.bin")
	if err == nil {
		t.Fatal("expected refuse")
	}
	se, ok := err.(*Error)
	if !ok || se.Kind != KindTooLarge {
		t.Fatalf("got %v", err)
	}
}

func TestSizeGuardDoesNotSumDeclaredReadsTowardInclusionCap(t *testing.T) {
	reads := 0
	fs := &path.FS{
		Lstat: func(string) (os.FileInfo, error) {
			return fakeInfo{size: 12 << 20}, nil
		},
		ReadFile: func(string) ([]byte, error) {
			reads++
			return []byte("x"), nil
		},
	}
	bindSizeGuard(fs)
	for i := 0; i < 6; i++ {
		if _, err := fs.ReadFile("declared-not-included.bin"); err != nil {
			t.Fatalf("read guard must not apply the 64 MiB inclusion cap: %v", err)
		}
	}
	if reads != 6 {
		t.Fatalf("reads %d", reads)
	}
}
