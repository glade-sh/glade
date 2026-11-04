import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';
const root = fileURLToPath(new URL('../../../', import.meta.url));
import vm from 'node:vm';
import test from 'node:test';

test('row observer ignores forward wall-clock jump', async () => {
  let wall = 0, scans = 0;
  const original = Date.now;
  Date.now = () => wall;
  try {
    const modulePath = path.resolve(root, 'internal/visualforce/testdata/v09_click_row_frame.mjs');
    let rowFrame;
    if (fs.existsSync(modulePath)) {
      ({rowFrame} = await import(pathToFileURL(modulePath)));
    } else {
      const source = fs.readFileSync(path.resolve(root, 'internal/visualforce/testdata/v09_click_browser.mjs'), 'utf8');
      const body = source.slice(source.indexOf('const rowSelector'), source.indexOf('\nasync function snapshot'));
      rowFrame = vm.runInNewContext(body + '\nrowFrame', {Date, setTimeout});
    }
    const frame = {locator: () => ({count: async () => { scans++; wall = 60000; return scans === 2 ? 1 : 0; }})};
    assert.equal(await rowFrame({frames: () => [frame]}, 'owned'), frame);
    assert.equal(scans, 2);
  } finally { Date.now = original; }
});
