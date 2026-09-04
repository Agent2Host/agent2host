# How Agent2Host works

Agent2Host separates three kinds of state that are easy to confuse.

```text
Your Agent System folder
        │ register
        ▼
Agent Space snapshot (~/.a2h by default)
        │ check / run a named agent
        ▼
Temporary run workspace ──► Native host process ──► Work root
```

## Agent System

The Agent System is the folder you author and keep under your own version control if you choose. It holds `system.json`, Agent declarations, SOP Markdown, and any files referenced by those declarations.

It answers questions such as:

- Which named agents belong together?
- What procedure, skill, context, tool, or hook belongs to each Agent?
- How should the work root be selected?

The System does not contain long-lived user work such as event plans, reports, or a source repository.

## Registered snapshot

`a2h register` validates the System and records an exact local snapshot under Agent Space. `a2h list` shows registered Systems; `a2h inspect system-id/agent-id` shows the selected snapshot.

This makes runs repeatable: editing a source file later does not silently change an already registered System. Register the folder again after an intentional change.

## Named Agent

A System can contain several Agents with different roles. You launch exactly one by its stable name:

```bash
a2h run system-id/agent-id --host codex
```

Only the selected Agent's reachable procedure, declared capabilities, and referenced files are prepared for that run. A System is not a single giant prompt copied wholesale to every host session.

## Temporary run workspace

Before the host starts, Agent2Host prepares a run-only workspace for the host-specific projection and local process state. It is not a user project directory. When the host session ends, Agent2Host removes the temporary workspace; if cleanup cannot finish safely, `a2h clean` can show or remove leftovers.

## Work root

The work root is the user-facing directory opened by the native host. It is separate from both the registered snapshot and the temporary workspace.

- A current-project System uses the project passed with `--project`, or the current directory.
- A fixed-archive System resolves a path below the current user's home directory.

Read [Choose a work root](../guides/work-roots.md) for examples and path rules.

## Why this separation matters

You can register a System from any location without making that location its project. You can clone Agent2Host into a temporary directory without causing an event planner to write event plans into that clone. You can update a System deliberately by registering again, rather than accidentally changing a session midway through a project.
