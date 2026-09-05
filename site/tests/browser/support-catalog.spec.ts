import { expect, test } from '@playwright/test'

test('support catalog filters complete ledger rows and paginates the display', async ({ page }) => {
  await page.goto('/guide/support-map')
  const status = page.getByRole('combobox', { name: 'Status' })
  const result = page.getByRole('status')

  await expect(result).toContainText('288 matching rows of 288 checked ledger rows. Showing 1–25. Page 1 of 12.')
  for (const [value, count] of [['supported', '269'], ['partial', '0'], ['unsupported', '19'], ['unknown', '0']]) {
    await status.selectOption(value)
    await expect(result).toContainText(`${count} matching row`)
  }

  await status.selectOption('all')
  await page.getByRole('searchbox', { name: 'Search APIs' }).fill('Visualforce full rendering lifecycle')
  await expect(result).toContainText('1 matching row')
  await expect(page.locator('.support-explorer-list')).toContainText('Visualforce full rendering lifecycle')

  await page.goto('/guide/workbench#check')
  const editor = page.getByRole('textbox', { name: 'Try capability-backed autocomplete.' })
  await editor.click()
  await editor.press('Control+End')
  await editor.pressSequentially('\nAccount.')
  await expect(page.locator('.cm-tooltip-autocomplete')).toContainText('Name')
})
