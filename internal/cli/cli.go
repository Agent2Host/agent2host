package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/agent2host/agent2host/internal/adapter"
	"github.com/agent2host/agent2host/internal/adapter/committed"
	"github.com/agent2host/agent2host/internal/compatibility"
	"github.com/agent2host/agent2host/internal/runtime"
	"github.com/agent2host/agent2host/internal/source/decode"
	srcpath "github.com/agent2host/agent2host/internal/source/path"
	"github.com/agent2host/agent2host/internal/source/rule"
	"github.com/agent2host/agent2host/internal/space"
)

var checkHosts = committed.Default()

// SetCheckHostsForTest replaces the Host registry used by check. Tests only.
func SetCheckHostsForTest(r *adapter.Registry) func() {
	prev := checkHosts
	checkHosts = r
	return func() { checkHosts = prev }
}

// Main runs the a2h CLI. args[0] is the program name.
func Main(args []string, stdout, stderr io.Writer) int {
	f, err := parseArgs(args[1:])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUsage
	}
	if f.printHelp || f.cmd == "help" {
		topic := f.cmd
		if topic == "help" && len(f.rest) == 1 {
			topic = f.rest[0]
		}
		if topic == "" || topic == "help" {
			return cmdHelp(stdout)
		}
		if err := cmdHelpTopic(stdout, topic); err != nil {
			fmt.Fprintln(stderr, err)
			return ExitUsage
		}
		return ExitOK
	}
	if f.jsonOut && f.verbose {
		fmt.Fprintln(stderr, "--json and --verbose cannot be used together")
		return ExitUsage
	}
	if f.verbose && f.cmd != "run" {
		fmt.Fprintln(stderr, "--verbose is only valid with run")
		return ExitUsage
	}
	if err := validateCommandFlags(f); err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUsage
	}
	if f.printVersion || f.cmd == "version" {
		if f.cmd != "" && f.cmd != "version" {
			fmt.Fprintln(stderr, "usage: a2h version")
			return ExitUsage
		}
		if len(f.rest) != 0 {
			fmt.Fprintln(stderr, "usage: a2h version")
			return ExitUsage
		}
		return cmdVersion(f.jsonOut, stdout, stderr)
	}
	home, err := resolveHome(f.home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	switch f.cmd {
	case "register":
		if len(f.rest) != 1 {
			fmt.Fprintln(stderr, "usage: a2h register <system-source-dir>")
			return ExitUsage
		}
		return cmdRegister(home, f.rest[0], f.jsonOut, stdout, stderr)
	case "list":
		if len(f.rest) != 0 {
			fmt.Fprintln(stderr, "usage: a2h list")
			return ExitUsage
		}
		return cmdList(home, f.jsonOut, stdout, stderr)
	case "inspect":
		if len(f.rest) != 1 {
			fmt.Fprintln(stderr, "usage: a2h inspect <system-id>/<agent-id>")
			return ExitUsage
		}
		return cmdInspect(home, f.rest[0], f.jsonOut, stdout, stderr)
	case "check":
		if len(f.rest) != 1 || f.host == "" {
			fmt.Fprintln(stderr, "usage: a2h check <system-id>/<agent-id> --host <host> [--project dir]")
			return ExitUsage
		}
		return cmdCheck(home, f.rest[0], f.host, f.project, f.requireStrictRead, f.jsonOut, stdout, stderr)
	case "run":
		if len(f.rest) < 1 || f.host == "" {
			fmt.Fprintln(stderr, "usage: a2h run <system-id>/<agent-id> --host <host> [--project dir] [--verbose] [--accept-warnings] [-- <native args>]")
			return ExitUsage
		}
		return cmdRun(home, f.rest[0], f.rest[1:], f.host, f.project, f.requireStrictRead, f.jsonOut, f.verbose, f.acceptWarnings, stdout, stderr)
	case "remove":
		if len(f.rest) != 1 {
			fmt.Fprintln(stderr, "usage: a2h remove <system-id>")
			return ExitUsage
		}
		return cmdRemove(home, f.rest[0], f.jsonOut, stdout, stderr)
	case "clean":
		if len(f.rest) != 0 {
			fmt.Fprintln(stderr, "usage: a2h clean [--runtime] [--quarantine] [--host-state --host <id>] [--dry-run]")
			return ExitUsage
		}
		if f.hostState && f.host == "" {
			fmt.Fprintln(stderr, "clean --host-state requires --host")
			return ExitUsage
		}
		return cmdClean(home, runtime.CleanOpts{
			Runtime:    f.runtimeScope,
			Quarantine: f.quarantine,
			HostState:  f.hostState,
			Host:       f.host,
			DryRun:     f.dryRun,
		}, f.jsonOut, stdout, stderr)
	case "resolve":
		// Hidden development command; always JSON.
		if len(f.rest) != 1 {
			fmt.Fprintln(stderr, "usage: a2h resolve <system-id>/<agent-id>")
			return ExitUsage
		}
		return cmdResolve(home, f.rest[0], stdout, stderr)
	case "":
		fmt.Fprintln(stderr, "usage: a2h [--home dir] [--json] register|list|inspect|check|run|remove|clean|version ...")
		return ExitUsage
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", f.cmd)
		return ExitUsage
	}
}

func cmdHelp(stdout io.Writer) int {
	fmt.Fprint(stdout, `Agent2Host — register an Agent System and run a named agent on a host.

Usage:
  a2h [--home dir] [--json] <command> ...

Commands:
  register <system-source-dir>
  list
  inspect <system-id>/<agent-id>
  check   <system-id>/<agent-id> --host <host> [--project dir] [--require-strict-read]
  run     <system-id>/<agent-id> --host <host> [--project dir] [--verbose] [--accept-warnings]
  remove  <system-id>
  clean   [--runtime] [--quarantine] [--host-state --host <id>] [--dry-run]
  version
  help

Hosts: claude-code, kiro, codex

The host working folder is the work root declared by the Agent System,
not the folder you registered from. See the README.

`)
	return ExitOK
}

func cmdHelpTopic(stdout io.Writer, topic string) error {
	text, ok := commandHelp[topic]
	if !ok {
		return fmt.Errorf("unknown command %q", topic)
	}
	fmt.Fprint(stdout, text)
	return nil
}

var commandHelp = map[string]string{
	"register": `Usage:
  a2h register <system-source-dir>

Stores a snapshot of the Agent System folder. Edit the files, then register again.
`,
	"list": `Usage:
  a2h list

Lists registered systems. The id is from system.json, not the folder name.
`,
	"inspect": `Usage:
  a2h inspect <system-id>/<agent-id>
`,
	"check": `Usage:
  a2h check <system-id>/<agent-id> --host <host> [--project dir] [--require-strict-read]

Reports whether that host can start this agent. Does not launch the host.
Does not create a missing fixed work root.
`,
	"run": `Usage:
  a2h run <system-id>/<agent-id> --host <host> [--project dir] [--verbose] [--accept-warnings]

Starts the host in the work root declared by the Agent System.
--project is only valid when work_root.mode is invocation.
`,
	"remove": `Usage:
  a2h remove <system-id>
`,
	"clean": `Usage:
  a2h clean [--runtime] [--quarantine] [--host-state --host <id>] [--dry-run]
`,
	"version": `Usage:
  a2h version
`,
}

func resolveHome(flagHome string) (string, error) {
	if flagHome != "" {
		return srcpath.ResolveRoot(flagHome)
	}
	if v := os.Getenv("A2H_HOME"); v != "" {
		return srcpath.ResolveRoot(v)
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return srcpath.ResolveRoot(filepath.Join(h, ".a2h"))
}

type parsed struct {
	home, host, cmd, project                    string
	rest                                        []string
	jsonOut, verbose, acceptWarnings            bool
	requireStrictRead                           bool
	runtimeScope, quarantine, hostState, dryRun bool
	printVersion                                bool
	printHelp                                   bool
}

func parseArgs(args []string) (parsed, error) {
	var f parsed
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--home":
			if i+1 >= len(args) {
				return parsed{}, fmt.Errorf("missing --home value")
			}
			i++
			f.home = args[i]
		case strings.HasPrefix(a, "--home="):
			f.home = strings.TrimPrefix(a, "--home=")
		case a == "--host":
			if i+1 >= len(args) {
				return parsed{}, fmt.Errorf("missing --host value")
			}
			i++
			f.host = args[i]
		case strings.HasPrefix(a, "--host="):
			f.host = strings.TrimPrefix(a, "--host=")
		case a == "--project":
			if i+1 >= len(args) {
				return parsed{}, fmt.Errorf("missing --project value")
			}
			i++
			f.project = args[i]
		case strings.HasPrefix(a, "--project="):
			f.project = strings.TrimPrefix(a, "--project=")
		case a == "--help", a == "-h":
			f.printHelp = true
		case a == "--version":
			f.printVersion = true
		case a == "--json":
			f.jsonOut = true
		case a == "--verbose":
			f.verbose = true
		case a == "--accept-warnings":
			f.acceptWarnings = true
		case a == "--require-strict-read":
			f.requireStrictRead = true
		case a == "--runtime":
			f.runtimeScope = true
		case a == "--quarantine":
			f.quarantine = true
		case a == "--host-state":
			f.hostState = true
		case a == "--dry-run":
			f.dryRun = true
		case a == "--force", a == "--yes":
			return parsed{}, fmt.Errorf("unknown flag %s (use --accept-warnings)", a)
		case a == "--":
			pos = append(pos, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "-"):
			return parsed{}, fmt.Errorf("unknown flag %s", a)
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) == 0 {
		return f, nil
	}
	f.cmd = pos[0]
	f.rest = pos[1:]
	return f, nil
}

func validateCommandFlags(f parsed) error {
	cmd := f.cmd
	if f.printHelp && (cmd == "" || cmd == "help") {
		cmd = "help"
	}
	if f.printVersion && (cmd == "" || cmd == "version") {
		cmd = "version"
	}
	switch cmd {
	case "check":
		return unusedFlags(cmd, map[string]bool{
			"--accept-warnings": f.acceptWarnings,
			"--runtime":         f.runtimeScope,
			"--quarantine":      f.quarantine,
			"--host-state":      f.hostState,
			"--dry-run":         f.dryRun,
		})
	case "run":
		return unusedFlags(cmd, map[string]bool{
			"--runtime":    f.runtimeScope,
			"--quarantine": f.quarantine,
			"--host-state": f.hostState,
			"--dry-run":    f.dryRun,
		})
	case "clean":
		if f.host != "" && !f.hostState {
			return fmt.Errorf("--host is only valid with clean --host-state")
		}
		return unusedFlags(cmd, map[string]bool{
			"--accept-warnings":     f.acceptWarnings,
			"--require-strict-read": f.requireStrictRead,
			"--project":             f.project != "",
		})
	case "register", "list", "inspect", "remove", "resolve", "version", "help", "":
		if f.host != "" {
			return unusedFlagMessage(cmd, "--host")
		}
		return unusedFlags(cmd, map[string]bool{
			"--accept-warnings":     f.acceptWarnings,
			"--require-strict-read": f.requireStrictRead,
			"--project":             f.project != "",
			"--runtime":             f.runtimeScope,
			"--quarantine":          f.quarantine,
			"--host-state":          f.hostState,
			"--dry-run":             f.dryRun,
		})
	default:
		return nil
	}
}

func unusedFlags(cmd string, flags map[string]bool) error {
	var names []string
	for name, set := range flags {
		if set {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil
	}
	return unusedFlagMessage(cmd, names[0])
}

func unusedFlagMessage(cmd, flag string) error {
	if cmd == "" {
		return fmt.Errorf("%s is not valid here", flag)
	}
	return fmt.Errorf("%s is not valid with %s", flag, cmd)
}

func open(home string) (*space.Space, error) {
	return space.Open(home)
}

func resolveAndPrintWorkRoot(run *space.ResolvedAgentRun, project string, stderr io.Writer) (runtime.WorkRootResolution, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return runtime.WorkRootResolution{}, err
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return runtime.WorkRootResolution{}, err
	}
	res, err := runtime.ResolveRunWorkRoot(run, project, cwd, userHome)
	if err != nil {
		return runtime.WorkRootResolution{}, err
	}
	if res.Mode == decode.WorkRootFixed && !res.Exists {
		fmt.Fprintf(stderr, "This Agent System will use or create:\n%s\n", res.Path)
	} else {
		fmt.Fprintf(stderr, "Work root (%s): %s\n", res.Mode, res.Path)
	}
	return res, nil
}

func exitWorkRoot(err error) int {
	if errors.Is(err, runtime.ErrWorkRootProject) || errors.Is(err, runtime.ErrWorkRootBadRel) {
		return ExitUsage
	}
	return ExitPrecondition
}

func cmdRegister(home, dir string, jsonOut bool, stdout, stderr io.Writer) int {
	sp, err := open(home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	rep, err := sp.Register(dir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitSpaceOrRegistry(err)
	}
	for _, w := range rep.Warnings {
		if w.Detail != "" {
			fmt.Fprintf(stderr, "warning: %s (%s)\n", w.Detail, w.ID)
		} else {
			fmt.Fprintf(stderr, "warning: %s\n", w.ID)
		}
	}
	if jsonOut {
		warnings := rep.Warnings
		if warnings == nil {
			warnings = []rule.Warning{}
		}
		return writeJSON(stdout, stderr, map[string]any{
			"system_id":         rep.SystemID,
			"version":           rep.Version,
			"artifact_revision": rep.Revision,
			"agents":            rep.Agents,
			"warnings":          warnings,
		})
	}
	fmt.Fprintf(stdout, "registered %s %s (%d agents)\n", rep.SystemID, rep.Revision, len(rep.Agents))
	return 0
}

func cmdList(home string, jsonOut bool, stdout, stderr io.Writer) int {
	sp, err := open(home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	rows, err := sp.List()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	if jsonOut {
		if rows == nil {
			rows = []space.ListedSystem{}
		}
		return writeJSON(stdout, stderr, map[string]any{"systems": rows})
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tVERSION\tAGENTS\tREVISION")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", r.ID, r.Version, r.AgentCount, r.ActiveRevision)
	}
	_ = w.Flush()
	return 0
}

func cmdInspect(home, target string, jsonOut bool, stdout, stderr io.Writer) int {
	sys, agent, err := space.ParseTarget(target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitParseTarget(err)
	}
	sp, err := open(home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	ins, err := sp.Inspect(sys, agent)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitSpaceOrRegistry(err)
	}
	if jsonOut {
		return writeJSON(stdout, stderr, ins)
	}
	fmt.Fprintf(stdout, "system:   %s\n", ins.SystemID)
	fmt.Fprintf(stdout, "agent:    %s\n", ins.AgentID)
	fmt.Fprintf(stdout, "version:  %s\n", ins.Version)
	fmt.Fprintf(stdout, "revision: %s\n", ins.ArtifactRevision)
	fmt.Fprintf(stdout, "source:   %s\n", ins.Source)
	fmt.Fprintf(stdout, "work root: %s", ins.WorkRoot.Mode)
	if ins.WorkRoot.PathFromHome != "" {
		fmt.Fprintf(stdout, " (%s)", ins.WorkRoot.PathFromHome)
	}
	fmt.Fprintln(stdout)
	if len(ins.Skills) > 0 {
		fmt.Fprintf(stdout, "skills:   %s\n", strings.Join(ins.Skills, ", "))
	}
	return 0
}

func cmdResolve(home, target string, stdout, stderr io.Writer) int {
	sys, agent, err := space.ParseTarget(target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitParseTarget(err)
	}
	sp, err := open(home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	run, err := sp.Resolve(sys, agent, "")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitSpaceOrRegistry(err)
	}
	return writeJSON(stdout, stderr, run)
}

const executionContract = "agent2host/execution-contract/v1alpha1"

func cmdCheck(home, target, host, project string, requireStrictRead, jsonOut bool, stdout, stderr io.Writer) int {
	sys, agent, err := space.ParseTarget(target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitParseTarget(err)
	}
	sp, err := open(home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	run, err := sp.Resolve(sys, agent, "")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitSpaceOrRegistry(err)
	}
	policy := adapter.RunPolicy{RequireStrictRead: requireStrictRead}
	if _, err := resolveAndPrintWorkRoot(run, project, stderr); err != nil {
		fmt.Fprintln(stderr, err)
		return exitWorkRoot(err)
	}
	pctx, _, err := runtime.PrepareContext(home, "", runtime.ContextCheck)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	out, err := adapter.RunPipeline(checkHosts, host, run, pctx, Version, policy)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitProject(err)
	}
	if jsonOut {
		if code := writeJSON(stdout, stderr, out.Report); code != ExitOK {
			return code
		}
	} else {
		fmt.Fprintf(stdout, "decision:   %s\n", out.Report.Decision)
		fmt.Fprintf(stdout, "activation: %s\n", out.Report.Activation.Mode)
		fmt.Fprintf(stdout, "host:       %s %s\n", out.Report.Host.ID, out.Report.Host.Version)
		fmt.Fprintf(stdout, "subject:    %s/%s\n", out.Report.Subject.SystemID, out.Report.Subject.AgentID)
		if out.Plans != nil {
			fmt.Fprintf(stdout, "plans:      %d files (not materialized)\n", len(out.Plans.Projection.Files))
		}
	}
	printReadProtection(stderr, requireStrictRead, host, out.Report, pctx)
	if !adapter.HostVersionVerified(host, out.Report.Host.Version) && out.Report.Host.Version != "" && out.Report.Host.Version != "unknown" {
		fmt.Fprintf(stderr, "host version %q is not in the adapter verified set; mappings may differ\n", out.Report.Host.Version)
	}
	if out.Report.Decision == "refused" {
		return ExitRefused
	}
	return ExitOK
}

func cmdRun(home, target string, nativeArgs []string, host, project string, requireStrictRead, jsonOut, verbose, acceptWarnings bool, stdout, stderr io.Writer) int {
	if err := runtime.RejectNativeArgs(nativeArgs); err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUsage
	}
	sys, agent, err := space.ParseTarget(target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitParseTarget(err)
	}
	sp, err := open(home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	run, err := sp.Resolve(sys, agent, "")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitSpaceOrRegistry(err)
	}
	recovered, err := runtime.RecoverLeftovers(home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	for _, id := range recovered.Quarantined {
		fmt.Fprintf(stderr, "A previous run could not be cleaned safely and was set aside (%s). Use a2h clean --quarantine to delete it.\n", id)
	}
	root, err := resolveAndPrintWorkRoot(run, project, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitWorkRoot(err)
	}
	pctx, reserved, err := runtime.PrepareContext(home, root.Path, runtime.ContextRun)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	auth, report, err := AuthorizeExecute(AuthorizeOpts{
		Registry:          checkHosts,
		Host:              host,
		Run:               run,
		Pctx:              pctx,
		Agent2HostVersion: Version,
		AcceptWarnings:    acceptWarnings,
		RequireStrictRead: requireStrictRead,
		TTY:               stdinIsTTY(),
		Stdin:             os.Stdin,
		Stderr:            stderr,
		ShowStartNotice:   !jsonOut,
	})
	if err != nil {
		if jsonOut && (report.Decision == "refused" || report.SchemaVersion != "") {
			_ = writeJSON(stdout, stderr, runJSON{SchemaVersion: executionContract, Kind: "run", Report: report})
		} else if !jsonOut {
			renderRunAuthorizationFailure(stdout, stderr, report, err, verbose)
		}
		return exitAuthorize(err)
	}
	root, err = runtime.EnsureWorkRoot(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitWorkRoot(err)
	}
	prep, err := runtime.BeginRun(reserved)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	hostOut, hostErr := stdout, stderr
	if jsonOut {
		hostOut, hostErr = stderr, stderr
	}
	var hostState adapter.HostStateBinder
	if a, selErr := checkHosts.Select(host); selErr == nil {
		hostState = a.HostState()
	}
	out, err := runtime.Execute(prep, auth, report, runtime.ExecOpts{
		Stdin:     os.Stdin,
		Stdout:    hostOut,
		Stderr:    hostErr,
		HostState: hostState,
		OnHostStarting: func() {
			if !jsonOut {
				fmt.Fprintf(stdout, "Starting %s in %s…\n", agent, hostDisplayName(host))
			}
		},
		OnHostStateBindFailure: func() {
			if !jsonOut {
				fmt.Fprintln(stderr, "Saved Host sign-in state could not be loaded. You may need to sign in again.")
			}
		},
		OnHostStateSaveFailure: func() {
			// renderRunOutcome reports this once, after the Host exits.
		},
	})
	if jsonOut {
		if code := writeJSON(stdout, stderr, runJSON{
			SchemaVersion: executionContract,
			Kind:          "run",
			Report:        report,
			Outcome:       &out,
		}); code != ExitOK {
			return code
		}
	} else {
		renderRunOutcome(stdout, stderr, report, out, err, verbose)
	}
	return exitExecute(out, err)
}

type runJSON struct {
	SchemaVersion string               `json:"schema_version"`
	Kind          string               `json:"kind"`
	Report        compatibility.Report `json:"report"`
	Outcome       *runtime.Outcome     `json:"outcome,omitempty"`
}

func cmdRemove(home, systemID string, jsonOut bool, stdout, stderr io.Writer) int {
	sp, err := open(home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitPrecondition
	}
	if err := sp.Remove(systemID); err != nil {
		fmt.Fprintln(stderr, err)
		return exitSpaceOrRegistry(err)
	}
	if err := runtime.InvalidateSystem(home, systemID); err != nil {
		fmt.Fprintf(stderr, "removed %s, but leftover run files could not be cleaned. Use a2h clean.\n", systemID)
		return ExitPrecondition
	}
	if jsonOut {
		return writeJSON(stdout, stderr, map[string]any{"removed": systemID})
	}
	fmt.Fprintf(stdout, "removed %s\n", systemID)
	return ExitOK
}

func cmdClean(home string, opts runtime.CleanOpts, jsonOut bool, stdout, stderr io.Writer) int {
	res, err := runtime.Clean(home, opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		if errors.Is(err, runtime.ErrHostStateNeedsHost) ||
			errors.Is(err, runtime.ErrUnknownHost) ||
			errors.Is(err, runtime.ErrPathEscape) {
			return ExitUsage
		}
		return ExitPrecondition
	}
	if opts.HostState && !opts.DryRun && len(res.Paths) > 0 {
		fmt.Fprintln(stderr, "Deleted Agent2Host's saved Host state copy only. This does not sign you out of the Host product.")
	}
	if jsonOut {
		return writeJSON(stdout, stderr, map[string]any{"dry_run": opts.DryRun, "paths": res.Paths})
	}
	if opts.DryRun {
		if len(res.Paths) == 0 {
			fmt.Fprintln(stdout, "nothing to delete")
			return ExitOK
		}
		fmt.Fprintln(stdout, "would delete:")
		for _, p := range res.Paths {
			fmt.Fprintln(stdout, p)
		}
		return ExitOK
	}
	fmt.Fprintf(stdout, "deleted %d items\n", len(res.Paths))
	return ExitOK
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func writeJSON(stdout, stderr io.Writer, v any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(stderr, err)
		return ExitInternal
	}
	return ExitOK
}
