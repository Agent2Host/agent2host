#!/usr/bin/env node
/**
 * Published Source contract fixtures:
 * - Ajv 8 Draft 2020-12 (agent, system, artifact)
 * - json_reader raw bytes (BOM, UTF-8, syntax, duplicate keys)
 * - semantic-reject: Schema-valid instances that register must still fail
 *
 * Does not apply JSON Schema default (SRC-SEC-OMIT / SRC-DEFAULT-* are semantic).
 */
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import Ajv2020 from "ajv/dist/2020.js";

const here = dirname(fileURLToPath(import.meta.url));
const schemaDir = join(here, "..");
const artifactPath = join(here, "../../../artifact/agent2host-system-v1.schema.json");

function loadJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function listJson(dir) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir)
    .filter((n) => n.endsWith(".json") && !n.endsWith(".expected.json"))
    .sort();
}

const common = loadJson(join(schemaDir, "common.schema.json"));
const system = loadJson(join(schemaDir, "system.schema.json"));
const agent = loadJson(join(schemaDir, "agent.schema.json"));
const artifact = loadJson(artifactPath);

const ajv = new Ajv2020({
  strict: true,
  allErrors: true,
  validateFormats: false,
});
ajv.addSchema(common);
ajv.addSchema(system);
ajv.addSchema(agent);
ajv.addSchema(artifact);

const expectedSidecar = loadJson(join(here, "expected-sidecar.schema.json"));
ajv.addSchema(expectedSidecar);

const validateAgent = ajv.getSchema(agent.$id);
const validateSystem = ajv.getSchema(system.$id);
const validateArtifact = ajv.getSchema(artifact.$id);
const validateSidecar = ajv.getSchema(expectedSidecar.$id);
if (!validateAgent || !validateSystem || !validateArtifact || !validateSidecar) {
  throw new Error("schema $id not registered");
}

function checkSidecar(label, data) {
  const ok = validateSidecar(data);
  if (!ok) {
    console.error(`FAIL ${label} sidecar schema`, validateSidecar.errors);
    failed++;
    return false;
  }
  return true;
}

let failed = 0;

function checkValid(label, validate, dir) {
  for (const name of listJson(dir)) {
    const inst = loadJson(join(dir, name));
    const ok = validate(inst);
    if (!ok) {
      console.error(`FAIL ${label}/${name}`, validate.errors);
      failed++;
    } else {
      console.log(`ok   ${label}/${name}`);
    }
  }
}

function checkInvalid(label, validate, dir) {
  for (const name of listJson(dir)) {
    const inst = loadJson(join(dir, name));
    const ok = validate(inst);
    if (ok) {
      console.error(`FAIL ${label}/${name} unexpectedly passed`);
      failed++;
    } else {
      console.log(`ok   ${label}/${name}`);
    }
  }
}

checkValid("valid", validateAgent, join(here, "valid"));
checkInvalid("invalid", validateAgent, join(here, "invalid"));

checkValid("system/valid", validateSystem, join(here, "system", "valid"));
checkInvalid("system/invalid", validateSystem, join(here, "system", "invalid"));

checkValid("artifact/valid", validateArtifact, join(here, "artifact", "valid"));
checkInvalid("artifact/invalid", validateArtifact, join(here, "artifact", "invalid"));

const expectedSec = loadJson(join(here, "normalize", "expected-baseline-security.json"));
for (const name of ["omit-all.agent.json", "empty-objects.agent.json", "partial-fields.agent.json"]) {
  const inst = loadJson(join(here, "normalize", name));
  const ok = validateAgent(inst);
  if (!ok) {
    console.error(`FAIL normalize/${name} schema`, validateAgent.errors);
    failed++;
  } else {
    console.log(
      `ok   normalize/${name} (schema; expected fill is ${JSON.stringify(expectedSec).length} bytes — semantic)`,
    );
  }
}

const expectedDefaults = loadJson(join(here, "system", "normalize", "expected-baseline-defaults.json"));
for (const name of ["omit-defaults.system.json", "empty-defaults.system.json", "partial-defaults.system.json"]) {
  const inst = loadJson(join(here, "system", "normalize", name));
  const ok = validateSystem(inst);
  if (!ok) {
    console.error(`FAIL system/normalize/${name} schema`, validateSystem.errors);
    failed++;
  } else {
    console.log(
      `ok   system/normalize/${name} (schema; expected fill is ${JSON.stringify(expectedDefaults).length} bytes — semantic)`,
    );
  }
}

const semanticDir = join(here, "semantic-reject");
for (const name of listJson(semanticDir)) {
  const inst = loadJson(join(semanticDir, name));
  const expectedPath = join(semanticDir, name.replace(/\.json$/, ".expected.json"));
  if (!existsSync(expectedPath)) {
    console.error(`FAIL semantic-reject/${name} missing ${name.replace(/\.json$/, ".expected.json")}`);
    failed++;
    continue;
  }
  const expected = loadJson(expectedPath);
  if (!checkSidecar(`semantic-reject/${name.replace(/\.json$/, ".expected.json")}`, expected)) {
    continue;
  }
  const kind = inst.kind;
  const validate = kind === "AgentSystem" ? validateSystem : validateAgent;
  const ok = validate(inst);
  if (!ok) {
    console.error(`FAIL semantic-reject/${name} should be schema-valid`, validate.errors);
    failed++;
  } else if (expected.schema_valid !== true || expected.register !== "fail" || !expected.rule_id) {
    console.error(`FAIL semantic-reject/${name} expected sidecar incomplete`);
    failed++;
  } else {
    console.log(
      `ok   semantic-reject/${name} (schema-valid; register fail ${expected.rule_id} — expected structurally validated)`,
    );
  }
}

function isValidUtf8(buf) {
  try {
    const dec = new TextDecoder("utf-8", { fatal: true });
    dec.decode(buf);
    return true;
  } catch {
    return false;
  }
}

function hasBom(buf) {
  return buf.length >= 3 && buf[0] === 0xef && buf[1] === 0xbb && buf[2] === 0xbf;
}

/** Returns true if any object in the JSON text repeats a key at the same object. */
function hasDuplicateKeys(text) {
  let i = 0;
  const s = text;
  function skipWs() {
    while (i < s.length && /\s/.test(s[i])) i++;
  }
  function parseString() {
    if (s[i] !== '"') throw new Error("expected string");
    i++;
    let out = "";
    while (i < s.length) {
      if (s[i] === "\\") {
        out += s[i] + (s[i + 1] ?? "");
        i += 2;
        continue;
      }
      if (s[i] === '"') {
        i++;
        return out;
      }
      out += s[i++];
    }
    throw new Error("unterminated string");
  }
  function skipLiteral(lit) {
    if (!s.startsWith(lit, i)) throw new Error(`expected ${lit}`);
    i += lit.length;
  }
  function parseValue() {
    skipWs();
    const c = s[i];
    if (c === "{") return parseObject();
    if (c === "[") return parseArray();
    if (c === '"') {
      parseString();
      return false;
    }
    if (c === "t") {
      skipLiteral("true");
      return false;
    }
    if (c === "f") {
      skipLiteral("false");
      return false;
    }
    if (c === "n") {
      skipLiteral("null");
      return false;
    }
    if (c === "-" || (c >= "0" && c <= "9")) {
      while (i < s.length && /[0-9.eE+-]/.test(s[i])) i++;
      return false;
    }
    throw new Error(`unexpected ${c}`);
  }
  function parseArray() {
    i++;
    skipWs();
    if (s[i] === "]") {
      i++;
      return false;
    }
    for (;;) {
      if (parseValue()) return true;
      skipWs();
      if (s[i] === ",") {
        i++;
        continue;
      }
      if (s[i] === "]") {
        i++;
        return false;
      }
      throw new Error("expected ]");
    }
  }
  function parseObject() {
    i++;
    const keys = new Set();
    skipWs();
    if (s[i] === "}") {
      i++;
      return false;
    }
    for (;;) {
      skipWs();
      const key = parseString();
      if (keys.has(key)) return true;
      keys.add(key);
      skipWs();
      if (s[i] !== ":") throw new Error("expected :");
      i++;
      if (parseValue()) return true;
      skipWs();
      if (s[i] === ",") {
        i++;
        continue;
      }
      if (s[i] === "}") {
        i++;
        return false;
      }
      throw new Error("expected }");
    }
  }
  return parseValue();
}

const readerDir = join(here, "json_reader");
const readerCatalog = loadJson(join(readerDir, "catalog.json"));
for (const row of readerCatalog) {
  const buf = readFileSync(join(readerDir, row.file));
  const expectInvalid = row.expect === "invalid";
  let reason = null;
  if (hasBom(buf)) reason = "bom";
  else if (!isValidUtf8(buf)) reason = "utf8";
  else {
    const text = buf.toString("utf8");
    try {
      if (hasDuplicateKeys(text)) reason = "duplicate-key";
      else JSON.parse(text);
    } catch {
      reason = "syntax";
    }
  }
  const invalid = reason !== null;
  if (!row.rule_id || row.validation_layer !== "json_reader") {
    console.error(`FAIL json_reader/${row.file} catalog row incomplete`);
    failed++;
  } else if (invalid !== expectInvalid) {
    console.error(`FAIL json_reader/${row.file} expected ${row.expect}, got ${invalid ? reason : "parsed"}`);
    failed++;
  } else {
    console.log(`ok   json_reader/${row.file} (${row.rule_id}; ${reason ?? "valid"})`);
  }
}

const validTrees = join(here, "trees", "valid");
if (existsSync(validTrees)) {
  for (const name of readdirSync(validTrees).sort()) {
    const clubRoot = join(validTrees, name);
    const sysPath = join(clubRoot, "system.json");
    if (!existsSync(sysPath)) continue;
    const expectedPath = join(clubRoot, "expected.json");
    if (!existsSync(expectedPath)) {
      console.error(`FAIL trees/valid/${name} missing expected.json`);
      failed++;
      continue;
    }
    const expected = loadJson(expectedPath);
    if (!checkSidecar(`trees/valid/${name}/expected.json`, expected)) {
      continue;
    }
    if (expected.register !== "success") {
      console.error(`FAIL trees/valid/${name}/expected.json register must be success`);
      failed++;
    }
    const sysInst = loadJson(sysPath);
    if (!validateSystem(sysInst)) {
      console.error(`FAIL trees/valid/${name}/system.json`, validateSystem.errors);
      failed++;
    } else {
      console.log(`ok   trees/valid/${name}/system.json`);
    }
    const agentsDir = join(clubRoot, "agents");
    if (!existsSync(agentsDir)) continue;
    for (const agentName of listJson(agentsDir)) {
      const inst = loadJson(join(agentsDir, agentName));
      if (!validateAgent(inst)) {
        console.error(`FAIL trees/valid/${name}/agents/${agentName}`, validateAgent.errors);
        failed++;
      } else {
        console.log(`ok   trees/valid/${name}/agents/${agentName}`);
      }
    }
    console.log(`ok   trees/valid/${name}/expected.json (structurally validated; not executed)`);
  }
}

const invalidTrees = join(here, "trees", "invalid");
if (existsSync(invalidTrees)) {
  for (const name of readdirSync(invalidTrees).sort()) {
    const root = join(invalidTrees, name);
    const expectedPath = join(root, "expected.json");
    if (!existsSync(expectedPath)) continue;
    const expected = loadJson(expectedPath);
    if (!checkSidecar(`trees/invalid/${name}/expected.json`, expected)) {
      continue;
    }
    const sysInst = loadJson(join(root, "system.json"));
    const sysOk = validateSystem(sysInst);
    if (expected.schema_valid && !sysOk) {
      console.error(`FAIL trees/invalid/${name}/system.json should be schema-valid`, validateSystem.errors);
      failed++;
      continue;
    }
    let agentsOk = true;
    const agentsDir = join(root, "agents");
    if (existsSync(agentsDir)) {
      for (const agentName of listJson(agentsDir)) {
        const inst = loadJson(join(agentsDir, agentName));
        if (!validateAgent(inst)) {
          console.error(`FAIL trees/invalid/${name}/agents/${agentName} should be schema-valid`, validateAgent.errors);
          agentsOk = false;
          failed++;
        }
      }
    }
    if (expected.register !== "fail" || !expected.rule_id) {
      console.error(`FAIL trees/invalid/${name}/expected.json incomplete`);
      failed++;
    } else if (sysOk && agentsOk) {
      console.log(`ok   trees/invalid/${name} (schema-valid; register fail ${expected.rule_id} — semantic)`);
    }
  }
}

const statusPath = join(here, "status.json");
if (!existsSync(statusPath)) {
  console.error("FAIL missing status.json (complete SRC-* index)");
  failed++;
} else {
  const status = loadJson(statusPath);
  const indexed = new Set(status.rules.map((r) => r.rule_id));
  const dupIndex = status.rules.map((r) => r.rule_id).filter((id, i, a) => a.indexOf(id) !== i);
  const counts = { executed: 0, catalog_only: 0, excluded: 0, missing: 0, not_applicable: 0 };
  for (const r of status.rules) {
    if (!(r.contract_status in counts)) {
      console.error(`FAIL status.json ${r.rule_id} unknown contract_status ${r.contract_status}`);
      failed++;
    } else {
      counts[r.contract_status]++;
    }
  }
  const sum =
    counts.executed + counts.catalog_only + counts.excluded + counts.missing + counts.not_applicable;
  if (dupIndex.length) {
    console.error(`FAIL status.json duplicate rule_id: ${[...new Set(dupIndex)].join(", ")}`);
    failed++;
  }
  if (counts.missing !== 0) {
    console.error(`FAIL Missing = ${counts.missing} (Freeze/RC requires 0)`);
    failed++;
  }
  if (sum !== indexed.size) {
    console.error(`FAIL status.json count mismatch: indexed=${indexed.size} sum=${sum}`);
    failed++;
  } else {
    console.log(
      `ok   status.json index=${indexed.size} executed=${counts.executed} catalog_only=${counts.catalog_only} excluded=${counts.excluded} missing=${counts.missing} not_applicable=${counts.not_applicable}`,
    );
  }
  for (const r of status.rules) {
    for (const rel of r.fixture ?? []) {
      const abs = join(here, rel);
      if (!existsSync(abs)) {
        console.error(`FAIL status.json ${r.rule_id} fixture missing: ${rel}`);
        failed++;
        continue;
      }
      if (rel.endsWith("/") || (existsSync(abs) && !rel.includes("."))) {
        const exp = join(abs, "expected.json");
        if (rel.startsWith("trees/") && existsSync(join(abs, "system.json")) && !existsSync(exp) && r.rule_id !== "SRC-PATH-TYPE") {
          console.error(`FAIL status.json ${r.rule_id} tree missing expected.json: ${rel}`);
          failed++;
        }
      }
    }
  }
}

const pathTypeDir = join(here, "trees", "generated", "path-type");
for (const name of ["catalog.json", "setup.mjs", "expected.json", "README.md"]) {
  if (!existsSync(join(pathTypeDir, name))) {
    console.error(`FAIL SRC-PATH-TYPE recipe missing ${name}`);
    failed++;
  }
}
if (existsSync(join(pathTypeDir, "catalog.json"))) {
  const catalog = loadJson(join(pathTypeDir, "catalog.json"));
  if (catalog.rule_id !== "SRC-PATH-TYPE" || catalog.contract_status !== "catalog_only") {
    console.error("FAIL SRC-PATH-TYPE catalog.json must stay catalog_only");
    failed++;
  } else {
    console.log("ok   SRC-PATH-TYPE recipe present (catalog_only; setup.mjs not run by this harness)");
  }
}
const pathTypeExpected = join(pathTypeDir, "expected.json");
if (existsSync(pathTypeExpected)) {
  checkSidecar("trees/generated/path-type/expected.json", loadJson(pathTypeExpected));
}

for (const extra of [
  ["evaluate/sandbox-required-false-must-not-refuse.expected.json", join(here, "evaluate", "sandbox-required-false-must-not-refuse.expected.json")],
  ["runtime/optional-secret-unset.expected.json", join(here, "runtime", "optional-secret-unset.expected.json")],
]) {
  if (!existsSync(extra[1])) {
    console.error(`FAIL missing ${extra[0]}`);
    failed++;
  } else {
    checkSidecar(extra[0], loadJson(extra[1]));
  }
}

if (failed) {
  console.error(`\n${failed} failure(s)`);
  process.exit(1);
}
console.log(
  "\nPublished contract fixtures: Ajv + json_reader + sidecar structural validation + completeness matched.",
);
