#!/usr/bin/env node
/**
 * Build throwaway SRC-PATH-TYPE nodes under work/. Does not run register.
 * Usage: node setup.mjs
 *
 * Unix sockets: bind under os.tmpdir() because Darwin sun_path is ~104 bytes.
 * Node server.close() unlinks the socket; this script listens, stats, then
 * exits without close() so a stale socket inode may remain for inspection.
 */
import { cpSync, mkdirSync, readFileSync, rmSync, statSync, writeFileSync, symlinkSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { createServer } from "node:net";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const work = join(here, "work");
const template = join(here, "source-template");

rmSync(work, { recursive: true, force: true });
mkdirSync(work, { recursive: true });

function copyBase(name) {
  const dest = join(work, name);
  cpSync(template, dest, { recursive: true });
  return dest;
}

function writeSop(root, sop) {
  const agentPath = join(root, "agents", "demo.agent.json");
  const agent = JSON.parse(readFileSync(agentPath, "utf8"));
  agent.sop = sop;
  writeFileSync(agentPath, `${JSON.stringify(agent, null, 2)}\n`);
}

const posix = process.platform !== "win32";
const report = [];

{
  const root = copyBase("symlink-leaf");
  writeSop(root, "./sops/link.sop.md");
  symlinkSync("demo.sop.md", join(root, "sops", "link.sop.md"));
  report.push({ id: "symlink-leaf", created: true });
}

{
  const root = copyBase("symlink-component");
  writeSop(root, "./sops-link/demo.sop.md");
  symlinkSync("sops", join(root, "sops-link"));
  report.push({ id: "symlink-component", created: true });
}

{
  const root = copyBase("directory");
  writeSop(root, "./sops/not-a-file.sop.md");
  mkdirSync(join(root, "sops", "not-a-file.sop.md"));
  report.push({ id: "directory", created: true });
}

if (posix) {
  const root = copyBase("fifo");
  writeSop(root, "./sops/fifo.sop.md");
  const r = spawnSync("mkfifo", [join(root, "sops", "fifo.sop.md")], { encoding: "utf8" });
  report.push({ id: "fifo", created: r.status === 0, stderr: r.stderr || undefined });
} else {
  report.push({ id: "fifo", created: false, skip_reason: "windows" });
}

if (posix) {
  const sockRoot = join(tmpdir(), "amx-pt", String(process.pid));
  rmSync(sockRoot, { recursive: true, force: true });
  cpSync(template, sockRoot, { recursive: true });
  writeSop(sockRoot, "./sops/s");
  const sockPath = join(sockRoot, "sops", "s");
  try {
    await new Promise((resolve, reject) => {
      const srv = createServer();
      srv.on("error", reject);
      srv.listen(sockPath, () => {
        srv.unref();
        resolve();
      });
    });
    const st = statSync(sockPath);
    report.push({
      id: "unix-socket",
      created: st.isSocket(),
      root: sockRoot,
      sop: "./sops/s",
      notes: "Short tmp root because Darwin sun_path is ~104 bytes. Do not use the long fixtures/work path.",
    });
  } catch (err) {
    report.push({
      id: "unix-socket",
      created: false,
      skip_reason: "listen_failed",
      error: String(err),
    });
  }
} else {
  report.push({ id: "unix-socket", created: false, skip_reason: "windows" });
}

report.push({
  id: "device",
  created: false,
  skip_reason: "synthetic_metadata",
  notes: "Do not mknod. Go tests MAY stub stat().",
});

writeFileSync(join(work, "setup-report.json"), `${JSON.stringify(report, null, 2)}\n`);
console.log(`SRC-PATH-TYPE work tree written under ${work}`);
console.log(JSON.stringify(report, null, 2));
