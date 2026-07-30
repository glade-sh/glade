#!/usr/bin/env node
import { createHash } from "node:crypto";
import { mkdir, readFile, readdir, rename, writeFile } from "node:fs/promises";
import { spawn } from "node:child_process";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const scriptRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const proofNames = ["verify", "test:unit", "build:site"];

export async function runReleaseCheck({
  siteRoot = process.env.SITE_RELEASE_CHECK_ROOT || scriptRoot,
  summaryPath = process.env.SITE_RELEASE_CHECK_SUMMARY || join(siteRoot, ".vitepress", "release-check.json"),
  quiet = false
} = {}) {
  siteRoot = resolve(siteRoot);
  const beforeDigest = await sourceDigest(siteRoot);
  const proofs = [];
  for (const name of proofNames) {
    const exitCode = await runNpm(siteRoot, name, quiet);
    proofs.push({ name, invocations: 1, exitCode });
  }
  const afterDigest = await sourceDigest(siteRoot);
  if (beforeDigest !== afterDigest) {
    throw new Error("site release inputs changed while checks were running");
  }
  const summary = {
    schemaVersion: 1,
    siteRoot,
    source: { digest: beforeDigest, afterDigest },
    proofs
  };
  await writeSummary(summaryPath, summary);
  if (!quiet) console.log(JSON.stringify(summary));
  return summary;
}

async function runNpm(siteRoot, name, quiet) {
  const result = await new Promise((resolveResult, reject) => {
    const child = spawn("npm", ["run", name], {
      cwd: siteRoot,
      env: Object.fromEntries(Object.entries(process.env).filter(([key]) => key !== "NODE_TEST_CONTEXT")),
      stdio: quiet ? "pipe" : "inherit"
    });
    let output = "";
    if (quiet) {
      child.stdout.on("data", (chunk) => { output += chunk; });
      child.stderr.on("data", (chunk) => { output += chunk; });
    }
    child.on("error", reject);
    child.on("close", (code, signal) => resolveResult({ code, signal, output }));
  });
  if (result.code !== 0) {
    const detail = result.signal ? ` signal=${result.signal}` : "";
    throw new Error(`site release proof failed: ${name}${detail}\n${result.output}`.trim());
  }
  return result.code;
}

async function sourceDigest(siteRoot) {
  const hash = createHash("sha256");
  const repoRoot = resolve(siteRoot, "..");
  await hashTree(hash, siteRoot, siteRoot);
  await hashFile(hash, join(repoRoot, "docs", "STDLIB_COVERAGE.md"), repoRoot);
  return hash.digest("hex");
}

async function hashTree(hash, root, directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  for (const entry of entries.sort((a, b) => a.name.localeCompare(b.name))) {
    const file = join(directory, entry.name);
    const pathFromRoot = relative(root, file);
    if (entry.isDirectory()) {
      if (pathFromRoot === "node_modules" || pathFromRoot === ".vitepress/cache" || pathFromRoot === ".vitepress/dist") continue;
      await hashTree(hash, root, file);
      continue;
    }
    if (!entry.isFile() || pathFromRoot === ".vitepress/release-check.json") continue;
    await hashFile(hash, file, root);
  }
}

async function hashFile(hash, file, root) {
  hash.update(relative(root, file));
  hash.update("\0");
  hash.update(await readFile(file));
  hash.update("\0");
}

async function writeSummary(summaryPath, summary) {
  await mkdir(dirname(summaryPath), { recursive: true });
  const temporaryPath = `${summaryPath}.tmp-${process.pid}`;
  await writeFile(temporaryPath, `${JSON.stringify(summary, null, 2)}\n`);
  await rename(temporaryPath, summaryPath);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  runReleaseCheck().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
