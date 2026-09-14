import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { chmod, mkdtemp, mkdir, readFile, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { delimiter, join } from 'node:path'
import { test } from 'node:test'

const troubleshooting = await readFile(new URL('../docs-src/help/troubleshooting.md', import.meta.url), 'utf8')

function commandBlock(heading) {
  const section = troubleshooting.slice(troubleshooting.indexOf(heading))
  const match = section.match(/```bash\n([\s\S]*?)```/)
  assert.ok(match, `${heading} should contain a shell block`)
  return match[1]
}

const projectRecovery = commandBlock('## Glade cannot find my project')
const testRecovery = commandBlock('## A test is not discovered')

async function runRecovery({ script, project = false, config = false, input = '', failInit = false }) {
  const root = await mkdtemp(join(tmpdir(), 'glade-recovery-test-'))
  const bin = join(root, 'bin')
  const log = join(root, 'commands.log')
  await mkdir(bin)
  const fake = join(bin, 'glade')
  await writeFile(fake, `#!/bin/sh
command=$1
{
  printf '%s' "$1"
  shift
  for arg in "$@"; do printf ' <%s>' "$arg"; done
  printf '\n'
} >>"$MOCK_LOG"
test "$command" = init && test "$MOCK_FAIL_INIT" = 1 && exit 1
exit 0
`)
  await chmod(fake, 0o755)
  if (project) await writeFile(join(root, 'sfdx-project.json'), '{}\n')
  if (config) await writeFile(join(root, 'glade.yml'), '# synthetic config\n')

  const result = spawnSync('/bin/sh', ['-c', script], {
    cwd: root,
    encoding: 'utf8',
    input,
    env: {
      ...process.env,
      PATH: `${bin}${delimiter}${process.env.PATH}`,
      MOCK_FAIL_INIT: failInit ? '1' : '0',
      MOCK_LOG: log
    }
  })
  const calls = await readFile(log, 'utf8').then((value) => value.trim().split('\n').filter(Boolean)).catch(() => [])
  return { result, calls }
}

test('project recovery cannot initialize outside a Salesforce DX project', async () => {
  const outside = await runRecovery({ script: projectRecovery })
  assert.notEqual(outside.result.status, 0)
  assert.deepEqual(outside.calls, [])

  const initialize = await runRecovery({ script: projectRecovery, project: true })
  assert.equal(initialize.result.status, 0)
  assert.deepEqual(initialize.calls, ['init <--project> <.> <--yes>', 'doctor <--project> <.>'])

  const existing = await runRecovery({ script: projectRecovery, project: true, config: true })
  assert.equal(existing.result.status, 0)
  assert.deepEqual(existing.calls, ['doctor <--project> <.>'])

  const failed = await runRecovery({ script: projectRecovery, project: true, failInit: true })
  assert.notEqual(failed.result.status, 0)
  assert.deepEqual(failed.calls, ['init <--project> <.> <--yes>'])
})

test('test recovery rejects placeholder input and executes one valid selector', async () => {
  for (const input of ['\n', '<YourTestClass>\n', 'Class Name\n']) {
    const invalid = await runRecovery({ script: testRecovery, project: true, config: true, input })
    assert.notEqual(invalid.result.status, 0)
    assert.deepEqual(invalid.calls, [])
  }

  const valid = await runRecovery({ script: testRecovery, project: true, config: true, input: 'SampleTest\n' })
  assert.equal(valid.result.status, 0)
  assert.deepEqual(valid.calls, ['test <--project> <.> <--class> <SampleTest>'])
})
