#!/usr/bin/env node
/**
 * Compatibility fixtures harness (HC-030 phase 2 Schema RC + HC-040 Assertions).
 * Ajv 8 Draft 2020-12. Not CMP-VAL-CONTRACT-FREEZE.
 *
 * Encodes CMP-EVAL-REASON-INSTANCE (result → reason_code).
 * Refuses code → class if/then (CMP-EVAL-REASON-AXIS).
 * Validates published Report schema, fixture packs, and the fixture index.
 */
import { readFileSync, readdirSync, existsSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import Ajv2020 from "ajv/dist/2020.js";

const here = dirname(fileURLToPath(import.meta.url));
const schemaPath = join(here, "..", "report.schema.json");
const statusPath = join(here, "status.json");

function loadJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function listJson(dir) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir)
    .filter((n) => n.endsWith(".json"))
    .sort();
}

function fail(msg) {
  console.error(`FAIL ${msg}`);
  process.exitCode = 1;
}

function pointerGet(doc, pointer) {
  if (!pointer || pointer[0] !== "/") throw new Error(`bad pointer ${pointer}`);
  const parts = pointer
    .slice(1)
    .split("/")
    .map((p) => p.replace(/~1/g, "/").replace(/~0/g, "~"));
  let cur = doc;
  for (const p of parts) {
    if (cur == null || typeof cur !== "object" || !(p in cur)) return undefined;
    cur = cur[p];
  }
  return cur;
}

const schema = loadJson(schemaPath);
const schemaText = readFileSync(schemaPath, "utf8");
const enumCodes = schema.$defs?.reason_code?.enum;
if (!Array.isArray(enumCodes)) {
  fail("schema $defs.reason_code.enum missing");
} else {
  console.log(`ok   reason_code enum present (${enumCodes.length})`);
}

const goldenPath = join(here, "valid", "golden-report.json");
loadJson(goldenPath);
console.log("ok   golden fixture present");

if (schemaText.includes("branch_id")) {
  fail("schema must not mention branch_id (CMP-ID-BRANCH scope)");
} else {
  console.log("ok   schema has no branch_id");
}

function walk(node, path) {
  if (!node || typeof node !== "object") return;
  if (node.if && node.then) {
    const ifProps = node.if.properties || {};
    const thenProps = node.then.properties || {};
    const thenReq = node.then.required || [];
    if (ifProps.reason_code && (thenProps.requirement_result || thenReq.includes("requirement_result"))) {
      fail(`code→class if/then at ${path} (CMP-EVAL-REASON-AXIS)`);
    }
  }
  for (const [k, v] of Object.entries(node)) {
    if (typeof v === "object") walk(v, `${path}.${k}`);
  }
}
walk(schema, "$");
console.log("ok   no reason_code → requirement_result if/then");

const ajv = new Ajv2020({
  strict: true,
  allErrors: true,
  validateFormats: false,
});
ajv.addSchema(schema);
const validate = ajv.getSchema(schema.$id);
if (!validate) {
  fail("schema $id not registered");
  process.exit(1);
}

let failed = 0;

for (const name of listJson(join(here, "valid"))) {
  const inst = loadJson(join(here, "valid", name));
  const ok = validate(inst);
  if (!ok) {
    console.error(`FAIL valid/${name}`, validate.errors);
    failed++;
  } else {
    console.log(`ok   valid/${name}`);
  }
}

for (const name of listJson(join(here, "invalid"))) {
  const inst = loadJson(join(here, "invalid", name));
  const ok = validate(inst);
  if (ok) {
    console.error(`FAIL invalid/${name} unexpectedly passed`);
    failed++;
  } else {
    console.log(`ok   invalid/${name}`);
  }
}

// --- HC-040 ---
if (!existsSync(statusPath)) {
  fail("status.json missing (HC-040)");
} else {
  const status = loadJson(statusPath);
  const ruleIds = status.rules.map((r) => r.rule_id);
  const ebIds = status.evaluate_branches.map((r) => r.branch_id);
  if (new Set(ebIds).size !== ebIds.length) fail("evaluate_branches branch_id not unique");
  if (new Set(ruleIds).size !== ruleIds.length) fail("status.json rule_id not unique");
  console.log(`ok   status.json rules=${ruleIds.length} evaluate_branches=${ebIds.length}`);

  function summarizeItems(items) {
    const s = {
      satisfied: 0,
      degraded: 0,
      unsatisfied: 0,
      unknown: 0,
      included: 0,
      omitted: 0,
      decision_impact: "pass",
    };
    let reqFail = false;
    let soft = false;
    for (const it of items || []) {
      const rr = it.requirement_result;
      if (rr === "satisfied") s.satisfied++;
      else if (rr === "degraded") {
        s.degraded++;
        soft = true;
      } else if (rr === "unsatisfied") {
        s.unsatisfied++;
        if (it.required === true) reqFail = true;
        else soft = true;
      } else if (rr === "unknown") {
        s.unknown++;
        if (it.required === true) reqFail = true;
        else soft = true;
      }
      if (it.disposition === "include") s.included++;
      else if (it.disposition === "omit") s.omitted++;
    }
    if (reqFail) s.decision_impact = "refuse";
    else if (soft) s.decision_impact = "warning";
    return s;
  }

  function recomputeDecision(report, req) {
    let refuse = false;
    let warn = false;
    const check = (rr, required, disp, projectable) => {
      if (required && (rr === "unsatisfied" || rr === "unknown")) refuse = true;
      if (rr === "degraded") warn = true;
      if (
        !required &&
        (rr === "unsatisfied" ||
          (projectable &&
            (rr === "unknown" || (disp === "omit" && rr !== "satisfied"))))
      ) {
        warn = true;
      }
    };
    check(report.activation?.requirement_result, true, "", false);
    check(
      report.capabilities?.sop?.requirement_result,
      true,
      report.capabilities?.sop?.disposition,
      true,
    );
    check(
      report.capabilities?.output_schema?.requirement_result,
      req.output_schema?.required === true,
      report.capabilities?.output_schema?.disposition,
      true,
    );
    for (const it of report.capabilities?.skills?.items || []) {
      check(it.requirement_result, it.required === true, it.disposition, true);
    }
    for (const it of report.capabilities?.context?.items || []) {
      check(it.requirement_result, it.required === true, it.disposition, true);
    }
    for (const it of report.capabilities?.mcp?.items || []) {
      check(it.requirement_result, it.required === true, it.disposition, true);
    }
    for (const it of report.capabilities?.hooks?.items || []) {
      check(it.requirement_result, it.required === true, it.disposition, true);
    }
    check(report.security?.permissions?.requirement_result, true, "", false);
    check(report.security?.approvals?.requirement_result, true, "", false);
    check(
      report.security?.sandbox?.requirement_result,
      req.security?.sandbox?.required === true,
      "",
      false,
    );
    check(
      report.security?.output_validation?.requirement_result,
      req.security?.output_validation?.required === true,
      "",
      false,
    );
    for (const it of report.security?.context_isolation?.items || []) {
      check(it.requirement_result, it.required === true, "", false);
    }
    for (const it of report.security?.mcp_tool_isolation?.items || []) {
      check(it.requirement_result, true, "", false);
    }
    for (const it of report.security?.secret_isolation?.items || []) {
      check(it.requirement_result, it.required === true, "", false);
    }
    if (refuse) return "refused";
    if (warn) return "allowed_with_warnings";
    return "allowed";
  }

  function nonSatisfiedItems(report) {
    const out = [];
    const add = (locator, node) => {
      const rr = node?.requirement_result;
      if (rr && rr !== "satisfied") out.push({ locator, result: rr });
    };
    add("/activation", report.activation);
    add("/capabilities/sop", report.capabilities?.sop);
    add("/capabilities/output_schema", report.capabilities?.output_schema);
    add("/security/permissions", report.security?.permissions);
    add("/security/approvals", report.security?.approvals);
    add("/security/sandbox", report.security?.sandbox);
    add("/security/output_validation", report.security?.output_validation);
    (report.capabilities?.skills?.items || []).forEach((it, i) =>
      add(`/capabilities/skills/items/${i}`, it),
    );
    (report.capabilities?.context?.items || []).forEach((it, i) =>
      add(`/capabilities/context/items/${i}`, it),
    );
    (report.capabilities?.mcp?.items || []).forEach((it, i) =>
      add(`/capabilities/mcp/items/${i}`, it),
    );
    (report.capabilities?.hooks?.items || []).forEach((it, i) =>
      add(`/capabilities/hooks/items/${i}`, it),
    );
    (report.security?.context_isolation?.items || []).forEach((it, i) =>
      add(`/security/context_isolation/items/${i}`, it),
    );
    (report.security?.mcp_tool_isolation?.items || []).forEach((it, i) =>
      add(`/security/mcp_tool_isolation/items/${i}`, it),
    );
    (report.security?.secret_isolation?.items || []).forEach((it, i) =>
      add(`/security/secret_isolation/items/${i}`, it),
    );
    return out;
  }

  function assertNoEvalFields(node, path) {
    if (Array.isArray(node)) {
      node.forEach((x, i) => assertNoEvalFields(x, `${path}/${i}`));
      return;
    }
    if (!node || typeof node !== "object") return;
    for (const k of ["decision", "requirement_result", "disposition", "reason_code"]) {
      if (Object.prototype.hasOwnProperty.call(node, k)) {
        fail(`assessment ${path}: forbidden Evaluate field ${k}`);
      }
    }
    for (const [k, v] of Object.entries(node)) assertNoEvalFields(v, `${path}/${k}`);
  }

  // Build sidecar index: fixture path → covered_branches
  const fixtureToClaims = new Map();
  const indexedFixtures = new Set();
  for (const row of status.evaluate_branches) {
    for (const f of row.fixtures || []) indexedFixtures.add(f);
  }
  const evalDir = join(here, "evaluate");
  for (const name of readdirSync(evalDir).sort()) {
    const dir = join(evalDir, name);
    if (!statSync(dir).isDirectory()) continue;
    const reportPath = join(dir, "expected-report.json");
    const sidePath = join(dir, "covered_branches.json");
    const reqPath = join(dir, "requirement.json");
    const assessPath = join(dir, "assessment.json");
    if (!existsSync(reportPath) || !existsSync(sidePath) || !existsSync(reqPath) || !existsSync(assessPath)) {
      fail(`evaluate/${name} missing expected-report/covered_branches/requirement/assessment`);
      failed++;
      continue;
    }
    const fixtureKey = `evaluate/${name}`;
    if (!indexedFixtures.has(fixtureKey)) {
      fail(`evaluate/${name}: orphan pack not cited in evaluate_branches[].fixtures`);
      failed++;
    }
    const report = loadJson(reportPath);
    const ok = validate(report);
    if (!ok) {
      console.error(`FAIL evaluate/${name}/expected-report.json`, validate.errors);
      failed++;
      continue;
    }
    const assessment = loadJson(assessPath);
    assertNoEvalFields(assessment, fixtureKey);
    for (const key of ["skills", "context", "mcp", "hooks"]) {
      const block = report.capabilities?.[key];
      if (block?.summary && Array.isArray(block.items)) {
        const expected = summarizeItems(block.items);
        if (JSON.stringify(block.summary) !== JSON.stringify(expected)) {
          fail(`evaluate/${name}: capabilities.${key}.summary ≠ recomputed from items`);
          failed++;
        }
      }
    }
    const side = loadJson(sidePath);
    const req = loadJson(reqPath);
    const recomputed = recomputeDecision(report, req);
    if (report.decision !== recomputed) {
      fail(`evaluate/${name}: decision ${report.decision} ≠ recomputed ${recomputed}`);
      failed++;
    }
    const claims = side.covered_branches || [];
    const locators = new Set();
    for (const c of claims) {
      if (locators.has(c.item_locator)) {
        fail(`evaluate/${name}: duplicate item_locator ${c.item_locator}`);
        failed++;
      }
      locators.add(c.item_locator);
      const item = pointerGet(report, c.item_locator);
      if (item === undefined) {
        fail(`evaluate/${name}: locator ${c.item_locator} does not resolve`);
        failed++;
        continue;
      }
      if (item.requirement_result !== c.requirement_result) {
        fail(
          `evaluate/${name}: ${c.branch_id} locator result ${item.requirement_result} ≠ claim ${c.requirement_result}`,
        );
        failed++;
      }
      if (c.requirement_result !== "satisfied") {
        if (item.reason_code !== c.reason_code) {
          fail(
            `evaluate/${name}: ${c.branch_id} locator reason ${item.reason_code} ≠ claim ${c.reason_code}`,
          );
          failed++;
        }
      }
      // Twin identity: required vs optional skills/contexts must match Requirement slice when present
      if (Array.isArray(req.skills) && c.item_locator.startsWith("/capabilities/skills/items/")) {
        const idx = Number(c.item_locator.split("/").pop());
        const reqItem = req.skills[idx];
        if (reqItem && typeof item.required === "boolean" && reqItem.required !== item.required) {
          fail(`evaluate/${name}: ${c.branch_id} required mismatch vs Requirement slice`);
          failed++;
        }
      }
    }
    const claimed = new Set(claims.map((c) => c.item_locator));
    for (const { locator, result } of nonSatisfiedItems(report)) {
      if (!claimed.has(locator)) {
        fail(`evaluate/${name}: unclaimed ${result} item ${locator}`);
        failed++;
      }
    }
    fixtureToClaims.set(fixtureKey, claims);
    console.log(`ok   evaluate/${name} (${claims.length} claims)`);
  }

  for (const row of status.evaluate_branches) {
    if (!row.fixtures?.length) {
      fail(`${row.branch_id}: fixtures[] empty`);
      failed++;
      continue;
    }
    let found = false;
    for (const f of row.fixtures) {
      const claims = fixtureToClaims.get(f);
      if (!claims) {
        fail(`${row.branch_id}: fixture ${f} missing`);
        failed++;
        continue;
      }
      const hit = claims.find((c) => c.branch_id === row.branch_id);
      if (!hit) continue;
      found = true;
      if (hit.requirement_result !== row.requirement_result || hit.reason_code !== row.reason_code) {
        fail(`${row.branch_id}: sidecar claim ≠ index row`);
        failed++;
      }
      if (hit.rule_id !== row.rule_id) {
        fail(`${row.branch_id}: sidecar rule_id ≠ index`);
        failed++;
      }
    }
    if (!found) {
      fail(`${row.branch_id}: not cited in any sidecar covered_branches`);
      failed++;
    }
  }
  console.log("ok   Assertion 2 sidecar ↔ index bidirectional + locator proof");

  // protocol present
  const proto = join(here, "protocol");
  if (!existsSync(join(proto, "row-10-revision-mismatch.json")) || !existsSync(join(proto, "row-10b-fingerprint-mismatch.json"))) {
    fail("protocol/ 10 and 10b fixtures missing");
  } else {
    console.log("ok   protocol/ 10 and 10b present (not in evaluate_branches)");
  }

  // AXIS valid fixtures must not appear as evaluate fixtures
  for (const row of status.evaluate_branches) {
    for (const f of row.fixtures || []) {
      if (f.includes("axis-")) {
        fail(`AXIS shape fixture must not be an evaluate branch fixture: ${f}`);
        failed++;
      }
    }
  }
}

if (failed) {
  process.exitCode = 1;
} else if (!process.exitCode) {
  console.log("all compatibility report fixtures passed");
}
