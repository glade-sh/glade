import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'

test('generated reports retain local capability and hosted boundary coverage', async () => {
  const gaps = await readFile(new URL('../../docs/KNOWN_GAPS.md', import.meta.url), 'utf8')
  const stdlib = await readFile(new URL('../../docs/STDLIB_COVERAGE.md', import.meta.url), 'utf8')
  for (const name of ['Flow', 'DataWeave', 'ConnectApi', 'Slack', 'hosted']) assert.ok(gaps.includes(name), name)
  for (const name of ['Cache.Org', 'Cache.Session', 'FormulaEval', 'Invocable.Action', 'ReplayId']) assert.ok(stdlib.includes(name), name)
})
