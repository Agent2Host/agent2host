# Source contract fixtures (published v1alpha1)

Published schema `$id` / `$ref` identity (`urn:agent2host:schema:source:v1alpha1:…`) under `spec/schemas/source/v1alpha1/`. These instances pin accept/reject, defaults, and expected semantic outcomes. Frozen `status.json` `contract_status` is the Contract Freeze ledger (do not rewrite `catalog_only` to mean Go progress).

Run from this directory:

```bash
npm install
npm test
```

## Layout

| Directory | Layer | Harness |
| --- | --- | --- |
| `valid/` `invalid/` | Agent JSON Schema | Ajv 8 must pass / fail |
| `system/valid/` `system/invalid/` | System JSON Schema | Ajv 8 |
| `system/normalize/` | `SRC-DEFAULT-SYSTEM` | Ajv schema-ok; fill is semantic (`expected-baseline-defaults.json`) |
| `normalize/` | `SRC-SEC-OMIT` | Ajv schema-ok; fill is semantic (`expected-baseline-security.json`) |
| `artifact/` | Artifact manifest Schema | Ajv 8 |
| `json_reader/` | BOM / UTF-8 / syntax / duplicate keys | raw bytes + `catalog.json` (JSON.parse cannot see duplicate keys) |
| `semantic-reject/` | Schema-valid, register must fail | Ajv pass + `*.expected.json` `rule_id` |
| `trees/valid/` `trees/invalid/` | Source System trees | Ajv on `system.json` + `agents/*.agent.json`; register expected in `expected.json` |
| `evaluate/` `runtime/` | Evaluate / Runtime expected outcomes | written truth; not Ajv |

JSON Schema `default` is annotation-only. Normalization is **SRC-SEC-OMIT** / **SRC-DEFAULT-***.

## Completeness (`status.json`)

[`status.json`](status.json) is the complete `SRC-*` index. `npm test` validates its internal consistency and validates fixtures against the published schemas. It requires:

```text
indexed SRC-* = status.json rule_id set
= executed + catalog_only + excluded + missing + not_applicable
```

`Missing` must be 0. Duplicate identifiers and invalid status values fail the harness. Whole-rule status is not directory status: **SRC-PATH-DOT** stays `catalog_only` even though `invalid/empty-path-segment.agent.json` Schema-rejects `foo//bar`. **SRC-PATH-NFC** is `not_applicable` in v1alpha1 (ASCII-only paths).

Catalog-only `expected.json` files are checked against [`expected-sidecar.schema.json`](expected-sidecar.schema.json). That is structural validation, not register execution.

## SRC-PATH-TYPE recipe

[`trees/generated/path-type/`](trees/generated/path-type/) locks construct method + expected `rule_id`. `node setup.mjs` writes gitignored `work/` (symlink, FIFO, socket, directory). Device is synthetic metadata only. **`npm test` does not run setup.mjs.** Whole-rule status: `catalog_only`.
