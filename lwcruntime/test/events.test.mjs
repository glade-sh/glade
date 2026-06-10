import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import {
  compileFixture,
  harnessHTML,
  repoRoot,
  startLightningServer,
} from "./helpers.mjs";

const fixture = "testdata/local-tests/lightning-out-vf";
const gladeOutJS = path.join(repoRoot, "internal/lwcruntime/embed/glade.out.js");

test("composed custom events bubble to host listener", async (t) => {
  if (!fs.existsSync(path.join(repoRoot, "third_party/lwc/node_modules"))) {
    t.skip("run npm install in third_party/lwc");
  }
  const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-events-"));
  compileFixture(fixture, outDir);

  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    manifest: {
      modules: {
        "c:eventchild": {
          url: "/lightning/modules/c/eventChild/eventChild.js",
          tag: "c-event-child",
        },
      },
    },
  };
  const moduleScript = `
    import "/lightning/glade.out.js";
    window.__selected = null;
    await new Promise((resolve) => {
      window.$Lightning.use("c:lightningOut", () => {
        window.$Lightning.createComponent("c:eventChild", {}, "host", (cmp) => {
          cmp.addEventListener("select", (e) => { window.__selected = e.detail.id; });
          resolve();
        });
      });
    });
  `;

  const server = await startLightningServer({
    compiledDir: outDir,
    gladeOutJS,
    pages: {},
  });
  server.pages["/events.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    page.on("pageerror", (err) => console.error("pageerror", err.message));
    await page.goto(`${server.baseURL}/events.html`, { waitUntil: "networkidle" });

    await page.locator("c-event-child button").click({ timeout: 10000 });
    const selected = await page.evaluate(() => window.__selected);
    assert.equal(selected, "1");
  } finally {
    await browser.close();
    await server.close();
  }
});
