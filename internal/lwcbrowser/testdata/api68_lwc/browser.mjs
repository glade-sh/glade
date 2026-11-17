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
  page.on('response', r => { if (r.status() >= 400) errors.push('HTTP ' + r.status() + ' ' + r.url()); });
  for (const item of cfg.items) {
    await page.goto(cfg.url + '/?' + item);
    try {
      await page.waitForFunction(() => window.__ready === true, undefined, {timeout: 15000});
    } catch (e) {
      errors.push(item + ' not ready: ' + e.message.split('\n')[0] + ' | ' + errors.slice(-3).join(' ; '));
      continue;
    }
    if (item.startsWith('b25_') && /_menu\d+$/.test(item)) {
      // Select a tab from the More Tabs menu after the first render, through
      // the menu item's own click, as the capture did.
      const names = ['First', 'Second', 'Third', 'Fourth', 'Fifth', 'Sixth', 'Seventh', 'Eighth', 'Ninth', 'Tenth', 'Eleventh', 'Twelfth'];
      const target = names[Number(item.match(/_menu(\d+)$/)[1]) - 1] + ' long tab';
      const deepFind = (root, test) => {
        for (const el of root.querySelectorAll('*')) {
          if (test(el)) return el;
          if (el.shadowRoot) { const hit = deepFind(el.shadowRoot, test); if (hit) return hit; }
        }
        return null;
      };
      await page.evaluate(() => {
        function find(root) {
          for (const el of root.querySelectorAll('*')) {
            if (el.nodeName.toLowerCase() === 'button' && el.getAttribute('title') === 'More Tabs') return el;
            if (el.shadowRoot) { const hit = find(el.shadowRoot); if (hit) return hit; }
          }
          return null;
        }
        find(document).click();
      });
      try {
        await page.waitForFunction(label => {
          function find(root) {
            for (const el of root.querySelectorAll('lightning-menu-item')) if ((el.shadowRoot || el).textContent.trim() === label) return el;
            for (const el of root.querySelectorAll('*')) if (el.shadowRoot) { const hit = find(el.shadowRoot); if (hit) return hit; }
            return null;
          }
          const hit = find(document);
          if (!hit) return false;
          (hit.shadowRoot && hit.shadowRoot.querySelector('a, [role="menuitem"]') || hit).click();
          window.__menuClicked = true;
          return true;
        }, target, {timeout: 5000});
        await page.waitForFunction(label => {
          function tabs(root, out = []) {
            for (const el of root.querySelectorAll('a[role="tab"]')) out.push(el);
            for (const el of root.querySelectorAll('*')) if (el.shadowRoot) tabs(el.shadowRoot, out);
            return out;
          }
          return tabs(document).some(a => a.getAttribute('aria-selected') === 'true' && a.textContent.trim() === label);
        }, target, {timeout: 5000});
      } catch (e) {
        errors.push(item + ' menu selection failed: ' + e.message.split('\n')[0]);
      }
    }
    if (item === 'b25' || item.startsWith('b25_')) {
      // Open the overflow menu, as the capture does, before reading it.
      await page.evaluate(() => {
        function find(root) {
          for (const el of root.querySelectorAll('*')) {
            if (el.nodeName.toLowerCase() === 'button' && el.getAttribute('title') === 'More Tabs') return el;
            if (el.shadowRoot) { const hit = find(el.shadowRoot); if (hit) return hit; }
          }
          return null;
        }
        const button = find(document);
        if (!button) throw new Error('More Tabs button not found');
        button.click();
      });
      // Wait for the opened menu, not a fixed delay: the button reports
      // expanded and, when the shape overflows, its items are present.
      try {
        await page.waitForFunction(overflowing => {
          function find(root, test) {
            for (const el of root.querySelectorAll('*')) {
              if (test(el)) return true;
              if (el.shadowRoot && find(el.shadowRoot, test)) return true;
            }
            return false;
          }
          const open = find(document, el => el.nodeName.toLowerCase() === 'button' && el.getAttribute('title') === 'More Tabs' && el.getAttribute('aria-expanded') === 'true');
          return open && (!overflowing || find(document, el => el.nodeName.toLowerCase() === 'lightning-menu-item'));
        }, !item.endsWith('_all'), {timeout: 5000});
      } catch (e) {
        errors.push(item + ' menu did not open: ' + e.message.split('\n')[0]);
      }
    }
    out[item] = await page.evaluate(() => {
      // Visible text lines and the attributes the native capture records, read
      // through every shadow root in document order.
      const lines = [];
      const attrs = {};
      function walk(node) {
        if (node.nodeType === Node.TEXT_NODE) {
          const text = node.nodeValue.replace(/[ \t\r\n\f]+/g, ' ').trim();
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
      // Structured views for the items whose native rows record them.
      function deepAll(root, selector, found = []) {
        for (const el of root.querySelectorAll(selector)) found.push(el);
        for (const el of root.querySelectorAll('*')) if (el.shadowRoot) deepAll(el.shadowRoot, selector, found);
        return found;
      }
      const text = el => (el.textContent || '').replace(/\s+/g, ' ').trim();
      const errorButtons = deepAll(document, 'button.slds-button_icon-error').map(button => {
        const attrs = {};
        for (const key of ['class', 'part', 'data-trigger', 'tabindex', 'aria-haspopup']) if (button.hasAttribute(key)) attrs[key] = button.getAttribute(key);
        return {attrs, text: text(button)};
      });
      const tabs = deepAll(document, 'a[role="tab"]').map(text);
      const menu = deepAll(document, 'lightning-menu-item').map(item => text(item.shadowRoot || item));
      const fieldInput = {};
      for (const field of deepAll(document, 'lightning-input-field')) {
        const input = field.shadowRoot && field.shadowRoot.querySelector('input');
        if (input) for (const key of ['inputmode', 'maxlength', 'name', 'placeholder', 'type']) if (input.hasAttribute(key)) fieldInput[key] = input.getAttribute(key);
      }
      const ariaInvalid = deepAll(document, 'input').map(input => input.getAttribute('aria-invalid'));
      // The elements each input names in aria-describedby: their aria-live and
      // whether they hold text, in the order the input lists them.
      const describedBy = deepAll(document, 'input').map(input => {
        const root = input.getRootNode();
        return (input.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean).map(id => {
          const target = root.querySelector('[id="' + id + '"]');
          return target ? {live: target.getAttribute('aria-live'), hasText: text(target) !== ''} : {missing: id};
        });
      });
      // Natural widths of the bar items, for derived expectations: the tab items
      // in the bar and the More Tabs item.
      const tabWidths = deepAll(document, 'a[role="tab"]').map(a => a.parentElement.getBoundingClientRect().width);
      const more = deepAll(document, 'li.slds-tabs_default__overflow-button')[0];
      const moreWidth = more ? more.getBoundingClientRect().width : null;
      return {lines, attrs, errorButtons, tabs, menu, fieldInput, ariaInvalid, describedBy, tabWidths, moreWidth, ...(window.__observation || {})};
    });
  }
  out.errors = errors;
} finally {
  await browser.close();
}
console.log(JSON.stringify(out));
