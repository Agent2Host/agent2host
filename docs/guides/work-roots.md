# Choose a work root

The work root is the folder in which the native host opens the agent. It is where the agent sees the user's project or archive and where user-facing files belong.

It is not:

- the directory from which you register the Agent System;
- Agent Space (`~/.a2h`), which stores registered snapshots;
- Agent2Host's temporary run workspace;
- necessarily the directory from which you type `a2h run`.

Every new `agent2host/v1alpha2` System must choose one mode in `system.json`.

## Current-project mode

Use `invocation` for an agent that works on whichever project the person chooses today: a code reviewer, test runner, or repository assistant.

```json
"work_root": { "mode": "invocation" }
```

Run it by naming an existing project:

```bash
a2h run demo-system/demo \
  --host claude-code \
  --project /absolute/path/to/project
```

If you omit `--project`, Agent2Host uses the directory from which you invoke `a2h`. This is the same familiar model as opening a coding host in a project directory.

## Fixed-archive mode

Use `fixed` for a System that owns a durable archive: event planning, research notes, reports, or an operating record. It makes the directory choice part of the System definition rather than a guess based on the terminal.

```json
"work_root": {
  "mode": "fixed",
  "path_from_home": "Documents/notes"
}
```

On one user's Mac, this resolves to:

```text
~/Documents/notes
```

On another user's machine, the same System resolves under that user's home directory. Do not write `/Users/...` or `~/...` in `path_from_home`.

When you run a fixed System:

```bash
a2h run <system-id>/<agent-id> --host codex
```

`--project` is an error. It cannot override a System's declared archive.

## First use of a fixed archive

`check` never creates the archive directory. During `run`, Agent2Host prints the resolved absolute path before creating a missing directory. It creates it only after the host is allowed to start and you accept any required warning confirmation.

This means a rejected launch, a failed check, or a declined warning does not leave an empty archive directory behind.

## Path rules for fixed archives

`path_from_home` uses `/` between path segments. Each segment may use normal macOS folder characters, including spaces, `@`, and Unicode. Do not use:

- an absolute path;
- `~`;
- `/` or `:` within one segment;
- `.` or `..` as a segment.

## Legacy Systems

An existing `agent2host/v1alpha1` System without `work_root` is treated as current-project mode for compatibility. New Systems should declare one of the two modes explicitly.

## Design choice

The System defines *how to select* its work root. Its SOP and skills define what hierarchy to create within that root. For example, an event-planning SOP can create one folder per event; Agent2Host does not prescribe that internal organization.
