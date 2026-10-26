import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
let input = '';
for await (const chunk of process.stdin) input += chunk;
const cfg = JSON.parse(input);
const {chromium} = await import(pathToFileURL(cfg.module + '/index.mjs').href);
const browser = await chromium.launch({headless:true,...(cfg.executable ? {executablePath:cfg.executable}: {})});
try {
    const page = await browser.newPage();
    page.on('pageerror', error => console.error(error.message));
    page.on('response', response => { if (response.status() >= 400) console.error('HTTP '+response.status()+' '+new URL(response.url()).pathname); });
    await page.goto(cfg.url);
    for (const [id, api, expected] of cfg.rows) {
        const selector = `[data-probe="${api.slice(0,2)}"]`;
        const result = page.locator(selector).filter({hasNotText:'pending'});
        await result.waitFor({timeout:15000});
        assert.deepEqual(JSON.parse(await result.textContent()), JSON.parse(expected), id);
    }
} finally { await browser.close(); }
