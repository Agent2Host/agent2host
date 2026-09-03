#!/usr/bin/env node
/**
 * Emit Ajv 8 accept/reject for the same frozen instance set Go TestFrozenSchemaVerdicts uses.
 * stdout: JSON array of {rel, kind, accept}.
 * Exit 2 if Ajv cannot be imported (caller should skip).
 */
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const schemaDir = join(here, "..");
const artifactPath = join(here, "../../../artifact/agent2host-system-v1.schema.json");

let Ajv2020;
try {
  Ajv2020 = (await import("ajv/dist/2020.js")).default;
} catch {
  process.stderr.write("ajv not installed\n");
  process.exit(2);
}

function loadJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

const skipNames = new Set([
  "catalog.json",
  "package.json",
  "status.json",
  "package-lock.json",
]);

function listSuffix(dir, suffix) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir)
    .filter(
      (n) =>
        n.endsWith(suffix) && !n.includes("expected") && !skipNames.has(n),
    )
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

const validateAgent = ajv.getSchema(agent.$id);
const validateSystem = ajv.getSchema(system.$id);
const validateArtifact = ajv.getSchema(artifact.$id);

const rows = [];

function add(relDir, kind, validate, suffix) {
  const dir = join(here, ...relDir.split("/"));
  for (const name of listSuffix(dir, suffix)) {
    const inst = loadJson(join(dir, name));
    rows.push({
      rel: relDir + "/" + name,
      kind,
      accept: !!validate(inst),
    });
  }
}

add("valid", "agent", validateAgent, ".agent.json");
add("invalid", "agent", validateAgent, ".agent.json");
add("system/valid", "system", validateSystem, ".system.json");
add("system/invalid", "system", validateSystem, ".system.json");
add("artifact/valid", "artifact", validateArtifact, ".json");
add("artifact/invalid", "artifact", validateArtifact, ".json");
add("normalize", "agent", validateAgent, ".agent.json");
add("system/normalize", "system", validateSystem, ".system.json");
add("semantic-reject", "agent", validateAgent, ".agent.json");
add("semantic-reject", "system", validateSystem, ".system.json");

process.stdout.write(JSON.stringify(rows));
