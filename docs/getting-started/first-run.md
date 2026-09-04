# Run your first Agent

This tutorial gives you one complete path from an installed `a2h` binary to a native host session. It uses an official example System from this repository: a complete, inspectable sample that demonstrates how a System is structured. It is not a marketplace or a production service.

## Before you begin

You need:

- `a2h` installed and available on your `PATH`; see [Install Agent2Host](install.md).
- One supported host installed and signed in. This tutorial uses Claude Code; replace `claude-code` with `kiro` or `codex` if that is the host you installed.
- Git, because release archives do not include the official example Systems yet.

## 1. Get the official example Systems

```bash
git clone https://github.com/agent2host/agent2host.git
cd agent2host
```

The official example Systems are currently under `test/systems/`. Despite that directory name, they are complete runnable samples for this alpha, not user-created test setup.

## 2. Register a complete System

Register `dev-studio`. Registration copies a pinned snapshot into your local Agent Space; it does not make the source folder the host's working folder.

```bash
a2h register ./test/systems/dev-studio
a2h list
a2h inspect dev-studio/release-notes
```

You should see `dev-studio` in `list`. The command name to use later is `dev-studio/release-notes`: the System id and Agent id from JSON, not the directory name.

## 3. Check before launch

Choose a project directory that already exists. The `release-notes` example uses the current-project work-root mode, so `--project` tells the host which project to open.

```bash
a2h check dev-studio/release-notes \
  --host claude-code \
  --project /absolute/path/to/a/project
```

`check` does not start the host or create files. It tells you one of three things:

| Result | Meaning |
| --- | --- |
| Allowed | The host can start this agent. |
| Allowed with warnings | It can start, but one or more restrictions are not verified. Read the warning before continuing. |
| Refused | The host cannot meet a required condition, so Agent2Host will not start it. |

See [Supported hosts and limitations](../reference/hosts-and-limits.md) for what warnings mean in this alpha.

## 4. Start the native host

```bash
a2h run dev-studio/release-notes \
  --host claude-code \
  --project /absolute/path/to/a/project
```

If Agent2Host shows warnings, it asks for confirmation. Read them, then type `y` only if you accept them. When the command is allowed, Claude Code starts in the project you selected. You now talk to Claude Code itself; Agent2Host does not open a second chat window.

When you leave the host session, Agent2Host removes its temporary run workspace. Your project files remain in the project directory you chose.

## What to try next

- Register an existing folder you own: [Use an existing Agent System](../guides/use-an-existing-system.md).
- Learn why this example opened your chosen project: [Choose a work root](../guides/work-roots.md).
- Write the smallest System yourself: [Write your first Agent System](../guides/write-your-first-system.md).
