---
hide:
  - navigation
---

# Define once. Run natively.

<p class="a2h-lead">Agent2Host lets you define one complete Agent System, register an exact local snapshot, and run one named Agent in Claude Code, Kiro, or Codex.</p>

[Install Agent2Host](getting-started/install.md){ .md-button .md-button--primary }
[Run your first Agent](getting-started/first-run.md){ .md-button }

## What Agent2Host does

An Agent is more than a prompt. It can have a procedure, reusable skills, context, MCP tools, hooks, permissions, approval rules, and a working location. Agent2Host keeps those parts together in a portable folder, then prepares only the selected Agent for the native host you choose.

```text
Agent System folder
        │  register an exact snapshot
        ▼
Local Agent Space (~/.a2h)
        │  check one Agent against one host
        ▼
Claude Code / Kiro / Codex opens in the correct work root
```

The host still owns the conversation and model session. Agent2Host does not replace it with another chat interface.

## Why the work root matters

Registration and work happen in different places. You may register a System from Downloads, keep its snapshot in Agent Space, and still open every session in a fixed folder under that user's home, such as `~/Documents/notes`.

A System declares one of two choices:

- **Current project** — open the folder selected for this run. This suits code reviewers and repository assistants.
- **Fixed archive** — always open the same folder below the current user's home directory. This suits plans, records, and reports.

[Understand work roots →](guides/work-roots.md)

## The lifecycle

<div class="grid cards" markdown>

-   **1. Define**

    Keep the complete Agent System in a portable folder you control.

-   **2. Register**

    Store a pinned local snapshot. Later source edits cannot silently change the registered version.

-   **3. Check**

    See what the selected host supports, what remains unverified, and what would prevent a safe launch.

-   **4. Run**

    Start that host's native process in the resolved work root.

</div>

## Before you start

You need:

- macOS or Linux;
- a terminal;
- an Agent2Host release binary, or Go 1.22 or newer for a source build;
- at least one supported host installed and signed in.

| Native host | Agent2Host id | Executable |
| --- | --- | --- |
| Claude Code | `claude-code` | `claude` |
| Kiro | `kiro` | `kiro-cli` |
| Codex | `codex` | `codex` |

Agent2Host does not install, authenticate, or update these hosts. See the full [installation prerequisites](getting-started/install.md#before-you-install).

## Choose your path

<div class="grid cards" markdown>

-   **Try the complete flow**

    Install the CLI, register an official example, check it, and open a native host.

    [Run your first Agent →](getting-started/first-run.md)

-   **Use a System you already have**

    Register your folder and start one of its named Agents.

    [Use an existing System →](guides/use-an-existing-system.md)

-   **Create your own System**

    Build the smallest valid definition, then add only the capabilities it needs.

    [Create your first System →](guides/write-your-first-system.md)

</div>

## Public alpha limitations

Agent2Host is pre-release software. Before using it for sensitive work, know these limits:

- Agent2Host does not guarantee that every supported host can strictly limit reads; an ordinary `run` may allow reads outside the work root;
- hosts can represent the same permission request differently, and an unverified restriction is shown as a warning rather than a guarantee;
- supported host behavior is verified only for the version families listed in the compatibility reference;
- Agent2Host does not preserve a host conversation between separate runs.

[Read all supported hosts and limitations →](reference/hosts-and-limits.md)

## Learn the definition

- [Add skills, context, MCP servers, hooks, and secrets](guides/add-capabilities.md)
- [Define permissions, approvals, and sandbox requirements](guides/permissions-and-safety.md)
- [Understand snapshots, projection, and native sessions](concepts/lifecycle.md)
- [Look up every public field](reference/source-format.md)
