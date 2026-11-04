import test from 'node:test';
import assert from 'node:assert/strict';
import {rowFrame} from './v09_click_row_frame.mjs';

for (const jump of [60000, -60000]) {
  test(`row observation ignores a ${jump}ms wall clock jump`, async () => {
    let elapsed = 0;
    let wall = 0;
    let scans = 0;
    const frame = {locator: selector => ({count: async () => {
      assert.equal(selector, '[data-v09-case="owned"]');
      scans++;
      return elapsed >= 200 ? 1 : 0;
    }})};
    const originalDateNow = Date.now;
    Date.now = () => wall;
    let result;
    try {
      result = await rowFrame({frames: () => [frame]}, 'owned', {
      now: () => elapsed,
      pause: async ms => { elapsed += ms; wall += ms + (elapsed === 100 ? jump : 0); },
      });
    } finally { Date.now = originalDateNow; }
    assert.equal(result, frame);
    assert.equal(elapsed, 200);
    assert.equal(wall, 200 + jump);
    assert.equal(scans, 3);
  });
}

test('missing and non-unique rows exhaust the observation budget', async () => {
  let elapsed = 0;
  let scans = 0;
  const frame = {locator: () => ({count: async () => { scans++; return 2; }})};
  await assert.rejects(rowFrame({frames: () => [frame]}, 'owned', {
    now: () => elapsed,
    pause: async ms => { assert.equal(ms, 100); elapsed += ms; },
  }), /OWNED_CLICK_ROW_MISSING/);
  assert.equal(elapsed, 12000);
  assert.equal(scans, 120);
});

test('default clock ignores a forward wall clock jump', async () => {
  let wall = 0;
  let scans = 0;
  const frame = {locator: () => ({count: async () => ++scans === 3 ? 1 : 0})};
  const originalDateNow = Date.now;
  Date.now = () => wall;
  try {
    const result = await rowFrame({frames: () => [frame]}, 'owned', {
      pause: async () => { wall += 60000; },
    });
    assert.equal(result, frame);
    assert.equal(scans, 3);
  } finally { Date.now = originalDateNow; }
});
