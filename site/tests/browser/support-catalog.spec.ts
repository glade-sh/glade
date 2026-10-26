import { readFileSync } from 'node:fs'
import { expect, test } from '@playwright/test'

const supportCounts: Record<string, number> = JSON.parse(
  readFileSync(new URL('../../docs-src/public/data/editor-support.json', import.meta.url), 'utf8')
).summary

test('support catalog filters complete ledger rows and paginates the display', async ({ page }) => {
  await page.goto('/guide/support-map')
  const result = page.getByRole('status')

  await expect(result).toHaveText('Page 1 of 12.')
  await expect(page.getByRole('heading', { name: 'Explore capability notes' })).toBeVisible()
  await expect(page.locator('.support-explorer-summary')).toHaveCount(0)
  await page.getByRole('searchbox', { name: 'Search capability notes' }).fill('Visualforce full rendering lifecycle')
  await expect(result).toContainText('1 matching note')
  await expect(page.locator('.support-explorer-list')).toContainText('Visualforce full rendering lifecycle')

  await page.getByRole('searchbox', { name: 'Search capability notes' }).fill('List.clear')
  await expect(result).toContainText('No matching notes. An API can be implemented without a note here.')
  await expect(result.getByRole('link', { name: 'Browse the coverage guides' })).toHaveAttribute('href', '#drill-down')

  await page.goto('/guide/workbench#check')
  const editor = page.getByRole('textbox', { name: 'Try capability-backed autocomplete.' })
  await editor.click()
  await editor.press('Control+End')
  await editor.pressSequentially('\nAccount.')
  await expect(page.locator('.cm-tooltip-autocomplete')).toContainText('Name')
})

test('support catalog status filters preserve ledger counts and row classifications', async ({ page }) => {
  await page.goto('/guide/support-map')
  const status = page.getByRole('combobox', { name: 'Status' })
  const result = page.getByRole('status')
  const rows = page.locator('.support-explorer-list > li')
  for (const [value, count] of Object.entries(supportCounts)) {
    await status.selectOption(value)
    await expect(result).toContainText(count === 0 ? 'No matching notes' : `${count} matching note`)
    await expect(rows).toHaveCount(Math.min(count, 25))
    for (const row of await rows.all()) {
      await expect(row.locator('.support-row-detail dd > code')).toHaveText(value)
    }
  }
})
