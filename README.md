# Agent2Host

Define an Agent System once. Register it on your machine. Check it, then run the same agent in Claude Code, Kiro, or Codex.

Agent2Host does not replace those hosts. It does not install them, and it does not add a new chat UI or model login. It compiles the selected agent into that host's own config and starts that host's own process.

This is a public alpha, not a stable 1.0.

## Install

You need Go 1.22 or newer, and at least one host already installed and signed in: `claude`, `kiro-cli`, or `codex`.

```bash
git clone https://github.com/Agent2Host/agent2host.git
cd agent2host
go build -o a2h ./cmd/a2h
```

Put `a2h` on your `PATH`. A local build prints `0.0.0-dev`.

## Quick start

The repository includes three complete example systems under `test/systems/`.

```bash
a2h register ./test/systems/dev-studio
a2h check dev-studio/code-reviewer --host claude-code
a2h run dev-studio/code-reviewer --host claude-code
```

`check` tells you whether this host can run this agent **before** anything starts.

The folder the host opens is the **work root**, declared by the Agent System (not the folder you registered from):

- **`invocation`** — the directory you pass with `--project`, or the directory you are in. Use this for reviewers that work on whatever repo you point at.
- **`fixed`** — a folder under your home directory, set in `system.json`. Use this for agents that keep long-lived drafts and reports. `run` cannot change that folder.

A `v1alpha1` system with no `work_root` is treated as `invocation`.

- **Allowed** — start.
- **Allowed with warnings** — you can start, but some restrictions are unverified. Agent2Host does not guarantee they are in force. A terminal asks `y/N`; scripts pass `--accept-warnings`.
- **Refused** — this host is missing a required sandbox, or it would silently do something the agent did not allow.

`run` compiles only that agent's reachable files, launches the host, and deletes the temporary run workspace when you leave.

## Write an Agent System

Declarative files are JSON. Behavior docs are Markdown.

```text
demo-system/
├── system.json
├── agents/
│   └── demo.agent.json
└── sops/
    └── demo.sop.md
```

`system.json`:

```json
{
  "schema_version": "agent2host/v1alpha2",
  "kind": "AgentSystem",
  "id": "demo-system",
  "name": "Demo System",
  "version": "0.1.0",
  "agents": ["./agents/demo.agent.json"],
  "work_root": { "mode": "invocation" }
}
```

For an archive that always lives under your home directory:

```json
"work_root": {
  "mode": "fixed",
  "path_from_home": "Desktop/Crossroads/Events"
}
```

`agents/demo.agent.json`:

```json
{
  "schema_version": "agent2host/v1alpha1",
  "kind": "Agent",
  "id": "demo",
  "name": "Demo Agent",
  "description": "Minimal demo agent.",
  "sop": "./sops/demo.sop.md"
}
```

Then:

```bash
a2h register ./demo-system
a2h check demo-system/demo --host claude-code
a2h run demo-system/demo --host claude-code --project /path/to/repo
```

## Commands

```bash
a2h register ./path/to/demo-system
a2h list
a2h inspect demo-system/demo
a2h check demo-system/demo --host claude-code [--project dir]
a2h run demo-system/demo --host claude-code [--project dir]
a2h remove demo-system
a2h clean
a2h version
```

Registered systems live under `~/.a2h/` (or `--home` / `A2H_HOME`).

## Hosts

| Host | Id |
| --- | --- |
| Claude Code | `claude-code` |
| Kiro | `kiro` |
| Codex | `codex` |

Agent2Host does not install these hosts. A new home directory does not import an existing host login; sign in to that host once in this home.

## Current limits

- Ordinary runs do not strictly confine filesystem reads. Pass `--require-strict-read` if you want `run` to refuse unless that stronger promise holds.
- Codex starts as a mapped session and may show a generic Codex banner.
- Chat history from a finished run is not kept. The next `run` starts a new conversation.

## License

Apache License 2.0. See [LICENSE](LICENSE).
