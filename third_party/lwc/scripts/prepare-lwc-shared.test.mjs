import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtemp, readFile, readdir, rename, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { buildPatchPlan, parseUnifiedPatch, replaceTargetsAtomically } from "./prepare-lwc-shared.mjs";

const here = path.dirname(fileURLToPath(import.meta.url));
const patchPath = path.join(here, "..", "patches", "lwc-shared-9.4.3-api68.patch");
const upgradePatchPath = path.join(here, "..", "patches", "lwc-shared-9.4.3-api67-to-api68.patch");
const sharedTargets = [
  "node_modules/@lwc/shared/dist/api-version.d.ts",
  "node_modules/@lwc/shared/dist/index.cjs",
  "node_modules/@lwc/shared/dist/index.js",
];

function sha256(value) {
  return createHash("sha256").update(value).digest("hex");
}

test("checked-in patches target only the three pinned shared version files", async () => {
  for (const file of [patchPath, upgradePatchPath]) {
    const patch = await readFile(file, "utf8");
    assert.deepEqual([...parseUnifiedPatch(patch).keys()].sort(), sharedTargets, file);
  }
});

test("upgrade delta adds the API 68 tail of the cumulative patch", async () => {
  const added = (patch) => new Map([...parseUnifiedPatch(patch)].map(([target, hunks]) => [
    target,
    hunks.flatMap((hunk) => hunk.lines.filter((line) => line.kind === "+").map((line) => line.value)),
  ]));
  const cumulative = added(await readFile(patchPath, "utf8"));
  const upgrade = added(await readFile(upgradePatchPath, "utf8"));
  for (const target of sharedTargets) {
    const lines = cumulative.get(target);
    const delta = upgrade.get(target);
    assert.ok(lines.some((line) => line.includes("V67_262_SUMMER_26")), `${target} cumulative patch keeps API 67`);
    assert.ok(delta.some((line) => line.includes("V68_264_WINTER_27")), `${target} delta adds API 68`);
    assert.ok(!delta.some((line) => line.includes("V66_260_SPRING_26")), `${target} delta leaves API 66 alone`);
    assert.deepEqual(delta, lines.slice(lines.length - delta.length), target);
  }
});

test("plan is idempotent for postimages and refuses check or mismatches without mutation", () => {
  const jsTarget = "node_modules/@lwc/shared/dist/index.js";
  const typesTarget = "node_modules/@lwc/shared/dist/api-version.d.ts";
  const patchText = [
    `--- a/${jsTarget}`,
    `+++ b/${jsTarget}`,
    "@@ -1 +1 @@",
    "-version 66",
    "+version 67",
    `--- a/${typesTarget}`,
    `+++ b/${typesTarget}`,
    "@@ -1 +1 @@",
    "-type APIVersion = 66",
    "+type APIVersion = 67",
    "",
  ].join("\n");
  const beforeJS = Buffer.from("version 66\n");
  const afterJS = Buffer.from("version 67\n");
  const beforeTypes = Buffer.from("type APIVersion = 66\n");
  const afterTypes = Buffer.from("type APIVersion = 67\n");
  const preimages = { [jsTarget]: sha256(beforeJS), [typesTarget]: sha256(beforeTypes) };
  const postimages = { [jsTarget]: sha256(afterJS), [typesTarget]: sha256(afterTypes) };
  const source = new Map([[jsTarget, beforeJS], [typesTarget, beforeTypes]]);

  const planned = buildPatchPlan({ patchText, targets: source, preimages, postimages });
  assert.equal(planned.state, "apply");
  assert.deepEqual(planned.writes.get(jsTarget), afterJS);
  assert.deepEqual(planned.writes.get(typesTarget), afterTypes);
  assert.deepEqual(source.get(jsTarget), beforeJS);
  assert.deepEqual(source.get(typesTarget), beforeTypes);

  const prepared = buildPatchPlan({
    patchText,
    targets: planned.writes,
    preimages,
    postimages,
    checkOnly: true,
  });
  assert.equal(prepared.state, "prepared");
  assert.equal(prepared.writes.size, 0);

  assert.throws(
    () => buildPatchPlan({ patchText, targets: source, preimages, postimages, checkOnly: true }),
    /preimages.*--check is read-only/
  );
  const mismatch = new Map([[jsTarget, beforeJS], [typesTarget, Buffer.from("locally changed\n")]]);
  assert.throws(() => buildPatchPlan({ patchText, targets: mismatch, preimages, postimages }), /no files written/);
  assert.deepEqual(mismatch.get(jsTarget), beforeJS);
  assert.equal(mismatch.get(typesTarget).toString(), "locally changed\n");
});

test("plan upgrades only a complete API 67 postimage set and refuses mixed states", () => {
  const jsTarget = "node_modules/@lwc/shared/dist/index.js";
  const typesTarget = "node_modules/@lwc/shared/dist/api-version.d.ts";
  const patch = (from, to) => [
    `--- a/${jsTarget}`,
    `+++ b/${jsTarget}`,
    "@@ -1 +1 @@",
    `-version ${from}`,
    `+version ${to}`,
    `--- a/${typesTarget}`,
    `+++ b/${typesTarget}`,
    "@@ -1 +1 @@",
    `-type APIVersion = ${from}`,
    `+type APIVersion = ${to}`,
    "",
  ].join("\n");
  const files = (version) => new Map([
    [jsTarget, Buffer.from(`version ${version}\n`)],
    [typesTarget, Buffer.from(`type APIVersion = ${version}\n`)],
  ]);
  const digests = (version) => Object.fromEntries([...files(version)].map(([target, contents]) => [target, sha256(contents)]));
  const options = {
    patchText: patch(66, 68),
    upgradeText: patch(67, 68),
    preimages: digests(66),
    postimages: digests(68),
    upgradeSources: digests(67),
  };

  const fresh = buildPatchPlan({ ...options, targets: files(66) });
  assert.equal(fresh.state, "apply");
  assert.deepEqual(fresh.originals, options.preimages);

  const upgraded = buildPatchPlan({ ...options, targets: files(67) });
  assert.equal(upgraded.state, "upgrade");
  assert.deepEqual(upgraded.originals, options.upgradeSources);
  assert.deepEqual(upgraded.writes, fresh.writes, "the delta and the cumulative patch reach the same bytes");
  assert.equal(buildPatchPlan({ ...options, targets: upgraded.writes, checkOnly: true }).state, "prepared");

  assert.throws(() => buildPatchPlan({ ...options, targets: files(67), checkOnly: true }), /API 67 postimages; --check is read-only/);
  const mixed = new Map([[jsTarget, files(66).get(jsTarget)], [typesTarget, files(67).get(typesTarget)]]);
  assert.throws(() => buildPatchPlan({ ...options, targets: mixed }), /refusing partial or unknown.*no files written/);
  const { upgradeText, ...withoutUpgrade } = options;
  assert.throws(() => buildPatchPlan({ ...withoutUpgrade, targets: files(67) }), /refusing partial or unknown/);

  const wrongDelta = { ...options, upgradeText: patch(67, 69) };
  assert.throws(() => buildPatchPlan({ ...wrongDelta, targets: files(67) }), /postimage hash mismatch.*no files written/);
});

test("injected replacement failure restores earlier targets and removes staged files", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "glade-lwc-shared-applier-"));
  try {
    const originals = new Map([
      ["first.js", Buffer.from("original first\n")],
      ["second.js", Buffer.from("original second\n")],
    ]);
    const writes = new Map([
      ["first.js", Buffer.from("patched first\n")],
      ["second.js", Buffer.from("patched second\n")],
    ]);
    const preimages = Object.fromEntries([...originals].map(([target, contents]) => [target, sha256(contents)]));
    const postimages = Object.fromEntries([...writes].map(([target, contents]) => [target, sha256(contents)]));
    for (const [target, contents] of originals) await writeFile(path.join(root, target), contents);

    let renameCount = 0;
    await assert.rejects(
      replaceTargetsAtomically({
        root,
        writes,
        originals,
        preimages,
        postimages,
        renameFile: async (source, destination) => {
          renameCount++;
          if (renameCount === 2) throw new Error("injected replacement failure");
          await rename(source, destination);
        },
      }),
      /injected replacement failure/
    );

    assert.equal(renameCount, 3, "the first replacement should be rolled back atomically");
    for (const [target, contents] of originals) {
      assert.deepEqual(await readFile(path.join(root, target)), contents);
    }
    assert.deepEqual((await readdir(root)).sort(), ["first.js", "second.js"]);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
