import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from '@playwright/test'
import { readJSONResponse } from './smoke-utils.mjs'

const args = new Map()
for (let index = 2; index < process.argv.length; index += 2) {
  args.set(process.argv[index], process.argv[index + 1])
}
const baseURLValue = args.get('--base-url')
const expectedCommit = args.get('--expected-commit')
if (!baseURLValue || !expectedCommit) {
  console.error('usage: node site/scripts/postdeploy-smoke.mjs --base-url https://glade.sh --expected-commit <sha>')
  process.exit(2)
}

const baseURL = new URL(baseURLValue)
const siteRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const { routes } = JSON.parse(await readFile(path.join(siteRoot, 'routes.json'), 'utf8'))
const redirectRules = (await readFile(path.join(siteRoot, 'docs-src/public/_redirects'), 'utf8'))
  .trim()
  .split('\n')
  .map((line) => line.trim().split(/\s+/))

async function get(pathname, options = {}) {
  return fetch(new URL(pathname, baseURL), options)
}

for (const entry of routes.filter((route) => ['nav', 'deep-link', 'noindex'].includes(route.classification))) {
  const response = await get(entry.route)
  assert.equal(response.status, 200, `${entry.route} should return 200`)
  const body = await response.text()
  assert.match(body, new RegExp(`<link[^>]+rel="canonical"[^>]+href="https://glade\\.sh${entry.route.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}"`))
  assert.doesNotMatch(body, /&lt;[a-z0-9-]+ [a-z0-9-]+&gt;/i, `${entry.route} contains a malformed copy token`)
  if (entry.classification === 'noindex') {
    assert.match(body, /<meta[^>]+name="robots"[^>]+content="noindex"/, `${entry.route} should be noindex`)
  }
}
for (const [source, destination] of redirectRules) {
  const response = await get(source, { redirect: 'manual' })
  assert.ok([301, 302, 307, 308].includes(response.status), `${source} should redirect`)
  assert.equal(new URL(response.headers.get('location'), baseURL).pathname + new URL(response.headers.get('location'), baseURL).hash, destination)
}

const homeResponse = await get('/')
assert.match(homeResponse.headers.get('cache-control') || '', /max-age=0|no-cache/)
assert.equal(homeResponse.headers.get('x-frame-options'), 'DENY')
assert.match(homeResponse.headers.get('permissions-policy') || '', /camera=\(\)/)
const home = await homeResponse.text()
const commit = home.match(/<meta[^>]+name="glade:commit"[^>]+content="([^"]+)"/)?.[1]
assert.equal(commit, expectedCommit, 'deployed commit marker does not match the requested commit')
assert.match(home, /class="home-cta primary"[^>]+href="\/guide\/quickstart"/, 'homepage primary CTA should start the first local check')

const assetPath = home.match(/(?:src|href)="(\/assets\/[^"]+\.(?:js|css))"/)?.[1]
assert.ok(assetPath, 'homepage should reference a fingerprinted asset')
const assetResponse = await get(assetPath)
assert.match(assetResponse.headers.get('cache-control') || '', /max-age=31536000/)
assert.match(assetResponse.headers.get('cache-control') || '', /immutable/)
assert.doesNotMatch(assetResponse.headers.get('cache-control') || '', /must-revalidate/)

const install = await get('/install.sh')
assert.equal(install.status, 200)
assert.match(install.headers.get('cache-control') || '', /max-age=0|no-cache/)
assert.match(install.headers.get('content-type') || '', /(?:text\/(?:plain|x-shellscript)|application\/(?:x-sh|x-shellscript))/i)
assert.match(await install.text(), /^#!\/usr\/bin\/env sh/)

const [siteBuildResponse, manifest, latestRelease, registry, sitemap] = await Promise.all([
  get('/site-build.json'),
  fetch('https://downloads.glade.sh/latest/release-manifest.json'),
  fetch('https://github.com/glade-sh/glade/releases/latest'),
  fetch('https://plugins.glade.sh/index.json'),
  get('/sitemap.xml')
])
assert.equal(siteBuildResponse.status, 200, 'site-build.json should be live')
assert.equal(manifest.status, 200, 'release manifest should be live')
assert.equal(latestRelease.status, 200, 'GitHub latest release should be live')
assert.equal(registry.status, 200, 'plugin registry should be live')
assert.equal(sitemap.status, 200, 'sitemap should be live')
const manifestPayload = await readJSONResponse(manifest, 'release manifest')
const siteBuild = await readJSONResponse(siteBuildResponse, 'site build')
const registryPayload = await readJSONResponse(registry, 'plugin registry')
assert.match(manifestPayload.version || '', /^v\d+\.\d+\.\d+$/, 'release manifest should name a stable version')
assert.equal(siteBuild.siteCommit, expectedCommit, 'site-build.json commit differs from the intended deployment')
assert.equal(siteBuild.releaseVersion, manifestPayload.version, 'site build release differs from the live manifest')
assert.ok(latestRelease.url.endsWith(`/releases/tag/${manifestPayload.version}`), 'GitHub latest release differs from the live manifest')
assert.match(home, new RegExp(`/releases/tag/${manifestPayload.version}`), 'homepage release link differs from the live manifest')
assert.ok(registryPayload && typeof registryPayload === 'object', 'plugin registry should contain a JSON object')

const checksums = await fetch(`https://downloads.glade.sh/${manifestPayload.version}/SHA256SUMS.txt`)
assert.equal(checksums.status, 200, 'SHA256SUMS.txt should be live')
const checksumText = await checksums.text()
for (const asset of manifestPayload.assets) {
  assert.match(checksumText, new RegExp(new URL(asset.url).pathname.split('/').pop().replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `${asset.url} should be checksummed`)
  const response = await fetch(asset.url, { method: 'HEAD' })
  assert.equal(response.status, 200, `${asset.url} should be live`)
}

const browser = await chromium.launch()
const page = await browser.newPage()
const browserErrors = []
const policyViolations = []
page.on('console', (message) => {
  if (message.type() === 'error') browserErrors.push(message.text())
})
page.on('pageerror', (error) => browserErrors.push(error.message))
await page.exposeFunction('recordPolicyViolation', (value) => policyViolations.push(value))
await page.addInitScript(() => {
  document.addEventListener('securitypolicyviolation', (event) => {
    window.recordPolicyViolation(`${event.violatedDirective}: ${event.blockedURI}`)
  })
})
for (const route of ['/', '/guide/', '/guide/quickstart', '/reference/cli', '/guide/workbench']) {
  await page.goto(new URL(route, baseURL).toString())
}
await page.goto(baseURL.toString())
await page.getByRole('button', { name: /search/i }).first().click()
await page.getByRole('searchbox').waitFor()
await browser.close()
assert.deepEqual(browserErrors, [], `browser errors: ${browserErrors.join('\n')}`)
assert.deepEqual(policyViolations, [], `CSP violations: ${policyViolations.join('\n')}`)

console.log(`Postdeploy smoke passed for ${expectedCommit} at ${baseURL.origin}.`)
