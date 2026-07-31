import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from '@playwright/test'

const siteRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const baseURL = new URL(process.argv[2] || 'http://127.0.0.1:4173')
const { routes } = JSON.parse(await readFile(path.join(siteRoot, 'routes.json'), 'utf8'))
const redirectRules = new Map(
  (await readFile(path.join(siteRoot, 'docs-src/public/_redirects'), 'utf8'))
    .trim()
    .split('\n')
    .map((line) => {
      const [source, destination, status] = line.trim().split(/\s+/)
      return [source, { destination, status }]
    })
)

async function get(pathname) {
  const response = await fetch(new URL(pathname, baseURL), { redirect: 'manual' })
  const body = await response.text()
  return { response, body }
}

for (const entry of routes.filter((route) => route.classification === 'nav' || route.classification === 'deep-link')) {
  const { response, body } = await get(entry.route)
  assert.equal(response.status, 200, `${entry.route} should return 200`)
  assert.match(body, new RegExp(`<link[^>]+rel="canonical"[^>]+href="https://glade\\.sh${entry.route.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}"`))
  assert.match(body, /<meta[^>]+name="description"[^>]+content="[^"]{24,}"/)
  assert.match(body, /<meta[^>]+property="og:url"[^>]+content="https:\/\/glade\.sh/)
}

for (const entry of routes.filter((route) => route.classification === 'redirect')) {
  assert.deepEqual(
    redirectRules.get(entry.route),
    { destination: entry.destination, status: '301' },
    `${entry.route} should have one exact permanent redirect rule`
  )
}
assert.equal(redirectRules.size, routes.filter((route) => route.classification === 'redirect').length)

const home = await get('/')
assert.match(home.body, /Run and test Salesforce Apex locally\./)
assert.match(home.body, /Latest stable release:[\s\S]*https:\/\/github\.com\/glade-sh\/glade\/releases\/tag\/v\d+\.\d+\.\d+/)

const install = await get('/install.sh')
assert.equal(install.response.status, 200)
assert.match(install.body, /^#!\/usr\/bin\/env sh/m)

const sitemap = await get('/sitemap.xml')
assert.equal(sitemap.response.status, 200)
assert.match(sitemap.body, /<loc>https:\/\/glade\.sh\/guide\/<\/loc>/)
assert.doesNotMatch(sitemap.body, /\/maintainer\/|\/guide\/cli-reference/)

const absent = await get('/public/help/screenshots/README')
assert.equal(absent.response.status, 404)

const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 390, height: 844 } })
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
for (const route of ['/', '/guide/quickstart', '/reference/cli', '/guide/workbench']) {
  await page.goto(new URL(route, baseURL).toString())
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  assert.ok(overflow <= 1, `${route} has ${overflow}px horizontal overflow`)
}
await browser.close()
assert.deepEqual(browserErrors, [], `browser errors: ${browserErrors.join('\n')}`)
assert.deepEqual(policyViolations, [], `CSP violations: ${policyViolations.join('\n')}`)

console.log(`Preview smoke passed for ${routes.length} classified routes at ${baseURL.origin}.`)
