#!/usr/bin/env node
import assert from 'node:assert/strict'
import { rename, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const sourceURL = 'https://downloads.glade.sh/latest/release-manifest.json'
const target = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'release-manifest.json')

export function validateManifest(manifest) {
  assert.equal(manifest.schemaVersion, 2, 'release manifest schema should be 2')
  assert.equal(manifest.channel, 'stable', 'site release manifest should use the stable channel')
  assert.match(manifest.version || '', /^v\d+\.\d+\.\d+$/, 'release manifest should name a stable version')
  assert.ok(Array.isArray(manifest.assets) && manifest.assets.length > 0, 'release manifest should advertise assets')
  const platforms = manifest.assets.map((asset) => `${asset.os}/${asset.arch}`).sort()
  assert.deepEqual(platforms, ['darwin/amd64', 'darwin/arm64', 'linux/amd64', 'linux/arm64'], 'stable manifest should contain every packaged platform exactly once')
  for (const asset of manifest.assets) {
    assert.match(asset.sha256 || '', /^[a-f0-9]{64}$/, `${asset.os}/${asset.arch} should include a SHA-256`)
    assert.equal(new URL(asset.url).pathname.includes(`/${manifest.version}/glade_${manifest.version}_`), true, `${asset.os}/${asset.arch} URL should use the stable version`)
  }
  return manifest
}

const transient = (status) => status === 408 || status === 429 || status >= 500

// Retry brief download-host failures with backoff, then fail the build.
export async function fetchManifest({ fetchImpl = fetch, attempts = 4, baseDelayMs = 1000, sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms)) } = {}) {
  let failure
  for (let attempt = 1; attempt <= attempts; attempt++) {
    try {
      const response = await fetchImpl(sourceURL, { signal: AbortSignal.timeout(15000) })
      if (response.status === 200) return validateManifest(await response.json())
      failure = new Error(`could not fetch ${sourceURL}: HTTP ${response.status}`)
      if (!transient(response.status)) break
    } catch (error) {
      if (error instanceof assert.AssertionError) throw error
      failure = new Error(`could not fetch ${sourceURL}: ${error.message}`)
    }
    if (attempt < attempts) {
      const delay = baseDelayMs * 2 ** (attempt - 1)
      console.warn(`${failure.message}; retrying in ${delay} ms (attempt ${attempt + 1} of ${attempts})`)
      await sleep(delay)
    }
  }
  throw failure
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  const formatted = `${JSON.stringify(await fetchManifest(), null, 2)}\n`
  const temporary = `${target}.tmp-${process.pid}`
  await writeFile(temporary, formatted)
  await rename(temporary, target)
  console.log(`Updated ${target} from ${sourceURL}.`)
}
