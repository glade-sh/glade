import { expect, test } from '@playwright/test'

test('replay controls and changing filenames fit their panels on narrow screens', async ({ page }) => {
  test.setTimeout(60_000)
  await page.goto('/guide/workbench')
  for (const appearance of ['light', 'dark']) {
    await page.evaluate((value) => localStorage.setItem('vitepress-theme-appearance', value), appearance)
    await page.reload()
    await page.evaluate(() => document.fonts.ready)
    for (const scenario of ['check', 'test', 'exec', 'debug']) {
      await page.locator(`[data-scenario-id="${scenario}"]`).click()
      const clipped = await page.locator('[data-scenario-workbench]').evaluate((workbench) => {
        const failures: string[] = []
        for (const element of workbench.querySelectorAll<HTMLElement>('.home-workflow-tab, .home-output-tab, .home-panel-top')) {
          const box = element.getBoundingClientRect()
          const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT)
          while (walker.nextNode()) {
            const node = walker.currentNode
            if (!node.textContent?.trim()) continue
            const range = document.createRange()
            range.selectNodeContents(node)
            for (const text of range.getClientRects()) {
              if (text.width && (text.left < box.left - 1 || text.right > box.right + 1)) failures.push(node.textContent.trim())
            }
          }
        }
        return failures
      })
      expect(clipped, `${appearance} ${scenario}`).toEqual([])
    }
    for (const button of await page.locator('.home-output-tab, .home-run-button, .home-command-strip button').all()) {
      const bounds = await button.boundingBox()
      expect(bounds!.height).toBeGreaterThanOrEqual(44)
    }
  }
})

test('capability autocomplete stays inside a phone viewport', async ({ page }) => {
  await page.goto('/guide/workbench')
  const editor = page.getByRole('textbox', { name: 'Try capability-backed autocomplete.' })
  await editor.click()
  await editor.press('Control+End')
  await editor.pressSequentially('\nAccount.')
  const completion = page.locator('.cm-tooltip-autocomplete')
  await expect(completion).toContainText('Name')
  const bounds = await completion.boundingBox()
  expect(bounds!.x).toBeGreaterThanOrEqual(0)
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(page.viewportSize()!.width)
  await editor.press('Escape')
  await expect(completion).toBeHidden()
})

test('Help summaries render command names as code instead of literal backticks', async ({ page }) => {
  for (const [route, snippets] of [
    ['first-local-check', ['glade doctor --project .', 'glade check']],
    ['profile-apex-debug-log', ['glade debug profile']],
    ['debug-apex-vscode', ['Debug Local Test']],
    ['changed-tests-before-pr', ['origin/main']],
    ['glade-org-sf-data-import', ['sf']]
  ] as const) {
    await page.goto(`/help/${route}`)
    const summary = page.locator('.docs-intro')
    await expect(summary).not.toContainText('`')
    await expect(summary.locator('code')).toHaveText([...snippets])
  }
})
