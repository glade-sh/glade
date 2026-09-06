import { expect, test } from '@playwright/test'

test('missing pages offer a main landmark and working recovery actions', async ({ page }) => {
  await page.goto('/404.html')
  await expect(page.getByRole('main')).toHaveCount(1)
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Page not found')
  const recovery = page.getByRole('navigation', { name: 'Page recovery' })
  for (const action of await recovery.getByRole('link').all()) {
    expect((await action.boundingBox())!.height).toBeGreaterThanOrEqual(44)
  }
  await recovery.getByRole('link', { name: 'Browse documentation' }).click()
  await expect(page).toHaveURL(/\/guide\/$/)
  await expect(page.getByRole('main')).toHaveCount(1)
})

test('documentation preserves a useful reading width on narrow screens', async ({ page }) => {
  const width = page.viewportSize()!.width
  test.skip(width > 390, 'This regression concerns narrow reading columns.')
  for (const route of ['/guide/', '/guide/installation', '/help/first-local-check']) {
    await page.goto(route)
    const heading = await page.locator('.vp-doc h1').boundingBox()
    expect(heading!.width, route).toBeGreaterThanOrEqual(width - 64)
    expect(heading!.x, route).toBeGreaterThanOrEqual(16)
    expect(heading!.x + heading!.width, route).toBeLessThanOrEqual(width - 16)
    const brand = await page.locator('.VPNavBarTitle .title').boundingBox()
    expect(Math.abs(brand!.x - heading!.x), `${route} brand aligns with reading gutter`).toBeLessThanOrEqual(1)
  }
})

test('documentation header destinations fit and mobile navigation remains usable at breakpoints', async ({ page }, testInfo) => {
  // Exercise the additional breakpoint edges once. Other projects retain their
  // real browser/touch settings and check their own configured viewport.
  const widths = testInfo.project.name === 'desktop-1440'
    ? [640, 641, 767, 768, 959, 960, 1024, 1279, 1280]
    : [page.viewportSize()!.width]

  for (const width of widths) {
    await page.setViewportSize({ width, height: 900 })
    await page.goto('/guide/')
    await page.evaluate(() => document.fonts.ready)
    const controls = await page.locator('.VPNavBar').evaluate((header) => {
      return Array.from(header.querySelectorAll<HTMLElement>('a, button')).flatMap((control) => {
        const box = control.getBoundingClientRect()
        if (!box.width || !box.height || getComputedStyle(control).visibility === 'hidden') return []
        const textBounds: { left: number; right: number }[] = []
        const walker = document.createTreeWalker(control, NodeFilter.SHOW_TEXT)
        let node: Node | null
        while ((node = walker.nextNode())) {
          if (!node.textContent?.trim()) continue
          const range = document.createRange()
          range.selectNode(node)
          for (const rect of range.getClientRects()) {
            if (rect.width && rect.height) textBounds.push({ left: rect.left, right: rect.right })
          }
        }
        const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2)
        return [{
          label: control.getAttribute('aria-label') || control.textContent?.trim(),
          left: box.left,
          right: box.right,
          hit: hit === control || Boolean(hit && control.contains(hit)),
          textBounds
        }]
      })
    })
    expect(controls.length).toBeGreaterThanOrEqual(3)
    for (const control of controls) {
      const label = `${width}px ${control.label}`
      expect(control.left, label).toBeGreaterThanOrEqual(0)
      expect(control.right, label).toBeLessThanOrEqual(width)
      expect(control.hit, label).toBe(true)
      for (const text of control.textBounds) {
        expect(text.left, label).toBeGreaterThanOrEqual(control.left - 1)
        expect(text.right, label).toBeLessThanOrEqual(control.right + 1)
      }
    }

    if (!(await page.locator('.VPNavBarMenu').isVisible())) {
      const menu = page.getByRole('button', { name: /mobile navigation/i })
      await menu.click()
      await expect(menu).toHaveAttribute('aria-expanded', 'true')
      const install = page.locator('.VPNavScreen').getByRole('link', { name: 'Install', exact: true })
      await expect(install).toBeVisible()
      await install.click()
      await expect(page).toHaveURL(/\/guide\/installation$/)
      await expect(page.getByRole('heading', { name: /^Installation/, level: 1 })).toBeVisible()
      await expect(menu).toHaveAttribute('aria-expanded', 'false')
    }
  }
})

test('fenced code exposes a full copy target without covering its language or source', async ({ page }) => {
  await page.goto('/guide/installation')
  await page.evaluate(() => document.fonts.ready)
  const block = page.locator('.vp-doc div.language-bash').first()
  const copy = block.locator('button.copy')
  await block.scrollIntoViewIfNeeded()
  // Keep the pointer away: the copy action must be discoverable before hover.
  await page.mouse.move(0, 0)
  const layout = await block.evaluate((element) => {
    const button = element.querySelector('button.copy')!
    const language = element.querySelector('span.lang')!
    const source = element.querySelector('pre code')!
    const range = document.createRange()
    range.selectNodeContents(source)
    return {
      button: button.getBoundingClientRect().toJSON(),
      language: language.getBoundingClientRect().toJSON(),
      opacity: Number(getComputedStyle(button).opacity),
      source: Array.from(range.getClientRects()).filter((rect) => rect.width && rect.height).map((rect) => rect.toJSON())
    }
  })
  expect(layout.button.width).toBeGreaterThanOrEqual(44)
  expect(layout.button.height).toBeGreaterThanOrEqual(44)
  expect(layout.opacity).toBe(1)
  const overlaps = (first: typeof layout.button, second: typeof layout.button) =>
    first.left < second.right && first.right > second.left && first.top < second.bottom && first.bottom > second.top
  expect(overlaps(layout.button, layout.language), 'Copy and language label').toBe(false)
  for (const line of layout.source) {
    expect(overlaps(layout.button, line), 'Copy and source text').toBe(false)
    expect(overlaps(layout.language, line), 'Language label and source text').toBe(false)
  }

  // Observe the copied payload without relying on OS clipboard permissions.
  await page.evaluate(() => Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: async (value: string) => { (window as any).__launchCopied = value } }
  }))
  await copy.click()
  await expect.poll(() => page.evaluate(() => (window as any).__launchCopied)).toBe('curl -fsSL https://glade.sh/install.sh | sh\nglade version')
  await expect.poll(() => copy.evaluate((button) => getComputedStyle(button, '::before').content)).toContain('Copied')
})

test('capability pagination stays aligned with a separate mobile page label', async ({ page }) => {
  const width = page.viewportSize()!.width
  await page.goto('/guide/support-map')
  const search = page.getByRole('searchbox', { name: 'Search capability notes' })
  expect(await search.evaluate((input) => parseFloat(getComputedStyle(input).fontSize))).toBeGreaterThanOrEqual(16)
  const pagination = page.getByRole('navigation', { name: 'Capability result pages' })
  await pagination.scrollIntoViewIfNeeded()
  const layout = await pagination.evaluate((nav) => ({
    buttons: Array.from(nav.querySelectorAll('button')).map((button) => {
      const range = document.createRange()
      range.selectNodeContents(button)
      return {
        ...button.getBoundingClientRect().toJSON(),
        label: button.textContent?.trim(),
        text: range.getBoundingClientRect().toJSON()
      }
    }),
    status: nav.querySelector('span')!.getBoundingClientRect().toJSON()
  }))
  expect(layout.buttons).toHaveLength(4)
  for (const box of layout.buttons) {
    expect(box.height).toBeGreaterThanOrEqual(44)
    expect(box.left).toBeGreaterThanOrEqual(0)
    expect(box.right).toBeLessThanOrEqual(width)
    expect(box.text.left, box.label).toBeGreaterThanOrEqual(box.left + 1)
    expect(box.text.right, box.label).toBeLessThanOrEqual(box.right - 1)
    expect(box.text.top, box.label).toBeGreaterThanOrEqual(box.top + 1)
    expect(box.text.bottom, box.label).toBeLessThanOrEqual(box.bottom - 1)
  }
  if (width <= 640) {
    expect(Math.max(...layout.buttons.map((box) => box.top)) - Math.min(...layout.buttons.map((box) => box.top))).toBeLessThanOrEqual(1)
    expect(layout.status.bottom).toBeLessThanOrEqual(layout.buttons[0].top)
  }
  await pagination.getByRole('button', { name: 'Next', exact: true }).click()
  await expect(page.getByRole('status')).toHaveText(/Page 2 of \d+\./)
  await expect(pagination.getByRole('button', { name: 'Previous', exact: true })).toBeEnabled()
})
