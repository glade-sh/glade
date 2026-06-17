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

test("apex wire adapter renders vm-backed data", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-wire-"));
  compileFixture(fixture, outDir);

  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    manifest: {
      modules: {
        "c:apexwirehost": {
          url: "/lightning/modules/c/apexWireHost/apexWireHost.js",
          tag: "c-apex-wire-host",
        },
      },
    },
  };
  const moduleScript = `
    import "/lightning/glade.out.js";
    await new Promise((resolve) => {
      window.$Lightning.use("c:lightningOut", () => {
        window.$Lightning.createComponent("c:apexWireHost", { recordId: "001XX0000000001" }, "host", resolve);
      });
    });
  `;

  const server = await startLightningServer({
    compiledDir: outDir,
    gladeOutJS,
    pages: {},
    wireHandlers: {
      "/lightning/wire/apex": (payload) => ({
        data: [
          {
            Id: payload.params?.recordId ?? "",
            Name: `items:${payload.params?.recordId ?? ""}`,
          },
        ],
      }),
    },
  });
  server.pages["/wire.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/wire.html`, { waitUntil: "networkidle" });
    const text = await page.locator("c-apex-wire-host .items").innerText({ timeout: 10000 });
    assert.equal(text, "items:001XX0000000001");
  } finally {
    await browser.close();
    await server.close();
  }
});
