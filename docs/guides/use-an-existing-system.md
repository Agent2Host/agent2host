# Use an existing Agent System

Use this guide when you already maintain an Agent System folder and want to run one of its named agents in Claude Code, Kiro, or Codex.

## 1. Register the folder

```bash
a2h register /absolute/path/to/your-agent-system
```

Registration validates the folder and creates an exact local snapshot in Agent Space. It does not turn the source folder into a project, and it does not start a host.

## 2. Find the name to run

```bash
a2h list
a2h inspect <system-id>/<agent-id>
```

Use the ids from JSON, not directory or filename guesses. After you register `./test/systems/dev-studio`, you run `dev-studio/code-reviewer`. `list` and `inspect` show the ids.

## 3. Check one host

```bash
a2h check <system-id>/<agent-id> --host codex
```

Checking is intentionally non-destructive: it does not open the host and does not create a missing fixed work root. It tells you whether the selected host can start this agent, can start it with caveats, or must refuse.

## 4. Run it in the right work root

For a current-project System:

```bash
a2h run <system-id>/<agent-id> \
  --host claude-code \
  --project /absolute/path/to/project
```

For a fixed-archive System:

```bash
a2h run <system-id>/<agent-id> --host codex
```

The second command deliberately has no `--project`: a fixed System chooses its own home-relative archive path. Agent2Host prints that resolved absolute location before it creates a missing folder or starts a host.

Read [Choose a work root](work-roots.md) for the distinction.

## Re-register after edits

Agent2Host runs the registered snapshot, not a live link to the source folder. After changing an SOP, skill, agent file, or System file, register again:

```bash
a2h register /absolute/path/to/your-agent-system
a2h inspect <system-id>/<agent-id>
```

This is intentional: a session has a stable definition even if the source folder changes later.

## Use a separate local Agent Space

By default, registrations live in `~/.a2h`. For an isolated trial or test, set a different home consistently on every command:

```bash
a2h --home "$HOME/.a2h-trial" register /absolute/path/to/your-agent-system
a2h --home "$HOME/.a2h-trial" list
a2h --home "$HOME/.a2h-trial" check <system-id>/<agent-id> --host codex
```

Write `~/.a2h-trial` or `"$HOME/.a2h-trial"`, not `\~/.a2h-trial`. A backslash prevents the shell from expanding `~` and creates a literal directory named `~` instead.

## Next step

If the selected host reports a warning or refuses the launch, see [Supported hosts and limitations](../reference/hosts-and-limits.md) and [Troubleshooting](../troubleshooting.md).
