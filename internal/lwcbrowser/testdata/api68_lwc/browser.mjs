import {pathToFileURL} from 'node:url';
let input = '';
for await (const chunk of process.stdin) input += chunk;
const cfg = JSON.parse(input);
const {chromium} = await import(pathToFileURL(cfg.module + '/index.mjs').href);
const browser = await chromium.launch({headless: true, ...(cfg.executable ? {executablePath: cfg.executable} : {})});
const out = {};
try {
  const page = await browser.newPage();
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  for (const item of cfg.items) {
    await page.goto(cfg.url + '/?' + item);
    await page.waitForFunction(() => window.__ready === true, {timeout: 15000});
    out[item] = await page.evaluate(() => {
      // Visible text lines and the attributes the native capture records, read
      // through every shadow root in document order.
      const lines = [];
      const attrs = {};
      function walk(node) {
        if (node.nodeType === Node.TEXT_NODE) {
          const text = node.nodeValue.replace(/\s+/g, ' ').trim();
          if (text) lines.push(text);
          return;
        }
        if (node.nodeType !== Node.ELEMENT_NODE && node.nodeType !== Node.DOCUMENT_FRAGMENT_NODE) return;
        if (node.nodeType === Node.ELEMENT_NODE) {
          if (['SVG', 'STYLE', 'SCRIPT'].includes(node.nodeName.toUpperCase())) return;
          for (const key of ['role', 'aria-label', 'aria-hidden', 'aria-haspopup', 'aria-expanded', 'aria-autocomplete', 'title', 'placeholder', 'inputmode', 'maxlength']) {
            if (node.hasAttribute(key)) (attrs[node.nodeName.toLowerCase() + '.' + (node.className || '').toString().split(' ')[0]] ||= {})[key] = node.getAttribute(key);
          }
          if (node.shadowRoot) walk(node.shadowRoot);
        }
        for (const child of node.childNodes) walk(child);
      }
      for (const el of document.body.children) walk(el);
      return {lines, attrs};
    });
  }
  out.errors = errors;
} finally {
  await browser.close();
}
console.log(JSON.stringify(out));
