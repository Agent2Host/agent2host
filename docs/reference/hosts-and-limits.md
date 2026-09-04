# Supported hosts and limitations

This page defines the public alpha boundary: which native hosts are supported, how to read a compatibility result, and what Agent2Host does not yet promise.

## Supported hosts

| Host | Agent2Host id | Executable | Verified version family |
| --- | --- | --- | --- |
| Claude Code | `claude-code` | `claude` | `2.1.*` |
| Kiro | `kiro` | `kiro-cli` | `2.20.*`, `2.21.*` |
| Codex | `codex` | `codex` | `0.149.*` |

Agent2Host does not install, authenticate, or update these hosts. Install and sign in to at least one before running `a2h check` or `a2h run`.

If your installed version is outside the verified family, the host may still start, but its native behavior may have changed. Read the result instead of assuming a newer version is either safe or broken.

## Compatibility results

| Result | Meaning | What `run` does |
| --- | --- | --- |
| Allowed | The selected host can start the Agent without a compatibility warning. | Starts. |
| Allowed with warnings | It can start, but Agent2Host found a difference or cannot verify one or more restrictions. | Interactive runs require confirmation; scripts need `--accept-warnings`. |
| Refused | A required condition is unavailable, or the host is known to allow an action the Agent did not authorize without an effective prompt. | Does not start. |

An unverified restriction is not treated as a security guarantee. Agent2Host tells you that it cannot prove the restriction and lets you decide whether to continue.

## Filesystem-read limitation

Agent2Host does not guarantee that every supported host can strictly limit what the agent may read. An ordinary `run` may allow reads outside the work root. Agent2Host prints a notice when that limit matters.

If you need that guarantee, pass `--require-strict-read`. If Agent2Host cannot prove the limit holds, it refuses to start:

```bash
a2h run system-id/agent-id \
  --host codex \
  --require-strict-read
```

## Permission-mapping limitation

Claude Code, Kiro, and Codex expose different native controls. Agent2Host maps one portable declaration into each host, but does not claim that their prompts, sandboxes, or permission interfaces are identical.

Possible outcomes include:

- the host enforces the requested restriction;
- the host asks before the relevant action;
- the host is known to allow an unrequested action silently, so Agent2Host refuses;
- Agent2Host cannot verify the effect, so it warns and makes no guarantee.

Read [Permissions and safety](../guides/permissions-and-safety.md) before defining a sensitive Agent.

## Other current limitations

- Codex may show its standard interface instead of a native named-Agent view.
- A completed `run` does not carry the host's chat history into a later run.
- Arbitrary native-host arguments are not forwarded.
- Homebrew installation is not available yet.
- Agent2Host does not manage model selection, accounts, host installation, or host upgrades.
- Compatibility is checked at launch time; it is not continuous monitoring of the host after launch.
- On Kiro, MCP and hook secret values are placed on that run's host process environment so they are not written into model-readable config. Agent2Host does not claim those values are hidden from the session or isolated between tools.

## What a warning does not mean

An allowed result is not a claim that all hosts behave identically. A warning is not a failed installation. A refusal does not mean the Agent System is invalid; it means the selected host cannot safely start that Agent under the current rules.
