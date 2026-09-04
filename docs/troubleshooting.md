# Troubleshooting

## `a2h: command not found`

The binary is not on your shell's `PATH`, or you built from source and are trying to invoke `a2h` instead of `./a2h`.

```bash
which a2h
echo "$PATH"
```

For a release install on macOS/zsh:

```bash
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

For a source build in the repository directory, use:

```bash
./a2h version
```

## The host is missing or not signed in

Agent2Host does not install or authenticate the host. Confirm its executable works first:

```bash
claude --version
kiro-cli --version
codex --version
```

Run only the command for the host you chose. Complete its own sign-in flow, then return to `a2h check`.

## `system-id/agent-id` is not found

The run name comes from JSON, not the source folder name. Inspect the registered snapshot:

```bash
a2h list
a2h inspect <system-id>/<agent-id>
```

If you registered into a separate home, include the same `--home` flag on these commands.

## I changed my System, but the host still sees the old instructions

Registration stores a snapshot. Register the source folder again:

```bash
a2h register /absolute/path/to/your-agent-system
```

Then use `a2h inspect` to confirm the active revision.

## The host opened the wrong folder

First inspect the System's work-root mode:

```bash
a2h inspect <system-id>/<agent-id>
```

- `invocation` uses `--project` or the directory where you invoke `a2h`.
- `fixed` uses `path_from_home` under the current user's home directory. `--project` cannot override it.

Read [Choose a work root](guides/work-roots.md) for the full model.

## A fixed work root was not created

`check` deliberately never creates it. `run` creates a missing fixed root only after the host is allowed to start and you accept any warning prompt. A refusal or a declined warning leaves no new directory.

## `check` warns or refuses

Read the exact message first. A warning means the host can start but Agent2Host cannot verify or reproduce every requested restriction. A refusal means the selected host cannot satisfy a required condition for this Agent.

Try a different supported host only if that is appropriate for the Agent, or adjust the Agent System's declared requirement deliberately. Do not remove a boundary merely to make a warning disappear.

See [Supported hosts and limitations](reference/hosts-and-limits.md).

## A run left temporary files behind

Start safely:

```bash
a2h clean --dry-run
```

Then use the relevant `a2h clean` scope. `clean` addresses Agent2Host local runtime material; it does not delete the source System folder or files in your work root.

## I used `\~/.a2h-trial` and now there is a folder named `~`

The backslash prevented your shell from expanding `~`. Use either of these forms next time:

```bash
a2h --home ~/.a2h-trial list
a2h --home "$HOME/.a2h-trial" list
```
