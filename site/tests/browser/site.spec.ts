import AxeBuilder from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'

const representativeRoutes = [
  '/',
  '/guide/',
  '/guide/installation',
  '/guide/quickstart',
  '/guide/support-map',
  '/guide/security-trust',
  '/reference/cli',
  '/guide/workbench',
  '/help/',
  '/private-corpus-assurance.html'
]

function observeBrowserErrors(page: Page) {
  const errors: string[] = []
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(message.text())
  })
  page.on('pageerror', (error) => errors.push(error.message))
  return errors
}

function contrastRatio(foreground: string, background: string) {
  const rgb = (color: string) => color.match(/\d+(?:\.\d+)?/g)!.slice(0, 3).map((value) => Number(value) * (color.startsWith('color(srgb') ? 255 : 1))
  const luminance = (color: string) => rgb(color).map((channel) => {
    const value = channel / 255
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
  }).reduce((total, channel, index) => total + channel * [0.2126, 0.7152, 0.0722][index], 0)
  const [first, second] = [luminance(foreground), luminance(background)].sort((a, b) => b - a)
  return (first + 0.05) / (second + 0.05)
}

async function expectNoHorizontalOverflow(page: Page) {
  const dimensions = await page.evaluate(() => ({
    viewport: document.documentElement.clientWidth,
    page: document.documentElement.scrollWidth
  }))
  expect(dimensions.page).toBeLessThanOrEqual(dimensions.viewport + 1)
}

test('homepage keeps CTAs, exact copy, and the local boundary available', async ({ page }) => {
  const errors = observeBrowserErrors(page)
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Run Apex locally. Keep your momentum.')
  await expect(page).toHaveTitle('Glade — Local Apex Runtime for Salesforce Developers')
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute('href', 'https://glade.sh/')
  await expect(page.locator('meta[property="og:title"]')).toHaveAttribute('content', 'Glade — Local Apex Runtime for Salesforce Developers')
  await expect(page.locator('meta[property="og:image"]')).toHaveAttribute('content', 'https://glade.sh/social-card.png')
  await expect(page.getByRole('main')).toHaveCount(1)
  await expect(page.getByRole('link', { name: 'Installation options' })).toBeVisible()
  await expect(page.locator('.site-footer').getByRole('link', { name: 'Docs', exact: true })).toBeVisible()
  await expect(page.locator('#install-command')).toHaveText('curl -fsSL https://glade.sh/install.sh | sh')
  await page.evaluate(() => { Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (value: string) => { (window as any).__copied = value } } }) })
  await page.getByRole('button', { name: 'Copy Glade install command' }).click()
  await expect(page.getByRole('status')).toContainText('Nothing has been installed.')
  expect(await page.evaluate(() => (window as any).__copied)).toBe('curl -fsSL https://glade.sh/install.sh | sh')
  await expect(page.getByText('Hosted services & final validation', { exact: true })).toBeVisible()
  await expect(page.locator('.compatibility-table').getByText('Salesforce', { exact: true })).toBeVisible()
  await expectNoHorizontalOverflow(page)
  expect(errors).toEqual([])
})

test('header install action stays compact while mobile navigation keeps a full touch target', async ({ page }) => {
  await page.goto('/guide/')
  const desktopInstall = page.locator('.VPNavBarMenuLink[href="/guide/installation"]')
  if (await desktopInstall.isVisible()) {
    const box = await desktopInstall.boundingBox()
    expect(box?.height).toBeLessThanOrEqual(40)
  } else {
    await page.getByRole('button', { name: /mobile navigation/i }).click()
    const mobileInstall = page.locator('.VPNavScreen').getByRole('link', { name: 'Install', exact: true })
    const box = await mobileInstall.boundingBox()
    expect(box?.height).toBeGreaterThanOrEqual(44)
  }
})

test('primary documentation routes use one shared background treatment', async ({ page }) => {
  const backgrounds = []
  for (const route of ['/guide/', '/guide/workflows', '/reference/cli', '/help/', '/guide/support-map', '/guide/security-trust']) {
    await page.goto(route)
    backgrounds.push(await page.evaluate(() => ({
      body: getComputedStyle(document.body).backgroundImage,
      contour: getComputedStyle(document.body, '::before').backgroundImage,
      opacity: getComputedStyle(document.body, '::before').opacity
    })))
  }
  expect(backgrounds[0].body).toBe('none')
  expect(backgrounds[0].contour).toBe('none')
  expect(backgrounds[0].opacity).toBe('1')
  expect(backgrounds.every((background) => JSON.stringify(background) === JSON.stringify(backgrounds[0]))).toBe(true)
})

test('docs landing identifies itself in the sidebar', async ({ page }) => {
  await page.goto('/guide/')
  await expect(page.locator('.VPSidebar a[aria-current="page"]')).toHaveText('Documentation home')
})

test('support status legend remains readable in light mode', async ({ page }) => {
  await page.goto('/')
  await page.evaluate(() => localStorage.setItem('vitepress-theme-appearance', 'light'))
  await page.goto('/guide/support-map')
  await expect(page.locator('.docs-status-unknown').first()).toHaveCSS('color', 'rgb(22, 107, 128)')
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
  const filter = page.getByRole('searchbox', { name: 'Filter command groups' })
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
  await page.goto('/guide/workflows')
  await page.getByRole('link', { name: 'Execute Apex and SOQL' }).click()
  await expect(page).toHaveURL(/\/guide\/playground$/)
  await expect(page.getByRole('heading', { name: /^Use the local Playground/, level: 1 })).toBeVisible()
  await expect(page.locator('.VPSidebar a[aria-current="page"]')).toHaveText('Execute anonymous Apex and SOQL')
  await page.goto('/guide/workbench#exec')
  await expect(page.getByRole('tab', { name: /Execute Apex locally/ })).toHaveAttribute('aria-selected', 'true')
  await expect(page.locator('#workbench-demo-panel')).toHaveAttribute('aria-labelledby', 'exec')
  await expect(page.locator('[data-run-scenario]')).toHaveText('Replay scenario')
  await expect(page.getByText('This is a scripted illustration:', { exact: false })).toBeVisible()

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
  await expect(page.locator('#workbench-demo-panel')).toHaveAttribute('aria-labelledby', 'test')
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
  await expect(page.locator('.glade-home')).toBeVisible()
  await expect(page.locator('script[src="/js/home.js"]')).toHaveCount(0)
  await page.locator('.site-footer').getByRole('link', { name: 'Docs', exact: true }).click()
  await expect(page).toHaveURL(/\/guide\/$/)
  await expect(page.locator('.glade-home')).toHaveCount(0)
  await page.locator('.VPNavBarTitle a').click()
  await expect(page).toHaveURL(/\/$/)
  await expect(page.locator('.glade-home')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Copy Glade install command' })).toBeVisible()
  await expect(page.locator('#demo-output')).toContainText('1 test executed · 1 passed · 0 failed')
  expect(errors).toEqual([])
})

test('direct capability explorer loads and browser history keep its controls warning-free', async ({ page }) => {
  const messages: string[] = []
  page.on('console', (message) => {
    if (message.type() === 'error' || message.type() === 'warning') messages.push(message.text())
  })
  page.on('pageerror', (error) => messages.push(error.message))

  await page.goto('/guide/workbench#exec')
  await expect(page.getByRole('button', { name: 'Replay scenario' })).toBeVisible()
  await page.locator('.VPNavBarTitle a').click()
  await expect(page).toHaveURL(/\/$/)
  await page.goBack()
  await expect(page).toHaveURL(/\/guide\/workbench#exec$/)
  await expect(page.getByRole('button', { name: 'Replay scenario' })).toBeVisible()
  await page.goForward()
  await expect(page).toHaveURL(/\/$/)
  expect(messages).toEqual([])
})

test('light appearance preserves normal-text contrast on the approved command and workflow surfaces', async ({ page }) => {
  await page.goto('/')
  await page.evaluate(() => localStorage.setItem('vitepress-theme-appearance', 'light'))
  await page.reload()
  for (const scenario of ['tests', 'debug', 'check']) {
    await page.locator(`[data-demo="${scenario}"]`).click()
    for (const selector of ['.editor-title', '.demo-tab', '#demo-code .line-code > span', '#console-title', '#demo-command', '.output-line > span', '.output-note', '.capability h2', '.capability p', '.compatibility-table [role="cell"]']) {
      const elements = page.locator(selector)
      expect(await elements.count(), selector).toBeGreaterThan(0)
      for (const element of await elements.all()) {
        const colors = await element.evaluate((node) => {
          const layers: number[][] = []
          let current: Element | null = node
          while (current) {
            const style = getComputedStyle(current)
            if (style.backgroundImage !== 'none') throw new Error('Contrast test requires flat command/workflow surfaces')
            const channels = style.backgroundColor.match(/[\d.]+/g)!.map(Number)
            const rgba = [...channels.slice(0, 3), channels[3] ?? 1]
            layers.push(rgba)
            if (rgba[3] === 1) break
            current = current.parentElement
          }
          if (layers.at(-1)?.[3] !== 1) throw new Error('No opaque surface behind text')
          const background = layers.reverse().reduce((back, front) => front.slice(0, 3).map((value, index) => value * front[3] + back[index] * (1 - front[3])), [0, 0, 0])
          return { foreground: getComputedStyle(node).color, background: `rgb(${background.join(', ')})` }
        })
        expect(contrastRatio(colors.foreground, colors.background), `${scenario}: ${selector}`).toBeGreaterThanOrEqual(4.5)
      }
    }
  }
  expect(await page.locator('#install-command').innerText()).toBe('curl -fsSL https://glade.sh/install.sh | sh')
})

test('representative routes have no serious or critical axe violations', async ({ page }) => {
  test.setTimeout(90_000)
  const errors = observeBrowserErrors(page)
  for (const theme of ['dark', 'light']) {
    await page.goto('/')
    await page.evaluate((appearance) => {
      localStorage.setItem('vitepress-theme-appearance', appearance)
    }, theme)
    for (const route of representativeRoutes) {
      await page.goto(route)
      const results = await new AxeBuilder({ page })
        .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
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
  const search = page.getByRole('searchbox', { name: 'Search capability notes' })
  await search.fill('Answers.findSimilar')
  await expect(page.getByRole('status')).toContainText('1 matching note')
  await expect(page.locator('.support-explorer-list')).toContainText('Answers.findSimilar')
  await page.getByRole('combobox', { name: 'Status' }).selectOption('unsupported')
  await expect(page.getByRole('status')).toContainText('No matching notes')
  await expectNoHorizontalOverflow(page)
  expect(errors).toEqual([])
})

test('light preference survives the dark home and forced-color content remains available', async ({ page }) => {
  await page.goto('/')
  await page.evaluate(() => localStorage.setItem('vitepress-theme-appearance', 'light'))
  await page.reload()
  await expect(page.locator('.glade-home')).toHaveCSS('background-color', 'rgb(11, 17, 25)')
  expect(await page.evaluate(() => localStorage.getItem('vitepress-theme-appearance'))).toBe('light')
  await page.emulateMedia({ forcedColors: 'active' })
  await expect(page.getByRole('heading', { level: 1 })).toContainText('Run Apex locally.')
  await expect(page.getByRole('link', { name: 'Installation options' })).toBeVisible()
})
