#!/usr/bin/env node
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { lstat, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { createRequire } from 'node:module'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const sha256 = bytes => createHash('sha256').update(bytes).digest('hex')
const stable = /^v\d+\.\d+\.\d+$/
const sourceSHA = /^[0-9a-f]{40}$/
const newer = (left, right) => left.localeCompare(right, 'en', { numeric: true }) > 0

export async function publishRelease(bucket, root, version, expectedSHA, expectedToolsSHA) {
  assert.match(version, stable, 'expected a stable release version')
  assert.match(expectedSHA, sourceSHA, 'expected the approved product SHA')
  assert.match(expectedToolsSHA, sourceSHA, 'expected the tagged Tools SHA')
  const files = new Map()
  async function stage(name) {
    const filename = path.join(root, version, name)
    assert.ok((await lstat(filename)).isFile(), `not a regular release file: ${name}`)
    const bytes = await readFile(filename)
    files.set(`${version}/${name}`, bytes)
    return bytes
  }
  const manifestBytes = await stage('release-manifest.json')
  const manifest = JSON.parse(manifestBytes)
  assert.equal(manifest.schemaVersion, 2)
  assert.equal(manifest.channel, 'stable')
  assert.equal(manifest.version, version)
  const ci = JSON.parse(await stage('required-ci-attestation.json'))
  const sf = JSON.parse(await stage('salesforce-release-authority.json'))
  const anchor = JSON.parse(await readFile(new URL('../.github/release-authorities.json', import.meta.url)))
  assert.equal(ci.sha, expectedSHA, 'CI approval SHA mismatch')
  assert.equal(sf.gladeSHA, expectedSHA, 'Salesforce approval SHA mismatch')
  assert.equal(ci.repository, 'glade-sh/glade')
  assert.equal(sf.repository, ci.repository)
  assert.equal(ci.conclusion, 'success')
  assert.equal(ci.event, 'push')
  assert.equal(sf.githubAppID, anchor.githubAppID)
  assert.equal(sf.checkName, anchor.checkName)
  assert.match(sf.toolsSHA, sourceSHA)
  assert.equal(sf.toolsSHA, expectedToolsSHA, 'Tools approval differs from the annotated tag')
  assert.match(sf.receiptSHA256, /^[0-9a-f]{64}$/)
  for (const id of [ci.run_id, ci.required_ci_job_id, sf.checkRunID, sf.workflowRunID, sf.workflowRunAttempt]) {
    assert.ok(Number.isSafeInteger(id) && id > 0, 'approval identifier missing')
  }
  assert.deepEqual(manifest.assets.map(a => `${a.os}/${a.arch}`).sort(), ['darwin/amd64', 'darwin/arm64', 'linux/amd64', 'linux/arm64'])
  const checksums = (await stage('SHA256SUMS.txt')).toString().trim().split('\n').sort()
  const expectedChecksums = []
  for (const asset of manifest.assets) {
    const name = `glade_${version}_${asset.os}_${asset.arch}.tar.gz`
    assert.equal(asset.url, `https://downloads.glade.sh/${version}/${name}`)
    assert.match(asset.sha256, /^[0-9a-f]{64}$/)
    assert.equal(sha256(await stage(name)), asset.sha256, `archive checksum mismatch: ${name}`)
    const checksum = `${asset.sha256}  ./${name}`
    assert.equal((await stage(`${name}.sha256`)).toString().trim(), checksum)
    JSON.parse(await stage(`${name}.sbom.json`))
    expectedChecksums.push(checksum)
  }
  assert.deepEqual(checksums, expectedChecksums.sort(), 'release checksum inventory mismatch')
  const indexBytes = await readFile(path.join(root, 'index.json'))
  const index = JSON.parse(indexBytes)
  assert.equal(index.latest, version)
  assert.equal(index.schemaVersion, 1)
  assert.equal(index.channel, 'stable')
  assert.ok(index.versions.some(row => row.version === version && row.manifest === `https://downloads.glade.sh/${version}/release-manifest.json`))

  async function snapshot(key) {
    const object = await bucket.get(key)
    return object ? { etag: object.etag, bytes: Buffer.from(await object.arrayBuffer()) } : null
  }
  const pointers = new Map([['latest/release-manifest.json', manifestBytes], ['index.json', indexBytes]])
  const previous = new Map()
  for (const key of pointers.keys()) {
    const old = await snapshot(key)
    previous.set(key, old)
    if (!old) continue
    const document = JSON.parse(old.bytes)
    const current = document.latest || document.version
    assert.match(current, stable, 'invalid current channel version')
    assert.ok(!newer(current, version), 'refusing to replace a newer channel')
    if (key === 'index.json') {
      for (const row of document.versions) {
        assert.ok(index.versions.some(candidate => candidate.version === row.version && candidate.manifest === row.manifest), 'new index drops an existing version')
      }
    }
  }
  async function putVerified(key, bytes, old, mutable) {
    if (old && old.bytes.equals(bytes)) return
    assert.ok(mutable || !old, `immutable object differs: ${key}`)
    const result = await bucket.put(key, bytes, {
      onlyIf: old ? { etagMatches: old.etag } : { etagDoesNotMatch: '*' },
      sha256: sha256(bytes),
      httpMetadata: {
        contentType: key.endsWith('.json') ? 'application/json' : key.endsWith('.tar.gz') ? 'application/gzip' : 'text/plain',
        cacheControl: mutable ? 'no-cache' : 'public, max-age=31536000, immutable',
      },
    })
    assert.ok(result, `conditional publication conflict: ${key}; retry after inspection`)
    const stored = await snapshot(key)
    assert.ok(stored && stored.bytes.equals(bytes), `release readback differs: ${key}`)
    console.log(`Verified ${key}`)
  }
  for (const [key, bytes] of files) await putVerified(key, bytes, await snapshot(key), false)
  for (const [key, bytes] of pointers) await putVerified(key, bytes, previous.get(key), true)
  for (const [key, bytes] of pointers) {
    const final = await snapshot(key)
    assert.ok(final && final.bytes.equals(bytes), `final channel differs: ${key}`)
  }
  return { version, sourceSHA: expectedSHA, toolsSHA: sf.toolsSHA, versionedObjects: files.size }
}

export async function releaseR2Fetch(request, env) {
  const key = new URL(request.url).searchParams.get('key') || ''
  const mutable = key === 'index.json' || key === 'latest/release-manifest.json'
  const filename = key.startsWith(`${env.VERSION}/`) ? key.slice(env.VERSION.length + 1) : ''
  if (!mutable && (!filename || filename.includes('/') || filename.includes('..'))) return new Response(null, { status: 403 })
  if (request.method === 'GET') {
    const object = await env.BUCKET.get(key)
    return object ? new Response(object.body, { headers: { etag: object.etag } }) : new Response(null, { status: 404 })
  }
  if (request.method !== 'PUT') return new Response(null, { status: 405 })
  const options = JSON.parse(request.headers.get('x-r2-options') || '{}')
  if (!(options.onlyIf?.etagDoesNotMatch === '*' || (mutable && options.onlyIf?.etagMatches)) || !/^[a-f0-9]{64}$/.test(options.sha256 || '')) return new Response(null, { status: 400 })
  const object = await env.BUCKET.put(key, request.body, options)
  return Response.json(object ? { etag: object.etag } : null)
}

export function previewBucket(fetch) {
  return {
    async get(key) {
      const response = await fetch(`http://localhost/?key=${encodeURIComponent(key)}`, { signal: AbortSignal.timeout(120000) })
      if (response.status === 404) return null
      assert.ok(response.ok, `remote R2 read failed: ${response.status}`)
      return { etag: response.headers.get('etag'), arrayBuffer: () => response.arrayBuffer() }
    },
    async put(key, body, options) {
      const response = await fetch(`http://localhost/?key=${encodeURIComponent(key)}`, { method: 'PUT', body, headers: { 'x-r2-options': JSON.stringify(options) }, signal: AbortSignal.timeout(120000) })
      assert.ok(response.ok, `remote R2 write failed: ${response.status}`)
      return response.json()
    },
  }
}

async function main() {
  assert.equal(process.argv.length, 5, 'usage: release-publish.mjs <unpacked-bundle> <version> <approved-product-sha>')
  const [root, version, expectedSHA] = process.argv.slice(2)
  assert.match(version, stable)
  assert.match(expectedSHA, sourceSHA)
  assert.equal(execFileSync('git', ['rev-parse', `${version}^{commit}`], { encoding: 'utf8' }).trim(), expectedSHA, 'tagged product SHA differs')
  const expectedToolsSHA = execFileSync('bash', [fileURLToPath(new URL('./verify-salesforce-check.sh', import.meta.url)), '--tag-tools-sha', version], { encoding: 'utf8' }).trim()
  assert.match(process.env.CLOUDFLARE_ACCOUNT_ID || '', /^[a-f0-9]{32}$/, 'CLOUDFLARE_ACCOUNT_ID is required')
  const require = createRequire(import.meta.url)
  const { unstable_dev } = require(process.env.WRANGLER_MODULE || 'wrangler')
  const temporary = await mkdtemp(path.join(os.tmpdir(), 'glade-release-publisher-'))
  let worker
  try {
    const configPath = path.join(temporary, 'wrangler.json')
    await writeFile(configPath, JSON.stringify({
      name: 'glade-release-publisher', compatibility_date: '2026-09-01',
      account_id: process.env.CLOUDFLARE_ACCOUNT_ID,
      vars: { VERSION: version },
      r2_buckets: [{ binding: 'BUCKET', bucket_name: 'glade-downloads', remote: true }],
    }), { mode: 0o600 })
    const script = path.join(temporary, 'publisher.mjs')
    await writeFile(script, `export default { fetch: ${releaseR2Fetch.toString()} };\n`, { mode: 0o600 })
    // Use asynchronous preview requests: getPlatformProxy can deadlock on remote bindings.
    worker = await unstable_dev(script, { config: configPath, local: false, experimental: { disableExperimentalWarning: true, disableDevRegistry: true } })
    console.log(JSON.stringify(await publishRelease(previewBucket((...args) => worker.fetch(...args)), root, version, expectedSHA, expectedToolsSHA)))
  } finally {
    await worker?.stop()
    await rm(temporary, { recursive: true, force: true })
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  main().catch(error => { console.error(error.message); process.exitCode = 1 })
}
