import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'

test('capability reports are identified as incomplete snapshots', async () => {
  for (const path of ['../../docs/README.md', '../docs-src/guide/support-map.md']) {
    const text = await readFile(new URL(path, import.meta.url), 'utf8')
    assert.match(text, /generated capability (?:reports|catalog)[\s\S]{0,200}incomplete/i)
    assert.match(text, /(?:Flow|Cache)[\s\S]{0,150}(?:Flow|Cache)/)
    assert.match(text, /COMPATIBILITY\.md|reference\/apex-support/)
  }
})
