package space_test

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/agent2host/agent2host/internal/space"
	"github.com/agent2host/agent2host/internal/space/registry"
)

func TestMain(m *testing.M) {
	if os.Getenv("A2H_LOCK_HELPER") != "" {
		os.Exit(runLockHelper())
	}
	os.Exit(m.Run())
}

func runLockHelper() int {
	home := os.Getenv("A2H_TEST_HOME")
	src := os.Getenv("A2H_TEST_SRC")
	sp, err := space.Open(home)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	rep, err := sp.Register(src)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(rep.SystemID, rep.Revision)
	return 0
}

func helperRegister(t *testing.T, home, src string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(),
		"A2H_LOCK_HELPER=1",
		"A2H_TEST_HOME="+home,
		"A2H_TEST_SRC="+src,
	)
	return cmd
}

func TestCrossProcessSameSystem(t *testing.T) {
	home := t.TempDir()
	src := copyTree(t, fixtureTree(t, "valid", "markdown-leading-dashes"))
	c1 := helperRegister(t, home, src)
	c2 := helperRegister(t, home, src)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, c := range []*exec.Cmd{c1, c2} {
		c := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := c.CombinedOutput()
			if err != nil {
				errs <- fmt.Errorf("%v: %s", err, out)
				return
			}
			errs <- nil
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("deadlock or timeout")
	}
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	reg, err := registry.New(home)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := reg.Get("fm")
	if err != nil {
		t.Fatal(err)
	}
	if rec.ActiveRevision == "" {
		t.Fatal("empty active revision")
	}
}

func TestCrossProcessDifferentSystems(t *testing.T) {
	home := t.TempDir()
	a := fixtureTree(t, "valid", "markdown-leading-dashes")
	b := fixtureTree(t, "valid", "env-example")
	c1 := helperRegister(t, home, a)
	c2 := helperRegister(t, home, b)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, c := range []*exec.Cmd{c1, c2} {
		c := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := c.CombinedOutput()
			if err != nil {
				errs <- fmt.Errorf("%v: %s", err, out)
				return
			}
			errs <- nil
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("deadlock or timeout")
	}
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	reg, err := registry.New(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Get("fm"); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Get("env-ex"); err != nil {
		t.Fatal(err)
	}
}
