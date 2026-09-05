import { expect, test } from '@playwright/test'

test('local search returns real results, handles empty searches, and restores focus', async ({ page }) => {
  await page.goto('/guide/quickstart')
  const searchButton = page.locator('.VPNavBarSearch button').first()
  await searchButton.click()
  const input = page.locator('#localsearch-input')
  await input.fill('shard-count')
  await expect(page.locator('#localsearch-list')).toContainText(/test|CLI/i)
  await input.fill('/guide/')
  await input.fill('zzzzqxjjvvkkqqzzzz')
  await expect(page.locator('.VPLocalSearchBox')).toContainText(/No results/i)
  await input.press('Escape')
  await expect(input).toHaveCount(0)
  await expect(searchButton).toBeFocused()
})

test('reading theme survives repeated home, docs, workbench, and history navigation', async ({ page }) => {
  await page.goto('/guide/quickstart')
  await page.evaluate(() => localStorage.setItem('vitepress-theme-appearance', 'light'))
  await page.reload()
  for (let round = 0; round < 2; round++) {
    await page.locator('.VPNavBarTitle a').click()
    await expect(page.locator('.glade-home')).toHaveCSS('background-color', 'rgb(11, 17, 25)')
    await page.locator('.site-footer a[href="/guide/"]').click()
    await expect(page.locator('html')).not.toHaveClass(/dark/)
    await page.goto('/guide/workbench#test')
    await expect(page.locator('[data-scenario-id="test"]')).toHaveAttribute('aria-selected', 'true')
    await page.goto('/guide/support-map?q=Answers.findSimilar')
    await expect(page.locator('.support-explorer-list')).toContainText('Answers.findSimilar')
    await page.goBack()
    await expect(page.locator('[data-scenario-id="test"]')).toHaveAttribute('aria-selected', 'true')
    expect(await page.evaluate(() => localStorage.getItem('vitepress-theme-appearance'))).toBe('light')
    await page.goto('/guide/quickstart')
  }
})

test('SPA navigation reuses route assets and keeps layout listeners stable', async ({ page }) => {
  await page.addInitScript(() => {
    const active = new Set<EventListenerOrEventListenerObject>()
    const add = EventTarget.prototype.addEventListener
    const remove = EventTarget.prototype.removeEventListener
    EventTarget.prototype.addEventListener = function (type, listener, options) {
      if (this === window && type === 'resize' && listener) active.add(listener)
      return add.call(this, type, listener, options)
    }
    EventTarget.prototype.removeEventListener = function (type, listener, options) {
      if (this === window && type === 'resize' && listener) active.delete(listener)
      return remove.call(this, type, listener, options)
    }
    const documentId = Math.random()
    Object.defineProperty(window, '__gladeResizeAudit', {
      value: () => ({ documentId, listeners: active.size })
    })
  })
  await page.goto('/')
  const counts = []
  for (let visit = 0; visit < 3; visit++) {
    for (const path of ['/guide/', '/guide/workbench', '/guide/support-map', '/']) {
      // Use existing same-origin links so the document survives and VitePress
      // actually mounts/unmounts the route consumers under test.
      await page.locator(`a[href="${path}"]`).first().evaluate((link: HTMLAnchorElement) => link.click())
      await page.waitForURL((url) => url.pathname === path)
      await expect(page.locator('h1')).toHaveCount(1)
    }
    await expect(page.locator('script[data-glade-route-asset="/js/home.js"][data-glade-asset-state="loaded"]')).toHaveCount(1)
    counts.push(await page.evaluate(() => Reflect.get(window, '__gladeResizeAudit')()))
  }
  expect(counts[2]).toEqual(counts[0])
})

test('articles and initial capability rows remain useful without JavaScript', async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false })
  const page = await context.newPage()
  await page.goto('/guide/quickstart')
  await expect(page.locator('main')).toContainText('SampleTest')
  await expect(page.locator('main')).toContainText('glade init --project . --yes')
  await page.goto('/reference/lwc-support')
  expect(await page.locator('main table tbody tr').count()).toBeGreaterThan(10)
  await page.goto('/guide/support-map')
  await expect(page.locator('.support-explorer-list > li')).toHaveCount(25)
  await context.close()
})


test('deep pages retain section navigation and readable article outlines', async ({ page }) => {
  for (const [path, section] of [
    ['/guide/quickstart', 'Docs'],
    ['/guide/workflows/apex-tests', 'Guides'],
    ['/reference/config', 'Reference'],
    ['/help/ci-setup', 'Help'],
    ['/guide/workbench', 'Compatibility']
  ]) {
    await page.goto(path)
    await expect(page.locator('.VPNavBarMenuLink.active')).toHaveText(section)
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' }).getByRole('link')).toHaveText(section)
  }
  await page.goto('/guide/quickstart')
  if (page.viewportSize()!.width >= 1280) {
    const outline = page.locator('.VPDocAsideOutline .outline-link').filter({ hasText: 'Initialize local project configuration' })
    await expect(outline).toBeVisible()
    await expect(outline).toHaveCSS('white-space', 'normal')
    expect(await outline.evaluate((link) => link.scrollWidth - link.clientWidth)).toBeLessThanOrEqual(1)
    await outline.click()
    await expect(page.locator('#_3-initialize-local-project-configuration')).toBeInViewport()
  }
})

test('Help leads with recovery and CI symptoms before the illustrated setup', async ({ page }) => {
  await page.goto('/maintainer/')
  await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toHaveText('Contributors')
  await page.goto('/help/ci-setup')
  const links = page.locator('.VPSidebar a')
  await expect(links.nth(0)).toHaveText('Help home')
  await expect(links.nth(1)).toHaveText('Fix a problem')
  await expect(page.locator('.vp-doc h2').first()).toHaveText(/Diagnose a failed run/)
  await page.getByRole('link', { name: 'the first local check', exact: true }).click()
  await expect(page.locator('#_3-initialize-local-project-configuration')).toBeInViewport()
})
