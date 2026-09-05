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
  source('../.vitepress/theme/home/GladeHome.vue'),
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
  assert.match(home, /import releaseManifest from '(?:\.\.\/)+release-manifest\.json'/)
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
  assert.match(home, /Run Apex locally/)
  assert.match(home, /INSTALL_COMMAND/)
  assert.match(home, /Your Salesforce DX project/)
  assert.match(quickstart, /glade doctor --project \./)
  assert.match(quickstart, /## You are done when/)
  assert.match(quickstart, /## Reset or clean up/)
  assert.match(home, /github\.com\/glade-sh\/glade\/blob\/main\/site\/install\.sh/)
})

test('navigation uses jobs, one canonical surface location, and separate recovery', () => {
  for (const label of ['Docs', 'Guides', 'Reference', 'Help', 'Install']) {
    assert.match(config, new RegExp(`text: '${label}'`))
  }
  assert.match(config, /\{ text: 'Help', link: '\/help\/', activeMatch: '[^']+' \}/)
  assert.match(config, /\{ text: 'Documentation home', link: '\/guide\/' \}/)
  assert.match(themeCSS, /VPNavBarMenuLink\[href="\/guide\/installation"\]/)
  for (const link of ['/guide/playground', '/guide/plugins', '/guide/editor']) {
    assert.equal(config.split(`link: '${link}'`).length - 1, 1, `${link} should have one canonical sidebar location`)
  }
  assert.match(config, /text: 'Advanced',[\s\S]*collapsed: true/)
  assert.match(config, /text: 'Illustrated walkthroughs'/)
  assert.match(config, /text: 'Fix a problem'/)
  assert.match(routesSource, /"route": "\/help\/troubleshooting"/)
  assert.match(help, /Complete a task/)
  assert.match(help, /Fix a problem/)
  assert.match(help, /What Glade runs locally/)
  assert.match(help, /Security & trust/)
  assert.doesNotMatch(workflows, /Product areas/)
})

test('homepage labels its simulated product view and preserves task destinations', () => {
  assert.match(home, /Interactive preview · simulated output/);
  for (const scenario of ['tests', 'debug', 'check']) assert.ok(home.includes(scenario));
  assert.match(home, /Salesforce/);
  assert.match(home, /onUnmounted/);
  assert.match(workflows, /href="\/guide\/workflows\/debug-apex"><strong>Debug Apex<\/strong>/);
  assert.match(workflows, /href="\/guide\/playground"><strong>Execute Apex and SOQL<\/strong>/);
});

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
