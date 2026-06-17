import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import {
  compileFixture,
  harnessHTML,
  requireLWCToolchain,
  repoRoot,
  startLightningServer,
} from "./helpers.mjs";

const fixture = "testdata/local-tests/lightning-out-vf";
const gladeOutJS = path.join(repoRoot, "internal/lwcruntime/embed/glade.out.js");

test("createComponent callback receives SUCCESS status", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-callback-"));
  compileFixture(fixture, outDir);

  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    manifest: {
      modules: {
        "c:counter": {
          url: "/lightning/modules/c/counter/counter.js",
          tag: "c-counter",
        },
      },
    },
  };
  const moduleScript = `
    import "/lightning/glade.out.js";
    window.__status = null;
    window.__error = "unset";
    await new Promise((resolve) => {
      window.$Lightning.use("c:lightningOut", () => {
        window.$Lightning.createComponent(
          "c:counter",
          { count: 1 },
          "host",
          (cmp, status, errorMessage) => {
            window.__status = status;
            window.__error = errorMessage;
            resolve();
          }
        );
      });
    });
  `;

  const server = await startLightningServer({ compiledDir: outDir, gladeOutJS, pages: {} });
  server.pages["/callback.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/callback.html`, { waitUntil: "networkidle" });
    const status = await page.evaluate(() => window.__status);
    const error = await page.evaluate(() => window.__error);
    assert.equal(status, "SUCCESS");
    assert.equal(error, undefined);
  } finally {
    await browser.close();
    await server.close();
  }
});
