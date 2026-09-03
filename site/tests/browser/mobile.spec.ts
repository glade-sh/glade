import { expect, test, type Page } from '@playwright/test'

function observeBrowserErrors(page: Page) {
  const errors: string[] = []
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(message.text())
  })
  page.on('pageerror', (error) => errors.push(error.message))
  return errors
}

test('mobile navigation, skip link, sidebar, and search remain reachable', async ({ page }) => {
  const errors = observeBrowserErrors(page)
  await page.goto('/guide/quickstart')

  await page.keyboard.press('Tab')
  const skipLink = page.getByText(/skip to content/i)
  await expect(skipLink).toBeFocused()

  const menuButton = page.getByRole('button', { name: /mobile navigation/i })
  await expect(menuButton).toBeVisible()
  await menuButton.click()
  await expect(page.locator('.VPNavScreen')).toBeVisible()
  await expect(page.locator('.VPNavBarSearch')).toBeVisible()
  const docsLink = page.locator('.VPNavScreen').getByRole('link', { name: 'Docs', exact: true })
  await docsLink.focus()
  await docsLink.press('Escape')
  await expect(page.locator('.VPNavScreen')).toBeHidden()
  await expect(menuButton).toHaveAttribute('aria-expanded', 'false')
  await expect(menuButton).toBeFocused()

  const localNav = page.getByRole('button', { name: /menu/i }).last()
  if (await localNav.isVisible()) {
    await localNav.click()
    await expect(page.getByRole('link', { name: 'First local check' })).toBeVisible()
  }
  const dimensions = await page.evaluate(() => ({
    viewport: document.documentElement.clientWidth,
    page: document.documentElement.scrollWidth
  }))
  expect(dimensions.page).toBeLessThanOrEqual(dimensions.viewport + 1)
  expect(errors).toEqual([])
})
