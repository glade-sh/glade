import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

test('release runs extension behavioral tests on the payload checkout before packaging', async () => {
  const workflow = await readFile(new URL('../.github/workflows/release.yml', import.meta.url), 'utf8')
  const payload = workflow.match(/^  shared-payload:\n([\s\S]*?)(?=^  [\w-]+:\n)/m)?.[1]
  assert.ok(payload)
  assert.match(payload, /actions\/checkout@/)
  const install = payload.indexOf('run: npm ci')
  const tests = payload.indexOf('run: npm test')
  const build = payload.indexOf('run: scripts/release-build.sh shared-payload')
  assert.ok(install >= 0 && install < tests && tests < build, 'extension tests must precede shared-payload build')
  assert.match(payload, /working-directory: contrib\/vscode-glade/)
})
