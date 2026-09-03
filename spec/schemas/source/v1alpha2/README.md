# Source v1alpha2

Agent System documents with `schema_version` `agent2host/v1alpha2` must declare `work_root`.

- `fixed` plus `path_from_home`: a folder under the user's home. Segment names follow macOS directory rules (`@`, spaces, and Unicode are allowed; `/` and `:` are not). `.` and `..` are rejected. This is not Source `portable_path`.
- `invocation`: the folder passed to `a2h check` / `a2h run` with `--project`, or the current directory.

Agent Spec files remain `agent2host/v1alpha1`. Systems still on `v1alpha1` (no `work_root`) are treated as `invocation`.
