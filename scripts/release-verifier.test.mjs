import assert from 'node:assert/strict'
import test from 'node:test'
import { verifyReleaseAuthority } from './release-publish.mjs'

const version = 'v1.2.3', expectedSHA = 'a'.repeat(40), expectedToolsSHA = 'b'.repeat(40)
const ci = { run_id: 1 }, sf = { checkRunID: 2 }, sbom = { bomFormat: 'CycloneDX' }
function fixture() {
  const files = new Map([
    [`${version}/required-ci-attestation.json`, Buffer.from(JSON.stringify(ci))],
    [`${version}/salesforce-release-authority.json`, Buffer.from(JSON.stringify(sf))],
  ])
  for (const os of ['darwin', 'linux']) for (const arch of ['amd64', 'arm64']) {
    const name = `glade_${version}_${os}_${arch}.tar.gz`
    files.set(`${version}/${name}`, Buffer.from(name))
    files.set(`${version}/${name}.sbom.json`, Buffer.from(JSON.stringify(sbom)))
  }
  return { files, version, expectedSHA, expectedToolsSHA }
}
function executor(calls, wrongSBOM = false) {
  return (cmd, args) => {
    calls.push([cmd, args])
    if (cmd === 'bash') return JSON.stringify(args[0].endsWith('verify-required-ci.sh') ? ci : sf)
    assert.equal(cmd, 'gh')
    for (const arg of ['glade-sh/glade', 'glade-sh/glade/.github/workflows/release.yml', expectedSHA, `refs/tags/${version}`]) assert.ok(args.includes(arg), `missing ${arg}`)
    if (args.includes('--predicate-type')) return JSON.stringify([{ verificationResult: { statement: { predicate: wrongSBOM ? {} : sbom } } }])
    return ''
  }
}

test('pins source and signer for both proofs of every archive and matches its SBOM', async () => {
  const calls = []
  await verifyReleaseAuthority(fixture(), executor(calls))
  assert.equal(calls.filter(([cmd]) => cmd === 'gh').length, 8)
})

test('rejects forged independent approvals and mismatched attested SBOM', async () => {
  const f = fixture(), calls = []
  f.files.set(`${version}/required-ci-attestation.json`, Buffer.from('{}'))
  await assert.rejects(verifyReleaseAuthority(f, executor(calls)), /CI authority differs/)
  assert.equal(calls.filter(([cmd]) => cmd === 'gh').length, 0)
  const g = fixture()
  g.files.set(`${version}/salesforce-release-authority.json`, Buffer.from('{}'))
  await assert.rejects(verifyReleaseAuthority(g, executor([])), /Salesforce authority differs/)
  await assert.rejects(verifyReleaseAuthority(fixture(), executor([], true)), /attested SBOM differs/)
})
