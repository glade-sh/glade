import assert from 'node:assert/strict'
import { test } from 'node:test'

test('performance timing budgets run only on the calibrated platform', async () => {
  let performancePolicy
  try {
    ({ performancePolicy } = await import('../scripts/performance-policy.mjs'))
  } catch {
    performancePolicy = undefined
  }

  assert.equal(typeof performancePolicy, 'function')
  assert.deepEqual(
    performancePolicy({ platform: 'darwin', runs: 5 }, 'darwin'),
    { runs: 5, enforceTimingBudgets: true }
  )
  assert.deepEqual(
    performancePolicy({ platform: 'darwin', runs: 5 }, 'linux'),
    { runs: 1, enforceTimingBudgets: false }
  )
})
