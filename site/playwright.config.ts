import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/browser',
  timeout: 30_000,
  expect: { timeout: 8_000 },
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  // Performance assertions need a quiet browser and CPU. Keep the viewport
  // matrix serial so unrelated page tests cannot distort those measurements.
  workers: 1,
  use: {
    baseURL: 'http://127.0.0.1:4173',
    colorScheme: 'dark',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure'
  },
  projects: [
    {
      name: 'desktop-1440',
      testIgnore: ['**/performance.spec.ts', '**/mobile.spec.ts'],
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1440, height: 900 },
        screenshot: 'off',
        trace: 'off'
      }
    },
    {
      name: 'desktop-1024',
      testIgnore: ['**/performance.spec.ts', '**/mobile.spec.ts'],
      use: { ...devices['Desktop Chrome'], viewport: { width: 1024, height: 768 } }
    },
    {
      name: 'tablet-768',
      testIgnore: ['**/performance.spec.ts', '**/mobile.spec.ts'],
      use: { ...devices['Desktop Chrome'], viewport: { width: 768, height: 1024 } }
    },
    {
      name: 'mobile-390',
      testIgnore: ['**/performance.spec.ts'],
      use: { ...devices['iPhone 13'], browserName: 'chromium', viewport: { width: 390, height: 844 } }
    },
    {
      name: 'mobile-320-reflow',
      testIgnore: ['**/performance.spec.ts'],
      use: {
        browserName: 'chromium',
        viewport: { width: 320, height: 800 },
        isMobile: true,
        hasTouch: true
      }
    },
    {
      name: 'performance-1440',
      testMatch: '**/performance.spec.ts',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1440, height: 900 },
        screenshot: 'off',
        trace: 'off'
      }
    }
  ],
  webServer: {
    command: process.env.GLADE_SITE_PREBUILT === '1'
      ? 'npm run preview -- --host 127.0.0.1 --port 4173'
      : 'npm run build:site && npm run preview -- --host 127.0.0.1 --port 4173',
    port: 4173,
    reuseExistingServer: !process.env.CI
  }
})
