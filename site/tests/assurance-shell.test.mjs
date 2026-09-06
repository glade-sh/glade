import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { styleAssuranceHTML } from '../scripts/style-assurance.mjs'

const index = '<link rel="preload stylesheet" href="/assets/style.abc-123.css" as="style"><link rel="preload stylesheet" href="/vp-icons.css" as="style">'
const source = await readFile(new URL('../docs-src/public/private-corpus-assurance.html', import.meta.url), 'utf8')

test('assurance shell shares the current main CSS without changing embedded evidence', () => {
  const built = styleAssuranceHTML(source, index)
  assert.equal(built.href, '/assets/style.abc-123.css')
  const payload = html => html.match(/<script id="assurance-data" type="application\/json">([\s\S]*?)<\/script>/)[1]
  assert.equal(payload(built.html), payload(source))
  assert.equal(styleAssuranceHTML(built.html, index).html, built.html)
  assert.equal((built.html.match(/data-glade-shared-style/g) || []).length, 1)
})

test('assurance style integration rejects missing or ambiguous main CSS', () => {
  assert.throws(() => styleAssuranceHTML(source, '<link rel="stylesheet" href="https://example.com/assets/style.a.css">'), /exactly one local/)
  assert.throws(() => styleAssuranceHTML(source, index + '<link rel="stylesheet" href="/assets/style.second.css">'), /exactly one local/)
  assert.throws(() => styleAssuranceHTML(source.replace('<!-- glade-shared-styles -->', ''), index), /one assurance/)
})
