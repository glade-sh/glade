import {test} from 'node:test';
import assert from 'node:assert/strict';
import {retryCase} from './v09_click_browser.mjs';

test('retries from a fresh page and reports every failed attempt', async () => {
  const visits = [], errors = [];
  const page = {goto: async url => visits.push(url)};
  let attempts = 0;
  const result = await retryCase(page, 'http://example.invalid/', 'case', async () => {
    if (++attempts < 3) throw new Error('popup delayed');
    return {status: 'observed'};
  }, message => errors.push(message));
  assert.deepEqual(result, {status: 'observed'});
  assert.equal(attempts, 3);
  assert.deepEqual(visits, ['http://example.invalid/', 'about:blank', 'http://example.invalid/', 'about:blank', 'http://example.invalid/']);
  assert.equal(errors.length, 2);
  assert.match(errors[0], /case.*attempt 1.*popup delayed/);
});

test('stops after three failures with an unobserved result', async () => {
  const errors = [];
  let attempts = 0;
  const result = await retryCase({goto: async () => {}}, 'http://example.invalid/', 'case', async () => {
    attempts++; throw new Error('missing popup');
  }, message => errors.push(message));
  assert.equal(attempts, 3);
  assert.equal(errors.length, 3);
  assert.deepEqual(result, {status: 'unobserved', reason: 'LOCAL_WIDGET_CLICK_NOT_OBSERVED'});
});
