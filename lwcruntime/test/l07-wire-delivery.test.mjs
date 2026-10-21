import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

// Run the owned page with independently scheduled property/function deliveries.
// Virtual time makes slow, missing, and out-of-order replies deterministic.
async function observe(api, operation, options = {}) {
  let now = 0, order = 0, connected = true, configured = 0;
  let property = { data: { undefined: true }, error: { undefined: true } };
  let emissions = [property];
  const timers = [], snapshots = [], mutations = [];
  const schedule = (run, delay) => timers.push({ run, at: now + delay, order: order++ });
  const child = {
    get parentNode() { return connected ? holder : null; },
    get isConnected() { return connected; },
    configure(adapter, marker, ordinal, id) {
      configured++;
      mutations.push({ marker, at: now });
      if (marker === undefined || ordinal === undefined) return;
      // Transient configurations intentionally have no delivery in native rows.
      if (['rapid-a', 'other', '__THROW__'].includes(marker) && operation !== 'rapid_suppress' && operation !== 'value_then_error') return;
      const value = marker === '__THROW__' ? { error: { message: 'L07 controlled error', status: 500 } } : { data: { marker, ordinal } };
      const change = configured;
      for (const kind of ['property', 'function']) {
        if (options.missing === kind && change === (options.missingInitial ? 1 : 2)) continue;
        const delay = kind === 'property' ? (options.propertyDelay ?? 4300) : (options.functionDelay ?? 4700);
        schedule(() => {
          const delivered = options.wrongOrdinal && change > 1 ? { data: { marker, ordinal: 2 } } : value;
          if (kind === 'property') property = delivered;
          else emissions.push(delivered);
        }, delay);
      }
    },
    snapshot(form) {
      snapshots.push({ at: now, form, configured });
      return { consumer: form, current: form === 'property' ? property : emissions.at(-1),
        connected: 1, disconnected: 0, functionEmissions: [...emissions], propertyDistinctRenderedStates: [] };
    },
    mutate() { return {}; },
  };
  const holder = {
    removeChild(value) { assert.equal(value, child); connected = false; },
    appendChild(value) { assert.equal(value, child); connected = true; },
  };
  const file = new URL(`../../internal/lwc/compile/testdata/l07_runtime/api${api}/force-app/main/default/lwc/familyL07Runtime${api}/familyL07Runtime${api}.js`, import.meta.url);
  const source = fs.readFileSync(file, 'utf8').replace(/^import .*;\r?\n/gm, '').replace('export default class ', 'globalThis.Fixture = class ');
  const context = vm.createContext({ LightningElement: class {}, Date: { now: () => now }, setTimeout: schedule });
  new vm.Script(source, { filename: file.pathname }).runInContext(context);
  const fixture = new context.Fixture();
  fixture.template = { querySelector: () => child };
  let complete = false, value, error;
  fixture.execute({ id: `r_${options.form ?? 'property'}_string`, operation, adapter: 'apex', form: options.form ?? 'property' })
    .then(result => { value = result; complete = true; }, caught => { error = caught; complete = true; });
  for (let step = 0; step < 30000 && !complete; step++) {
    // Drain async helper continuations before advancing virtual time.
    for (let i = 0; i < 12; i++) await Promise.resolve();
    if (complete) break;
    timers.sort((a, b) => a.at - b.at || a.order - b.order);
    assert.ok(timers.length, 'fixture stalled without a timer');
    const timer = timers.shift(); now = timer.at; timer.run();
  }
  assert.ok(complete, 'fixture did not reach a bounded outcome');
  return { value, error, mutations, snapshots, now };
}

for (const api of [59, 67, 68]) {
  for (const form of ['property', 'function']) {
    for (const operation of ['initial', 'replace', 'same', 'multiple', 'activate', 'rapid', 'rapid_suppress', 'cache_return', 'error_then_value', 'value_then_error', 'disconnect_config']) {
      for (const [propertyDelay, functionDelay] of [[4300, 4700], [4700, 4300]]) {
        test(`API ${api} ${form} ${operation} waits for both deliveries ${propertyDelay}/${functionDelay}`, async () => {
          const { value, error, mutations } = await observe(api, operation, { form, propertyDelay, functionDelay });
          assert.ifError(error);
          assert.equal(value.initialWindowMs, 4000);
          assert.equal(value.settledWindowMs, 4000);
          const initial = value.frames[0];
          if (operation === 'activate') assert.equal(initial.current.data.undefined, true);
          else {
            assert.equal(initial.current.data.marker, 'owned');
            assert.equal(initial.functionEmissions.at(-1).data.marker, 'owned');
          }
          const expected = { replace: 'next', multiple: 'next', activate: 'active', rapid: 'rapid-b', rapid_suppress: 'rapid-a', error_then_value: 'good', disconnect_config: 'detached' }[operation] ?? 'owned';
          const final = value.frames.at(-1);
          if (operation === 'value_then_error') {
            assert.equal(final.current.error.message, 'L07 controlled error');
            assert.equal(final.functionEmissions.at(-1).error.message, 'L07 controlled error');
            assert.equal(final.functionEmissions.at(-2).data.marker, 'good');
            assert.equal(final.functionEmissions.length, 4);
          } else {
            assert.equal(final.current.data.marker, expected);
            assert.equal(final.functionEmissions.at(-1).data.marker, expected);
          }
          if (operation === 'same' || operation === 'cache_return') assert.equal(final.functionEmissions.length, 3, 'observe the repeated cached delivery');
          if (operation === 'error_then_value' || operation === 'rapid' || operation === 'cache_return') {
            assert.equal(mutations[2].at, mutations[1].at, 'retain the transient wait(0)');
            assert.equal(final.functionEmissions.length, 3, 'do not manufacture transient emissions');
          }
        });
      }
    }
  }
  for (const missing of ['property', 'function']) {
    for (const missingInitial of [false, true]) {
      test(`API ${api} rejects missing ${missing} at ${missingInitial ? 'initial' : 'settled'} snapshot`, async () => {
        const { error } = await observe(api, 'replace', { missing, missingInitial });
        assert.match(error?.message ?? '', /wire delivery deadline/);
      });
    }
  }
  test(`API ${api} rejects the wrong terminal ordinal`, async () => {
    const { error } = await observe(api, 'replace', { wrongOrdinal: true });
    assert.match(error?.message ?? '', /wire delivery deadline/);
  });
}
