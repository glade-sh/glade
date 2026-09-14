import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { chmod, mkdtemp, mkdir, readFile, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { delimiter, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'

const helperURL = new URL('../../scripts/verify-release-download.sh', import.meta.url)
const helperPath = fileURLToPath(helperURL)
const helper = await readFile(helperURL, 'utf8')
const securityPolicy = await readFile(new URL('../../SECURITY.md', import.meta.url), 'utf8')
const securityGuide = await readFile(new URL('../docs-src/guide/security-trust.md', import.meta.url), 'utf8')

function embeddedVerifier(source) {
  const match = source.match(/<!-- release-verifier:start -->\s*```(?:bash|sh)\n([\s\S]*?)```\s*<!-- release-verifier:end -->/)
  assert.ok(match, 'documentation should contain the marked release verifier')
  return match[1].trim()
}

test('published release-verification blocks stay synchronized with the checked helper', () => {
  for (const source of [securityPolicy, securityGuide]) {
    assert.equal(embeddedVerifier(source), helper.trim())
  }
})

test('manual verification fails closed and never extracts or executes the archive', () => {
  assert.match(helper, /^set -eu$/m)
  assert.match(helper, /curl --proto '=https' --proto-redir '=https' --tlsv1\.2/)
  assert.match(helper, /--fail --show-error --silent --location --max-time 60/)
  assert.match(helper, /gh attestation verify "\$archive" -R glade-sh\/glade/)
  assert.equal((helper.match(/--signer-workflow glade-sh\/glade\/\.github\/workflows\/release\.yml/g) || []).length, 2)
  assert.equal((helper.match(/--source-ref "refs\/tags\/\$\{version\}"/g) || []).length, 2)
  assert.match(helper, /checksum mismatch; do not extract or run the archive/)
  assert.match(helper, /provenance verification failed; do not extract or run the archive/)
  assert.match(helper, /CycloneDX attestation verification failed; do not extract or run the archive/)
  assert.doesNotMatch(helper, /\btar\s+-|\.\/glade\s+version/)
})

const fakeCurl = `#!/bin/sh
{
  printf 'curl'
  for arg in "$@"; do printf ' <%s>' "$arg"; done
  printf '\n'
} >>"$MOCK_LOG"
output=
url=
while test "$#" -gt 0; do
  case "$1" in
    --output) shift; output="$1" ;;
    https://*) url="$1" ;;
  esac
  shift
done
case "$MOCK_CASE:$url" in
  manifest_download_fail:*release-manifest.json|archive_download_fail:*.tar.gz|checksum_download_fail:*SHA256SUMS.txt) exit 22 ;;
esac
case "$url" in
  *release-manifest.json)
    body='{ "version": "v0.2.15" }'
    test "$MOCK_CASE" = malformed_manifest && body='{ invalid json'
    test "$MOCK_CASE" = unsafe_version && body='{ "version": "v0.2.15/../../other" }'
    test "$MOCK_CASE" = preview_version && body='{ "version": "v0.2.16-preview.1" }'
    ;;
  *SHA256SUMS.txt)
    entry='0000000000000000000000000000000000000000000000000000000000000000  ./glade_v0.2.15_linux_amd64.tar.gz'
    body="$entry"
    test "$MOCK_CASE" = checksum_missing && body='0000000000000000000000000000000000000000000000000000000000000000  ./other.tar.gz'
    test "$MOCK_CASE" = checksum_duplicate && body="$entry
$entry"
    test "$MOCK_CASE" = checksum_bad_hex && body='xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx  ./glade_v0.2.15_linux_amd64.tar.gz'
    ;;
  *) body='synthetic archive' ;;
esac
printf '%s\\n' "$body" >"$output"
`

const fakeChecksum = `#!/bin/sh
{
  printf 'checksum'
  for arg in "$@"; do printf ' <%s>' "$arg"; done
  printf '\n'
} >>"$MOCK_LOG"
test "$MOCK_CASE" != checksum_mismatch
`

const fakeGh = `#!/bin/sh
{
  printf 'gh'
  for arg in "$@"; do printf ' <%s>' "$arg"; done
  printf '\n'
} >>"$MOCK_LOG"
predicate=0
for arg in "$@"; do test "$arg" = --predicate-type && predicate=1; done
test "$MOCK_CASE" = provenance_fail && test "$predicate" -eq 0 && exit 1
test "$MOCK_CASE" = cyclonedx_fail && test "$predicate" -eq 1 && exit 1
exit 0
`

async function writeExecutable(path, source) {
  await writeFile(path, source)
  await chmod(path, 0o755)
}

test('release verifier stops at every synthetic download and trust failure', async () => {
  const failureModes = [
    'manifest_download_fail',
    'malformed_manifest',
    'unsafe_version',
    'preview_version',
    'archive_download_fail',
    'checksum_download_fail',
    'checksum_missing',
    'checksum_duplicate',
    'checksum_bad_hex',
    'checksum_mismatch',
    'provenance_fail',
    'cyclonedx_fail'
  ]

  for (const mode of [...failureModes, 'success']) {
    const root = await mkdtemp(join(tmpdir(), 'glade-verifier-test-'))
    const bin = join(root, 'bin')
    const downloads = join(root, 'downloads')
    const log = join(root, 'calls.log')
    await mkdir(bin)
    await mkdir(downloads)
    await writeExecutable(join(bin, 'curl'), fakeCurl)
    await writeExecutable(join(bin, 'shasum'), fakeChecksum)
    await writeExecutable(join(bin, 'sha256sum'), fakeChecksum)
    await writeExecutable(join(bin, 'gh'), fakeGh)
    await writeExecutable(join(bin, 'uname'), `#!/bin/sh\nprintf '%s\\n' uname >>"$MOCK_LOG"\ntest "$1" = -s && printf 'Linux\\n' || printf 'x86_64\\n'\n`)

    const result = spawnSync('/bin/sh', [helperPath], {
      cwd: root,
      encoding: 'utf8',
      env: {
        ...process.env,
        PATH: `${bin}${delimiter}${process.env.PATH}`,
        MOCK_CASE: mode,
        MOCK_LOG: log,
        TMPDIR: downloads
      }
    })

    assert.equal(result.status === 0, mode === 'success', `${mode}: ${result.stderr}`)
    const calls = await readFile(log, 'utf8')
    if (mode === 'checksum_mismatch') assert.doesNotMatch(calls, /^gh(?: |$)/m)
    if (mode === 'provenance_fail') assert.equal(calls.match(/^gh(?: |$)/gm)?.length, 1)
    if (mode === 'success') {
      const curlCalls = calls.match(/^curl.*$/gm) || []
      assert.equal(curlCalls.length, 3)
      for (const call of curlCalls) {
        assert.match(call, /<--proto> <=https> <--proto-redir> <=https> <--tlsv1\.2>/)
        assert.match(call, /<--fail> <--show-error> <--silent> <--location> <--max-time> <60>/)
      }
      assert.match(curlCalls[0], /<--output> <manifest\.json> <https:\/\/downloads\.glade\.sh\/latest\/release-manifest\.json>$/)
      assert.match(curlCalls[1], /<--output> <glade_v0\.2\.15_linux_amd64\.tar\.gz> <https:\/\/downloads\.glade\.sh\/v0\.2\.15\/glade_v0\.2\.15_linux_amd64\.tar\.gz>$/)
      assert.match(curlCalls[2], /<--output> <SHA256SUMS\.txt> <https:\/\/downloads\.glade\.sh\/v0\.2\.15\/SHA256SUMS\.txt>$/)
      assert.match(calls, /^checksum <-a> <256> <-c> <selected-checksum\.txt>$/m)
      const ghCalls = calls.match(/^gh.*$/gm) || []
      assert.equal(ghCalls.length, 2)
      for (const call of ghCalls) {
        assert.match(call, /^gh <attestation> <verify> <glade_v0\.2\.15_linux_amd64\.tar\.gz> <-R> <glade-sh\/glade>/)
        assert.match(call, /<--signer-workflow> <glade-sh\/glade\/\.github\/workflows\/release\.yml>/)
        assert.match(call, /<--source-ref> <refs\/tags\/v0\.2\.15>/)
      }
      assert.match(ghCalls[1], /<--predicate-type> <https:\/\/cyclonedx\.org\/bom>/)
      assert.match(result.stdout, /Not extracted, installed, or executed/)
    }
  }
})
