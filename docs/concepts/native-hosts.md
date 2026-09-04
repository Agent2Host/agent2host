# Native hosts and compatibility

Agent2Host is intentionally not a fourth agent host. Claude Code, Kiro, and Codex each have their own configuration models, session interfaces, tool approval behavior, and version-specific features. Agent2Host prepares a selected Agent for one of them and launches that host's own process.

## What stays native

After `a2h run`, the host owns the conversation and model interaction:

- Claude Code remains Claude Code.
- Kiro remains Kiro.
- Codex remains Codex.

Agent2Host does not add a unified chat window or pretend that every host has identical controls.

## Why `check` exists

An Agent System can request a sandbox, a network default, shell approval behavior, a working directory boundary, MCP tools, hooks, or a named-Agent presentation. A host may support some of those requests differently, may not support one, or may not have enough verified evidence for Agent2Host to promise that a restriction is active.

Run `check` before `run`:

```bash
a2h check system-id/agent-id --host kiro
```

The result is one of:

| Result | What Agent2Host does |
| --- | --- |
| Allowed | Lets the session start. |
| Allowed with warnings | Lets it start only after an interactive confirmation, or `--accept-warnings` for a non-interactive call. |
| Refused | Does not start the host. |

## A warning is not an invisible fallback

Warnings mean Agent2Host found a difference or could not verify a requested restriction. It does not silently claim that every host behaves the same. Read the text printed before accepting a run.

The specific limits and verified Host-version ranges for this alpha are listed in [Supported hosts and limitations](../reference/hosts-and-limits.md).
