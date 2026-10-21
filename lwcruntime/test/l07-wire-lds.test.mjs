import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

// Execute the owned page, controlling the two LDS consumers independently.
// The virtual clock leaves the original observation windows intact.
async function observe(api, operation, options = {}) {
  let now = 0, order = 0, configured = 0, activeId;
  let property = { data: { undefined: true }, error: { undefined: true } };
  const emissions = [property], timers = [], events = [], records = new Map();
  const schedule = (run, delay) => timers.push({ run, at: now + delay, order: order++ });
  const deliver = (id, phase, complete = () => {}) => {
    const record = records.get(id);
    if (!record) return; // The suppressed deleted-record request has no native emission.
    const value = { data: { id, name: record.name } };
    let received = 0;
    for (const kind of ['property', 'function']) {
      if (options.missing === kind && options.missingPhase === phase) continue;
      const delay = kind === 'property' ? options.propertyDelay ?? 4300 : options.functionDelay ?? 4700;
      schedule(() => {
        if (kind === 'property') property = value;
        else emissions.push(value);
        events.push({ kind, phase, id, at: now });
        if (++received === 2) complete();
      }, delay);
    }
  };
  const child = {
    parentNode: {},
    isConnected: true,
    configure(adapter, marker, ordinal, id) {
      assert.equal(adapter, 'lds');
      configured++;
      activeId = id;
      events.push({ kind: 'configure', id, at: now });
      if (id != null) deliver(id, configured === 1 ? 'initial' : 'configuration');
    },
    snapshot(form) {
      return {
        consumer: form, current: form === 'property' ? property : emissions.at(-1),
        connected: 1, disconnected: 0, functionEmissions: [...emissions],
        propertyDistinctRenderedStates: [],
      };
    },
  };
  const file = new URL(`../../internal/lwc/compile/testdata/l07_runtime/api${api}/force-app/main/default/lwc/familyL07Runtime${api}/familyL07Runtime${api}.js`, import.meta.url);
  const source = fs.readFileSync(file, 'utf8')
    .replace(/^import .*;\r?\n/gm, '')
    .replace('export default class ', 'globalThis.Fixture = class ');
  const context = vm.createContext({
    LightningElement: class {}, Date: { now: () => now }, setTimeout: schedule,
    async createRecord({ fields }) {
      const id = `owned-${records.size + 1}`;
      records.set(id, { name: fields.Name });
      events.push({ kind: 'create', id, at: now });
      return { id };
    },
    async deleteRecord(id) {
      assert.ok(records.delete(id));
      events.push({ kind: 'delete', id, at: now });
    },
    async changeOwned({ recordId, name }) {
      assert.ok(records.has(recordId));
      records.get(recordId).name = name;
      events.push({ kind: 'change', id: recordId, at: now });
    },
    async notifyRecordUpdateAvailable(notified) {
      assert.equal(notified.length, 1);
      assert.equal(notified[0].recordId, activeId);
      await new Promise((resolve, reject) => {
        // A normal notification promise owns its delivery boundary. The page
        // must snapshot that boundary without waiting for additional replies.
        deliver(activeId, 'notify', resolve);
        if (options.prematureNotify) resolve();
        else if (options.missing && options.missingPhase === 'notify') {
          schedule(() => reject(new Error('notification delivery deadline')), 10000);
        }
      });
      events.push({ kind: 'notify-resolved', id: activeId, at: now });
    },
  });
  new vm.Script(source, { filename: file.pathname }).runInContext(context);
  const fixture = new context.Fixture();
  fixture.template = { querySelector: () => child };
  const form = options.form ?? 'property';
  let complete = false, value, error;
  fixture.execute({ id: `r_lds_${form}_${operation}`, operation, adapter: 'lds', form })
    .then(result => { value = result; complete = true; }, caught => { error = caught; complete = true; });
  for (let step = 0; step < 30000 && !complete; step++) {
    for (let i = 0; i < 12; i++) await Promise.resolve();
    if (complete) break;
    timers.sort((a, b) => a.at - b.at || a.order - b.order);
    assert.ok(timers.length, 'fixture stalled without a timer');
    const timer = timers.shift();
    now = timer.at;
    timer.run();
  }
  assert.ok(complete, 'fixture did not reach a bounded outcome');
  return { value, error, events };
}

function expectRecord(frame, id, count, suffix = 'A') {
  const latest = frame.functionEmissions.at(-1);
  assert.equal(frame.current.data?.id, id, `${frame.label}: selected consumer record`);
  assert.equal(latest.data?.id, id, `${frame.label}: function consumer record`);
  assert.ok(frame.current.data.name.endsWith(`_${suffix}`), `${frame.label}: selected consumer name`);
  assert.equal(frame.current.data.name, latest.data.name, `${frame.label}: both consumers have the same name`);
  assert.equal(frame.functionEmissions.length, count, `${frame.label}: observed function emissions`);
}

for (const api of [59, 67, 68]) {
  for (const form of ['property', 'function']) {
    for (const [propertyDelay, functionDelay] of [[4300, 4700], [4700, 4300]]) {
      for (const operation of ['initial', 'pending_to_cached', 'pending_to_undefined', 'same', 'notify_changed', 'notify_unchanged', 'error_to_undefined']) {
        test(`API ${api} LDS ${form} ${operation} synchronizes ${propertyDelay}/${functionDelay}`, async () => {
          const { value, error, events } = await observe(api, operation, { form, propertyDelay, functionDelay });
          assert.ifError(error);
          assert.equal(value.initialWindowMs, 4000);
          assert.equal(value.settledWindowMs, 4000);
          expectRecord(value.frames[0], 'owned-1', 2);
          const final = value.frames.at(-1);
          if (operation.startsWith('pending_to_')) {
            const intermediate = value.frames.find(frame => frame.label === 'intermediate-config');
            expectRecord(intermediate, 'owned-2', 3, 'B');
            const changes = events.filter(event => event.kind === 'configure');
            const deliveredB = events.filter(event => event.id === 'owned-2' && ['property', 'function'].includes(event.kind));
            assert.equal(deliveredB.length, 2);
            assert.ok(deliveredB.every(event => event.at <= changes[2].at), 'deliver B to both consumers before suppressing or switching it');
            expectRecord(final, operation === 'pending_to_cached' ? 'owned-1' : 'owned-2', operation === 'pending_to_cached' ? 4 : 3, operation === 'pending_to_cached' ? 'A' : 'B');
          } else if (operation.startsWith('notify_')) {
            const notified = value.frames.find(frame => frame.label === 'notify-promise-resolved');
            const suffix = operation === 'notify_changed' ? 'CHANGED' : 'A';
            expectRecord(notified, 'owned-1', 3, suffix);
            expectRecord(final, 'owned-1', 3, suffix);
            const resolved = events.find(event => event.kind === 'notify-resolved');
            const replies = events.filter(event => event.phase === 'notify');
            assert.equal(replies.length, 2);
            assert.ok(replies.every(event => event.at <= resolved.at), 'notification promise resolves after both independently scheduled replies');
          } else {
            expectRecord(final, 'owned-1', operation === 'same' ? 3 : 2);
            if (operation === 'error_to_undefined') {
              const deleted = value.frames.find(frame => frame.label === 'deleted-record-request');
              expectRecord(deleted, 'owned-1', 2);
              assert.equal(events.filter(event => event.kind === 'delete').length, 1);
              assert.ok(value.frames.every(frame => !frame.current.error));
              assert.ok(final.functionEmissions.every(emission => !emission.error || emission.error.undefined), 'do not manufacture a deleted-record error emission');
            }
          }
        });
      }
      for (const operation of ['notify_changed', 'notify_unchanged']) {
        test(`API ${api} LDS ${form} ${operation} exposes a premature notification promise ${propertyDelay}/${functionDelay}`, async () => {
          const { value, error, events } = await observe(api, operation, { form, propertyDelay, functionDelay, prematureNotify: true });
          assert.ifError(error);
          const notified = value.frames.find(frame => frame.label === 'notify-promise-resolved');
          const suffix = operation === 'notify_changed' ? 'CHANGED' : 'A';
          expectRecord(notified, 'owned-1', 2);
          assert.throws(() => expectRecord(notified, 'owned-1', 3, suffix), { name: 'AssertionError' }, 'the promise-boundary frame must still disagree with the native row');
          expectRecord(value.frames.at(-1), 'owned-1', 3, suffix);
          const resolved = events.find(event => event.kind === 'notify-resolved');
          const replies = events.filter(event => event.phase === 'notify');
          assert.ok(replies.some(event => event.kind === 'function'));
          assert.ok(replies.every(event => event.at > resolved.at), 'later delivery must not repair the earlier promise-boundary snapshot');
        });
      }
    }
    for (const missing of ['property', 'function']) {
      for (const [operation, missingPhase] of [['initial', 'initial'], ['pending_to_cached', 'configuration'], ['same', 'configuration'], ['notify_unchanged', 'notify'], ['notify_changed', 'notify']]) {
        // An identical property value has no new observable state. Outside a
        // notification promise, its repeated function emission is the boundary.
        if (missing === 'property' && operation === 'same') continue;
        test(`API ${api} LDS ${form} ${operation} rejects missing ${missing} ${missingPhase} delivery`, async () => {
          const { error } = await observe(api, operation, { form, missing, missingPhase });
          assert.equal(error?.message, missingPhase === 'notify' ? 'notification delivery deadline' : 'wire delivery deadline');
        });
      }
    }
  }
}
