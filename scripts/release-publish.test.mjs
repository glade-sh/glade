import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { mkdtemp, mkdir, writeFile, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { publishRelease, previewBucket, releaseR2Fetch } from './release-publish.mjs'

const version = 'v1.2.3', sha = '1'.repeat(40), toolsSHA = '2'.repeat(40)
const hash = bytes => createHash('sha256').update(bytes).digest('hex')

async function fixture(t) {
  const root = await mkdtemp(path.join(os.tmpdir(), 'glade-publish-test-'))
  t.after(() => rm(root, { recursive: true, force: true }))
  await mkdir(path.join(root, version))
  const assets = [], checksums = []
  for (const os of ['darwin', 'linux']) for (const arch of ['amd64', 'arm64']) {
    const name = `glade_${version}_${os}_${arch}.tar.gz`, bytes = Buffer.from(name)
    assets.push({ os, arch, url: `https://downloads.glade.sh/${version}/${name}`, sha256: hash(bytes) })
    checksums.push(`${hash(bytes)}  ./${name}\n`)
    await writeFile(path.join(root, version, name), bytes)
    await writeFile(path.join(root, version, `${name}.sha256`), checksums.at(-1))
    await writeFile(path.join(root, version, `${name}.sbom.json`), '{}')
  }
  const manifest = { schemaVersion: 2, channel: 'stable', version, assets }
  const files = {
    'release-manifest.json': JSON.stringify(manifest),
    'SHA256SUMS.txt': checksums.join(''),
    'required-ci-attestation.json': JSON.stringify({ schema_version: 1, repository: 'glade-sh/glade', sha, conclusion: 'success', event: 'push', run_id: 1, required_ci_job_id: 2 }),
    'salesforce-release-authority.json': JSON.stringify({ schemaVersion: 1, repository: 'glade-sh/glade', gladeSHA: sha, toolsSHA, githubAppID: 4101915, checkName: 'Salesforce Correctness', receiptSHA256: '3'.repeat(64), checkRunID: 3, workflowRunID: 4, workflowRunAttempt: 1 }),
  }
  for (const [name, body] of Object.entries(files)) await writeFile(path.join(root, version, name), body)
  await writeFile(path.join(root, 'index.json'), JSON.stringify({ schemaVersion: 1, channel: 'stable', latest: version, versions: [{ version, manifest: `https://downloads.glade.sh/${version}/release-manifest.json` }] }))
  const objects = new Map(), calls = []
  const bucket = {
    async get(key) {
      const bytes = objects.get(key)
      calls.push(['get', key])
      return bytes ? { etag: hash(bytes), arrayBuffer: async () => bytes } : null
    },
    async put(key, body, options) {
      calls.push(['put', key, options])
      const old = objects.get(key)
      if (options.onlyIf.etagDoesNotMatch === '*' && old) return null
      if (options.onlyIf.etagMatches && (!old || hash(old) !== options.onlyIf.etagMatches)) return null
      objects.set(key, Buffer.from(body))
      return { etag: hash(body) }
    },
  }
  return { root, objects, calls, bucket }
}

test('publishes create-only files, verifies them, advances pointers last, and resumes', async t => {
  const { root, bucket: storage, calls } = await fixture(t)
  const env = { VERSION: version, BUCKET: {
    async get(key) { const object = await storage.get(key); return object && { etag: object.etag, body: await object.arrayBuffer() } },
    async put(key, body, options) { return storage.put(key, Buffer.from(await new Response(body).arrayBuffer()), options) },
  } }
  const bucket = previewBucket((url, init) => releaseR2Fetch(new Request(url, init), env))
  await publishRelease(bucket, root, version, sha, toolsSHA)
  const writes = calls.filter(([method]) => method === 'put')
  assert.equal(writes.length, 18)
  assert.deepEqual(writes.slice(-2).map(([, key]) => key), ['latest/release-manifest.json', 'index.json'])
  for (const [, key, options] of writes.slice(0, -2)) {
    assert.equal(options.onlyIf.etagDoesNotMatch, '*')
    assert.ok(calls.some(([method, readKey]) => method === 'get' && readKey === key))
  }
  await publishRelease(bucket, root, version, sha, toolsSHA)
  assert.equal(calls.filter(([method]) => method === 'put').length, 18)
})

test('remote preview rejects unrelated keys and unconditional or versioned overwrite requests', async () => {
  const env = { VERSION: version, BUCKET: { get() { assert.fail('unexpected read') }, put() { assert.fail('unexpected write') } } }
  for (const [key, options, status] of [
    ['v0.0.1/release-manifest.json', { onlyIf: { etagDoesNotMatch: '*' } }, 403],
    [`${version}/release-manifest.json`, {}, 400],
    [`${version}/release-manifest.json`, { onlyIf: { etagMatches: 'existing' } }, 400],
  ]) {
    const response = await releaseR2Fetch(new Request(`http://localhost/?key=${encodeURIComponent(key)}`, { method: 'PUT', body: 'x', headers: { 'x-r2-options': JSON.stringify({ sha256: 'a'.repeat(64), ...options }) } }), env)
    assert.equal(response.status, status)
  }
})

test('never overwrites a conflicting versioned object or advances pointers', async t => {
  const { root, bucket, objects } = await fixture(t)
  const key = `${version}/release-manifest.json`
  objects.set(key, Buffer.from('previous immutable bytes'))
  await assert.rejects(publishRelease(bucket, root, version, sha, toolsSHA), /differs/)
  assert.equal(objects.get(key).toString(), 'previous immutable bytes')
  assert.equal(objects.has('index.json'), false)
})

test('rejects wrong approval SHA before any publication', async t => {
  const { root, bucket, calls } = await fixture(t)
  await assert.rejects(publishRelease(bucket, root, version, 'f'.repeat(40), toolsSHA), /approval/)
  assert.equal(calls.length, 0)
})

test('does not roll a newer channel backward', async t => {
  const { root, bucket, objects, calls } = await fixture(t)
  objects.set('index.json', Buffer.from(JSON.stringify({ latest: 'v1.2.4', versions: [] })))
  await assert.rejects(publishRelease(bucket, root, version, sha, toolsSHA), /newer/)
  assert.equal(calls.some(([method]) => method === 'put'), false)
})

test('rejects approval for a different tagged Tools commit before publication', async t => {
  const { root, bucket, calls } = await fixture(t)
  await assert.rejects(publishRelease(bucket, root, version, sha, 'f'.repeat(40)), /Tools approval/)
  assert.equal(calls.length, 0)
})

test('rechecks both pointers when another publisher changes an initially identical pointer', async t => {
  const { root, bucket, objects } = await fixture(t)
  await publishRelease(bucket, root, version, sha, toolsSHA)
  objects.delete(`${version}/SHA256SUMS.txt`)
  const put = bucket.put.bind(bucket)
  bucket.put = async (...args) => {
    const result = await put(...args)
    if (args[0] === `${version}/SHA256SUMS.txt`) {
      objects.set('latest/release-manifest.json', Buffer.from(JSON.stringify({ version: 'v1.2.4' })))
    }
    return result
  }
  await assert.rejects(publishRelease(bucket, root, version, sha, toolsSHA), /final channel/)
})
