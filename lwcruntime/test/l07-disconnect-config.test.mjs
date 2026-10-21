import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import vm from "node:vm";

// Execute the actual fixture with a virtual clock so either wire response can
// deterministically finish beyond the old one-second snapshot boundary.
async function observe(api, options = {}) {
  let now = 0, next = 0, connected = true;
  const timers = [], framesAt = [];
  const schedule = (run, delay) => timers.push({ run, at: now + delay, order: next++ });
  let property = { data: { marker: "owned", ordinal: 1 } };
  let emissions = [{ data: { marker: "owned", ordinal: 1 } }];
  const child = {
    get parentNode() { return connected ? holder : null; },
    get isConnected() { return connected; },
    configure(_adapter, marker, ordinal) {
      if (marker !== "detached") { emissions.push({ data: { marker, ordinal } }); return; }
      if (options.propertyDelay !== null) schedule(() => {
        property = { data: { marker, ordinal: options.wrongOrdinal ? 2 : ordinal } };
      }, options.propertyDelay ?? 1400);
      if (options.functionDelay !== null) schedule(() => {
        emissions.push({ data: { marker, ordinal } });
      }, options.functionDelay ?? 1700);
      if (options.reattach) schedule(() => { connected = true; }, 1100);
    },
    snapshot(form) {
      framesAt.push({ at: now, connected });
      return {
        consumer: form, current: property, connected: 1, disconnected: 0, observedAt: now, observedConnected: connected,
        functionEmissions: [...emissions], propertyDistinctRenderedStates: [],
      };
    },
  };
  const holder = {
    removeChild(value) { assert.equal(value, child); connected = false; },
    appendChild(value) { assert.equal(value, child); connected = true; },
  };
  const file = new URL(`../../internal/lwc/compile/testdata/l07_runtime/api${api}/force-app/main/default/lwc/familyL07Runtime${api}/familyL07Runtime${api}.js`, import.meta.url);
  const source = fs.readFileSync(file, "utf8")
    .replace(/^import .*;\r?\n/gm, "")
    .replace("export default class ", "globalThis.Fixture = class ");
  const context = vm.createContext({
    LightningElement: class {}, Date: { now: () => now }, setTimeout: schedule,
  });
  new vm.Script(source, { filename: file.pathname }).runInContext(context);
  const fixture = new context.Fixture();
  fixture.template = { querySelector: () => child };
  const id = options.id ?? "r_property_disconnect_config";
  let complete = false, value, error;
  fixture.execute({ id, operation: "disconnect_config", adapter: "apex", form: id.includes("function") ? "function" : "property" })
    .then(result => { value = result; complete = true; }, caught => { error = caught; complete = true; });
  for (let step = 0; step < 10000 && !complete; step++) {
    for (let i = 0; i < 12; i++) await Promise.resolve();
    if (complete) break;
    timers.sort((a, b) => a.at - b.at || a.order - b.order);
    assert.ok(timers.length, "fixture stalled without a timer");
    const timer = timers.shift();
    now = timer.at;
    timer.run();
  }
  assert.ok(complete, "fixture did not reach a bounded outcome");
  return { value, error, framesAt, connected };
}

for (const api of [59, 67, 68]) {
  for (const [propertyDelay, functionDelay] of [[1400, 1700], [1700, 1400], [50, 80]]) {
    test(`disconnect config API ${api} waits for both wires (${propertyDelay}/${functionDelay})`, async () => {
      const { value, error, framesAt } = await observe(api, { propertyDelay, functionDelay });
      assert.ifError(error);
      const frame = value.frames.find(frame => frame.label === "detached-config");
      assert.equal(frame.current.data.marker, "detached");
      assert.equal(frame.functionEmissions.at(-1).data.marker, "detached");
      assert.equal(frame.current.data.ordinal, 1);
      assert.equal(value.initialWindowMs, 4000);
      assert.equal(value.settledWindowMs, 4000);
      assert.equal(value.frames[0].observedAt, 4000);
      assert.ok(frame.observedAt >= 5000, "retain the original one-second minimum");
      assert.equal(frame.observedConnected, false);
      assert.equal(value.frames.at(-1).observedAt - frame.observedAt, 4000);
    });
  }
  for (const options of [{ propertyDelay: null }, { functionDelay: null }, { wrongOrdinal: true }]) {
    test(`disconnect config API ${api} rejects incomplete configuration ${JSON.stringify(options)}`, async () => {
      const { error, connected } = await observe(api, options);
      assert.match(error?.message ?? "", /wire (?:configuration|delivery) deadline/);
      assert.equal(connected, false, "deadline must not reconnect and manufacture a successful observation");
    });
  }
  test(`disconnect config API ${api} rejects an attachment change while waiting`, async () => {
    const { error } = await observe(api, { reattach: true });
    assert.match(error?.message ?? "", /detached wire lifecycle changed/);
  });
  test(`disconnect config API ${api} synchronizes the function-row window too`, async () => {
    const { value, error } = await observe(api, { id: "r_function_disconnect_config" });
    assert.ifError(error);
    const frame = value.frames.find(frame => frame.label === "detached-config");
    assert.equal(frame.current.data.marker, "detached");
    assert.equal(frame.functionEmissions.at(-1).data.marker, "detached");
  });
}
