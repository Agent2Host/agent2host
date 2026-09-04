# Permissions and safety

Agent2Host lets an Agent System state what an Agent may do, when an authorized shell command must be confirmed, and whether a technical sandbox is mandatory. You write one portable declaration; the selected adapter translates it into that host's native controls.

## Three controls, three questions

Do not collapse these objects into one idea:

| Object | Question it answers | Example |
| --- | --- | --- |
| `permissions` | Is the action authorized, and where? | May this Agent write in the work root? |
| `approvals` | Must an already-authorized shell command pass a host confirmation first? | Ask before each shell command. |
| `sandbox` | Is a technical isolation mechanism required? | Refuse to run unless a read-only sandbox exists. |

Permission always comes first. A prompt cannot expand a denied permission. If writing outside the declared scope is not authorized, the correct result is denial—not a prompt that permanently grants broader access.

## Filesystem permissions

The current format has one portable scope: `working_directory`. It means the [resolved work root](work-roots.md) for this run, not the Agent System source folder and not Agent Space.

### Read and write the work root

```json
"permissions": {
  "filesystem": {
    "read": ["working_directory"],
    "write": ["working_directory"]
  },
  "network": { "default": "deny" }
}
```

This authorizes reading user data and changing files within the work root.

In the current format, `write` is one complete mutation permission. It includes:

- creating files and folders;
- modifying, appending to, or truncating files;
- renaming and moving files;
- changing file metadata;
- deleting files and folders.

For a rename or move, both the source and destination must be inside the authorized write scope. `write` does not grant read access outside the declared read scope.

!!! important "There is no portable edit-without-delete mode yet"
    Agent2Host currently cannot express “allow create and edit, but deny delete.” A native host may apply an extra restriction, but an Agent System cannot depend on that behavior across all hosts.

### Read only

```json
"permissions": {
  "filesystem": {
    "read": ["working_directory"],
    "write": []
  },
  "network": { "default": "deny" }
}
```

Use this for review, audit, or analysis that must not change user files.

### No user-file access

```json
"permissions": {
  "filesystem": {
    "read": [],
    "write": []
  },
  "network": { "default": "deny" }
}
```

This concerns user data. The host may still need to read its own executable, libraries, and runtime files in order to start.

## Network permission

Network is an explicit allow-or-deny choice:

```json
"network": { "default": "allow" }
```

Use `allow` when the Agent's work requires web search, remote APIs, package registries, or a networked MCP server.

```json
"network": { "default": "deny" }
```

Use `deny` when network access is not part of the Agent's job. Denying network does not automatically mean every possible exit is proven blocked. If a host blocks some known web and shell paths but Agent2Host cannot verify complete coverage, `check` allows the launch with an explicit unverified warning and makes no network-isolation guarantee.

## Shell approvals

`approvals.shell_execute` controls the minimum confirmation behavior for shell commands that are already within the Agent's permissions.

| Value | Meaning |
| --- | --- |
| `always` | Every authorized shell execution must pass an effective host confirmation. |
| `on_boundary` | Shell remains subject to the host's approval system; the host may keep a standing allow-list for ordinary in-scope commands. |
| `never` | The Agent System does not require a shell confirmation for authorized commands. |

Example:

```json
"approvals": { "shell_execute": "on_boundary" }
```

`always` is the strictest request. If a host cannot guarantee a separate confirmation before every shell execution, `check` refuses that host rather than weakening the request silently. `never` removes the Agent2Host requirement; a host or organization policy may still ask more often.

## Sandbox requirement

Sandbox settings describe whether technical isolation is mandatory and which portable mode the Agent requests.

### Optional workspace-write sandbox

```json
"sandbox": {
  "required": false,
  "mode": "workspace_write"
}
```

This is the default. Agent2Host still maps filesystem and network permissions, but the absence of a host sandbox does not refuse the run by itself.

### Required read-only sandbox

```json
"sandbox": {
  "required": true,
  "mode": "read_only"
}
```

This tells Agent2Host to refuse rather than fall back to an unsandboxed or writable session. A confirmation prompt cannot waive a required sandbox.

The public format supports `read_only` and `workspace_write`. It intentionally does not accept a full-access or bypass mode.

## Defaults when fields are omitted

Omitting `permissions`, `approvals`, and `sandbox` is exactly equivalent to this baseline:

```json
{
  "permissions": {
    "filesystem": {
      "read": ["working_directory"],
      "write": ["working_directory"]
    },
    "network": { "default": "deny" }
  },
  "approvals": { "shell_execute": "on_boundary" },
  "sandbox": {
    "required": false,
    "mode": "workspace_write"
  }
}
```

These defaults are convenient, but explicit declarations are easier for another person to review. For a production System, write the intended boundaries in the Agent file.

## Complete policy examples

### Offline project editor

Reads and changes the selected project, does not request network access, and uses ordinary host approval behavior:

```json
"permissions": {
  "filesystem": {
    "read": ["working_directory"],
    "write": ["working_directory"]
  },
  "network": { "default": "deny" }
},
"approvals": { "shell_execute": "on_boundary" },
"sandbox": { "required": false, "mode": "workspace_write" }
```

The public [`dev-studio/implementer`](https://github.com/Agent2Host/agent2host/blob/main/test/systems/dev-studio/agents/implementer.agent.json) uses this shape.

### Network-enabled researcher

Reads and writes the work root and explicitly authorizes network access:

```json
"permissions": {
  "filesystem": {
    "read": ["working_directory"],
    "write": ["working_directory"]
  },
  "network": { "default": "allow" }
},
"approvals": { "shell_execute": "on_boundary" },
"sandbox": { "required": false, "mode": "workspace_write" }
```

See the complete [`research-lab/web-researcher`](https://github.com/Agent2Host/agent2host/blob/main/test/systems/research-lab/agents/web-researcher.agent.json) declaration.

### Read-only deployment review

Requires a technical read-only sandbox and refuses a host that cannot provide it:

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

See the complete [`ops-desk/deploy-guard`](https://github.com/Agent2Host/agent2host/blob/main/test/systems/ops-desk/agents/deploy-guard.agent.json) declaration.

### Confirm every shell command

```json
"approvals": { "shell_execute": "always" }
```

Use this only when the workflow truly requires a host-enforced confirmation before every shell execution. It is expected to refuse hosts that offer only a broader “ask when needed” policy.

## How `check` decides

For an action the Agent did not authorize, Agent2Host distinguishes three cases:

| Observed host behavior | Result |
| --- | --- |
| The host is known to perform it without an effective prompt. | Refused. |
| The host is known to deny it or ask effectively before it. | Allowed for that restriction. |
| Agent2Host cannot verify the behavior. | Allowed with an unverified warning; no guarantee is made. |

For an action the Agent did authorize, the host must make it usable—either directly or after an effective prompt. A requested capability that the host cannot provide is refused.

Always run `check` against the actual host and installed version:

```bash
a2h check system-id/agent-id --host claude-code --project /path/to/project
```

## How native hosts differ

Mature hosts expose similar ideas with different configuration languages:

- [Claude Code permissions](https://code.claude.com/docs/en/permissions) use native `allow`, `ask`, and `deny` rules, with denial taking precedence.
- [Codex configuration](https://developers.openai.com/codex/config-reference/) separates approval policy from sandbox and network settings.
- [Kiro custom-agent configuration](https://kiro.dev/docs/cli/custom-agents/configuration-reference/) documents Kiro's own tool and permission model.

Agent2Host follows the same separation of authorization, confirmation, and technical boundaries, but you **do not** copy those native JSON or TOML fields into an Agent System. Use Agent2Host's portable fields; the adapter generates host configuration for the current run.

The public alpha is verified against Claude Code `2.1.*`, Kiro `2.20.*` and `2.21.*`, and Codex `0.149.*`. Kiro's latest documentation may describe fields newer than the verified Kiro families, so it is design context—not a promise that the current adapter emits every latest field.

Read [Supported hosts and limitations](../reference/hosts-and-limits.md) for the current guarantees and known gaps.
