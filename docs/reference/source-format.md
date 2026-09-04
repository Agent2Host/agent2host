# Agent System reference

This page is a user-facing reference for the current public format. It is not a replacement for the published JSON schemas; `a2h register` is the final validator and rejects unknown JSON keys.

## System file: `system.json`

New Systems use `agent2host/v1alpha2` and require these fields:

| Field | Type | Purpose |
| --- | --- | --- |
| `schema_version` | string | Exactly `agent2host/v1alpha2`. |
| `kind` | string | Exactly `AgentSystem`. |
| `id` | string | Stable System id. |
| `version` | string | SemVer without a leading `v`. |
| `agents` | array of paths | One or more `*.agent.json` files. |
| `work_root` | object | How the host's working directory is selected. |

Optional fields are `name`, `description`, `skills`, `defaults`, and `extensions`.

```json
{
  "schema_version": "agent2host/v1alpha2",
  "kind": "AgentSystem",
  "id": "demo-system",
  "name": "Demo System",
  "description": "A small example.",
  "version": "0.1.0",
  "agents": ["./agents/demo.agent.json"],
  "work_root": { "mode": "invocation" }
}
```

## Work root

Choose one:

```json
{ "mode": "invocation" }
```

or:

```json
{
  "mode": "fixed",
  "path_from_home": "Documents/notes"
}
```

See [Choose a work root](../guides/work-roots.md) for the behavior and path rules.

## Agent file: `*.agent.json`

Agent files currently use `agent2host/v1alpha1`. Their required fields are:

| Field | Type | Purpose |
| --- | --- | --- |
| `schema_version` | string | Exactly `agent2host/v1alpha1`. |
| `kind` | string | Exactly `Agent`. |
| `id` | string | Stable Agent id within the System. |
| `sop` | portable path | Main SOP Markdown file. |

`name` and `description` are optional but recommended. The Agent can also declare `skills`, `contexts`, `mcp_servers`, `hooks`, `environment`, `permissions`, `approvals`, `sandbox`, `output`, and `extensions`.

```json
{
  "schema_version": "agent2host/v1alpha1",
  "kind": "Agent",
  "id": "demo",
  "name": "Demo Agent",
  "description": "A minimal example.",
  "sop": "./sops/demo.sop.md"
}
```

## IDs, versions, and paths

| Item | Rule |
| --- | --- |
| Local ids | Begin with lowercase `a-z`; continue with lowercase letters, digits, and `-`; maximum 63 characters. |
| System version | SemVer without a `v` prefix, for example `0.1.0` or `0.1.0-alpha.1`. |
| Definition-tree paths | Optional leading `./`, then ASCII portable path segments using letters, digits, `.`, `_`, and `-`. |
| Definition-tree path exclusions | No spaces, `@`, backslashes, empty segments, `.` or `..` segments, or paths that escape the System folder. |

The definition-tree rule applies to SOPs, skills, contexts, scripts, assets, MCP program files, hooks, and output schemas. `work_root.path_from_home` follows different, user-home path rules.

## Skills

Catalog a skill on the System:

```json
"skills": {
  "review": {
    "name": "Code review",
    "description": "A reusable review procedure.",
    "document": "./skills/review.skill.md",
    "contexts": ["./contexts/review-checklist.md"],
    "scripts": ["./scripts/review-hint.py"],
    "assets": ["./assets/glossary.txt"],
    "mcp_tools": [
      { "server_id": "repo-tools", "tool_name": "get_diff_summary" }
    ]
  }
}
```

Then attach it to an Agent with a skill id, or `{ "id": "review", "required": false }`.

## Contexts

```json
"contexts": [
  {
    "path": "./contexts/style-guide.md",
    "loading": "on_demand",
    "isolation": "best_effort",
    "required": false
  }
]
```

`loading` is currently `on_demand` when present. `isolation` can be `required`, `best_effort`, or `none`.

## MCP servers

```json
"mcp_servers": {
  "repo-tools": {
    "transport": "stdio",
    "command": "python3",
    "args": ["-u", "./mcp/repo-tools.py"],
    "files": ["./mcp/repo-tools.py"],
    "tools": ["get_diff_summary", "read_file_hunk"]
  }
}
```

Only `stdio` transport is part of the current format. `tools` is the allow-list; the strings are opaque MCP tool names, not Agent2Host ids.

## Hooks

Hooks are grouped under `session_start`, `before_tool_call`, `after_tool_call`, and `agent_stop`.

```json
"hooks": {
  "before_tool_call": [
    {
      "command": "python3",
      "args": ["./hooks/before-tool.py"],
      "files": ["./hooks/before-tool.py"],
      "required": true
    }
  ]
}
```

## Environment bindings

Environment bindings refer only to an uppercase local variable name:

```json
"environment": [
  {
    "value_from": { "environment": "SERVICE_TOKEN" },
    "required": true
  }
]
```

Place such a binding on an Agent, MCP server, or hook as appropriate. The secret value remains outside the System source.

## Permissions, approvals, and sandbox

```json
"permissions": {
  "filesystem": {
    "read": ["working_directory"],
    "write": []
  },
  "network": { "default": "deny" }
},
"approvals": { "shell_execute": "on_boundary" },
"sandbox": { "required": true, "mode": "read_only" }
```

`working_directory` is the resolved work root. The only current network defaults are `allow` and `deny`. Shell approval values are `always`, `on_boundary`, and `never`. Sandbox modes are `read_only` and `workspace_write`.

The host may not enforce every request in exactly the same way. Always run `check` and read its result.

Filesystem `write` covers create, modify, append, truncate, rename, move, metadata changes, and delete within the declared scope. The current format has no finer portable write verbs. For the full behavior, defaults, examples, and host differences, read [Permissions and safety](../guides/permissions-and-safety.md).

## Structured output

An Agent can point to a JSON Schema:

```json
"output": {
  "schema": "./schemas/report.schema.json",
  "enforcement": "best_effort"
}
```

`enforcement` may be `best_effort` or `required`; omitted means `best_effort`. A schema path alone does not force validation.
