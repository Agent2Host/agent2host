# Official acceptance systems

These three Agent Systems are the public acceptance corpus. CI and `go test` register them from this directory.

They are `v1alpha2` systems that declare `work_root.mode` `invocation` (current directory or `--project`). Agent Spec files stay `v1alpha1`.

| System | What it covers |
| --- | --- |
| `dev-studio` | Several agents, MCP, hooks, optional skill, workspace files |
| `ops-desk` | Required sandbox, operations workflows |
| `research-lab` | Network allow and deny, MCP, hooks |

They are examples, not a marketplace. Do not add a fourth system here unless it is a complete, offline, reviewable acceptance corpus.
