#!/usr/bin/env node
import assert from 'node:assert/strict'
import { readFile, rename, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const sourceURL = 'https://downloads.glade.sh/latest/release-manifest.json'
const target = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'release-manifest.json')
const check = process.argv.includes('--check')

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

const response = await fetch(sourceURL)
assert.equal(response.status, 200, `could not fetch ${sourceURL}`)
const formatted = `${JSON.stringify(validateManifest(await response.json()), null, 2)}\n`

if (check) {
  assert.equal(await readFile(target, 'utf8'), formatted, 'checked release manifest differs from the published stable manifest')
} else {
  const temporary = `${target}.tmp-${process.pid}`
  await writeFile(temporary, formatted)
  await rename(temporary, target)
  console.log(`Updated ${target} from ${sourceURL}.`)
}
