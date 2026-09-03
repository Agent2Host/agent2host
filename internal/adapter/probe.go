package adapter

import (
	"bytes"
	"os/exec"
	"strings"
)

// LookPathFunc locates a Host executable. Tests inject a fake.
type LookPathFunc func(file string) (string, error)

// VersionFunc reads a Host version string from an executable.
type VersionFunc func(exe string) (string, error)

func defaultLookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func defaultVersion(exe string) (string, error) {
	cmd := exec.Command(exe, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(bytes.SplitN(out, []byte("\n"), 2)[0]))
	if line == "" {
		return "unknown", nil
	}
	return line, nil
}

func ProbeBinary(hostID, bin string, look LookPathFunc, version VersionFunc) (ProbeResult, error) {
	if look == nil {
		look = defaultLookPath
	}
	if version == nil {
		version = defaultVersion
	}
	p := ProbeResult{HostID: hostID}
	exe, err := look(bin)
	if err != nil || exe == "" {
		p.HostVersion = "unknown"
		p.Fingerprint = Fingerprint(hostID, "missing", bin)
		return p, nil
	}
	p.Found = true
	p.Executable = normalizeExe(exe)
	ver, err := version(p.Executable)
	if err != nil || ver == "" {
		ver = "unknown"
	}
	p.HostVersion = ver
	p.Fingerprint = Fingerprint(hostID, p.HostVersion, p.Executable)
	return p, nil
}
