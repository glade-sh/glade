import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { mkdtemp, mkdir, writeFile, readFile, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { publishRelease } from './release-publish.mjs'

const version = 'v1.2.3', sha = '1'.repeat(40), tools = '2'.repeat(40)
const hash = b => createHash('sha256').update(b).digest('hex')
async function fixture(t) {
  const root = await mkdtemp(path.join(os.tmpdir(), 'publisher-authority-'))
  t.after(() => rm(root, { recursive: true, force: true }))
  const dir = path.join(root, version)
  await mkdir(dir)
  const assets = [], checks = []
  for (const os of ['darwin', 'linux']) for (const arch of ['amd64', 'arm64']) {
    const name = `glade_${version}_${os}_${arch}.tar.gz`, bytes = Buffer.from(name), digest = hash(bytes)
    assets.push({ os, arch, url: `https://downloads.glade.sh/${version}/${name}`, sha256: digest })
    checks.push(`${digest}  ./${name}`)
    await writeFile(path.join(dir, name), bytes)
    await writeFile(path.join(dir, `${name}.sha256`), checks.at(-1) + '\n')
    await writeFile(path.join(dir, `${name}.sbom.json`), '{}')
  }
  const docs = {
    'release-manifest.json': { schemaVersion: 2, channel: 'stable', version, assets },
    'required-ci-attestation.json': { repository: 'glade-sh/glade', sha, conclusion: 'success', event: 'push', run_id: 1, required_ci_job_id: 2 },
    'salesforce-release-authority.json': { repository: 'glade-sh/glade', gladeSHA: sha, toolsSHA: tools, githubAppID: 4101915, checkName: 'Salesforce Correctness', receiptSHA256: '3'.repeat(64), checkRunID: 3, workflowRunID: 4, workflowRunAttempt: 1 },
  }
  for (const [name, doc] of Object.entries(docs)) await writeFile(path.join(dir, name), JSON.stringify(doc))
  await writeFile(path.join(dir, 'SHA256SUMS.txt'), checks.join('\n') + '\n')
  await writeFile(path.join(root, 'index.json'), JSON.stringify({ schemaVersion: 1, channel: 'stable', latest: version, versions: [{ version, manifest: `https://downloads.glade.sh/${version}/release-manifest.json` }] }))
  const objects = new Map(), writes = []
  const bucket = { async get(k) { const b = objects.get(k); return b && { etag: hash(b), arrayBuffer: async () => b } }, async put(k, b) { writes.push(k); objects.set(k, Buffer.from(b)); return { etag: hash(b) } } }
  return { root, dir, bucket, writes, objects }
}

test('rejected independent authority causes zero bucket writes', async t => {
  for (const reason of ['forged approval', 'wrong source', 'missing provenance', 'missing CycloneDX']) {
    const f = await fixture(t)
    await assert.rejects(publishRelease(f.bucket, f.root, version, sha, tools, async () => { throw Error(reason) }), new RegExp(reason))
    assert.deepEqual(f.writes, [])
  }
})

test('publishes the verified staged bytes even if source changes during verification', async t => {
  const f = await fixture(t)
  const name = `glade_${version}_linux_amd64.tar.gz`
  let observed = false
  await publishRelease(f.bucket, f.root, version, sha, tools, async ({ files }) => {
    observed = true
    assert.equal(files.get(`${version}/${name}`).toString(), name)
    await writeFile(path.join(f.dir, name), 'substituted')
  })
  assert.ok(observed)
  assert.equal(f.objects.get(`${version}/${name}`).toString(), name)
})


// Exercise the actual default verifier through fake process boundaries.  No
// test substitutes the verifier passed to publishRelease in these cases.
async function fakeAuthorityProcesses(t, f, mode) {
  const bin = path.join(f.root, 'bin'), log = path.join(f.root, 'proof.log')
  await mkdir(bin)
  const runner = `#!${process.execPath}\n` + `
const fs = require('node:fs');
const path = require('node:path');
const args = process.argv.slice(2);
const mode = process.env.CASE343_FAKE_MODE;
const dir = process.env.CASE343_FAKE_DIR;
if (path.basename(process.argv[1]) === 'bash') {
  const ci = args[0].endsWith('/verify-required-ci.sh');
  const sf = args[0].endsWith('/verify-salesforce-check.sh');
  if (!ci && !sf) process.exit(21);
  if ((ci && mode === 'forged-ci') || (sf && mode === 'forged-sf')) console.log('{}');
  else process.stdout.write(fs.readFileSync(path.join(dir, ci ? 'required-ci-attestation.json' : 'salesforce-release-authority.json')));
} else {
  fs.appendFileSync(process.env.CASE343_FAKE_LOG, JSON.stringify(args) + '\\n');
  const required = ['attestation', 'verify', '--repo', 'glade-sh/glade', '--signer-workflow', 'glade-sh/glade/.github/workflows/release.yml', '--source-digest', '${sha}', '--source-ref', 'refs/tags/${version}'];
  if (!required.every(x => args.includes(x))) process.exit(22);
  const filename = args[2];
  if (fs.readFileSync(filename, 'utf8') !== path.basename(filename)) process.exit(23);
  const sbom = args.includes('--predicate-type');
  if ((mode === 'wrong-source' && !sbom) || (mode === 'missing-provenance' && !sbom) || (mode === 'missing-cyclonedx' && sbom)) process.exit(24);
  if (sbom) console.log(JSON.stringify([{verificationResult:{statement:{predicate: mode === 'wrong-sbom' ? {forged:true} : {}}}}]));
}
`
  await writeFile(path.join(bin, 'bash'), runner, { mode: 0o755 })
  await writeFile(path.join(bin, 'gh'), runner, { mode: 0o755 })
  const old = { PATH: process.env.PATH, CASE343_FAKE_MODE: process.env.CASE343_FAKE_MODE, CASE343_FAKE_DIR: process.env.CASE343_FAKE_DIR, CASE343_FAKE_LOG: process.env.CASE343_FAKE_LOG }
  process.env.PATH = `${bin}${path.delimiter}${old.PATH}`
  process.env.CASE343_FAKE_MODE = mode
  process.env.CASE343_FAKE_DIR = f.dir
  process.env.CASE343_FAKE_LOG = log
  t.after(() => { for (const [key, value] of Object.entries(old)) value === undefined ? delete process.env[key] : process.env[key] = value })
  return log
}

test('default publisher verifier authenticates all staged archives before writing', async t => {
  const f = await fixture(t), log = await fakeAuthorityProcesses(t, f, 'success')
  await publishRelease(f.bucket, f.root, version, sha, tools)
  const calls = (await readFile(log, 'utf8')).trim().split('\n').map(JSON.parse)
  assert.equal(calls.length, 8)
  assert.equal(calls.filter(args => args.includes('--predicate-type')).length, 4)
  assert.equal(f.writes.length, 18)
})

for (const mode of ['forged-ci', 'forged-sf', 'wrong-source', 'missing-provenance', 'missing-cyclonedx', 'wrong-sbom']) {
  test(`default publisher verifier rejects ${mode} before bucket writes`, async t => {
    const f = await fixture(t)
    await fakeAuthorityProcesses(t, f, mode)
    const error = mode === 'forged-ci' ? /CI authority differs/ : mode === 'forged-sf' ? /Salesforce authority differs/ : mode === 'wrong-sbom' ? /attested SBOM differs/ : /Command failed/
    await assert.rejects(publishRelease(f.bucket, f.root, version, sha, tools), error)
    assert.deepEqual(f.writes, [])
  })
}
