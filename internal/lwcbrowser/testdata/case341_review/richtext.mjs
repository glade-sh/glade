import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { readFile } from 'node:fs/promises';

const require = createRequire(import.meta.url);
const { chromium } = require(process.argv[2]);
let source = await readFile(process.argv[3], 'utf8');
source = source.replace(/import \{[\s\S]*?\} from "lwc";\n/, '').split('function renderFormattedRichText')[0];
const browser = await chromium.launch({headless: true, ...(process.env.GLADE_LWC_BROWSER_EXECUTABLE ? {executablePath: process.env.GLADE_LWC_BROWSER_EXECUTABLE} : {})});
try {
  const page = await browser.newPage();
  const results = await page.evaluate((source) => {
    const render = Function(source + '; return renderRichContent;')();
    const api = {i: (nodes, f) => nodes.map(f), t: value => value, h: (tag, attrs, children) => children};
    const text = value => Array.isArray(value) ? value.map(text).join('') : (value || '');
    const inputs = ['<pre>\n\nhello</pre>', '<listing>\n\nworld</listing>', '<plaintext>\nhello<b>x</b>', '<p>safe</p><script>ignore</script>'];
    return inputs.map(input => {
      const first = document.createElement('template');
      first.innerHTML = input;
      for (const node of first.content.querySelectorAll('script')) node.remove();
      const second = document.createElement('template');
      second.innerHTML = first.innerHTML;
      return {input, expected: second.content.textContent, actual: text(render(api, input))};
    });
  }, source);
  for (const row of results) assert.equal(row.actual, row.expected, JSON.stringify(row));
  console.log('rich-text baseline preservation passed');
} finally { await browser.close(); }
