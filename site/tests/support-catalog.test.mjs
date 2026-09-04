import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'

import { parseCoverageRows as parseGeneratorRows } from '../scripts/build-editor-support.mjs'

const coverage = await readFile(new URL('../../docs/STDLIB_COVERAGE.md', import.meta.url), 'utf8')
const catalog = JSON.parse(await readFile(new URL('../docs-src/public/data/editor-support.json', import.meta.url), 'utf8'))

function parseLedgerRows(markdown) {
  const rows = []
  const rowPattern = /^\| ([^|]+) \| `([^`]+)` \| `([^`]+)` \| ([^|]+) \|$/
  for (const line of markdown.split('\n')) {
    const match = rowPattern.exec(line)
    if (!match) continue
    const [, area, api, status, notes] = match
    if (!['supported', 'partial', 'stub', 'unsupported', 'unknown'].includes(status.trim())) continue
    rows.push({
      id: JSON.stringify([area.trim(), api.trim()]),
      area: area.trim(),
      api: api.trim(),
      status: status.trim(),
      notes: notes.trim()
    })
  }
  return rows
}

test('support catalog preserves every ledger row and derives each summary count from those rows', () => {
  const rows = parseLedgerRows(coverage)
  assert.deepEqual(catalog.rows, rows)
  assert.equal(new Set(catalog.rows.map((row) => row.id)).size, catalog.rows.length)
  for (const status of Object.keys(catalog.summary)) {
    assert.equal(catalog.summary[status], catalog.rows.filter((row) => row.status === status).length)
  }
  assert.equal(Object.values(catalog.summary).reduce((sum, count) => sum + count, 0), catalog.rows.length)
  assert.deepEqual(catalog.statusLabels, {
    supported: 'Runs locally',
    partial: 'Runs locally with limits',
    stub: 'Runs locally with limits',
    unsupported: 'Requires Salesforce',
    unknown: 'Not measured'
  })
})

test('support catalog retains non-completion labels and unsupported rows', () => {
  assert.ok(catalog.rows.some((row) => row.api === 'Visualforce full rendering lifecycle' && row.status === 'unsupported'))
  assert.match(catalog.rootCompletions.find((item) => item.label === 'Answers')?.info || '', /not a hosted search service/)
})

test('support catalog rejects duplicate ledger identities', () => {
  assert.throws(
    () => parseGeneratorRows('| Area | `Area.api` | `supported` | first |\n| Area | `Area.api` | `unsupported` | second |'),
    /duplicate support ledger row/i
  )
})
