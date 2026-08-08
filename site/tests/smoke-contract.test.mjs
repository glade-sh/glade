import assert from 'node:assert/strict'
import { test } from 'node:test'

test('JSON smoke validation accepts a parseable response without a media type', async () => {
  let smokeUtils
  try {
    smokeUtils = await import('../scripts/smoke-utils.mjs')
  } catch {
    smokeUtils = undefined
  }
  assert.equal(typeof smokeUtils?.readJSONResponse, 'function')
  const value = await smokeUtils.readJSONResponse(
    new Response('{"version":"v1.2.3"}', { status: 200 }),
    'fixture'
  )
  assert.deepEqual(value, { version: 'v1.2.3' })
})
