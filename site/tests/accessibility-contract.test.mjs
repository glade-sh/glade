import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'

const workbench = await readFile(new URL('../.vitepress/theme/GladeEditorWorkbench.vue', import.meta.url), 'utf8')
const page = await readFile(new URL('../docs-src/guide/workbench.md', import.meta.url), 'utf8')
const home = await readFile(new URL('../docs-src/public/js/home.js', import.meta.url), 'utf8')
const config = await readFile(new URL('../.vitepress/config.ts', import.meta.url), 'utf8')
const docsEnhancer = await readFile(new URL('../.vitepress/theme/DocsEnhancer.vue', import.meta.url), 'utf8')

test('the CodeMirror editor has a programmatic name and instructions', () => {
  assert.match(workbench, /EditorView\.contentAttributes\.of\(/)
  assert.match(workbench, /'aria-labelledby': 'apex-editor-heading'/)
  assert.match(workbench, /'aria-describedby': 'apex-editor-instructions'/)
  assert.match(workbench, /id="apex-editor-heading"/)
  assert.match(workbench, /id="apex-editor-instructions"/)
  assert.match(workbench, /editorView\.scrollDOM\.tabIndex = 0/)
  assert.match(workbench, /Apex editor scroll area/)
})

test('the workbench uses complete tab relationships without character shortcuts', () => {
  assert.doesNotMatch(home, /\^\[1-4\]\$/)
  assert.doesNotMatch(home, /e\.key\.toLowerCase\(\) === "r"/)
  assert.doesNotMatch(home, /e\.key\.toLowerCase\(\) === "c"/)
  assert.doesNotMatch(page, /aria-pressed/)
  assert.match(page, /aria-controls="command-output-panel"/)
  assert.match(page, /role="tabpanel" aria-labelledby="output-tab-output" tabindex="0"/)
})

test('search and the user-selected theme remain available on every route', () => {
  assert.match(config, /appearance: true/)
  assert.doesNotMatch(config, /force-dark/)
  assert.doesNotMatch(home, /hideHomeSearch/)
})

test('sidebar disclosure controls have one keyboard target', () => {
  assert.match(docsEnhancer, /repairSidebarDisclosureControls/)
  assert.match(docsEnhancer, /\.item\[role="button"\] \.caret\[role="button"\]/)
  assert.match(docsEnhancer, /removeAttribute\('tabindex'\)/)
})

test('the capability explorer only uses valid CodeMirror stream highlighting tags', async () => {
  const language = await readFile(new URL('../.vitepress/theme/editor/apexLanguage.ts', import.meta.url), 'utf8')
  assert.doesNotMatch(language, /definition\(typeName\)/)
})
