# CLI reference

Run `a2h help` for the command overview, or `a2h <command> --help` for one command. These are the user-facing commands in the current alpha.

```text
a2h [--home <directory>] [--json] <command> ...
```

## Global options

| Option | Meaning |
| --- | --- |
| `--home <directory>` | Use a different local Agent Space instead of `~/.a2h`. |
| `A2H_HOME` | Environment-variable alternative to `--home`. |
| `--json` | Print machine-readable output where that command supports it. |

Use the same `--home` value consistently for `register`, `list`, `check`, and `run`. A separately chosen home is a separate local registry.

## Register

```bash
a2h register <system-source-directory>
```

Validates the System and records a pinned snapshot. Run it again after changing any referenced source file.

## List and inspect

```bash
a2h list
a2h inspect <system-id>/<agent-id>
```

`list` shows registered Systems. `inspect` shows the selected Agent in the active snapshot, including its System version, revision, work-root mode, and attached skills.

## Check

```bash
a2h check <system-id>/<agent-id> \
  --host <claude-code|kiro|codex> \
  [--project <existing-directory>] \
  [--require-strict-read]
```

Checks whether the selected host can start that Agent. It does not start a host or materialize a host configuration.

`--project` is valid only when the System uses current-project work-root mode. `--require-strict-read` asks Agent2Host to refuse unless it can make the stronger filesystem-read promise; see [Supported hosts and limitations](hosts-and-limits.md).

## Run

```bash
a2h run <system-id>/<agent-id> \
  --host <claude-code|kiro|codex> \
  [--project <existing-directory>] \
  [--require-strict-read] \
  [--verbose] \
  [--accept-warnings]
```

Prepares the native host for one Agent and starts it in the resolved work root.

- `--project` applies only to current-project mode.
- `--require-strict-read` refuses launch unless the stronger read-boundary requirement holds.
- `--verbose` prints run details after the session ends.
- `--accept-warnings` accepts an `allowed_with_warnings` result for non-interactive scripts. Interactive terminals normally ask `Start this session? [y/N]`.

The current alpha does **not** forward arbitrary native-host arguments. Do not append `-- <host arguments>`; the command rejects them intentionally.

## Remove a System

```bash
a2h remove <system-id>
```

Removes the registered snapshot. It does not delete the original Agent System folder or user files in a work root.

## Clean local leftovers

```bash
a2h clean [--runtime] [--quarantine] [--host-state --host <id>] [--dry-run]
```

Use `a2h clean` to handle leftover local run material after an interrupted or incomplete cleanup. Start with `--dry-run` to see paths before deletion.

## Version

```bash
a2h version
```

Prints the CLI version, commit, and build time. Release binaries report their release version; local source builds report `0.0.0-dev`.

## Exit status

For scripts, the stable broad outcomes are:

| Code | Meaning |
| --- | --- |
| `0` | Completed successfully. |
| `1` | The launch was refused or warnings were not accepted. |
| `2` | Command syntax or option usage was invalid. |
| `3` | A required local condition was missing or invalid. |
| `4` | The host process started but exited unsuccessfully. |
| `70` | Agent2Host itself encountered an internal failure. |
