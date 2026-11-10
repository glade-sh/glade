// Return observed host attributes; native expectations remain in org.tsv.
import fs from 'node:fs';
import { createRequire } from 'node:module';

const config = JSON.parse(fs.readFileSync(0, 'utf8'));
const require = createRequire(import.meta.url);
const { chromium } = require(config.playwrightModule);
const browser = await chromium.launch({
  headless: true,
  ...(config.executablePath ? { executablePath: config.executablePath } : {}),
  timeout: 15000,
});
try {
  const page = await browser.newPage();
  const values = {};
  const errors = {};
  let diagnostics = [];
  page.on('pageerror', error => diagnostics.push(error.message));
  page.on('console', message => {
    if (message.type() === 'error') diagnostics.push(message.text() + ' ' + message.location().url);
  });
  for (const item of config.cases) {
    diagnostics = [];
    try {
      await page.goto(config.url + '?component=' + encodeURIComponent(item.component), {
        waitUntil: 'domcontentloaded', timeout: 30000,
      });
      const host = page.locator(item.tag);
      await host.waitFor({ state: 'attached', timeout: 30000 });
      await page.waitForFunction(tag => {
        const element = document.querySelector(tag);
        return element && element.hasAttribute('data-capture');
      }, item.tag, { timeout: 30000 });
      values[item.id] = await host.getAttribute('data-capture');
      if (diagnostics.length) errors[item.id] = diagnostics.join('; ');
    } catch (error) {
      errors[item.id] = error.message + (diagnostics.length ? '; ' + diagnostics.join('; ') : '');
    }
  }
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
