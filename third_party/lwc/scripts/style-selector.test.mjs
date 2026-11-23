import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { buildStylePatchPlan, planStylePatch } from "./prepare-lwc-style.mjs";
import { capture } from "./style-selector-golden.mjs";

const hash = (value) => createHash("sha256").update(value).digest("hex");

test("stylesheet fixture output and diagnostics match the original compiler", async () => {
  const expected = JSON.parse(await readFile(new URL("./style-selector-golden.json", import.meta.url), "utf8"));
  assert.deepEqual(capture(), expected);
});

test("selector patch plans reject unknown, partial and unchecked states", () => {
  const before = { "a.js": hash("old a\n"), "b.js": hash("old b\n") };
  const after = { "a.js": hash("new a\n"), "b.js": hash("new b\n") };
  const originals = new Map([["a.js", Buffer.from("old a\n")], ["b.js", Buffer.from("old b\n")]]);
  const patchText = ["--- a/a.js", "+++ b/a.js", "@@ -1 +1 @@", "-old a", "+new a", "--- a/b.js", "+++ b/b.js", "@@ -1 +1 @@", "-old b", "+new b", ""].join("\n");
  const options = { originals, patchText, before, after };
  assert.throws(() => buildStylePatchPlan({ ...options, checkOnly: true }), /--check is read-only/);
  const plan = buildStylePatchPlan(options);
  assert.equal(plan.state, "apply");
  assert.equal(buildStylePatchPlan({ ...options, originals: plan.writes, checkOnly: true }).state, "prepared");
  assert.throws(() => buildStylePatchPlan({ ...options, originals: new Map([["a.js", plan.writes.get("a.js")], ["b.js", originals.get("b.js")]]) }), /partial or unknown/);
  assert.throws(() => buildStylePatchPlan({ ...options, after: { ...after, "b.js": hash("wrong") } }), /postimage hash mismatch/);
  assert.throws(() => buildStylePatchPlan({ ...options, originals: new Map([["a.js", originals.get("a.js")]]) }), /complete pinned/);
});

test("installed selector patch verifies its package and postimages", async () => {
  assert.equal((await planStylePatch(fileURLToPath(new URL("..", import.meta.url)), true)).state, "prepared");
});
