import {performance} from 'node:perf_hooks';

export async function rowFrame(page, id, {
  now = () => performance.now(),
  pause = ms => new Promise(resolve => setTimeout(resolve, ms)),
} = {}) {
  const deadline = now() + 12000;
  while (now() < deadline) {
    for (const frame of page.frames()) {
      const selector = '[data-v09-case=' + JSON.stringify(id) + ']';
      try { if (await frame.locator(selector).count() === 1) return frame; } catch (_) {}
    }
    await pause(100);
  }
  throw new Error('OWNED_CLICK_ROW_MISSING');
}
