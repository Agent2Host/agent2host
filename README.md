# Agent2Host

Define an Agent System once. Register it on your machine. Check it, then run the same agent in Claude Code, Kiro, or Codex.

Agent2Host does not replace those hosts. It does not install them, and it does not add a new chat UI or model login. It compiles the selected agent into that host's own config and starts that host's own process.

This is a public alpha (`v0.1.0-alpha.1`), not a stable 1.0. There is no Homebrew install yet.

Pick one path:

1. [I just want to try it](#1-i-just-want-to-try-it) — install, then register a bundled example.
2. [I already have an Agent System](#2-i-already-have-an-agent-system) — install, register your folder, check, run.
3. [I want to write my own](#3-i-want-to-write-my-own) — minimal files, then work root, then optional skills / MCP / hooks.

## Before you start

1. You are on macOS or Linux.
2. At least one supported host is already installed and signed in: `claude`, `kiro-cli`, or `codex`.
3. You can use a normal terminal.

`a2h help` and `a2h <command> --help` print usage.

## Install

### Released binary (recommended)

Download the archive that matches your machine from [Releases](https://github.com/agent2host/agent2host/releases/tag/v0.1.0-alpha.1):

| Machine | Archive |
| --- | --- |
| Apple Silicon Mac | `a2h-v0.1.0-alpha.1-darwin-arm64.tar.gz` |
| Intel Mac | `a2h-v0.1.0-alpha.1-darwin-amd64.tar.gz` |
| Linux x86_64 | `a2h-v0.1.0-alpha.1-linux-amd64.tar.gz` |
| Linux arm64 | `a2h-v0.1.0-alpha.1-linux-arm64.tar.gz` |

Apple Silicon Mac:

```bash
mkdir -p "$HOME/bin"
curl -L -o /tmp/a2h.tgz \
  https://github.com/agent2host/agent2host/releases/download/v0.1.0-alpha.1/a2h-v0.1.0-alpha.1-darwin-arm64.tar.gz
tar -xzf /tmp/a2h.tgz -C "$HOME/bin"
```

The archive contains `a2h` and `LICENSE`. Put `$HOME/bin` on your `PATH` (macOS zsh uses `~/.zshrc`; Linux bash uses `~/.bashrc`):

```bash
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc    # macOS
# echo 'export PATH="$HOME/bin:$PATH"' >> ~/.bashrc  # Linux
source ~/.zshrc    # or: source ~/.bashrc
```

Confirm:

```bash
which a2h
a2h version
a2h help
```

A released binary prints `0.1.0-alpha.1`.

Optional integrity check (Apple Silicon name shown; change the archive name if yours differs):

```bash
curl -L -o /tmp/SHA256SUMS \
  https://github.com/agent2host/agent2host/releases/download/v0.1.0-alpha.1/SHA256SUMS
shasum -a 256 /tmp/a2h.tgz
grep darwin-arm64 /tmp/SHA256SUMS
```

The `shasum` hash must match the line for that archive in `SHA256SUMS`.

### Build from source (optional)

You need Go 1.22 or newer. After `go build`, the binary is **only in the current directory**. The rest of this README writes `a2h`; from the clone, use `./a2h` unless you copy it onto your `PATH`.

```bash
git clone https://github.com/agent2host/agent2host.git
cd agent2host
go build -o a2h ./cmd/a2h
./a2h version
```

To use `a2h` from any directory:

```bash
mkdir -p "$HOME/bin"
cp a2h "$HOME/bin/"
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc    # macOS
# echo 'export PATH="$HOME/bin:$PATH"' >> ~/.bashrc  # Linux
source ~/.zshrc    # or: source ~/.bashrc
```

A local build prints `0.0.0-dev`.

## 1. I just want to try it

The folders under `test/systems/` are **runnable user examples**, not internal test junk. They ship in the git clone so you can register a complete system without writing one. You need the clone for these files; the release tarball is only the `a2h` binary.

```bash
git clone https://github.com/agent2host/agent2host.git
cd agent2host
a2h register ./test/systems/dev-studio
a2h list
a2h inspect dev-studio/code-reviewer
a2h check dev-studio/code-reviewer --host claude-code
a2h run dev-studio/code-reviewer --host claude-code
```

If you built from source in this directory and did not copy `a2h` to `$HOME/bin`, use `./a2h` in every command above.

When `run` is allowed, the chosen host starts as its own process and uses the work root as the working directory. You talk to the host, not to a separate Agent2Host chat window.

`dev-studio` is a current-project system: the host opens `--project` or the directory you are in. You do not `cd` into a special folder first.

Also available: `ops-desk`, `research-lab`. Host ids: `claude-code`, `kiro`, `codex`.

What you may see, and what it means: [Warnings and limits](#warnings-and-limits).

## 2. I already have an Agent System

You already have a folder with `system.json` and at least one agent.

```bash
a2h register /absolute/path/to/your-agent-system
a2h list
a2h inspect <system-id>/<agent-id>
a2h check <system-id>/<agent-id> --host claude-code
a2h run <system-id>/<agent-id> --host claude-code
```

The name you run is **`system-id/agent-id` from the JSON**, not the folder name. After you register `./test/systems/dev-studio`, you run `dev-studio/code-reviewer`. `list` and `inspect` show the ids.

`register` stores a snapshot. Edits on disk do nothing until you register that folder again.

The host's working folder is the **work root** declared in `system.json`. It is not the folder you registered from, and not the Agent2Host source checkout.

- **`invocation`** — `--project <dir>` if you pass it (that directory must already exist), otherwise the directory you are in.
- **`fixed`** — a folder under your user home from `path_from_home` (for example `Documents/notes` → `~/Documents/notes`). `run` cannot change it. `--project` is an error. Starting `a2h` from a clone or a random directory does not change a `fixed` root. `check` never creates a missing folder. `run` prints the absolute path and creates it only after the host is allowed to start.

A `v1alpha1` system with no `work_root` is treated as `invocation`.

Optional isolated home:

```bash
a2h --home ~/.a2h-alpha register /absolute/path/to/your-agent-system
a2h --home "$HOME/.a2h-alpha" list
```

Do not write `\~/.a2h-alpha`. That creates a directory literally named `~` in the current folder.

Then see [Warnings and limits](#warnings-and-limits).

## 3. I want to write my own

An Agent System is one folder. JSON declares structure. Markdown declares behavior. Extra JSON keys are rejected. Do not put passwords, tokens, or `.env` files in this folder.

### Minimum that runs

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

`sops/demo.sop.md` is ordinary Markdown: who the agent is, what to do, what never to do. It runs in the work root, not in this definition folder.

Ids (system, agent, skill) start with a letter, then `a-z`, `0-9`, `-`, max 63 characters. `a2h run` uses those ids, not file names. System `schema_version` is `agent2host/v1alpha2`. Agent files stay `agent2host/v1alpha1`.

Paths **inside** the definition tree (`sop`, skills, scripts) allow optional `./` and `A-Za-z0-9._-` only — no spaces, no `@`. That is not the rule for `work_root.path_from_home`.

Choose a work root:

- `"mode": "invocation"` — tools that should work on whatever project you point at.
- `"mode": "fixed"` plus `"path_from_home": "Documents/notes"` — a durable folder under **that user's** home. Do not write `/Users/...` or `~/...`. macOS folder names may include `@` and spaces. Do not use `/`, `:`, `.`, or `..` as a segment.

```bash
a2h register ./demo-system
a2h inspect demo-system/demo
a2h check demo-system/demo --host claude-code
a2h run demo-system/demo --host claude-code --project /path/to/repo
```

Register again after every edit.

### Optional pieces (copy from the bundled examples)

A larger tree can also have `skills/`, `contexts/`, `scripts/`, `assets/`, `mcp/`, `hooks/`, and `schemas/`. Folder names are conventional. **Only files listed in JSON are registered.**

Filled-in examples — these paths are live links on GitHub. Clone the repository if you want the files on disk. A binary-only install does not include them.

| What | Copy from |
| --- | --- |
| Several agents, skills, MCP, hooks | [`test/systems/dev-studio`](test/systems/dev-studio) |
| Agent with MCP + hooks | [`test/systems/dev-studio/agents/code-reviewer.agent.json`](test/systems/dev-studio/agents/code-reviewer.agent.json) |
| Skill catalog on the system | [`test/systems/dev-studio/system.json`](test/systems/dev-studio/system.json) |
| Network allow / deny | [`test/systems/research-lab`](test/systems/research-lab) |
| Required sandbox | [`test/systems/ops-desk`](test/systems/ops-desk) |

Do not put secrets in JSON. Bind them with the agent's `environment` fields so they stay on the machine. `check` / `run` compare declared permissions to what the host can do; if a required control cannot be promised, you get a warning or a refuse.

## Warnings and limits

- **Allowed** — start.
- **Allowed with warnings** — you can start, but some restrictions are unverified. A terminal asks `y/N`. Scripts use `--accept-warnings`.
- **Refused** — this host is missing a required sandbox, or it would silently do something the agent did not allow.

Ordinary runs do **not** strictly confine filesystem reads. A notice that the session can read outside the project folder is a current alpha limit, not a failed install. `--require-strict-read` makes `run` refuse unless the stronger promise holds.

If a `fixed` folder does not exist yet, `run` prints `This Agent System will use or create:` and the path. Decline a warning or a refuse, and that folder is not created.

To upgrade the binary, repeat the install step and overwrite `$HOME/bin/a2h`. `a2h clean` removes leftover run workspaces, not registered systems.

Other limits: Codex may show a generic banner; chat history is not kept after `run`; `brew install` is not available.

Registered systems live under `~/.a2h/` unless you set `--home` or `A2H_HOME`. A new home does not import an existing host login.

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

## Hosts

| Host | Id |
| --- | --- |
| Claude Code | `claude-code` |
| Kiro | `kiro` |
| Codex | `codex` |

## License

Apache License 2.0. See [LICENSE](LICENSE).
