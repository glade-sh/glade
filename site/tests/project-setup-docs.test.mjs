import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'

const read = (path) => readFile(new URL(`../docs-src/${path}`, import.meta.url), 'utf8')

test('site config explains local dependency discovery and explicit org setup', async () => {
  const page = await read('reference/config.md')
  assert.match(page, /`project\.dependencyPaths`/)
  assert.match(page, /dependency_missing/)
  assert.match(page, /dependency_ambiguous/)
  assert.match(page, /no longer infers `PersonAccounts`/)
  assert.match(page, /`test\.permissionSets`/)
  assert.match(page, /`test\.permissionSetGroups`/)
  assert.match(page, /glade doctor --project \. --json/)
})

test('doctor references describe optional setup advice within schema 1.0', async () => {
  const page = await read('reference/json-schema.md')
  const doctor = page.split('## Doctor JSON')[1].split('## Diagnostic object')[0]
  assert.match(doctor, /schema version `1\.0`/)
  for (const field of ['orgSetup', 'kind', 'feature', 'names', 'reason', 'files', 'yaml']) {
    assert.ok(doctor.includes('`' + field + '`'), `doctor JSON should describe ${field}`)
  }
  assert.match(doctor, /omitted/)
  assert.match(await read('reference/cli.md'), /orgSetup/)
})
