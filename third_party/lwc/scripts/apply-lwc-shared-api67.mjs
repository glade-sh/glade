#!/usr/bin/env node
// Applies the checked-in, bounded API-version identity patch to @lwc/shared 9.4.3.
// Usage: node scripts/apply-lwc-shared-api67.mjs [--check] [<third_party/lwc-root>]
// The optional root contains package.json and node_modules; it is not node_modules itself.
// With no root, use this script's enclosing third_party/lwc directory.

import { createHash, randomUUID } from "node:crypto";
import { lstat, open, readFile, rename, rm } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptPath = fileURLToPath(import.meta.url);
const scriptDir = path.dirname(scriptPath);
const defaultToolchainRoot = path.resolve(scriptDir, "..");
const patchPath = path.join(scriptDir, "..", "patches", "lwc-shared-9.4.3-api67.patch");
const sharedPackageIntegrity = "sha512-fASEsiokgNbSPKl0BxI836FeORTjKvSCTiFfYVx8YR3HNacCrwFODazrA+HLXZC11TDQ69q1kaJcmZoDFSo4Xg==";
const maxPatchTargets = 3;
const maxTargetBytes = 256 * 1024;
const maxOriginalBytes = maxPatchTargets * maxTargetBytes;
// At most three original backups and three replacements are staged; rollback
// adds one original-sized temporary file at a time.
const maxTemporaryBytes = 2 * maxOriginalBytes + maxTargetBytes;

const preimageSHA256 = Object.freeze({
  "node_modules/@lwc/shared/dist/index.js": "fa6e1b78f03f5f82ec995ed1b416e91f24eae2f0dc23a36b8d25b01e725a4693",
  "node_modules/@lwc/shared/dist/index.cjs": "cf551eabbf60b149ca9a5ed533a4ed6fbf7e36e1cc8164f2e838165502c974db",
  "node_modules/@lwc/shared/dist/api-version.d.ts": "67ecc27f5eb3b94a1a0b09337a442fdfbf89858f192785187a9091aacd4aabe6",
});

const postimageSHA256 = Object.freeze({
  "node_modules/@lwc/shared/dist/index.js": "6955c7eb529ba2abd7d0678d2cca688388192c94f629ee0f643a6278f558fa27",
  "node_modules/@lwc/shared/dist/index.cjs": "a131380555c162cabfe02ee0b917c54e5e6b8ca538822987b350a0e3cec8f52a",
  "node_modules/@lwc/shared/dist/api-version.d.ts": "00a9f517ea41c455849c5a7c7ee09e79f3d189fa97fc2072862f8657d3019a6e",
});

function sha256(value) {
  return createHash("sha256").update(value).digest("hex");
}

function splitText(value) {
  const hasFinalNewline = value.endsWith("\n");
  const lines = value.split("\n");
  if (hasFinalNewline) lines.pop();
  return { lines, hasFinalNewline };
}

function joinText(lines, hasFinalNewline) {
  return `${lines.join("\n")}${hasFinalNewline ? "\n" : ""}`;
}

export function parseUnifiedPatch(patchText) {
  const lines = patchText.split(/\r?\n/);
  const files = new Map();
  let index = 0;

  while (index < lines.length) {
    const line = lines[index++];
    if (line === "" || line.startsWith("#")) continue;
    if (!line.startsWith("--- a/")) throw new Error(`unexpected patch line: ${line}`);
    const target = line.slice("--- a/".length);
    if (lines[index++] !== `+++ b/${target}`) throw new Error(`patch target header mismatch for ${target}`);
    if (files.has(target)) throw new Error(`duplicate patch target ${target}`);

    const hunks = [];
    while (index < lines.length && lines[index].startsWith("@@ ")) {
      const header = lines[index++];
      const match = header.match(/^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(?:.*)$/);
      if (!match) throw new Error(`invalid patch hunk header: ${header}`);
      const hunk = {
        oldStart: Number(match[1]),
        oldCount: match[2] === undefined ? 1 : Number(match[2]),
        newCount: match[4] === undefined ? 1 : Number(match[4]),
        lines: [],
      };
      let oldCount = 0;
      let newCount = 0;
      while (index < lines.length) {
        const hunkLine = lines[index];
        if (hunkLine.startsWith("@@ ") || hunkLine.startsWith("--- a/")) break;
        if (hunkLine === "\\ No newline at end of file") {
          index++;
          continue;
        }
        if (!/^[ +\-]/.test(hunkLine)) break;
        index++;
        const kind = hunkLine[0];
        hunk.lines.push({ kind, value: hunkLine.slice(1) });
        if (kind !== "+") oldCount++;
        if (kind !== "-") newCount++;
      }
      if (oldCount !== hunk.oldCount || newCount !== hunk.newCount) {
        throw new Error(`patch hunk counts do not match for ${target}`);
      }
      hunks.push(hunk);
    }
    if (hunks.length === 0) throw new Error(`patch contains no hunks for ${target}`);
    files.set(target, hunks);
  }
  return files;
}

export function applyUnifiedPatch(targets, patchText) {
  const hunksByPath = parseUnifiedPatch(patchText);
  if (hunksByPath.size !== targets.size) {
    throw new Error(`patch targets ${hunksByPath.size} files but ${targets.size} preimages were supplied`);
  }
  const outputs = new Map();
  for (const [target, hunks] of hunksByPath) {
    if (!targets.has(target)) throw new Error(`unexpected patch target ${target}`);
    const original = Buffer.from(targets.get(target)).toString("utf8");
    const { lines: input, hasFinalNewline } = splitText(original);
    const output = [];
    let cursor = 0;
    for (const hunk of hunks) {
      const hunkStart = hunk.oldCount === 0 ? hunk.oldStart : hunk.oldStart - 1;
      if (hunkStart < cursor || hunkStart > input.length) {
        throw new Error(`patch hunk position is invalid for ${target}`);
      }
      output.push(...input.slice(cursor, hunkStart));
      cursor = hunkStart;
      for (const line of hunk.lines) {
        if (line.kind === " ") {
          if (input[cursor] !== line.value) throw new Error(`patch context mismatch in ${target} at line ${cursor + 1}`);
          output.push(input[cursor++]);
        } else if (line.kind === "-") {
          if (input[cursor] !== line.value) throw new Error(`patch preimage mismatch in ${target} at line ${cursor + 1}`);
          cursor++;
        } else {
          output.push(line.value);
        }
      }
    }
    output.push(...input.slice(cursor));
    outputs.set(target, Buffer.from(joinText(output, hasFinalNewline), "utf8"));
  }
  return outputs;
}

export function buildPatchPlan({ patchText, targets, preimages = preimageSHA256, postimages = postimageSHA256, checkOnly = false }) {
  const expectedTargets = Object.keys(preimages).sort();
  if (JSON.stringify([...targets.keys()].sort()) !== JSON.stringify(expectedTargets)) {
    throw new Error("the complete pinned @lwc/shared API 67 target set was not supplied");
  }

  const states = new Map();
  for (const target of expectedTargets) {
    const digest = sha256(targets.get(target));
    states.set(target, digest === preimages[target] ? "preimage" : digest === postimages[target] ? "postimage" : "mismatch");
  }
  const allPostimages = [...states.values()].every((state) => state === "postimage");
  const allPreimages = [...states.values()].every((state) => state === "preimage");
  if (allPostimages) return { state: "prepared", writes: new Map() };
  if (!allPreimages) {
    const detail = [...states].filter(([, state]) => state !== "preimage").map(([target, state]) => `${target}=${state}`).join(", ");
    throw new Error(`refusing partial or unknown @lwc/shared patch state; no files written (${detail})`);
  }
  if (checkOnly) {
    throw new Error("API 67 toolchain preparation is required: all @lwc/shared targets are still at pinned 9.4.3 preimages; --check is read-only");
  }

  const writes = applyUnifiedPatch(targets, patchText);
  for (const target of expectedTargets) {
    if (sha256(writes.get(target)) !== postimages[target]) {
      throw new Error(`computed @lwc/shared postimage hash mismatch for ${target}; no files written`);
    }
  }
  return { state: "apply", writes };
}

async function readBoundedRegularFile(filePath, label) {
  const before = await lstat(filePath);
  if (!before.isFile()) throw new Error(`${label} is not a regular file: ${filePath}`);
  if (before.size > maxTargetBytes) throw new Error(`${label} exceeds the ${maxTargetBytes}-byte per-file limit: ${filePath}`);
  const handle = await open(filePath, "r");
  try {
    const opened = await handle.stat();
    if (!opened.isFile() || opened.dev !== before.dev || opened.ino !== before.ino || opened.size > maxTargetBytes) {
      throw new Error(`${label} changed identity or exceeded its bound while opening: ${filePath}`);
    }
    const contents = Buffer.alloc(maxTargetBytes + 1);
    let bytesRead = 0;
    while (bytesRead < contents.length) {
      const result = await handle.read(contents, bytesRead, contents.length - bytesRead, bytesRead);
      if (result.bytesRead === 0) break;
      bytesRead += result.bytesRead;
    }
    const after = await handle.stat();
    if (bytesRead > maxTargetBytes || bytesRead !== opened.size || after.size !== opened.size) {
      throw new Error(`${label} changed size or exceeded its bound while being read: ${filePath}`);
    }
    return Buffer.from(contents.subarray(0, bytesRead));
  } finally {
    await handle.close();
  }
}

async function writeTemporaryFile(filePath, contents, mode) {
  const handle = await open(filePath, "wx", mode & 0o777);
  try {
    await handle.writeFile(contents);
    await handle.chmod(mode & 0o777);
    await handle.sync();
  } finally {
    await handle.close();
  }
}

async function cleanupTemporaryFiles(entries, retained = new Set()) {
  const errors = [];
  for (const entry of entries) {
    for (const filePath of [entry.replacementPath, entry.backupPath, entry.rollbackPath]) {
      if (!filePath || retained.has(filePath)) continue;
      try {
        await rm(filePath, { force: true });
      } catch (error) {
        errors.push({ filePath, error });
      }
    }
  }
  return errors;
}

export async function replaceTargetsAtomically({
  root,
  writes,
  originals,
  preimages = preimageSHA256,
  postimages = postimageSHA256,
  renameFile = rename,
}) {
  const expectedTargets = Object.keys(preimages).sort();
  const sameKeys = (keys) => JSON.stringify([...keys].sort()) === JSON.stringify(expectedTargets);
  if (expectedTargets.length === 0 || expectedTargets.length > maxPatchTargets
    || !sameKeys(Object.keys(postimages)) || !sameKeys(writes.keys()) || !sameKeys(originals.keys())) {
    throw new Error("atomic replacement requires the complete bounded target, original, and output sets");
  }

  let originalTotal = 0;
  let replacementTotal = 0;
  const rootPath = path.resolve(root);
  const entries = [];
  for (const target of expectedTargets) {
    const original = Buffer.from(originals.get(target));
    const replacement = Buffer.from(writes.get(target));
    if (original.length > maxTargetBytes || replacement.length > maxTargetBytes) {
      throw new Error(`atomic replacement exceeds the ${maxTargetBytes}-byte per-file limit for ${target}`);
    }
    originalTotal += original.length;
    replacementTotal += replacement.length;
    if (originalTotal > maxOriginalBytes || replacementTotal > maxOriginalBytes
      || originalTotal + replacementTotal > maxTemporaryBytes) {
      throw new Error("atomic replacement exceeds its bounded original or temporary byte budget");
    }
    if (sha256(original) !== preimages[target]) throw new Error(`refusing unknown original bytes for ${target}; no files written`);
    if (sha256(replacement) !== postimages[target]) throw new Error(`refusing unknown replacement bytes for ${target}; no files written`);

    if (path.isAbsolute(target)) throw new Error(`refusing absolute patch target ${target}`);
    const targetPath = path.resolve(rootPath, ...target.split("/"));
    const relative = path.relative(rootPath, targetPath);
    if (!relative || relative === ".." || relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative)) {
      throw new Error(`refusing patch target outside the toolchain root: ${target}`);
    }
    const info = await lstat(targetPath);
    if (!info.isFile() || info.size > maxTargetBytes) {
      throw new Error(`patch target is not a bounded regular file: ${targetPath}`);
    }
    const onDisk = await readBoundedRegularFile(targetPath, "patch target");
    if (!onDisk.equals(original)) throw new Error(`patch target changed after prevalidation; no files written: ${targetPath}`);

    const temporaryBase = `${targetPath}.glade-api67-${process.pid}-${randomUUID()}`;
    entries.push({
      target,
      targetPath,
      original,
      replacement,
      mode: info.mode,
      replacementPath: `${temporaryBase}.new`,
      backupPath: `${temporaryBase}.original`,
      rollbackPath: `${temporaryBase}.rollback`,
      committed: false,
    });
  }

  let temporaryBytes = 0;
  try {
    for (const entry of entries) {
      temporaryBytes += entry.original.length + entry.replacement.length;
      if (temporaryBytes > maxTemporaryBytes) throw new Error("staged patch exceeds the bounded temporary byte budget");
      await writeTemporaryFile(entry.replacementPath, entry.replacement, entry.mode);
      await writeTemporaryFile(entry.backupPath, entry.original, entry.mode);
    }

    // Recheck every target after staging and before the first replacement.
    for (const entry of entries) {
      const current = await readBoundedRegularFile(entry.targetPath, "patch target");
      if (!current.equals(entry.original)) throw new Error(`patch target changed during staging; no files written: ${entry.targetPath}`);
    }

    for (const entry of entries) {
      const current = await readBoundedRegularFile(entry.targetPath, "patch target");
      if (!current.equals(entry.original)) throw new Error(`patch target changed before replacement: ${entry.targetPath}`);
      await renameFile(entry.replacementPath, entry.targetPath);
      entry.committed = true;
    }

    for (const entry of entries) {
      const current = await readBoundedRegularFile(entry.targetPath, "patched target");
      if (sha256(current) !== postimages[entry.target]) throw new Error(`postimage verification failed for ${entry.target}`);
    }
  } catch (error) {
    const retained = new Set();
    const rollbackErrors = [];
    for (const entry of [...entries].reverse()) {
      if (!entry.committed) continue;
      try {
        const current = await readBoundedRegularFile(entry.targetPath, "replacement target during rollback");
        if (sha256(current) !== postimages[entry.target]) {
          retained.add(entry.backupPath);
          rollbackErrors.push(`${entry.target}: current file is unknown; original retained at ${entry.backupPath}`);
          continue;
        }
        if (temporaryBytes + entry.original.length > maxTemporaryBytes) {
          throw new Error(`rollback for ${entry.target} exceeds the bounded temporary byte budget`);
        }
        await writeTemporaryFile(entry.rollbackPath, entry.original, entry.mode);
        await renameFile(entry.rollbackPath, entry.targetPath);
        const restored = await readBoundedRegularFile(entry.targetPath, "restored target");
        if (sha256(restored) !== preimages[entry.target]) {
          retained.add(entry.backupPath);
          rollbackErrors.push(`${entry.target}: restored bytes did not verify; original retained at ${entry.backupPath}`);
          continue;
        }
        entry.committed = false;
      } catch (rollbackError) {
        retained.add(entry.backupPath);
        rollbackErrors.push(`${entry.target}: ${rollbackError.message}; original retained at ${entry.backupPath}`);
      }
    }
    const cleanupErrors = await cleanupTemporaryFiles(entries, retained);
    if (rollbackErrors.length > 0 || cleanupErrors.length > 0) {
      const details = [...rollbackErrors, ...cleanupErrors.map(({ filePath, error: cleanupError }) => `${filePath}: ${cleanupError.message}`)].join("; ");
      throw new Error(`${error.message}; rollback/cleanup incomplete: ${details}`, { cause: error });
    }
    throw error;
  }

  const cleanupErrors = await cleanupTemporaryFiles(entries);
  if (cleanupErrors.length > 0) {
    const details = cleanupErrors.map(({ filePath, error }) => `${filePath}: ${error.message}`).join("; ");
    throw new Error(`all API 67 postimages verified, but temporary cleanup failed: ${details}`);
  }
}

async function parseJSON(filePath, label) {
  let text;
  try {
    text = await readFile(filePath, "utf8");
  } catch (error) {
    throw new Error(`cannot read ${label} at ${filePath}: ${error.message}`);
  }
  try {
    return JSON.parse(text);
  } catch (error) {
    throw new Error(`invalid ${label} at ${filePath}: ${error.message}`);
  }
}

async function verifyPinnedPackage(toolchainRoot) {
  const rootPackagePath = path.join(toolchainRoot, "package.json");
  const lockPath = path.join(toolchainRoot, "package-lock.json");
  const sharedPackagePath = path.join(toolchainRoot, "node_modules/@lwc/shared/package.json");
  const [rootPackage, lock, sharedPackage] = await Promise.all([
    parseJSON(rootPackagePath, "toolchain package.json"),
    parseJSON(lockPath, "toolchain package-lock.json"),
    parseJSON(sharedPackagePath, "installed @lwc/shared package.json"),
  ]);
  const lockEntry = lock.packages?.["node_modules/@lwc/shared"];
  if (rootPackage.name !== "glade-lwc-toolchain" || lock.packages?.[""]?.name !== "glade-lwc-toolchain") {
    throw new Error(`expected the Glade third_party/lwc package root, got ${toolchainRoot}`);
  }
  if (sharedPackage.name !== "@lwc/shared" || sharedPackage.version !== "9.4.3" || lockEntry?.version !== "9.4.3" || lockEntry?.integrity !== sharedPackageIntegrity) {
    throw new Error(`@lwc/shared must match the pinned 9.4.3 package-lock integrity at ${toolchainRoot}`);
  }
}

export async function prepareToolchain(toolchainRoot, { checkOnly = false } = {}) {
  const root = path.resolve(toolchainRoot);
  await verifyPinnedPackage(root);
  const patchText = await readFile(patchPath, "utf8");
  const targets = new Map();
  let targetBytes = 0;
  for (const target of Object.keys(preimageSHA256)) {
    const filePath = path.join(root, ...target.split("/"));
    let contents;
    try {
      contents = await readBoundedRegularFile(filePath, "pinned @lwc/shared target");
    } catch (error) {
      throw new Error(`cannot read pinned @lwc/shared target ${filePath}: ${error.message}`);
    }
    targetBytes += contents.length;
    if (targetBytes > maxOriginalBytes) {
      throw new Error(`pinned @lwc/shared targets exceed the ${maxOriginalBytes}-byte aggregate original limit`);
    }
    targets.set(target, contents);
  }

  const plan = buildPatchPlan({ patchText, targets, checkOnly });
  if (plan.state === "apply") {
    // Every package pin, target hash, patch hunk and computed postimage was
    // checked above before bounded same-directory staging and atomic replacement.
    await replaceTargetsAtomically({ root, writes: plan.writes, originals: targets });
    const verified = new Map();
    for (const target of Object.keys(postimageSHA256)) {
      verified.set(target, await readBoundedRegularFile(path.join(root, ...target.split("/")), "verified target"));
    }
    buildPatchPlan({ patchText, targets: verified, checkOnly: true });
  }
  return { state: plan.state, toolchainRoot: root };
}

function usage() {
  return [
    "Usage: node scripts/apply-lwc-shared-api67.mjs [--check] [<third_party/lwc-root>]",
    "The optional root is the directory containing package.json and node_modules, not node_modules itself.",
    "Without a root, the script uses the third_party/lwc directory containing this script.",
    "--check verifies all pinned API 67 postimages and never writes; otherwise the patch is applied idempotently.",
  ].join("\n");
}

async function main(args) {
  let checkOnly = false;
  const roots = [];
  for (const arg of args) {
    if (arg === "--check") checkOnly = true;
    else if (arg === "--help" || arg === "-h") {
      process.stdout.write(`${usage()}\n`);
      return;
    } else if (arg.startsWith("-")) {
      throw new Error(`unknown option ${arg}\n${usage()}`);
    } else roots.push(arg);
  }
  if (roots.length > 1) throw new Error(`expected at most one toolchain root\n${usage()}`);
  const toolchainRoot = roots.length === 1 ? roots[0] : defaultToolchainRoot;
  const result = await prepareToolchain(toolchainRoot, { checkOnly });
  if (checkOnly) {
    process.stdout.write(`prepared @lwc/shared 9.4.3 API 67 targets at ${result.toolchainRoot} (read-only; all postimages verified)\n`);
  } else {
    process.stdout.write(`@lwc/shared 9.4.3 API 67 patch ${result.state === "prepared" ? "already present" : "applied"} at ${result.toolchainRoot}\n`);
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === scriptPath) {
  main(process.argv.slice(2)).catch((error) => {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  });
}
