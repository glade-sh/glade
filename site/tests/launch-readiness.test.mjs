import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'

const read = (path) => readFile(new URL(`../docs-src/${path}`, import.meta.url), 'utf8')

test('quickstart offers a real sample test and a focused existing-project route', async () => {
  const page = await read('guide/quickstart.md')
  assert.match(page, /Try the sample/)
  assert.match(page, /Use my Salesforce DX project/)
  assert.match(page, /--example refinement-service/)
  assert.match(page, /--class RefinementServiceTest/)
  assert.match(page, /zero tests/i)
  assert.match(page, /65\.0.*66\.0.*67\.0/)
  assert.match(page, /export PATH="\$HOME\/\.local\/bin:\$PATH"/)
  assert.match(page, /github\.com\/glade-sh\/glade\/issues/)
  assert.doesNotMatch(page, /\[Execute Apex and SOQL\]\(\/guide\/workbench#exec\)/)
})

test('packaged-user setup and endpoint examples match their documented contracts', async () => {
  for (const path of ['guide/workflows/lwc-preview.md', 'guide/modules.md']) {
    const page = await read(path)
    assert.match(page, /glade toolchain status/)
    assert.doesNotMatch(page, /glade toolchain install/)
  }
  for (const path of ['guide/workflows/visualforce-preview.md', 'guide/modules.md']) {
    const page = await read(path)
    assert.match(page, /services\/data\/v65\.0\/glade\/visualforce\/support/)
    assert.doesNotMatch(page, /services\/data\/v61\.0\/glade\/visualforce\/support/)
  }
})

test('editor setup verifies the installed extension without requiring a screenshot theme', async () => {
  const editor = await read('guide/editor.md')
  assert.match(editor, /--list-extensions --show-versions/)
  assert.doesNotMatch(editor, /doctor command reports the selected editor and installed Glade/)
  for (const path of ['help/run-one-apex-test.md', 'help/anonymous-apex-scratch.md', 'help/local-data-environments.md']) {
    assert.doesNotMatch(await read(path), /- .*Catppuccin|^- .*clean.*profile/gim)
  }
})

test('pilot links to pinned advisory CI and preserves safe feedback boundaries', async () => {
  const pilot = await read('guide/tester-field-guide.md')
  const ci = await read('guide/ci-artifacts.md')
  assert.match(pilot, /\/guide\/ci-artifacts#advisory-pilot/)
  assert.match(ci, /GLADE_VERSION=v\d+\.\d+\.\d+/)
  assert.match(ci, /continue-on-error: true/)
  assert.match(ci, /if: always\(\)/)
  assert.match(pilot, /private package names/i)
})

test('AI guidance keeps valid Salesforce behavior and sandbox boundaries explicit', async () => {
  const page = await read('guide/ai-assisted-apex.md')
  assert.match(page, /Do not rewrite valid Salesforce behavior/)
  assert.match(page, /not an OS (?:security )?sandbox/)
})
