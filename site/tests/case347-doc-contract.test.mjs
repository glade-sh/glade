import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import assert from 'node:assert/strict';

const source = path => readFile(new URL(`../docs-src/${path}`, import.meta.url), 'utf8');

test('automation diagnostic targets exist', async () => {
  const errors = await source('reference/errors.md');
  for (const code of ['gladeauto001', 'gladeauto002', 'gladeauto003', 'gladeauto004']) {
    assert.ok(errors.includes(`{#${code}}`), code);
  }
});

test('test result reference distinguishes empty and stopped', async () => {
  const schema = await source('reference/json-schema.md');
  assert.match(schema, /`empty` means a successful summary with zero tests/);
  assert.match(schema, /`stopped` and exits\s+1/);
});
