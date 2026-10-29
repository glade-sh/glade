// Only owned control DOM and click effects cross stdout. Native helpers are
// neither inspected nor replaced; popups use the browser's real user actions.
import fs from 'node:fs';
import {createRequire} from 'node:module';
const require = createRequire(import.meta.url);
const rowSelector = id => '[data-v09-case=' + JSON.stringify(id) + ']';

async function rowFrame(page, id) {
  const deadline = Date.now() + 12000;
  while (Date.now() < deadline) {
    for (const frame of page.frames()) {
      try { if (await frame.locator(rowSelector(id)).count() === 1) return frame; } catch (_) {}
    }
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw new Error('OWNED_CLICK_ROW_MISSING');
}

async function snapshot(frame, id, roles) {
  return frame.evaluate(({id, roles}) => {
    const row = document.querySelector('[data-v09-case="' + id + '"]');
    const input = [...row.querySelectorAll('input')].find(node => /(^|:)owned$/.test(node.id));
    const stable = value => {
      if (value === null) return null;
      let text = String(value);
      for (const [raw, token] of [[roles.accountId, 'OWNED_ACCOUNT_ID'], [roles.ownerId, 'OWNER_ID'], [roles.ownerName, 'OWNER_NAME']]) {
        if (raw) { text = text.split(raw).join(token); if (token !== 'OWNER_NAME') text = text.split(raw.slice(0, 15)).join(token); }
      }
      return text;
    };
    const attributes = node => Object.fromEntries(['type', 'value', 'size', 'maxlength', 'disabled', 'readonly'].map(name => [name, stable(node.getAttribute(name))]));
    const shortcut = row.querySelector('.dateFormat a');
    const hidden = input ? Object.fromEntries([...row.querySelectorAll('input[type="hidden"]')]
      .filter(node => node.id.startsWith(input.id + '_')).map(node => [node.id.slice(input.id.length), stable(node.value)])) : {};
    return {inputPresent: !!input, value: input ? stable(input.value) : null,
      attributes: input ? attributes(input) : null, hidden, shortcutText: shortcut ? shortcut.textContent : null,
      shortcutPresent: !!shortcut, lookupAnchorPresent: !!row.querySelector('a[id$="_lkwgt"]'),
      untouched: row.querySelector('[data-v09-untouched]').value};
  }, {id, roles});
}

// Observe bounded DOM quiescence without choosing a value or success outcome.
// Keep every sample and elapsed time; a stable no-op remains a no-op.
async function settle(frame, id, roles) {
  const started = Date.now();
  const samples = [];
  let previous = null;
  let unchangedSince = started;
  while (Date.now() - started < 1500) {
    const value = await snapshot(frame, id, roles);
    const encoded = JSON.stringify(value);
    const now = Date.now();
    samples.push({elapsedMs: now - started, value});
    if (encoded !== previous) { previous = encoded; unchangedSince = now; }
    if (now - unchangedSince >= 250) return {status: 'observed', reason: null, settled: true, samples, value};
    await new Promise(resolve => setTimeout(resolve, 50));
  }
  return {status: 'unobserved', reason: 'DOM_QUIESCENCE_NOT_OBSERVED', settled: false, samples, value: samples[samples.length - 1].value};
}

async function dateClick(frame, id, roles) {
  const initial = await snapshot(frame, id, roles);
  if (!initial.inputPresent) return {initial, rejected: {reason: 'RENDERED_OFF', actionable: false}, unchanged: await snapshot(frame, id, roles)};
  const root = frame.locator(rowSelector(id));
  const input = root.locator('input[id$=":owned"],input[id="owned"]');
  const shortcut = root.locator('.dateFormat a');
  const errors = [];
  const page = frame.page();
  const listener = () => errors.push('NATIVE_SCRIPT_ERROR');
  page.on('pageerror', listener);
  try {
    await shortcut.click({timeout: 8000, noWaitAfter: true});
    const firstSettle = await settle(frame, id, roles);
    const first = firstSettle.value;
    await input.fill('INVALID_OWNED_DATE');
    const invalidSettle = await settle(frame, id, roles);
    const invalid = invalidSettle.value;
    await shortcut.click({timeout: 8000, noWaitAfter: true});
    const repairedSettle = await settle(frame, id, roles);
    const repaired = repairedSettle.value;
    await shortcut.click({timeout: 8000, noWaitAfter: true});
    const repeatedSettle = await settle(frame, id, roles);
    const repeated = repeatedSettle.value;
    return {initial, first, invalid, repaired, repeated,
      settling: {first: firstSettle, invalid: invalidSettle, repaired: repairedSettle, repeated: repeatedSettle},
      firstEqualsShortcut: first.value === initial.shortcutText,
      repairedEqualsShortcut: repaired.value === initial.shortcutText,
      repeatedUnchanged: JSON.stringify(repaired) === JSON.stringify(repeated), errors};
  } finally { page.off('pageerror', listener); }
}

async function lookupPopup(page, frame, id, search) {
  const row = frame.locator(rowSelector(id));
  const input = row.locator('input[id$=":owned"],input[id="owned"]');
  if (search !== null) await input.fill(search);
  const pending = page.waitForEvent('popup', {timeout: 10000});
  await row.locator('a[id$="_lkwgt"]').click({timeout: 8000, noWaitAfter: true});
  const popup = await pending;
  await popup.waitForLoadState('domcontentloaded', {timeout: 15000});
  return popup;
}

async function ownedResult(popup, accountId) {
  const deadline = Date.now() + 12000;
  while (Date.now() < deadline && !popup.isClosed()) {
    for (const frame of popup.frames()) {
      const token = JSON.stringify(accountId.slice(0, 15));
      const links = frame.locator('a[href*=' + token + '],a[onclick*=' + token + ']');
      if (await links.count() > 0) return links.first();
    }
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  return null;
}

async function lookupClick(page, frame, id, roles) {
  const initial = await snapshot(frame, id, roles);
  if (!initial.inputPresent) return {initial, rejected: {reason: 'RENDERED_OFF', actionable: false}, unchanged: await snapshot(frame, id, roles)};
  const popups = [];
  try {
    const cancelled = await lookupPopup(page, frame, id, null);
    popups.push(cancelled);
    const cancelOpened = !cancelled.isClosed();
    await cancelled.close();
    const afterCancel = await snapshot(frame, id, roles);
    const missingSearch = 'V09_NO_MATCH_7f2c';
    const rejected = await lookupPopup(page, frame, id, missingSearch);
    popups.push(rejected);
    const missingOwnedResult = (await ownedResult(rejected, roles.accountId)) === null;
    await rejected.close();
    const afterNoMatch = await snapshot(frame, id, roles);
    const selected = await lookupPopup(page, frame, id, 'V09 owned');
    popups.push(selected);
    const result = await ownedResult(selected, roles.accountId);
    if (!result) return {initial, cancelOpened, afterCancel, missingOwnedResult, afterNoMatch,
      selection: {status: 'unobserved', reason: 'OWNED_LOOKUP_RESULT_NOT_OBSERVED'}};
    const closed = selected.waitForEvent('close', {timeout: 8000}).then(() => true, () => false);
    await result.click({timeout: 8000, noWaitAfter: true});
    const popupClosed = await closed;
    await frame.waitForFunction(({id, accountId}) => {
      const row = document.querySelector('[data-v09-case="' + id + '"]');
      const selected = row.querySelector('input[id$="_lkid"]');
      return selected && selected.value.slice(0, 15) === accountId.slice(0, 15);
    }, {id, accountId: roles.accountId}, {timeout: 8000});
    const afterSelect = await snapshot(frame, id, roles);
    return {initial, cancelOpened, afterCancel, cancelUnchanged: JSON.stringify(initial) === JSON.stringify(afterCancel),
      missingOwnedResult, afterNoMatch, selection: {status: popupClosed ? 'observed' : 'partial',
        reason: popupClosed ? null : 'LOOKUP_SELECTION_CLOSE_NOT_OBSERVED', afterSelect, popupClosed,
        selectedOwnedID: afterSelect.hidden._lkid === 'OWNED_ACCOUNT_ID', untouchedUnchanged: afterSelect.untouched === initial.untouched}};
  } finally { for (const popup of popups) if (!popup.isClosed()) await popup.close().catch(() => {}); }
}

async function searchPresence(page, frame, id, roles, term) {
  const initial = await snapshot(frame, id, roles);
  const popup = await lookupPopup(page, frame, id, term);
  try {
    const ownedResultPresent = (await ownedResult(popup, roles.accountId)) !== null;
    await popup.close();
    return {initial, term, ownedResultPresent, popupClosed: popup.isClosed(), afterCancel: await snapshot(frame, id, roles)};
  } finally { if (!popup.isClosed()) await popup.close().catch(() => {}); }
}

(async () => {
  const cfg = JSON.parse(fs.readFileSync(0, 'utf8'));
  const {chromium} = require(cfg.playwrightModule);
  const browser = await chromium.launch({headless: true, executablePath: cfg.executablePath || undefined});
  const context = await browser.newContext({locale: 'en-US', timezoneId: 'UTC'});
  const page = await context.newPage();
  const output = {};
  try {
    for (const id of cfg.ids) {
      const spec = cfg.mapping[id];
      const url = new URL('/apex/' + spec.page, cfg.url);
      url.searchParams.set('ownedCase', id);
      // A local widget click can miss its popup on a busy runner; retry the whole case on a fresh page
      // load before reporting it unobserved, and keep each swallowed error visible on stderr.
      output[id] = {status: 'unobserved', reason: 'LOCAL_WIDGET_CLICK_NOT_OBSERVED'};
      for (let attempt = 1; attempt <= 3; attempt++) {
        await page.goto(url.href, {waitUntil: 'domcontentloaded'});
        try {
          const frame = await rowFrame(page, id);
          output[id] = spec.search_term ? await searchPresence(page, frame, id, cfg.roles, spec.search_term) : spec.mode === 'Lookup' ? await lookupClick(page, frame, id, cfg.roles) : await dateClick(frame, id, cfg.roles);
          break;
        } catch (error) {
          process.stderr.write(`v09 ${id} attempt ${attempt}: ${String(error && error.message || error).split('\n')[0]}\n`);
        }
        await page.goto('about:blank');
      }
      await page.goto('about:blank');
    }
  } finally { await browser.close(); }
  process.stdout.write(JSON.stringify(output));
})().catch(error => { process.stderr.write(String(error)); process.exitCode = 1; });
