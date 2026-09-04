import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'

async function source(path) {
  return readFile(new URL(path, import.meta.url), 'utf8').catch(() => '')
}

const [
  packageJSON,
  releaseManifestSource,
  home,
  config,
  quickstart,
  workflows,
  help,
  routesSource,
  writeSiteBuild,
  builtSiteCheck,
  postdeploySmoke,
  supportExplorer,
  themeIndex,
  homeCSS,
  themeCSS,
  editorGuide,
  lspReference,
  dapReference,
  editorMaintainer,
  playground
] = await Promise.all([
  source('../package.json'),
  source('../release-manifest.json'),
  source('../docs-src/index.md'),
  source('../.vitepress/config.ts'),
  source('../docs-src/guide/quickstart.md'),
  source('../docs-src/guide/workflows.md'),
  source('../docs-src/help/index.md'),
  source('../routes.json'),
  source('../scripts/write-site-build.mjs'),
  source('../scripts/check-built-site.mjs'),
  source('../scripts/postdeploy-smoke.mjs'),
  source('../.vitepress/theme/GladeSupportExplorer.vue'),
  source('../.vitepress/theme/index.ts'),
  source('../docs-src/public/css/home.css'),
  source('../.vitepress/theme/custom.css'),
  source('../docs-src/guide/editor.md'),
  source('../docs-src/reference/lsp.md'),
  source('../docs-src/reference/dap.md'),
  source('../docs-src/maintainer/editor-extension.md'),
  source('../docs-src/guide/playground.md')
])

test('stable release and deployed commit share one generated site-build contract', () => {
  assert.ok(releaseManifestSource, 'checked stable release manifest should exist')
  const releaseManifest = JSON.parse(releaseManifestSource)
  assert.equal(releaseManifest.channel, 'stable')
  assert.match(releaseManifest.version, /^v\d+\.\d+\.\d+$/)
  assert.ok(releaseManifest.assets.length >= 4)
  assert.match(home, /import releaseManifest from '\.\.\/release-manifest\.json'/)
  assert.doesNotMatch(home, /releases\/tag\/v\d+\.\d+\.\d+|>v\d+\.\d+\.\d+</)
  assert.match(packageJSON, /write-site-build\.mjs/)
  assert.match(writeSiteBuild, /site-build\.json/)
  assert.match(writeSiteBuild, /releaseVersion/)
  assert.match(writeSiteBuild, /CF_PAGES_COMMIT_SHA/)
  assert.match(builtSiteCheck, /site-build\.json/)
})

test('postdeploy smoke reconciles homepage, live manifest, latest release, checksums, and assets', () => {
  for (const contract of [
    /site-build\.json/,
    /releases\/latest/,
    /SHA256SUMS\.txt/,
    /manifestPayload\.assets/,
    /releaseVersion/,
    /primary CTA/i,
    /malformed copy token/i
  ]) assert.match(postdeploySmoke, contract)
})

test('first-run copy establishes project context before project-aware doctor', () => {
  assert.match(home, /Apex feedback without the deploy wait\./)
  assert.match(home, /curl -fsSL https:\/\/glade\.sh\/install\.sh \| sh<br>glade version/)
  assert.doesNotMatch(home, /glade doctor/)
  assert.match(quickstart, /glade doctor --project \./)
  assert.match(quickstart, /## You are done when/)
  assert.match(quickstart, /## Reset or clean up/)
  assert.match(home, /github\.com\/glade-sh\/glade\/blob\/main\/site\/install\.sh/)
})

test('navigation uses jobs, one canonical surface location, and separate recovery', () => {
  for (const label of ['Docs', 'Workflows', 'Reference', 'Support', 'Install']) {
    assert.match(config, new RegExp(`text: '${label}'`))
  }
  assert.match(config, /\{ text: 'Support', link: '\/help\/' \}/)
  assert.match(config, /\{ text: 'Documentation home', link: '\/guide\/' \}/)
  assert.match(themeCSS, /VPNavBarMenuLink\[href="\/guide\/installation"\]/)
  for (const link of ['/help/anonymous-apex-scratch', '/guide/plugins', '/guide/editor']) {
    const guideNavigation = config.slice(0, config.indexOf("text: 'Task guides'"))
    assert.equal(guideNavigation.split(`link: '${link}'`).length - 1, 1, `${link} should have one guide-navigation location`)
  }
  assert.match(config, /text: 'Advanced',[\s\S]*collapsed: true/)
  assert.match(config, /text: 'Task guides'/)
  assert.match(config, /text: 'Troubleshooting'/)
  assert.match(routesSource, /"route": "\/help\/troubleshooting"/)
  assert.match(help, /Complete a task/)
  assert.match(help, /Fix a problem/)
  assert.match(help, /What Glade runs locally/)
  assert.match(help, /Security & trust/)
  assert.doesNotMatch(workflows, /Product areas/)
})

test('homepage shows one real product view and job-oriented workflow choices', () => {
  assert.match(home, /run-one-apex-test-02-codelens\.png/)
  assert.match(home, /alt="[^"]*(?:Apex test|CodeLens)[^"]*"/i)
  for (const job of ['Run Apex tests', 'Debug or execute Apex', 'Work with local data', 'Add Glade to CI']) {
    assert.match(home, new RegExp(job))
  }
  assert.match(home, /ResetPasswordResult\.getPassword[\s\S]*Requires Salesforce/)
  assert.doesNotMatch(home, /Answers\.findSimilar[\s\S]*Requires Salesforce/)
  assert.match(workflows, /href="\/guide\/workflows\/debug-apex"><strong>Debug Apex<\/strong>/)
  assert.match(workflows, /href="\/help\/anonymous-apex-scratch"><strong>Execute Apex and SOQL<\/strong>/)
  assert.match(workflows, /glade exec --project \. "System\.debug\('local'\);"/)
  assert.equal((home.match(/Salesforce remains the final validation gate\./g) || []).length, 1)
  assert.match(home, /Runs locally with limits/)
})

test('support explorer is generated, searchable, status-filterable, and announced', () => {
  assert.match(themeIndex, /GladeSupportExplorer/)
  assert.match(supportExplorer, /editorSupportCatalog/)
  assert.match(supportExplorer, /editorSupportCatalog\.summary/)
  assert.match(supportExplorer, /type="search"/)
  assert.match(supportExplorer, /aria-live="polite"/)
  assert.match(supportExplorer, /Runs locally with limits/)
  assert.match(supportExplorer, /Requires Salesforce/)
})

test('editor user workflow is separate from protocol and extension development reference', () => {
  assert.match(editorGuide, /^# Use Glade in VS Code/m)
  assert.match(editorGuide, /run-one-apex-test-03-test-explorer\.png/)
  assert.doesNotMatch(editorGuide, /npm --prefix contrib\/vscode-glade/)
  assert.match(lspReference, /^# LSP reference/m)
  assert.match(lspReference, /glade lsp --project \./)
  assert.match(dapReference, /^# DAP reference/m)
  assert.match(dapReference, /glade dap --project/)
  assert.match(editorMaintainer, /^# Develop and package the VS Code extension/m)
  assert.match(editorMaintainer, /npm --prefix contrib\/vscode-glade run package/)
  for (const route of ['/reference/lsp', '/reference/dap', '/maintainer/editor-extension']) {
    assert.match(routesSource, new RegExp(`"route": "${route}"`))
  }
})

test('playground starts with a real product view and one short scenario', () => {
  assert.match(playground, /playground-overview\.png/)
  assert.match(playground, /## Read the surface/)
  assert.match(playground, /## Two-minute first scenario/)
  assert.match(playground, /Status shows `pass`/)
  assert.match(playground, /reset-on-start/)
})

test('built content and narrow layouts guard copy integrity and reflow', () => {
  assert.match(builtSiteCheck, /malformed angle-bracket placeholder/)
  assert.match(builtSiteCheck, /duplicate id/)
  assert.match(homeCSS, /@media \(max-width: 360px\)/)
  assert.match(homeCSS, /home-loop-metrics[\s\S]*grid-template-columns: repeat\(2, minmax\(0, 1fr\)\)/)
  assert.match(themeCSS, /support-explorer/)
})
