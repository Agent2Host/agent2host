package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/compatibility"
)

const (
	runsDirName       = "runs"
	recordsDirName    = "records"
	quarantineDirName = "quarantine"
	recoveredMarkName = "recovered"
	finalizedMarkName = "finalized"
	begunMarkName     = "begun"

	recordRetainDays  = 30
	recordRetainCount = 1000
)

// RecoverReport is what leftover recovery did before a new run starts.
type RecoverReport struct {
	Recovered   []string
	Quarantined []string
}

// CleanOpts selects which home classes clean may touch.
type CleanOpts struct {
	Runtime    bool
	Quarantine bool
	HostState  bool
	Host       string
	DryRun     bool
}

// CleanResult lists paths that were deleted, or would be deleted.
type CleanResult struct {
	Paths []string `json:"paths"`
}

func recordsDir(home string) string { return filepath.Join(home, recordsDirName) }
func runsDir(home string) string    { return filepath.Join(home, runsDirName) }
func quarantineDir(home string) string {
	return filepath.Join(home, quarantineDirName)
}
func recordPath(home, runID string) string {
	return filepath.Join(recordsDir(home), runID+".json")
}

func writeRecord(p Prepared, r compatibility.Report, out Outcome, runErr error) error {
	rec := Record{
		RunID:             p.RunID,
		SystemID:          r.Subject.SystemID,
		AgentID:           r.Subject.AgentID,
		Revision:          r.Subject.Revision,
		HostID:            r.Host.ID,
		HostVersion:       r.Host.Version,
		Decision:          r.Decision,
		Fingerprint:       r.Probe.Fingerprint,
		AdapterVersion:    r.Adapter.Version,
		Agent2HostVersion: r.Agent2HostVersion,
		Class:             out.Class,
		ExitCode:          out.ExitCode,
		Stage:             out.Stage,
		OmittedSecrets:    out.Omitted,
		RecordedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	if runErr != nil {
		rec.Error = runErr.Error()
	}
	body, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(recordsDir(p.Home), runtimeDirMode); err != nil {
		return err
	}
	return os.WriteFile(recordPath(p.Home, p.RunID), body, runtimeFileMode)
}

func deleteRunWorkspace(p Prepared) error {
	if p.Root == "" {
		return nil
	}
	return os.RemoveAll(p.Root)
}

func quarantineRun(p Prepared) error {
	id := p.RunID
	if id == "" {
		id = filepath.Base(p.Root)
	}
	dest := filepath.Join(quarantineDir(p.Home), id)
	if err := os.MkdirAll(filepath.Dir(dest), runtimeDirMode); err != nil {
		return err
	}
	if _, err := os.Stat(dest); err == nil {
		dest = dest + "-" + time.Now().UTC().Format("20060102T150405")
	}
	return os.Rename(p.Root, dest)
}

// RecoverLeftovers restores crash leftovers or moves failed recovery to
// quarantine. A missing secret baseline may be deleted; any other inspect
// error is isolated, not treated as "no baseline".
func RecoverLeftovers(home string) (RecoverReport, error) {
	var rep RecoverReport
	entries, err := os.ReadDir(runsDir(home))
	if err != nil {
		if os.IsNotExist(err) {
			return rep, nil
		}
		return rep, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		root := filepath.Join(runsDir(home), e.Name())
		if runIsLive(root) {
			continue
		}
		p := Prepared{
			RunID:     e.Name(),
			Home:      home,
			Root:      root,
			Workspace: filepath.Join(root, "workspace"),
			Private:   filepath.Join(root, "private"),
		}
		base := filepath.Join(root, "secret-baseline")
		_, baseErr := os.Stat(base)
		if baseErr == nil {
			if err := restoreRunBaselines(p, base); err != nil {
				if qerr := quarantineAfterRun(p); qerr != nil {
					return rep, fmt.Errorf("%w: %v", ErrQuarantine, qerr)
				}
				rep.Quarantined = append(rep.Quarantined, e.Name())
				continue
			}
			if err := markRecovered(root); err != nil {
				if qerr := quarantineAfterRun(p); qerr != nil {
					return rep, fmt.Errorf("%w: %v", ErrQuarantine, qerr)
				}
				rep.Quarantined = append(rep.Quarantined, e.Name())
				continue
			}
			rep.Recovered = append(rep.Recovered, e.Name())
			continue
		}
		if !os.IsNotExist(baseErr) {
			if qerr := quarantineAfterRun(p); qerr != nil {
				return rep, fmt.Errorf("%w: %v", ErrQuarantine, qerr)
			}
			rep.Quarantined = append(rep.Quarantined, e.Name())
			continue
		}
		if err := deleteRunWorkspace(p); err != nil {
			if qerr := quarantineAfterRun(p); qerr != nil {
				return rep, fmt.Errorf("%w: %v", ErrQuarantine, qerr)
			}
			rep.Quarantined = append(rep.Quarantined, e.Name())
			continue
		}
		rep.Recovered = append(rep.Recovered, e.Name())
	}
	return rep, nil
}

func restoreRunBaselines(p Prepared, base string) error {
	return filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		if len(parts) != 2 {
			return nil
		}
		class := adapter.DestinationClass(parts[0])
		fileRel := parts[1]
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		destRoot, err := rootFor(p, class)
		if err != nil {
			if class == adapter.DestAuthProfile {
				return nil
			}
			return err
		}
		dest, err := joinUnder(destRoot, fileRel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), runtimeDirMode); err != nil {
			return err
		}
		return os.WriteFile(dest, body, 0o600)
	})
}

func restoreStaleSecretBaselines(home string) error {
	_, err := RecoverLeftovers(home)
	return err
}

// Clean applies the selected scopes. Default / --runtime never deletes
// quarantine, Host state, registered systems, or live-locked runs.
func Clean(home string, opts CleanOpts) (CleanResult, error) {
	if !opts.Runtime && !opts.Quarantine && !opts.HostState {
		opts.Runtime = true
	}
	var paths []string
	if opts.Runtime {
		runs, err := leftoverRunPaths(home)
		if err != nil {
			return CleanResult{}, err
		}
		paths = append(paths, runs...)
		expired, err := expiredRecordPaths(home)
		if err != nil {
			return CleanResult{}, err
		}
		paths = append(paths, expired...)
	}
	if opts.Quarantine {
		q, err := listDirPaths(quarantineDir(home))
		if err != nil {
			return CleanResult{}, err
		}
		paths = append(paths, q...)
	}
	if opts.HostState {
		hostDir, err := hostStateDir(home, opts.Host)
		if err != nil {
			return CleanResult{}, err
		}
		if _, err := os.Stat(hostDir); err == nil {
			paths = append(paths, hostDir)
		} else if !os.IsNotExist(err) {
			return CleanResult{}, err
		}
	}
	sort.Strings(paths)
	if opts.DryRun {
		return CleanResult{Paths: paths}, nil
	}
	for _, p := range paths {
		if err := os.RemoveAll(p); err != nil {
			return CleanResult{Paths: paths}, err
		}
	}
	return CleanResult{Paths: paths}, nil
}

// InvalidateSystem deletes Runtime leftovers for one system. It does not
// touch Host state or other systems.
func InvalidateSystem(home, systemID string) error {
	recs, err := listRecords(home)
	if err != nil {
		return err
	}
	for _, rec := range recs {
		if rec.rec.SystemID != systemID {
			continue
		}
		root := filepath.Join(runsDir(home), rec.rec.RunID)
		if runIsLive(root) {
			continue
		}
		_ = os.RemoveAll(root)
		if err := os.Remove(rec.path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	entries, err := os.ReadDir(runsDir(home))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		root := filepath.Join(runsDir(home), e.Name())
		if runIsLive(root) {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, "record.json"))
		if err != nil {
			continue
		}
		var rec Record
		if json.Unmarshal(body, &rec) != nil || rec.SystemID != systemID {
			continue
		}
		if err := os.RemoveAll(root); err != nil {
			return err
		}
	}
	return nil
}

func leftoverRunPaths(home string) ([]string, error) {
	return listDirPathsFiltered(runsDir(home), func(path string) bool {
		if runIsLive(path) {
			return false
		}
		return runHasMark(path, recoveredMarkName) || runHasMark(path, finalizedMarkName)
	})
}

func markBegun(root string) error     { return markRun(root, begunMarkName) }
func markRecovered(root string) error { return markRun(root, recoveredMarkName) }
func markFinalized(root string) error { return markRun(root, finalizedMarkName) }

func markRun(root, name string) error {
	if root == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(root, name), []byte("1\n"), runtimeFileMode)
}

func runHasMark(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}

func hostStateDir(home, host string) (string, error) {
	if host == "" {
		return "", ErrHostStateNeedsHost
	}
	if !adapter.SupportedHost(host) {
		return "", fmt.Errorf("%w: %s", ErrUnknownHost, host)
	}
	root := filepath.Join(home, adapter.AuthProfilesDirName)
	dest := filepath.Join(root, host)
	if err := pathInside(root, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func pathInside(root, dest string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	if !strictChild(absRoot, absDest) {
		return fmt.Errorf("%w: host state path escapes %s", ErrPathEscape, absRoot)
	}
	if resolvedDest, err := filepath.EvalSymlinks(absDest); err == nil {
		resolvedRoot := absRoot
		if r, err := filepath.EvalSymlinks(absRoot); err == nil {
			resolvedRoot = r
		}
		if !strictChild(resolvedRoot, resolvedDest) {
			return fmt.Errorf("%w: host state path escapes %s", ErrPathEscape, resolvedRoot)
		}
	}
	return nil
}

func strictChild(root, dest string) bool {
	rel, err := filepath.Rel(root, dest)
	if err != nil || rel == "." || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func listDirPaths(dir string) ([]string, error) {
	return listDirPathsFiltered(dir, func(string) bool { return true })
}

func listDirPathsFiltered(dir string, keep func(string) bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if !keep(p) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

type storedRecord struct {
	path string
	rec  Record
	at   time.Time
}

func listRecords(home string) ([]storedRecord, error) {
	entries, err := os.ReadDir(recordsDir(home))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []storedRecord
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		p := filepath.Join(recordsDir(home), e.Name())
		body, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var rec Record
		if json.Unmarshal(body, &rec) != nil {
			continue
		}
		at, err := time.Parse(time.RFC3339, rec.RecordedAt)
		if err != nil {
			at = time.Time{}
		}
		out = append(out, storedRecord{path: p, rec: rec, at: at})
	}
	return out, nil
}

func expiredRecordPaths(home string) ([]string, error) {
	recs, err := listRecords(home)
	if err != nil {
		return nil, err
	}
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].at.After(recs[j].at)
	})
	cutoff := time.Now().UTC().Add(-recordRetainDays * 24 * time.Hour)
	var expired []string
	kept := 0
	for _, rec := range recs {
		if kept < recordRetainCount && !rec.at.Before(cutoff) {
			kept++
			continue
		}
		expired = append(expired, rec.path)
	}
	return expired, nil
}
