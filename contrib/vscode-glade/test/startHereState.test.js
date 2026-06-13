const assert = require("assert");
const state = require("../out/startHereState");

const store = new state.StartHereState();
assert.strictEqual(store.snapshot().watchRunning, false);
assert.strictEqual(store.snapshot().lastRun, undefined);

store.setWatchRunning(true);
store.setLastRun({ label: "Changed tests", passed: 2, failed: 0, durationMs: 500 });
store.setLocalOrgSummary({ objects: 2, records: 12, users: 1, profiles: 1, permissions: 0 });

assert.deepStrictEqual(store.snapshot().lastRun, {
  label: "Changed tests",
  passed: 2,
  failed: 0,
  durationMs: 500,
});
assert.strictEqual(store.snapshot().watchRunning, true);
assert.strictEqual(store.snapshot().localOrgSummary.records, 12);
