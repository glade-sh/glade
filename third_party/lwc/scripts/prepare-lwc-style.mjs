// Pinned, bounded preparation of the selector reuse patch for @lwc/style-compiler.
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { applyUnifiedPatch, readBoundedRegularFile, replaceTargetsAtomically } from "./prepare-lwc-shared.mjs";

const patchPath = fileURLToPath(new URL("../patches/lwc-style-compiler-9.4.3-selector-reuse.patch", import.meta.url));
const integrity = "sha512-3kaU18upSqtKRLqcyC9HnZe+2xyldkj2z2eN8niiGooJUjvQVSIfWH+eVTTyzk6vK9e+9mLTO6V27+tbUgEUqQ==";
const preimages = Object.freeze({
  "node_modules/@lwc/style-compiler/dist/index.js": "53b284615aaccd1ea5aa70dbd89ba7c8b6254b7845f3a2692c397a5a17ed836b",
  "node_modules/@lwc/style-compiler/dist/index.cjs": "994d2804b10f62e6c5bcac412b25f50dce2679824748c75a37c0ebe445611bad",
});
const postimages = Object.freeze({
  "node_modules/@lwc/style-compiler/dist/index.js": "ccff35673c0c5337958cc964341e8150cbc54662293b6d0219bd871c6e14ed94",
  "node_modules/@lwc/style-compiler/dist/index.cjs": "e89043bc48b11f65a5e9299d8707d1cdfade5059d69a601d0a82a507d8ef15f0",
});
const hash = (value) => createHash("sha256").update(value).digest("hex");

export function buildStylePatchPlan({ originals, patchText, before = preimages, after = postimages, checkOnly = false }) {
  const targets = Object.keys(before);
  if (JSON.stringify([...originals.keys()].sort()) !== JSON.stringify([...targets].sort()) ||
      JSON.stringify(Object.keys(after).sort()) !== JSON.stringify([...targets].sort())) {
    throw new Error("the complete pinned @lwc/style-compiler target set is required");
  }
  const all = (pins) => targets.every((target) => hash(originals.get(target)) === pins[target]);
  if (all(after)) return { state: "prepared" };
  if (!all(before)) throw new Error("refusing partial or unknown @lwc/style-compiler patch state; no files written");
  if (checkOnly) throw new Error("@lwc/style-compiler preparation is required; --check is read-only");
  const writes = applyUnifiedPatch(originals, patchText);
  for (const target of targets) {
    if (hash(writes.get(target)) !== after[target]) throw new Error(`@lwc/style-compiler postimage hash mismatch for ${target}; no files written`);
  }
  return { state: "apply", originals, writes };
}

export async function planStylePatch(root, checkOnly = false) {
  const [lockText, packageText, patchText] = await Promise.all([
    readFile(path.join(root, "package-lock.json"), "utf8"),
    readFile(path.join(root, "node_modules/@lwc/style-compiler/package.json"), "utf8"),
    readFile(patchPath, "utf8"),
  ]);
  const entry = JSON.parse(lockText).packages?.["node_modules/@lwc/style-compiler"];
  const pkg = JSON.parse(packageText);
  if (entry?.version !== "9.4.3" || entry?.integrity !== integrity || pkg.name !== "@lwc/style-compiler" || pkg.version !== "9.4.3") {
    throw new Error("@lwc/style-compiler must match the pinned 9.4.3 package-lock integrity");
  }
  const originals = new Map();
  for (const target of Object.keys(preimages)) {
    originals.set(target, await readBoundedRegularFile(path.join(root, ...target.split("/")), "pinned @lwc/style-compiler target"));
  }
  return buildStylePatchPlan({ originals, patchText, checkOnly });
}

export async function applyStylePatch(root, plan) {
  if (plan.state === "prepared") return;
  await replaceTargetsAtomically({ root, writes: plan.writes, originals: plan.originals, preimages, postimages });
  await planStylePatch(root, true);
}
