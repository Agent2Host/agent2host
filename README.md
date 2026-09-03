# Agent2Host

Define an Agent System once. Register it locally. Check it, then run it in the coding host you already use.

```bash
a2h register ./your-system
a2h run your-system/your-agent --host claude-code
a2h run your-system/your-agent --host kiro
a2h run your-system/your-agent --host codex
```

Agent2Host does not replace Claude Code, Kiro, or Codex. It does not install those hosts, and it does not provide a new chat UI or model login. It compiles the selected agent into that host's own config and launches the host's own process.

This is a **public alpha** (`v0.1.0-alpha.N`), not a stable `v0.1.0`.

## Install

You need Go, plus at least one host already installed and signed in: `claude`, `kiro-cli`, or `codex`.

```bash
go build -o a2h ./cmd/a2h
```

Local builds print `0.0.0-dev`. Put `a2h` on `PATH`.

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
  "schema_version": "agent2host/v1alpha1",
  "kind": "AgentSystem",
  "id": "demo-system",
  "name": "Demo System",
  "version": "0.1.0",
  "agents": ["./agents/demo.agent.json"]
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

## Commands

```bash
a2h register ./path/to/demo-system
a2h list
a2h inspect demo-system/demo
a2h check demo-system/demo --host claude-code
a2h run demo-system/demo --host claude-code
a2h remove demo-system
a2h clean
a2h version
```

`register` stores a pinned snapshot under `~/.a2h/` (override with `--home` or `A2H_HOME`).

`check` tells you what the chosen host can honor before you start. `run` compiles only that agent's reachable files, launches the host, and deletes the temporary run workspace when you leave.

If `check` warns, a normal terminal asks whether to continue. Type `y` only after reading the warning. `--accept-warnings` is for scripts.

## Hosts

| Host | Id |
| --- | --- |
| Claude Code | `claude-code` |
| Kiro | `kiro` |
| Codex | `codex` |

Agent2Host does not install these hosts. A new `A2H_HOME` does not import your existing host login folders; complete that host's own sign-in once in this home.

## Current limits

- Ordinary runs do **not** strictly confine filesystem reads. Use `--require-strict-read` when you want `run` to refuse unless that stronger promise holds.
- Codex is a mapped launch and may show a generic Codex banner.
- Chat history from a finished run is not kept. The next `run` starts a new conversation.
- `check` / `run` are product paths, not a stable shipped contract yet.

## License

Apache License 2.0. See [LICENSE](LICENSE).
