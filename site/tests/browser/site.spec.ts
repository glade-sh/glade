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

test('homepage keeps search, CTAs, exact copy, and the local boundary available', async ({ page }) => {
  const errors = observeBrowserErrors(page)
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Apex feedback without the deploy wait.' })).toBeVisible()
  await expect(page).toHaveTitle('Glade — Local Apex Runtime for Salesforce Developers')
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute('href', 'https://glade.sh/')
  await expect(page.locator('meta[property="og:title"]')).toHaveAttribute('content', 'Glade — Local Apex Runtime for Salesforce Developers')
  await expect(page.locator('meta[property="og:image"]')).toHaveAttribute('content', 'https://glade.sh/social-card.png')
  await expect(page.getByRole('main')).toHaveCount(1)
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
  await expect(page.getByText('ResetPasswordResult.getPassword', { exact: true })).toBeVisible()
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
  expect(backgrounds[0].body).toContain('radial-gradient')
  expect(backgrounds[0].contour).not.toBe('none')
  expect(Number(backgrounds[0].opacity)).toBeGreaterThan(0)
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
  await expect(page.locator('.docs-status-unknown').first()).toHaveCSS('color', 'rgb(40, 105, 183)')
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
  await page.goto('/guide/workflows')
  await page.getByRole('link', { name: 'Execute Apex and SOQL' }).click()
  await expect(page).toHaveURL(/\/help\/anonymous-apex-scratch$/)
  await expect(page.getByRole('heading', { name: 'Use Anonymous Apex Scratch in VS Code' })).toBeVisible()

  await page.goto('/guide/workbench#exec')
  await expect(page.getByRole('tab', { name: /Execute Apex locally/ })).toHaveAttribute('aria-selected', 'true')
  await expect(page.locator('#workbench-demo-panel')).toHaveAttribute('aria-labelledby', 'exec')
  await expect(page.locator('[data-run-scenario]')).toHaveText('Replay example')
  await expect(page.getByText('Illustrative replay — this page does not execute edited Apex.')).toBeVisible()

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

test('direct capability explorer loads and browser history keep its controls warning-free', async ({ page }) => {
  const messages: string[] = []
  page.on('console', (message) => {
    if (message.type() === 'error' || message.type() === 'warning') messages.push(message.text())
  })
  page.on('pageerror', (error) => messages.push(error.message))

  await page.goto('/guide/workbench#exec')
  await expect(page.getByRole('button', { name: 'Replay example' })).toBeVisible()
  await page.locator('.VPNavBarTitle a').click()
  await expect(page).toHaveURL(/\/$/)
  await page.goBack()
  await expect(page).toHaveURL(/\/guide\/workbench#exec$/)
  await expect(page.getByRole('button', { name: 'Replay example' })).toBeVisible()
  await page.goForward()
  await expect(page).toHaveURL(/\/$/)
  expect(messages).toEqual([])
})

test('light terminal foregrounds meet normal text contrast on the dark command surface', async ({ page }) => {
  await page.goto('/')
  await page.evaluate(() => localStorage.setItem('vitepress-theme-appearance', 'light'))
  await page.reload()
  const styles = await page.evaluate(() => {
    const card = document.querySelector('.home-loop-visual')!
    const style = getComputedStyle(card)
    const command = getComputedStyle(document.querySelector('.home-loop-command')!)
    const code = getComputedStyle(document.querySelector('.home-loop-command code')!)
    return {
      base: style.backgroundColor, gradient: style.backgroundImage,
      grid: getComputedStyle(card, '::before').backgroundImage,
      heading: getComputedStyle(document.querySelector('.home-loop-top strong')!).color,
      command: command.backgroundColor, codeBackground: code.backgroundColor, code: code.color
    }
  })
  const rgba = (color: string) => {
    const channels = color.match(/[\d.]+/g)!.map(Number)
    return [...channels.slice(0, 3), channels[3] ?? 1]
  }
  const composite = (front: string, back: string) => {
    const fg = rgba(front), bg = rgba(back)
    return `rgb(${fg.slice(0, 3).map((value, index) => value * fg[3] + bg[index] * (1 - fg[3])).join(', ')})`
  }
  // Bound this two-stop card by both endpoints and the maximum grid overlay.
  // Fail closed if its background structure changes; this is not a CSS renderer.
  expect(rgba(styles.base)[3]).toBe(1)
  const stops = styles.gradient.match(/rgba?\([^)]+\)/g) ?? []
  const gridStops = (styles.grid.match(/rgba?\([^)]+\)/g) ?? []).filter((color) => rgba(color)[3] > 0)
  expect(stops).toHaveLength(2)
  expect(gridStops).toHaveLength(2)
  for (const stop of stops) {
    const card = composite(stop, styles.base)
    for (const background of [card, gridStops.reduce((back, front) => composite(front, back), card)]) {
      expect(contrastRatio(styles.heading, background), 'heading against card').toBeGreaterThanOrEqual(4.5)
      const command = composite(styles.codeBackground, composite(styles.command, background))
      expect(contrastRatio(styles.code, command), 'code against command').toBeGreaterThanOrEqual(4.5)
    }
  }
  for (const selector of ['.home-loop-result strong', '.home-loop-state-label', '.home-loop-result p', '.home-loop-metrics span', '.home-loop-metrics strong']) {
    for (const element of await page.locator(selector).all()) {
      const colors = await element.evaluate((node) => ({
        foreground: getComputedStyle(node).color,
        background: getComputedStyle(node.closest('.home-loop-result, .home-loop-metrics > span')!).backgroundColor
      }))
      expect(contrastRatio(colors.foreground, colors.background), selector).toBeGreaterThanOrEqual(4.5)
    }
  }
  for (const selector of ['.home-workflow-card strong', '.home-workflow-card span', '.home-support-preview code', '.home-support-preview a']) {
    const colors = await page.locator(selector).first().evaluate((node) => ({
      foreground: getComputedStyle(node).color,
      surface: getComputedStyle(node.closest('.home-workflow-card, .home-support-preview')!).backgroundColor,
      page: getComputedStyle(document.body).backgroundColor
    }))
    expect(contrastRatio(colors.foreground, composite(colors.surface, colors.page)), selector).toBeGreaterThanOrEqual(4.5)
  }
  expect(await page.locator('#install-cmd').innerText()).toContain('sh\nglade version')
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
  await page.getByRole('combobox', { name: 'Status' }).selectOption('unsupported')
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
