import { expect, test } from '@playwright/test'

test('debug source keeps its breakpoint, line numbers, and code in separate gutters', async ({ page }) => {
  await page.goto('/')
  await page.evaluate(() => document.fonts.ready)
  const source = page.locator('.debug-code')
  await source.scrollIntoViewIfNeeded()
  await expect(source.locator('.line-no')).toHaveText(['1', '2', '3', '4', '5', '6', '7', '8'])

  const layout = await page.locator('.breakpoint-row').evaluate((row) => {
    const number = row.querySelector('.line-no')!
    const digit = Array.from(number.childNodes).find((node) => node.nodeType === Node.TEXT_NODE && node.textContent?.trim() === '5')!
    const range = document.createRange()
    range.selectNode(digit)
    return {
      digit: range.getBoundingClientRect().toJSON(),
      marker: row.querySelector('.breakpoint-dot')!.getBoundingClientRect().toJSON(),
      code: row.querySelector('.line-code')!.getBoundingClientRect().toJSON(),
      codeStarts: Array.from(row.parentElement!.querySelectorAll('.line-code')).map((line) => line.getBoundingClientRect().left)
    }
  })
  // Measure the actual glyph, not just its containing element: the old dot
  // overlapped the 5 even though both element boxes were inside the editor.
  expect(layout.marker.right + 6).toBeLessThanOrEqual(layout.digit.left)
  expect(layout.digit.right + 6).toBeLessThanOrEqual(layout.code.left)
  expect(new Set(layout.codeStarts).size).toBe(1)
  await expect(page.locator('.inspector-head')).toContainText('Paused at breakpoint · line 5')

  await expect(source).toHaveAttribute('aria-label', 'Illustrative debug source')
  await source.focus()
  await expect(source).toBeFocused()
  if (await source.evaluate((element) => element.scrollWidth > element.clientWidth)) {
    await source.press('ArrowRight')
    await expect.poll(() => source.evaluate((element) => element.scrollLeft)).toBeGreaterThan(0)
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})

test('workflow tab labels stay inside their controls in every selected state', async ({ page }) => {
  await page.goto('/')
  await page.evaluate(() => document.fonts.ready)
  for (const selector of ['.demo-tabs', '.tool-tabs']) {
    const tabs = page.locator(`${selector} [role="tab"]`)
    for (const tab of await tabs.all()) {
      await tab.click()
      await expect(tab).toHaveAttribute('aria-selected', 'true')
      const geometry = await page.locator(selector).evaluate((list) => {
        const bounds = list.getBoundingClientRect()
        return Array.from(list.querySelectorAll<HTMLElement>('[role="tab"]')).map((button) => {
          const box = button.getBoundingClientRect()
          const text = Array.from(button.childNodes).find((node) => node.nodeType === Node.TEXT_NODE && node.textContent?.trim())!
          const range = document.createRange()
          range.selectNode(text)
          const textBox = range.getBoundingClientRect()
          return {
            label: button.textContent?.trim(),
            buttonWithinList: box.left >= bounds.left && box.right <= bounds.right,
            textWithinButton: textBox.left >= box.left && textBox.right <= box.right,
            textLines: range.getClientRects().length,
            height: box.height
          }
        })
      })
      for (const item of geometry) {
        expect(item.buttonWithinList, item.label).toBe(true)
        expect(item.textWithinButton, item.label).toBe(true)
        expect(item.textLines, item.label).toBe(1)
        if (page.viewportSize()!.width <= 760) expect(item.height, item.label).toBeGreaterThanOrEqual(44)
      }
    }
  }
  await page.locator('#tool-vscode').click()
  await expect(page.locator('#panel-vscode')).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})

test('touch tablets retain generous workflow controls without changing the desktop layout', async ({ browser }, testInfo) => {
  const context = await browser.newContext({ baseURL: testInfo.project.use.baseURL, viewport: { width: 768, height: 1024 }, hasTouch: true })
  try {
    const page = await context.newPage()
    await page.goto('/')
    expect(await page.evaluate(() => matchMedia('(pointer: coarse)').matches)).toBe(true)
    for (const selector of ['.run-button', '.copy-button', '.tool-tab', '.nav-actions .button-small', '.demo-tab']) {
      for (const control of await page.locator(selector).all()) {
        expect((await control.boundingBox())!.height, selector).toBeGreaterThanOrEqual(44)
      }
    }
    const header = (await page.locator('.console-header').boundingBox())!
    const action = (await page.locator('.run-button').boundingBox())!
    expect(action.y - header.y).toBeGreaterThanOrEqual(8)
    expect(header.y + header.height - action.y - action.height).toBeGreaterThanOrEqual(8)
  } finally {
    await context.close()
  }
})
