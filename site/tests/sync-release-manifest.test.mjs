import assert from 'node:assert/strict'
import { test } from 'node:test'
import { fetchManifest } from '../scripts/sync-release-manifest.mjs'

const version = 'v1.2.3'
const manifest = {
  schemaVersion: 2,
  channel: 'stable',
  version,
  assets: ['darwin/amd64', 'darwin/arm64', 'linux/amd64', 'linux/arm64'].map((platform) => {
    const [os, arch] = platform.split('/')
    return { os, arch, sha256: 'a'.repeat(64), url: `https://downloads.glade.sh/${version}/glade_${version}_${os}_${arch}.tar.gz` }
  })
}
const ok = () => ({ status: 200, json: async () => manifest })

function scripted(...results) {
  const calls = []
  const fetchImpl = async (url) => {
    calls.push(url)
    const result = results.shift()()
    if (result instanceof Error) throw result
    return result
  }
  return { calls, fetchImpl }
}

test('release manifest sync retries transient download-host failures with backoff', async () => {
  const delays = []
  const { calls, fetchImpl } = scripted(() => new Error('socket hang up'), () => ({ status: 503 }), ok)
  const result = await fetchManifest({ fetchImpl, sleep: async (ms) => delays.push(ms) })
  assert.equal(result.version, version)
  assert.equal(calls.length, 3)
  assert.ok(calls.every((url) => url === 'https://downloads.glade.sh/latest/release-manifest.json'))
  assert.deepEqual(delays, [1000, 2000])
})

test('release manifest sync fails after its retries run out', async () => {
  const delays = []
  const { calls, fetchImpl } = scripted(() => ({ status: 502 }), () => ({ status: 502 }), () => ({ status: 502 }), () => ({ status: 502 }))
  await assert.rejects(fetchManifest({ fetchImpl, sleep: async (ms) => delays.push(ms) }), /HTTP 502/)
  assert.equal(calls.length, 4)
  assert.deepEqual(delays, [1000, 2000, 4000])
})

test('release manifest sync does not retry a missing or invalid manifest', async () => {
  const missing = scripted(() => ({ status: 404 }))
  await assert.rejects(fetchManifest({ fetchImpl: missing.fetchImpl, sleep: async () => assert.fail('no retry') }), /HTTP 404/)
  assert.equal(missing.calls.length, 1)
  const invalid = scripted(() => ({ status: 200, json: async () => ({ ...manifest, channel: 'beta' }) }))
  await assert.rejects(fetchManifest({ fetchImpl: invalid.fetchImpl, sleep: async () => assert.fail('no retry') }), /stable channel/)
  assert.equal(invalid.calls.length, 1)
})
