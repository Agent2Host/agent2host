# Agent2Host

You already use Claude Code, Kiro, or Codex — or you are about to. You want one agent (who it is, how it works, which tools it may use, what it must not do, where it works) that you can open in any of those hosts without rewriting the setup each time.

Agent2Host does that. You write the agent as a folder, register a snapshot on this machine, check that the host you picked can honor the rules, then Agent2Host starts **that host's own process**.

It is not a new chat app and not a new model. It does not install Claude Code, Kiro, or Codex, and it does not sign you in. If you only want to open those hosts and talk, you do not need Agent2Host.

This is a public alpha, not a stable 1.0. `a2h version` prints the exact build.

```text
Folder on disk
    → register (snapshot on this machine)
    → check one named agent on one host
    → that host opens in the declared work root
```

## Before you start

1. You are on macOS or Linux.
2. At least one supported host is already installed and signed in: `claude`, `kiro-cli`, or `codex`.
3. You can use a normal terminal.

`a2h help` and `a2h <command> --help` print usage.

| Host | Id | Executable |
| --- | --- | --- |
| Claude Code | `claude-code` | `claude` |
| Kiro | `kiro` | `kiro-cli` |
| Codex | `codex` | `codex` |

## Install

Homebrew is the recommended path. It installs only the Agent2Host binary.

```bash
brew install agent2host/tap/a2h
which a2h
a2h version
a2h help
```

Later: `brew update && brew upgrade a2h`.

Without Homebrew, download the archive for your machine from the [latest Release](https://github.com/agent2host/agent2host/releases/latest). Copy the tag from that page, then:

| Machine | Archive suffix |
| --- | --- |
| Apple Silicon Mac | `darwin-arm64.tar.gz` |
| Intel Mac | `darwin-amd64.tar.gz` |
| Linux x86_64 | `linux-amd64.tar.gz` |
| Linux arm64 | `linux-arm64.tar.gz` |

```bash
# Paste the latest tag from the Releases page, then:
TAG=
mkdir -p "$HOME/bin"
curl -fL -o /tmp/a2h.tgz \
  "https://github.com/agent2host/agent2host/releases/download/${TAG}/a2h-${TAG}-darwin-arm64.tar.gz"
tar -xzf /tmp/a2h.tgz -C "$HOME/bin"
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
which a2h
a2h version
```

Linux: use the matching archive and add `$HOME/bin` in `~/.bashrc`. Every release also publishes `SHA256SUMS` for the same tag.

To build from source you need Go 1.22 or newer. After `go build -o a2h ./cmd/a2h` the binary is only in the current directory and prints `0.0.0-dev`. Use `./a2h` unless you copy it onto your `PATH`.

## Use it

Install first, then pick one path.

### 1. Try a bundled example

The folders under `test/systems/` are runnable examples. They are in the git clone, not in the release tarball.

```bash
git clone https://github.com/agent2host/agent2host.git
cd agent2host
a2h register ./test/systems/dev-studio
a2h list
a2h inspect dev-studio/code-reviewer
a2h check dev-studio/code-reviewer --host claude-code
a2h run dev-studio/code-reviewer --host claude-code --project /absolute/path/to/a/project
```

If you built from source here and did not copy `a2h` to `$HOME/bin`, use `./a2h`. Also available: `ops-desk`, `research-lab`.

When `run` is allowed, you talk to the host, not to a second Agent2Host window.

### 2. Register a folder you already have

```bash
a2h register /absolute/path/to/your-agent-system
a2h list
a2h inspect <system-id>/<agent-id>
a2h check <system-id>/<agent-id> --host claude-code
a2h run <system-id>/<agent-id> --host claude-code
```

The name you run is **`system-id/agent-id` from the JSON**, not the folder name. After you register `./test/systems/dev-studio`, you run `dev-studio/code-reviewer`. `list` and `inspect` show the ids.

`register` stores a snapshot. Edits on disk do nothing until you register that folder again.

### 3. Write your own

An Agent System is one folder. JSON declares structure. Markdown declares behavior. Extra JSON keys are rejected. Do not put passwords, tokens, or `.env` files in this folder.

New systems use `agent2host/v1alpha2` and must declare `work_root`. Minimum that runs:

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

`agents/demo.agent.json` (agent files stay on `v1alpha1`):

```json
{
  "schema_version": "agent2host/v1alpha1",
  "kind": "Agent",
  "id": "demo",
  "name": "Demo Agent",
  "description": "A minimal Agent2Host example.",
  "sop": "./sops/demo.sop.md"
}
```

`sops/demo.sop.md` is ordinary Markdown. Then `a2h register ./demo-system` and run `demo-system/demo`. Skills, context, MCP, hooks, and permissions: [`docs/guides/write-your-first-system.md`](docs/guides/write-your-first-system.md) and [`docs/guides/add-capabilities.md`](docs/guides/add-capabilities.md). Those files are in the clone.

## Work root

The host does **not** work in the folder you registered from. `system.json` chooses one mode:

- **`invocation`** — `--project <dir>` if you pass it (that directory must already exist), otherwise the directory you are in. Used by `dev-studio`.
- **`fixed`** — a folder under the user's home from `path_from_home` (for example `Documents/notes` → `~/Documents/notes`). `run` cannot change it. `--project` is an error. `check` never creates a missing folder. `run` prints the absolute path and creates it only after the host is allowed to start.

Older systems still on `v1alpha1` with no `work_root` are treated as `invocation` for compatibility.

## What `check` and `run` tell you

- **Allowed** — start.
- **Allowed with warnings** — it can start, but some restrictions are unverified. A terminal asks `y/N`. Scripts use `--accept-warnings`.
- **Refused** — this host is missing a required safeguard, or it would silently do something the agent did not allow.

Agent2Host does not guarantee that every supported host can strictly limit what the agent may read. An ordinary `run` may allow reads outside the work root. If you need that guarantee, pass `--require-strict-read`. If Agent2Host cannot prove the limit holds, it refuses to start.

## Commands

```bash
a2h help
a2h run --help
a2h register ./path/to/demo-system
a2h list
a2h inspect demo-system/demo
a2h check demo-system/demo --host claude-code [--project dir]
a2h run demo-system/demo --host claude-code [--project dir]
a2h remove demo-system
a2h clean
a2h version
```

Registered systems live under `~/.a2h/` unless you set `--home` or `A2H_HOME`. `a2h clean` removes leftover run workspaces, not registered systems.

## More detail in this repository

These pages are files in the clone under `docs/`. They are not required to finish install and the first `run` above. The same pages are also at https://agent2host.github.io/agent2host/. Use the clone if you are offline; do not treat the website as the only manual.

- [Install (other platforms, checksums, source)](docs/getting-started/install.md)
- [First run](docs/getting-started/first-run.md)
- [Write a System](docs/guides/write-your-first-system.md)
- [Skills, context, MCP, hooks, secrets](docs/guides/add-capabilities.md)
- [Permissions](docs/guides/permissions-and-safety.md)
- [Work roots](docs/guides/work-roots.md)
- [CLI](docs/reference/cli.md)
- [Format](docs/reference/source-format.md)
- [Hosts and limits](docs/reference/hosts-and-limits.md)

## License

Apache License 2.0. See [LICENSE](LICENSE).
