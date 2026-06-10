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

test("counter component updates reactively on click", async (t) => {
  if (!fs.existsSync(path.join(repoRoot, "third_party/lwc/node_modules"))) {
    t.skip("run npm install in third_party/lwc");
  }
  const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-mount-"));
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
    await new Promise((resolve) => {
      window.$Lightning.use("c:lightningOut", () => {
        window.$Lightning.createComponent("c:counter", { count: 1 }, "host", resolve);
      });
    });
  `;

  const server = await startLightningServer({ compiledDir: outDir, gladeOutJS, pages: {} });
  server.pages["/mount.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    page.on("pageerror", (err) => console.error("pageerror", err.message));
    page.on("console", (msg) => {
      if (msg.type() === "error") console.error("console", msg.text());
    });
    await page.goto(`${server.baseURL}/mount.html`, { waitUntil: "networkidle" });

    const counter = page.locator("c-counter");
    await counter.waitFor({ state: "attached", timeout: 10000 });

    const textBefore = await counter.innerText();
    assert.match(textBefore, /1/);

    await counter.locator("button").click();
    const textAfter = await counter.innerText();
    assert.match(textAfter, /2/, `expected count to increment, got ${textAfter}`);
  } finally {
    await browser.close();
    await server.close();
  }
});
