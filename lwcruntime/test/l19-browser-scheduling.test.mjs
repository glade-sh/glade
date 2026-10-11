import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import { createL19BatchAllocator, l19PageCount } from "../../internal/lwc/compile/testdata/l19_browser_scheduling.mjs";

const fixture = JSON.parse(fs.readFileSync(new URL("../../internal/lwc/compile/testdata/l19_salesforce.json", import.meta.url), "utf8"));

function projectRows(api, project) {
  return [...fixture.cases, ...fixture.controls]
    .filter(row => row.kind === "runtime" && Object.hasOwn(row.expected, api) && (row.runtime_project?.[api] ?? "") === project)
    .map(row => ({ id: row.id, spec: row.spec, observer: row.observer[api], cachePhase: row.cache_phase?.[api] ?? "", projection: row.projection ?? "" }));
}

function assertCompleteUnion(rows, allocated) {
  assert.equal(allocated.length, rows.length);
  assert.equal(new Set(allocated.map(row => row.id)).size, rows.length);
  assert.deepEqual(allocated.map(row => row.id).sort(), rows.map(row => row.id).sort());
  for (const row of allocated) assert.equal(rows.find(expected => expected.id === row.id), row);
}

test("L19 ordinary API projects retain every native fifteen-row batch and worker order", () => {
  assert.equal(l19PageCount, 6);
  for (const [api, count] of [["59.0", 141], ["67.0", 132]]) {
    const rows = projectRows(api, "");
    assert.equal(rows.length, count);
    const next = createL19BatchAllocator(rows, false);
    const allocated = [];
    for (let worker = 0; worker < l19PageCount; worker++) {
      for (let offset = worker * 15; offset < rows.length; offset += l19PageCount * 15) {
        const batch = next(worker);
        assert.equal(batch.index, offset);
        assert.deepEqual(batch.rows, rows.slice(offset, offset + 15));
        allocated.push(...batch.rows);
      }
      assert.equal(next(worker), null);
    }
    assertCompleteUnion(rows, allocated);
  }
});

test("L19 controls keep complete rows while free workers claim the next row", () => {
  for (const [api, project, count] of [
    ["59.0", "controls", 38], ["67.0", "controls", 47],
    ["59.0", "interactions", 18], ["67.0", "interactions", 18],
  ]) {
    const rows = projectRows(api, project);
    assert.equal(rows.length, count);
    assert.ok(rows.every(row => ["fresh", "same-page-remount"].includes(row.cachePhase)));
    const next = createL19BatchAllocator(rows, true);
    const allocated = [];
    // Worker 5 repeatedly finishes while other pages remain busy. Its work
    // must come from the shared queue rather than a fixed per-worker stripe.
    const completionOrder = [0, 1, 2, 3, 4, 5, 5, 5, 1, 5, 2];
    for (let index = 0; index < rows.length; index++) {
      const worker = completionOrder[index % completionOrder.length];
      const batch = next(worker);
      assert.equal(batch.index, index);
      assert.equal(batch.rows.length, 1);
      assert.equal(batch.rows[0], rows[index]);
      allocated.push(...batch.rows);
    }
    for (let worker = 0; worker < l19PageCount; worker++) assert.equal(next(worker), null);
    assertCompleteUnion(rows, allocated);
  }
});

test("L19 rejects workers outside its existing six-page limit without consuming rows", () => {
  const rows = projectRows("67.0", "interactions");
  for (const controlProject of [false, true]) {
    const next = createL19BatchAllocator(rows, controlProject);
    for (const worker of [-1, 6, 100, 0.5, "0"]) assert.throws(() => next(worker), RangeError);
    assert.equal(next(0).rows[0], rows[0]);
  }
});
