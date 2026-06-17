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

test("getRecord wire adapter renders account name", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-record-"));
  compileFixture(fixture, outDir);

  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    manifest: {
      modules: {
        "c:recordwirehost": {
          url: "/lightning/modules/c/recordWireHost/recordWireHost.js",
          tag: "c-record-wire-host",
        },
      },
    },
  };
  const moduleScript = `
    import "/lightning/glade.out.js";
    await new Promise((resolve) => {
      window.$Lightning.use("c:lightningOut", () => {
        window.$Lightning.createComponent("c:recordWireHost", { recordId: "001XX0000000001" }, "host", resolve);
      });
    });
  `;

  const server = await startLightningServer({
    compiledDir: outDir,
    gladeOutJS,
    pages: {},
    wireHandlers: {
      "/lightning/wire/getRecord": () => ({
        data: {
          fields: {
            Name: { value: "Acme", displayValue: "Acme" },
          },
        },
      }),
    },
  });
  server.pages["/record.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/record.html`, { waitUntil: "networkidle" });
    const text = await page.locator("c-record-wire-host .name").innerText({ timeout: 10000 });
    assert.equal(text, "Acme");
  } finally {
    await browser.close();
    await server.close();
  }
});
