import { expect, test, type Page } from '@playwright/test'
import { readFileSync } from 'node:fs'
import path from 'node:path'

// The preview server does not apply _headers, so read the policy from the built
// file and add it to every document response. A violation then fails the test
// the way it would block the page in production.
const headersFile = path.join(process.cwd(), '.vitepress', 'dist', '_headers')
const policy = readFileSync(headersFile, 'utf8').match(/^\s+Content-Security-Policy:\s*(.+)$/m)?.[1].trim() || ''

const routes = [
  '/',
  '/guide/quickstart',
  '/guide/support-map',
  '/guide/workbench',
  '/guide/editor',
  '/reference/cli',
  '/help/',
  '/private-corpus-assurance.html',
  '/404.html'
]

async function enforcePolicy(page: Page) {
  const violations: string[] = []
  const errors: string[] = []
  await page.exposeFunction('__reportCSP', (detail: string) => violations.push(detail))
  await page.addInitScript(() => {
    document.addEventListener('securitypolicyviolation', (event) => {
      void (window as any).__reportCSP(`${event.violatedDirective} blocked ${event.blockedURI || 'inline'} (${event.sourceFile || 'page'}:${event.lineNumber})`)
    })
  })
  await page.route('**/*', async (route) => {
    if (route.request().resourceType() !== 'document') return route.continue()
    const response = await route.fetch()
    await route.fulfill({ response, headers: { ...response.headers(), 'content-security-policy': policy } })
  })
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(message.text())
  })
  page.on('pageerror', (error) => errors.push(error.message))
  return { violations, errors }
}

test('the built _headers file carries a strict policy', () => {
  expect(policy).toContain("default-src 'self'")
  expect(policy).toContain("frame-ancestors 'none'")
  expect(policy).not.toMatch(/script-src[^;]*'unsafe-inline'/)
})

for (const colorScheme of ['light', 'dark'] as const) {
  test.describe(`${colorScheme} appearance`, () => {
    test.use({ colorScheme })

    for (const route of routes) {
      test(`${route} loads with no policy violations`, async ({ page }) => {
        const seen = await enforcePolicy(page)
        await page.goto(route)
        await page.waitForLoadState('networkidle')
        await expect(page.locator('body')).toBeVisible()
        expect(seen.violations).toEqual([])
        expect(seen.errors).toEqual([])
      })
    }

    test('home demo tabs and local search run under the policy', async ({ page }) => {
      const seen = await enforcePolicy(page)
      await page.goto('/')
      await page.waitForLoadState('networkidle')
      for (const scenario of ['tests', 'debug', 'check']) {
        await page.locator(`[data-demo="${scenario}"]`).click()
        await expect(page.locator(`[data-demo="${scenario}"]`)).toHaveAttribute('aria-selected', 'true')
      }
      await page.goto('/guide/quickstart')
      await page.locator('.VPNavBarSearch button').first().click()
      const input = page.locator('#localsearch-input')
      await input.fill('shard-count')
      await expect(page.locator('#localsearch-list')).toContainText(/test|CLI/i)
      await input.press('Escape')
      expect(seen.violations).toEqual([])
      expect(seen.errors).toEqual([])
    })
  })
}

test('the policy is active: an injected inline script is blocked', async ({ page }) => {
  const seen = await enforcePolicy(page)
  await page.goto('/guide/quickstart')
  const ran = await page.evaluate(() => {
    const script = document.createElement('script')
    script.textContent = 'window.__injected = true'
    document.head.append(script)
    return Boolean((window as any).__injected)
  })
  expect(ran).toBe(false)
  await expect.poll(() => seen.violations.length).toBeGreaterThan(0)
})
