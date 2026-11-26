import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'

const read = (path) => readFile(new URL(path, import.meta.url), 'utf8')

test('support map derives the stable download and puts recent evidence first', async () => {
  const page = await read('../docs-src/guide/support-map.md')
  assert.match(page, /Stable download: Glade \{\{ releaseManifest\.version \}\}/)
  const [current, historical] = page.split('## Earlier releases')
  assert.match(current, /477 of 477 required checks, including 412 compiler, 53 runtime, and 12 lifecycle/)
  assert.match(current, /\/releases\/download\/v0\.3\.15\/salesforce-release-authority\.json/)
  assert.doesNotMatch(current, /v0\.2\.15/)
  assert.match(historical, /v0\.2\.15[^\n]*475 of 475 required Salesforce checks/)
})

test('trust pages lead with release verification and scope the old inventory', async () => {
  for (const path of ['../../docs/SECURITY_TRUST.md', '../docs-src/guide/security-trust.md']) {
    const page = await read(path)
    const [releaseProof, historical] = page.split('### Historical example')
    assert.match(releaseProof, /archive[\s\S]*SBOM[\s\S]*verify/i)
    assert.doesNotMatch(releaseProof, /128–129/)
    assert.match(historical, /v0\.2\.15 binaries[\s\S]*128–129/)
  }
})
