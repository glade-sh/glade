import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtemp, readFile, readdir, rename, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { buildPatchPlan, parseUnifiedPatch, replaceTargetsAtomically } from "./apply-lwc-shared-api67.mjs";

const here = path.dirname(fileURLToPath(import.meta.url));
const patchPath = path.join(here, "..", "patches", "lwc-shared-9.4.3-api67.patch");

function sha256(value) {
  return createHash("sha256").update(value).digest("hex");
}

test("checked-in patch targets only the three pinned shared version files", async () => {
  const patch = await readFile(patchPath, "utf8");
  assert.deepEqual([...parseUnifiedPatch(patch).keys()].sort(), [
    "node_modules/@lwc/shared/dist/api-version.d.ts",
    "node_modules/@lwc/shared/dist/index.cjs",
    "node_modules/@lwc/shared/dist/index.js",
  ]);
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

test("injected replacement failure restores earlier targets and removes staged files", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "glade-api67-applier-"));
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
