import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'

const read = (path) => readFile(new URL(path, import.meta.url), 'utf8')

test('support map derives the current stable version from the manifest', async () => {
  const page = await read('../docs-src/guide/support-map.md')
  assert.match(page, /Stable download: Glade \{\{ releaseManifest\.version \}\}/)
  assert.doesNotMatch(page, /v0\.2\.15[^\n]*Current stable distribution/)
  assert.match(page, /v0\.2\.15[^\n]*Historical distribution/)
})

test('trust pages distinguish historical inventory from later release artifacts', async () => {
  for (const path of ['../../docs/SECURITY_TRUST.md', '../docs-src/guide/security-trust.md']) {
    const page = await read(path)
    assert.match(page, /later releases[\s\S]{0,150}binaries, SBOMs, and attestations/i)
    assert.match(page, /historical v0\.2\.15 evidence/i)
  }
})
