# Create your first Agent System

An Agent System is a folder containing a System declaration, one or more named Agent declarations, and Markdown procedures. Start with the smallest possible System. Add skills, tools, hooks, and extra files only when the agent needs them.

## Create the folder

```text
demo-system/
├── system.json
├── agents/
│   └── demo.agent.json
└── sops/
    └── demo.sop.md
```

The folder names are conventional. The important rule is that every file the registered snapshot needs is referenced from JSON. An unlisted file on disk is not included.

## Define the System

Create `system.json`:

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

The System owns the list of agents and decides how their work root is selected. New Systems use `agent2host/v1alpha2` because it requires an explicit `work_root` choice.

## Define one named Agent

Create `agents/demo.agent.json`:

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

The Agent id is the name you use in commands. With the System above, the complete name is `demo-system/demo`.

## Write its procedure

Create `sops/demo.sop.md`:

```markdown
# Demo Agent

You are a careful project assistant.

1. First inspect the current work root.
2. Explain what you found before changing any file.
3. Write only inside the current work root.
```

SOP Markdown defines behavior in plain language. It is not a place to repeat structured configuration such as tool names or secret bindings.

## Register, check, and run

From the directory that contains `demo-system`:

```bash
a2h register ./demo-system
a2h inspect demo-system/demo
a2h check demo-system/demo --host claude-code --project /absolute/path/to/project
a2h run demo-system/demo --host claude-code --project /absolute/path/to/project
```

The `invocation` work root tells Agent2Host to use `--project`, or your current directory if you leave the flag out.

## Rules that prevent common errors

| Item | Rule |
| --- | --- |
| System, Agent, and Skill ids | Start with lowercase `a-z`; then use lowercase letters, digits, and `-`; maximum 63 characters. |
| System version | Semantic Versioning without a `v` prefix, such as `0.1.0`. |
| Definition-tree paths | Use optional `./`, then portable segments made from `A-Z`, `a-z`, `0-9`, `.`, `_`, and `-`. Do not use spaces, `@`, `..`, or backslashes. |
| Extra JSON keys | Rejected. Keep prose in Markdown and structured settings in JSON. |
| Secrets | Never put a token, password, or `.env` file in the System folder. |

The portable path rule applies to files in the definition tree, such as SOPs, skills, and scripts. It does **not** apply to a fixed work root; see [Choose a work root](work-roots.md).

## Next step

- Pick the right durable or current-project location: [Choose a work root](work-roots.md).
- Add only the capabilities the agent needs: [Add capabilities](add-capabilities.md).
- Declare boundaries deliberately: [Permissions and safety](permissions-and-safety.md).
- Consult the complete field reference: [Agent System format](../reference/source-format.md).
