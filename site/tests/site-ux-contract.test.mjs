import assert from 'node:assert/strict'
import { access, readFile } from 'node:fs/promises'
import { test } from 'node:test'

const config = await readFile(new URL('../.vitepress/config.ts', import.meta.url), 'utf8')
const routes = JSON.parse(await readFile(new URL('../routes.json', import.meta.url), 'utf8'))
const home = await readFile(new URL('../docs-src/index.md', import.meta.url), 'utf8')
const guide = await readFile(new URL('../docs-src/guide/index.md', import.meta.url), 'utf8')
const redirects = await readFile(new URL('../docs-src/public/_redirects', import.meta.url), 'utf8')
const ciWorkflow = await readFile(new URL('../../.github/workflows/ci.yml', import.meta.url), 'utf8')
const browserTests = await readFile(new URL('./browser/site.spec.ts', import.meta.url), 'utf8')
const performanceTests = await readFile(new URL('./browser/performance.spec.ts', import.meta.url), 'utf8')
const builtSiteCheck = await readFile(new URL('../scripts/check-built-site.mjs', import.meta.url), 'utf8')
const performanceBaseline = JSON.parse(await readFile(new URL('./performance-baseline.json', import.meta.url), 'utf8'))

test('public IA has a guide landing, five-destination navigation, and scoped sidebars', () => {
  assert.match(guide, /^# Glade documentation/m)
  for (const label of ['Docs', 'Install', 'Workflows', 'Reference', 'Support']) assert.match(config, new RegExp(`text: '${label}'`))
  assert.match(config, /\{ text: 'Support', link: '\/help\/' \}/)
  assert.match(config, /\{ text: 'Documentation home', link: '\/guide\/' \}/)
  for (const prefix of ['/guide/', '/reference/', '/help/', '/maintainer/']) assert.match(config, new RegExp(`'${prefix}': \\[`))
  assert.doesNotMatch(config, /text: 'Product areas'/)
  assert.match(redirects, /\/guide\/cli-reference \/reference\/cli 301/)
})

test('homepage has the four-band local-first message and release trust links', () => {
  assert.match(home, /Apex feedback without the deploy wait\./)
  assert.match(home, /Move from project to proof in three steps\./)
  assert.match(home, /Run local Apex tests from VS Code\./)
  assert.match(home, /Know the boundary before you rely on a result\./)
  assert.match(home, /Checksums, SBOM, and attestations/)
  assert.doesNotMatch(home, /home-data-section|home-plugin-section/)
})

test('homepage trust links target a checked built fragment', () => {
  assert.match(home, /\/guide\/security-trust#release-proof/)
  assert.match(builtSiteCheck, /missing local fragment/)
  assert.match(builtSiteCheck, /decodeURIComponent/)
})

test('routes classify every published page and metadata is generated per route', () => {
  assert.ok(routes.routes.some((entry) => entry.route === '/guide/' && entry.classification === 'nav'))
  assert.ok(routes.routes.every((entry) => ['nav', 'deep-link', 'redirect', 'noindex'].includes(entry.classification)))
  assert.match(config, /sitemap:/)
  assert.match(config, /transformHead\(ctx\)/)
  assert.match(config, /rel: 'canonical'/)
  assert.match(config, /property: 'og:url'/)
})

test('homepage metadata names the local Apex runtime without losing its social card', () => {
  assert.match(home, /^title: Glade — Local Apex Runtime for Salesforce Developers$/m)
  assert.match(config, /route === '\/' \? 'Glade — Local Apex Runtime for Salesforce Developers'/)
  assert.match(config, /property: 'og:image'/)
  assert.match(config, /social-card\.png/)
})

test('legacy docs routes redirect to the guide without Markdown duplicates', () => {
  for (const route of ['/docs', '/docs/']) {
    const entry = routes.routes.find((candidate) => candidate.route === route)
    assert.deepEqual(entry, { route, classification: 'redirect', destination: '/guide/' })
  }
  assert.match(redirects, /^\/docs \/guide\/ 301$/m)
  assert.match(redirects, /^\/docs\/ \/guide\/ 301$/m)
})

test('redirect-only routes have no duplicate Markdown source', async () => {
  const redirectEntries = routes.routes.filter((entry) => entry.classification === 'redirect')
  assert.ok(redirectEntries.length > 0)
  for (const entry of redirectEntries) {
    assert.equal(entry.source, undefined, `${entry.route} should not retain a searchable source`)
    assert.equal(typeof entry.destination, 'string', `${entry.route} should declare its destination`)
    await assert.rejects(access(new URL(`../docs-src${entry.route}.md`, import.meta.url)))
  }
})

test('CI enforces the rendered site gates', () => {
  assert.match(ciWorkflow, /playwright install (?:--with-deps )?chromium/)
  assert.match(ciWorkflow, /npm run check:built --prefix site/)
  assert.match(ciWorkflow, /npm run test:browser --prefix site/)
  assert.match(ciWorkflow, /npm run smoke:preview --prefix site/)
})

test('browser projects do not report conditional no-op cases as passes', () => {
  assert.doesNotMatch(browserTests, /if \(!testInfo\.project\.name\.startsWith\('mobile'\)\) return/)
  assert.doesNotMatch(performanceTests, /if \(testInfo\.project\.name !== 'desktop-1440'\) return/)
})

test('performance policy gates timing only when the runner is calibrated', () => {
  assert.equal(typeof performanceBaseline.runner.platform, 'string')
  assert.match(performanceTests, /performancePolicy\(baseline\.runner, process\.platform\)/)
  assert.match(performanceTests, /if \(policy\.enforceTimingBudgets\)/)
  assert.doesNotMatch(performanceTests, /stored\.tbtMs \* 1\.15/)
  assert.match(performanceTests, /TBT budget/)
})
