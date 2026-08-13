import AxeBuilder from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'

const representativeRoutes = [
  '/',
  '/guide/',
  '/guide/installation',
  '/guide/quickstart',
  '/guide/support-map',
  '/reference/cli',
  '/guide/workbench',
  '/help/'
]

function observeBrowserErrors(page: Page) {
  const errors: string[] = []
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(message.text())
  })
  page.on('pageerror', (error) => errors.push(error.message))
  return errors
}

async function expectNoHorizontalOverflow(page: Page) {
  const dimensions = await page.evaluate(() => ({
    viewport: document.documentElement.clientWidth,
    page: document.documentElement.scrollWidth
  }))
  expect(dimensions.page).toBeLessThanOrEqual(dimensions.viewport + 1)
}

test('homepage keeps search, CTAs, exact copy, and the local boundary available', async ({ page }) => {
  const errors = observeBrowserErrors(page)
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Apex feedback without the deploy wait.' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Install Glade' })).toBeVisible()
  await expect(page.locator('.VPNavBarSearch')).toBeVisible()

  const install = page.locator('#install-cmd')
  await expect(install).toHaveAttribute(
    'data-copy-text',
    'curl -fsSL https://glade.sh/install.sh | sh\nglade version'
  )
  await page.getByRole('button', { name: 'Copy install command' }).click()
  await expect(page.getByRole('status')).toContainText('copied')
  await expect(page.getByText('Requires Salesforce', { exact: true })).toBeVisible()
  await expectNoHorizontalOverflow(page)
  expect(errors).toEqual([])
})

test('quickstart supports direct navigation and code copy', async ({ page }) => {
  const errors = observeBrowserErrors(page)
  await page.goto('/guide/')
  await page.getByRole('link', { name: 'Start the first local check' }).click()
  await expect(page).toHaveURL(/\/guide\/quickstart/)
  await expect(page.getByRole('heading', { name: 'Run your first local Apex check' })).toBeVisible()
  const copyButton = page.locator('.vp-doc button.copy').first()
  await expect(copyButton).toBeVisible()
  await copyButton.click()
  await expect(copyButton).toHaveClass(/copied/)
  await expectNoHorizontalOverflow(page)
  expect(errors).toEqual([])
})

test('CLI filter reports results and remains keyboard reachable', async ({ page }) => {
  const errors = observeBrowserErrors(page)
  await page.goto('/reference/cli')
  const filter = page.getByRole('searchbox', { name: 'Filter commands' })
  await filter.focus()
  await expect(filter).toBeFocused()
  await filter.fill('doctor')
  await expect(page.locator('[data-command-filter-status]')).toContainText(/command group.*match/i)
  await expect(page.locator('.docs-command-card:not([hidden])')).toHaveCount(1)
  await expectNoHorizontalOverflow(page)
  expect(errors).toEqual([])
})

test('workbench tabs, output, hashes, run, copy, and editor names remain operable', async ({ page }) => {
  const errors = observeBrowserErrors(page)
  await page.goto('/guide/workbench#check')
  await expect(page.getByRole('textbox', { name: 'Try capability-backed autocomplete.' })).toBeVisible()

  const scenarioTabs = page.getByRole('tab', {
    name: /Catch deploy issues|Run focused tests|Execute Apex locally|Profile debug log/
  })
  await expect(scenarioTabs).toHaveCount(4)
  const checkTab = page.getByRole('tab', { name: /Catch deploy issues/ })
  await expect(checkTab).toHaveAttribute('aria-selected', 'true')
  await checkTab.press('ArrowRight')
  const testTab = page.getByRole('tab', { name: /Run focused tests/ })
  await expect(testTab).toHaveAttribute('aria-selected', 'true')
  await expect(page).toHaveURL(/#test$/)

  await page.getByRole('tab', { name: 'JSON', exact: true }).click()
  await expect(page.locator('[data-command-output]')).toContainText('"status"')
  const runButton = page.locator('[data-run-scenario]')
  await runButton.click()
  await expect(runButton).toBeEnabled()
  await page.getByRole('button', { name: 'Copy workbench JSON command' }).click()
  await expect(page.locator('[data-copy-status]')).toContainText('copied')
  await expectNoHorizontalOverflow(page)
  expect(errors).toEqual([])
})

test('SPA navigation returns home without losing route-scoped behavior', async ({ page }) => {
  const errors = observeBrowserErrors(page)
  await page.goto('/')
  await expect(page.locator('link[href="/css/home.css"]')).toHaveCount(1)
  const mobileMenu = page.getByRole('button', { name: /mobile navigation/i })
  if (await mobileMenu.isVisible()) await mobileMenu.click()
  await page.getByRole('link', { name: 'Docs', exact: true }).click()
  await expect(page).toHaveURL(/\/guide\/$/)
  await expect(page.locator('link[href="/css/home.css"]')).toHaveCount(0)
  await page.locator('.VPNavBarTitle a').click()
  await expect(page).toHaveURL(/\/$/)
  await expect(page.locator('link[href="/css/home.css"]')).toHaveCount(1)
  await expect(page.getByRole('button', { name: 'Copy install command' })).toBeVisible()
  expect(errors).toEqual([])
})

test('representative routes have no serious or critical axe violations', async ({ page }) => {
  const errors = observeBrowserErrors(page)
  for (const theme of ['dark', 'light']) {
    await page.goto('/')
    await page.evaluate((appearance) => {
      localStorage.setItem('vitepress-theme-appearance', appearance)
    }, theme)
    for (const route of representativeRoutes) {
      await page.goto(route)
      const results = await new AxeBuilder({ page })
        .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
        .analyze()
      const highImpact = results.violations.filter(
        (violation) => violation.impact === 'serious' || violation.impact === 'critical'
      )
      expect(highImpact, `${theme} ${route}: ${highImpact.map((item) => item.id).join(', ')}`).toEqual([])
      await expectNoHorizontalOverflow(page)
    }
  }
  expect(errors).toEqual([])
})

test('ordinary docs do not load scenario JavaScript', async ({ page }) => {
  await page.goto('/guide/quickstart')
  const scripts = await page.evaluate(() =>
    performance
      .getEntriesByType('resource')
      .map((entry) => entry.name)
      .filter((name) => name.endsWith('.js'))
  )
  expect(scripts.some((name) => name.endsWith('/js/home.js'))).toBe(false)
})

test('support rows can be searched and filtered with announced results', async ({ page }) => {
  const errors = observeBrowserErrors(page)
  await page.goto('/guide/support-map')
  const search = page.getByRole('searchbox', { name: 'Search APIs' })
  await search.fill('Answers.findSimilar')
  await expect(page.getByRole('status')).toContainText('1 checked row')
  await expect(page.locator('.support-explorer-list')).toContainText('Answers.findSimilar')
  await page.getByRole('combobox', { name: 'Status' }).selectOption('supported')
  await expect(page.getByRole('status')).toContainText('0 checked rows')
  await expectNoHorizontalOverflow(page)
  expect(errors).toEqual([])
})

test('light and forced-color modes preserve readable content', async ({ page }) => {
  await page.goto('/')
  await page.evaluate(() => {
    localStorage.setItem('vitepress-theme-appearance', 'light')
  })
  await page.reload()
  const lightBackground = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--bg').trim())
  expect(lightBackground).toBe('#f7faf7')

  await page.emulateMedia({ forcedColors: 'active' })
  await expect(page.getByRole('heading', { name: 'Apex feedback without the deploy wait.' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Install Glade' })).toBeVisible()
})
