# Add capabilities

Start with a working SOP-only System. This guide adds one capability at a time and shows exactly which file owns it. The examples follow the same structure as the public [`dev-studio`](https://github.com/Agent2Host/agent2host/tree/main/test/systems/dev-studio) example System.

## Know where each part belongs

| Capability | Declared on | Source files |
| --- | --- | --- |
| Skill | System; selected by an Agent | `*.skill.md`, plus optional context, scripts, and assets |
| Context | Agent or Skill | Markdown or another declared file |
| MCP server | Agent | Server program and any declared support files |
| Hook | Agent | Hook program and any declared support files |
| Secret binding | Agent, MCP server, or hook | Environment-variable name only; never the value |
| Permissions | Agent | JSON declaration only |

Agent2Host registers only files reached from the declarations. Putting an extra file in the folder does not automatically expose it to an Agent.

The examples below assume this starting tree:

```text
dev-studio/
├── system.json
├── agents/
│   └── implementer.agent.json
├── sops/
│   └── implementer.sop.md
├── skills/
├── contexts/
├── scripts/
├── mcp/
└── hooks/
```

## Add a skill

A Skill is a reusable procedure. Define it once in the System catalog, then attach it to each Agent that needs it.

### 1. Write the skill document

Create `skills/debugging.skill.md`:

```markdown
# Debugging

Use this procedure when a test or command fails.

1. Reproduce the smallest failing case before editing.
2. Record the command, exit code, and relevant output.
3. Form one concrete hypothesis.
4. Change the smallest relevant surface.
5. Run the failing check again, then the surrounding test suite.
```

The Skill document contains reusable behavior. Do not put secret values or host-specific configuration in it.

### 2. Catalog the skill in `system.json`

Add a `skills` entry at the top level of the System:

```json
{
  "schema_version": "agent2host/v1alpha2",
  "kind": "AgentSystem",
  "id": "dev-studio",
  "name": "Development Studio",
  "version": "0.1.0",
  "work_root": { "mode": "invocation" },
  "agents": ["./agents/implementer.agent.json"],
  "skills": {
    "debugging": {
      "name": "Debugging",
      "description": "A repeatable procedure for diagnosing failures.",
      "document": "./skills/debugging.skill.md"
    }
  }
}
```

### 3. Attach the skill to an Agent

In `agents/implementer.agent.json`, add its id:

```json
"skills": ["debugging"]
```

A string means the Skill is required. To make it optional on hosts that cannot reproduce it, use:

```json
"skills": [{ "id": "debugging", "required": false }]
```

## Attach context

Context provides facts that support the SOP or a Skill: a handbook, product brief, checklist, or style guide.

Create `contexts/engineering-handbook.md`:

```markdown
# Engineering handbook

- Preserve existing behavior unless the task explicitly changes it.
- Keep changes limited to the requested scope.
- Run the closest automated test before handing work back.
- Report any check that could not be run.
```

Attach it in the Agent file:

```json
"contexts": [
  {
    "path": "./contexts/engineering-handbook.md",
    "loading": "on_demand",
    "isolation": "best_effort",
    "required": true
  }
]
```

The current format supports `on_demand` loading. `isolation` can be:

- `none` — no separation requested;
- `best_effort` — use separation when the host can provide it;
- `required` — refuse a host that cannot meet the requirement.

A Skill may also own context. Put its paths in that Skill's `contexts` array when the documents belong to the reusable procedure rather than one Agent.

## Connect an MCP server

An MCP server is a local process that exposes tools. Agent2Host needs three things: how to start it, which program files belong in the snapshot, and which tool names the Agent may use.

Add this declaration to the Agent:

```json
"mcp_servers": {
  "repo-tools": {
    "transport": "stdio",
    "command": "python3",
    "args": ["-u", "./mcp/repo-tools.py"],
    "files": ["./mcp/repo-tools.py"],
    "tools": ["list_changed_files", "read_file_hunk"]
  }
}
```

The current format supports `stdio`. `tools` is an allow-list: declaring a server does not authorize every tool that server might expose.

Create `mcp/repo-tools.py` as a minimal, runnable MCP server:

```python
#!/usr/bin/env python3
import json
import sys

TOOLS = [
    {
        "name": "list_changed_files",
        "description": "List changed paths.",
        "inputSchema": {
            "type": "object",
            "properties": {},
            "additionalProperties": False,
        },
    },
    {
        "name": "read_file_hunk",
        "description": "Read a selected diff hunk.",
        "inputSchema": {
            "type": "object",
            "properties": {"path": {"type": "string"}},
            "required": ["path"],
            "additionalProperties": False,
        },
    },
]


def reply(message_id, result=None, error=None):
    message = {"jsonrpc": "2.0", "id": message_id}
    message["error" if error else "result"] = error or result
    print(json.dumps(message), flush=True)


for line in sys.stdin:
    request = json.loads(line)
    method = request.get("method")
    message_id = request.get("id")

    if method == "initialize":
        reply(message_id, {
            "protocolVersion": "2024-11-05",
            "capabilities": {"tools": {}},
            "serverInfo": {"name": "repo-tools", "version": "0.1.0"},
        })
    elif method == "notifications/initialized":
        continue
    elif method == "tools/list":
        reply(message_id, {"tools": TOOLS})
    elif method == "tools/call":
        name = request.get("params", {}).get("name")
        arguments = request.get("params", {}).get("arguments", {})
        if name == "list_changed_files":
            text = json.dumps({"files": []})
        elif name == "read_file_hunk":
            text = json.dumps({"path": arguments.get("path"), "hunk": ""})
        else:
            reply(message_id, error={"code": -32601, "message": "unknown tool"})
            continue
        reply(message_id, {"content": [{"type": "text", "text": text}]})
    elif method == "ping":
        reply(message_id, {})
```

This server is deliberately small; a real server can use any language. Its declared executable and dependencies must already exist on the user's machine. See the repository's complete [`repo-tools.py`](https://github.com/Agent2Host/agent2host/blob/main/test/systems/dev-studio/mcp/repo-tools.py) for the public example.

## Add a hook

Hooks run local commands at defined lifecycle moments. The available groups are:

| Hook group | Moment |
| --- | --- |
| `session_start` | When the host session begins |
| `before_tool_call` | Before a host tool call |
| `after_tool_call` | After a host tool call |
| `agent_stop` | When the Agent stops |

Create `hooks/before-tool.py`:

```python
#!/usr/bin/env python3
import sys

print("dev-studio: before tool call", file=sys.stderr)
```

Declare it on the Agent:

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

`files` ensures the registered snapshot contains the program. A required hook can cause `check` to refuse a host that cannot reproduce it. Use `required: false` only when omission is acceptable.

## Bind a secret safely

Never put a token, password, private key, `.env` file, or literal secret in the Agent System. Declare only the name of a local environment variable on the component that consumes it.

For the MCP server above:

```json
"mcp_servers": {
  "repo-tools": {
    "transport": "stdio",
    "command": "python3",
    "args": ["-u", "./mcp/repo-tools.py"],
    "files": ["./mcp/repo-tools.py"],
    "tools": ["list_changed_files", "read_file_hunk"],
    "environment": [
      {
        "value_from": { "environment": "REPO_TOOLS_TOKEN" },
        "required": true
      }
    ]
  }
}
```

Set the value on the machine before running:

```bash
export REPO_TOOLS_TOKEN='your-local-value'
```

The System stores `REPO_TOOLS_TOKEN`, not its value. Put an `environment` array directly on the Agent for a host-session variable, or on a hook for a value used only by that hook.

On Kiro, Agent2Host writes `${REPO_TOOLS_TOKEN}` in the host config and puts the value on the Kiro process environment, as Kiro documents. That keeps the value out of the config file. It does not hide the value from the rest of that session.

## See the complete Agent file

After the additions above, `agents/implementer.agent.json` can look like this:

```json
{
  "schema_version": "agent2host/v1alpha1",
  "kind": "Agent",
  "id": "implementer",
  "name": "Implementer",
  "description": "Implements a scoped change in the selected project.",
  "sop": "./sops/implementer.sop.md",
  "skills": ["debugging"],
  "contexts": [
    {
      "path": "./contexts/engineering-handbook.md",
      "loading": "on_demand",
      "isolation": "best_effort",
      "required": true
    }
  ],
  "mcp_servers": {
    "repo-tools": {
      "transport": "stdio",
      "command": "python3",
      "args": ["-u", "./mcp/repo-tools.py"],
      "files": ["./mcp/repo-tools.py"],
      "tools": ["list_changed_files", "read_file_hunk"],
      "environment": [
        {
          "value_from": { "environment": "REPO_TOOLS_TOKEN" },
          "required": true
        }
      ]
    }
  },
  "hooks": {
    "before_tool_call": [
      {
        "command": "python3",
        "args": ["./hooks/before-tool.py"],
        "files": ["./hooks/before-tool.py"],
        "required": true
      }
    ]
  },
  "permissions": {
    "filesystem": {
      "read": ["working_directory"],
      "write": ["working_directory"]
    },
    "network": { "default": "deny" }
  },
  "approvals": { "shell_execute": "on_boundary" },
  "sandbox": { "required": false, "mode": "workspace_write" }
}
```

## Register and check the result

Register again whenever the source changes; registration creates a new pinned snapshot.

```bash
export REPO_TOOLS_TOKEN='your-local-value'
a2h register ./dev-studio
a2h inspect dev-studio/implementer
a2h check dev-studio/implementer \
  --host claude-code \
  --project /absolute/path/to/project
```

Registration catches invalid fields and missing declared files. `check` then evaluates whether the selected host can carry the Agent's required Skill, Context, MCP tools, Hook, and safety settings. Read every warning before using `run`.

## Continue with production examples

The repository includes three complete runnable example Systems:

- [`dev-studio`](https://github.com/Agent2Host/agent2host/tree/main/test/systems/dev-studio) — multiple Agents, Skills, Context, MCP, Hooks, and structured output;
- [`research-lab`](https://github.com/Agent2Host/agent2host/tree/main/test/systems/research-lab) — network-enabled and network-denied research Agents;
- [`ops-desk`](https://github.com/Agent2Host/agent2host/tree/main/test/systems/ops-desk) — read-only and required-sandbox workflows.

Next, read [Permissions and safety](permissions-and-safety.md) before choosing the Agent's boundaries.
