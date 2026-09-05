import { readFile } from 'node:fs/promises'
import { expect, test, type Browser } from '@playwright/test'
import { performancePolicy } from '../../scripts/performance-policy.mjs'

type Metrics = {
  jsCssBytes: number
  lcpMs: number
  cls: number
  tbtMs: number
}

type Baseline = {
  runner: {
    platform: string
    runs: number
  }
  routes: Record<string, { median: Metrics }>
}

const baseline = JSON.parse(
  await readFile(new URL('../performance-baseline.json', import.meta.url), 'utf8')
) as Baseline

function median(samples: Metrics[], key: keyof Metrics) {
  return [...samples].sort((left, right) => left[key] - right[key])[Math.floor(samples.length / 2)][key]
}

async function capture(browser: Browser, route: string) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  const page = await context.newPage()
  const session = await context.newCDPSession(page)
  await session.send('Network.enable')
  await session.send('Network.emulateNetworkConditions', {
    offline: false,
    latency: 100,
    downloadThroughput: 200_000,
    uploadThroughput: 100_000,
    connectionType: 'wifi'
  })
  await session.send('Emulation.setCPUThrottlingRate', { rate: 4 })
  await page.addInitScript(() => {
    window.__gladePerf = { lcpMs: 0, cls: 0, tbtMs: 0 }
    new PerformanceObserver((list) => {
      const entries = list.getEntries()
      if (entries.length > 0) window.__gladePerf.lcpMs = entries.at(-1)?.startTime || 0
    }).observe({ type: 'largest-contentful-paint', buffered: true })
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries() as PerformanceEntryList & { hadRecentInput?: boolean; value?: number }[]) {
        if (!entry.hadRecentInput) window.__gladePerf.cls += entry.value || 0
      }
    }).observe({ type: 'layout-shift', buffered: true })
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) window.__gladePerf.tbtMs += Math.max(0, entry.duration - 50)
    }).observe({ type: 'longtask', buffered: true })
  })
  await page.goto(`http://127.0.0.1:4173${route}`, { waitUntil: 'networkidle' })
  await page.waitForTimeout(500)
  const result = await page.evaluate(() => {
    const assets = performance
      .getEntriesByType('resource')
      .filter((entry) => /^http:\/\/127\.0\.0\.1:4173\/.+\.(?:js|css)(?:\?|$)/.test(entry.name)) as PerformanceResourceTiming[]
    return {
      jsCssBytes: assets.reduce((total, entry) => total + entry.encodedBodySize, 0),
      ...window.__gladePerf
    }
  })
  await context.close()
  return result
}

test('asset size and calibrated performance stay inside their stored budgets', async ({ browser }, testInfo) => {
  test.setTimeout(120_000)
  const policy = performancePolicy(baseline.runner, process.platform)

  for (const route of ['/', '/guide/quickstart']) {
    const samples: Metrics[] = []
    for (let run = 0; run < policy.runs; run += 1) samples.push(await capture(browser, route))
    const current = {
      jsCssBytes: median(samples, 'jsCssBytes'),
      lcpMs: median(samples, 'lcpMs'),
      cls: median(samples, 'cls'),
      tbtMs: median(samples, 'tbtMs')
    }
    await testInfo.attach(`metrics-${route === '/' ? 'home' : 'quickstart'}`, { body: JSON.stringify({ route, current, samples, runner: process.platform }), contentType: 'application/json' })
    console.log(JSON.stringify({ route, current }))
    const stored = baseline.routes[route].median

    expect(current.jsCssBytes, `${route} JS/CSS bytes`).toBeLessThanOrEqual(stored.jsCssBytes * 1.05)
    // Real CDP throttling inherits the host's CPU speed. Keep deterministic
    // asset growth checked everywhere, but compare lab timing only on the
    // platform where this profile was calibrated.
    if (policy.enforceTimingBudgets) {
      // LCP entries are reported in whole milliseconds. Round the percentage
      // boundary up so a sub-millisecond fraction cannot fail an integer sample.
      expect(current.lcpMs, `${route} LCP baseline`).toBeLessThanOrEqual(Math.ceil(stored.lcpMs * 1.15))
      expect(current.cls, `${route} CLS baseline`).toBeLessThanOrEqual(Math.max(stored.cls * 1.15, 0.01))
      expect(current.lcpMs, `${route} LCP budget`).toBeLessThanOrEqual(2_500)
      expect(current.cls, `${route} CLS budget`).toBeLessThanOrEqual(0.1)
      // Near-zero TBT varies with scheduler contention even on the same runner.
      // Keep it as an absolute user-facing budget instead of a relative gate.
      expect(current.tbtMs, `${route} TBT budget`).toBeLessThanOrEqual(200)
    }
  }
})

declare global {
  interface Window {
    __gladePerf: Metrics
  }
}
