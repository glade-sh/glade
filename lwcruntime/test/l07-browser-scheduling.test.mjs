import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import { createL07PageGroups, l07PageCount, runL07PageGroups } from "../../internal/lwc/compile/testdata/l07_browser_scheduling.mjs";

const rows = JSON.parse(fs.readFileSync(new URL("../../internal/lwc/compile/testdata/l07_salesforce.json", import.meta.url), "utf8"))
  .filter(row => row.kind === "runtime");
const fixtures = ["59.0", "67.0"].map(api => ({ api, url: `http://fixture-${api}/`, specs: rows }));

test("L07 retains all native page assignments and ordered cache histories at both APIs", () => {
  const pages = createL07PageGroups(fixtures);
  assert.equal(l07PageCount, 3);
  assert.equal(pages.length, 6);
  assert.equal(rows.length, 110);
  for (const fixture of fixtures) {
    const actual = pages.filter(page => page.api === fixture.api).sort((a, b) => a.index - b.index);
    const apex = rows.filter(row => row.adapter === "apex");
    const lds = rows.filter(row => row.adapter === "lds");
    const expected = [apex, lds.filter((_, index) => index % 2 === 0), lds.filter((_, index) => index % 2 === 1)];
    assert.deepEqual(actual.map(page => page.specs.length), [84, 13, 13]);
    const seen = new Set();
    for (let index = 0; index < actual.length; index++) {
      assert.equal(actual[index].url, fixture.url);
      assert.deepEqual(actual[index].specs, expected[index]);
      for (let offset = 0; offset < expected[index].length; offset++) {
        assert.strictEqual(actual[index].specs[offset], expected[index][offset]);
        assert.equal(seen.has(expected[index][offset].id), false);
        seen.add(expected[index][offset].id);
      }
    }
    assert.deepEqual([...seen].sort(), rows.map(row => row.id).sort());
  }
});

test("L07 overlaps both Apex histories while a bounded third slot drains whole LDS pages", async () => {
  const pages = createL07PageGroups(fixtures);
  const started = [], completed = [], active = new Set();
  const releases = new Map();
  let maximum = 0;
  const running = runL07PageGroups(pages, async page => {
    started.push(page);
    active.add(page);
    maximum = Math.max(maximum, active.size);
    await new Promise(resolve => releases.set(page, resolve));
    active.delete(page);
    completed.push(page);
  });
  assert.deepEqual(started.map(page => [page.api, page.index]), [["59.0", 0], ["67.0", 0], ["59.0", 1]]);
  for (const page of pages.slice(2)) {
    assert.ok(active.has(page));
    releases.get(page)();
    // Let the completed page release its slot and the worker claim its next
    // entire history. Both long Apex pages remain active throughout.
    await new Promise(resolve => setImmediate(resolve));
    assert.ok(active.has(pages[0]));
    assert.ok(active.has(pages[1]));
  }
  releases.get(pages[0])();
  releases.get(pages[1])();
  await running;
  assert.equal(maximum, 3);
  assert.equal(active.size, 0);
  assert.deepEqual(started, pages);
  assert.deepEqual(new Set(completed), new Set(pages));
  assert.equal(completed.length, 6);
});
