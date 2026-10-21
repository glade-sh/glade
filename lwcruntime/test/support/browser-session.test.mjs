import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const testDirectory = new URL("../", import.meta.url);

test("browser test files reuse one file-scoped Playwright session", async () => {
  const files = (await readdir(testDirectory)).filter((name) => name.endsWith(".test.mjs"));
  const launchFiles = [];

  for (const name of files) {
    const source = await readFile(new URL(name, testDirectory), "utf8");
    if (!/\bchromium\.launch\s*\(/.test(source)) continue;

    launchFiles.push(name);
    assert.ok(/(?:from\s+|import\s*\()["']\.\/shared-playwright\.mjs["']/.test(source), `${name} should use the shared Playwright wrapper`);
    assert.ok(!/from ["']playwright["']/.test(source), `${name} should not import Playwright directly`);
  }

  assert.ok(launchFiles.length > 0, "browser suite contains launch sites");

  const { createSharedBrowserSession } = await import("../shared-browser-session.mjs");
  let launches = 0;
  let contextCloses = 0;
  let browserCloses = 0;
  const context = { close: async () => { contextCloses++; } };
  const browser = {
    contexts: () => [context],
    close: async () => { browserCloses++; },
  };
  const session = createSharedBrowserSession(async () => {
    launches++;
    return browser;
  });

  const first = await session.launch({ headless: true });
  const second = await session.launch({ headless: true });
  assert.strictEqual(first, second);
  assert.equal(launches, 1);
  await first.close();
  assert.equal(contextCloses, 1);
  assert.equal(browserCloses, 0);
  await session.close();
  assert.equal(browserCloses, 1);
});
