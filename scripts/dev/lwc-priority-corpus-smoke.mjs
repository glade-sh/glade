#!/usr/bin/env node
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const corpusRoot = process.env.GLADE_LWC_CORPUS || "/Users/matt/.sf-repo-analysis/repos";
const projects = [
  "src-nbm-solhub-develop",
  "src-nmb-nc-develop",
  "src-nmb-nu-develop",
  "src-nmb-nudev-develop",
  "sf-cred-pkg-develop",
];
const basePort = Number(process.env.GLADE_LWC_SMOKE_PORT || 18080);
const hold = process.argv.includes("--hold");
const durationArg = process.argv.find((arg) => arg.startsWith("--duration-ms="));
const durationMs = durationArg ? Number(durationArg.split("=", 2)[1]) : (hold ? 0 : 15000);
const procs = [];

function run(project, port) {
  const cwd = path.join(corpusRoot, project);
  const child = spawn("go", ["run", "./cmd/glade", "dev", "lwc", "--project", cwd, "--port", String(port)], {
    cwd: repoRoot,
    stdio: ["ignore", "pipe", "pipe"],
  });
  child.stdout.on("data", (data) => process.stdout.write(`[${project}] ${data}`));
  child.stderr.on("data", (data) => process.stderr.write(`[${project}] ${data}`));
  child.on("exit", (code, signal) => {
    if (signal !== "SIGTERM" && code !== 0) {
      process.stderr.write(`[${project}] exited code=${code} signal=${signal || ""}\n`);
    }
  });
  return { project, port, child };
}

function stopAll() {
  for (const proc of procs) {
    if (!proc.child.killed) {
      proc.child.kill("SIGTERM");
    }
  }
}

process.on("SIGINT", () => {
  stopAll();
  process.exit(130);
});
process.on("SIGTERM", () => {
  stopAll();
  process.exit(143);
});

for (const [index, project] of projects.entries()) {
  const proc = run(project, basePort + index);
  procs.push(proc);
  console.log(`${proc.project} http://127.0.0.1:${proc.port}/`);
}

if (durationMs > 0) {
  setTimeout(() => {
    stopAll();
  }, durationMs);
}
