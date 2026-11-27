# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: performance.spec.ts >> asset size and calibrated performance stay inside their stored budgets
- Location: tests/browser/performance.spec.ts:71:1

# Error details

```
Error: / LCP baseline

expect(received).toBeLessThanOrEqual(expected)

Expected: <= 1003
Received:    1004
```

# Test source

```ts
  1   | import { readFile } from 'node:fs/promises'
  2   | import { expect, test, type Browser } from '@playwright/test'
  3   | import { performancePolicy } from '../../scripts/performance-policy.mjs'
  4   | 
  5   | type Metrics = {
  6   |   jsCssBytes: number
  7   |   lcpMs: number
  8   |   cls: number
  9   |   tbtMs: number
  10  | }
  11  | 
  12  | type Baseline = {
  13  |   runner: {
  14  |     platform: string
  15  |     runs: number
  16  |   }
  17  |   routes: Record<string, { median: Metrics }>
  18  | }
  19  | 
  20  | const baseline = JSON.parse(
  21  |   await readFile(new URL('../performance-baseline.json', import.meta.url), 'utf8')
  22  | ) as Baseline
  23  | 
  24  | function median(samples: Metrics[], key: keyof Metrics) {
  25  |   return [...samples].sort((left, right) => left[key] - right[key])[Math.floor(samples.length / 2)][key]
  26  | }
  27  | 
  28  | async function capture(browser: Browser, route: string) {
  29  |   const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  30  |   const page = await context.newPage()
  31  |   const session = await context.newCDPSession(page)
  32  |   await session.send('Network.enable')
  33  |   await session.send('Network.emulateNetworkConditions', {
  34  |     offline: false,
  35  |     latency: 100,
  36  |     downloadThroughput: 200_000,
  37  |     uploadThroughput: 100_000,
  38  |     connectionType: 'wifi'
  39  |   })
  40  |   await session.send('Emulation.setCPUThrottlingRate', { rate: 4 })
  41  |   await page.addInitScript(() => {
  42  |     window.__gladePerf = { lcpMs: 0, cls: 0, tbtMs: 0 }
  43  |     new PerformanceObserver((list) => {
  44  |       const entries = list.getEntries()
  45  |       if (entries.length > 0) window.__gladePerf.lcpMs = entries.at(-1)?.startTime || 0
  46  |     }).observe({ type: 'largest-contentful-paint', buffered: true })
  47  |     new PerformanceObserver((list) => {
  48  |       for (const entry of list.getEntries() as PerformanceEntryList & { hadRecentInput?: boolean; value?: number }[]) {
  49  |         if (!entry.hadRecentInput) window.__gladePerf.cls += entry.value || 0
  50  |       }
  51  |     }).observe({ type: 'layout-shift', buffered: true })
  52  |     new PerformanceObserver((list) => {
  53  |       for (const entry of list.getEntries()) window.__gladePerf.tbtMs += Math.max(0, entry.duration - 50)
  54  |     }).observe({ type: 'longtask', buffered: true })
  55  |   })
  56  |   await page.goto(`http://127.0.0.1:4173${route}`, { waitUntil: 'networkidle' })
  57  |   await page.waitForTimeout(500)
  58  |   const result = await page.evaluate(() => {
  59  |     const assets = performance
  60  |       .getEntriesByType('resource')
  61  |       .filter((entry) => /^http:\/\/127\.0\.0\.1:4173\/.+\.(?:js|css)(?:\?|$)/.test(entry.name)) as PerformanceResourceTiming[]
  62  |     return {
  63  |       jsCssBytes: assets.reduce((total, entry) => total + entry.encodedBodySize, 0),
  64  |       ...window.__gladePerf
  65  |     }
  66  |   })
  67  |   await context.close()
  68  |   return result
  69  | }
  70  | 
  71  | test('asset size and calibrated performance stay inside their stored budgets', async ({ browser }, testInfo) => {
  72  |   test.setTimeout(120_000)
  73  |   const policy = performancePolicy(baseline.runner, process.platform)
  74  | 
  75  |   for (const route of ['/', '/guide/quickstart']) {
  76  |     const samples: Metrics[] = []
  77  |     for (let run = 0; run < policy.runs; run += 1) samples.push(await capture(browser, route))
  78  |     const current = {
  79  |       jsCssBytes: median(samples, 'jsCssBytes'),
  80  |       lcpMs: median(samples, 'lcpMs'),
  81  |       cls: median(samples, 'cls'),
  82  |       tbtMs: median(samples, 'tbtMs')
  83  |     }
  84  |     await testInfo.attach(`metrics-${route === '/' ? 'home' : 'quickstart'}`, { body: JSON.stringify({ route, current, samples, runner: process.platform }), contentType: 'application/json' })
  85  |     console.log(JSON.stringify({ route, current }))
  86  |     const stored = baseline.routes[route].median
  87  | 
  88  |     expect(current.jsCssBytes, `${route} JS/CSS bytes`).toBeLessThanOrEqual(stored.jsCssBytes * 1.05)
  89  |     // Real CDP throttling inherits the host's CPU speed. Keep deterministic
  90  |     // asset growth checked everywhere, but compare lab timing only on the
  91  |     // platform where this profile was calibrated.
  92  |     if (policy.enforceTimingBudgets) {
  93  |       // LCP entries are reported in whole milliseconds. Round the percentage
  94  |       // boundary up so a sub-millisecond fraction cannot fail an integer sample.
> 95  |       expect(current.lcpMs, `${route} LCP baseline`).toBeLessThanOrEqual(Math.ceil(stored.lcpMs * 1.15))
      |                                                      ^ Error: / LCP baseline
  96  |       expect(current.cls, `${route} CLS baseline`).toBeLessThanOrEqual(Math.max(stored.cls * 1.15, 0.01))
  97  |       expect(current.lcpMs, `${route} LCP budget`).toBeLessThanOrEqual(2_500)
  98  |       expect(current.cls, `${route} CLS budget`).toBeLessThanOrEqual(0.1)
  99  |       // Near-zero TBT varies with scheduler contention even on the same runner.
  100 |       // Keep it as an absolute user-facing budget instead of a relative gate.
  101 |       expect(current.tbtMs, `${route} TBT budget`).toBeLessThanOrEqual(200)
  102 |     }
  103 |   }
  104 | })
  105 | 
  106 | declare global {
  107 |   interface Window {
  108 |     __gladePerf: Metrics
  109 |   }
  110 | }
  111 | 
```