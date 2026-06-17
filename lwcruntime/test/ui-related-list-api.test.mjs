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

const fixture = "testdata/local-tests/lwc-shell";
const gladeOutJS = path.join(repoRoot, "internal/lwcruntime/embed/glade.out.js");

test("uiRelatedListApi getRelatedListRecords renders child rows", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-related-list-"));
  compileFixture(fixture, outDir);

  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    manifest: {
      modules: {
        "c:relatedlistprobe": {
          url: "/lightning/modules/c/relatedListProbe/relatedListProbe.js",
          tag: "c-related-list-probe",
        },
      },
    },
  };
  const moduleScript = `
    import "/lightning/glade.out.js";
    await new Promise((resolve) => {
      window.$Lightning.use("c:lightningOut", () => {
        window.$Lightning.createComponent("c:relatedListProbe", {
          parentRecordId: "001000000000001AAA",
          relatedListId: "Contacts"
        }, "host", resolve);
      });
    });
  `;

  const calls = [];
  const server = await startLightningServer({
    compiledDir: outDir,
    gladeOutJS,
    pages: {},
    wireHandlers: {
      "/lightning/wire/getRelatedListRecords": (payload) => {
        calls.push(payload);
        return {
          data: {
            count: 1,
            relatedListId: "Contacts",
            parentRecordId: "001000000000001AAA",
            childObjectApiName: "Contact",
            records: [{
              id: "003000000000001AAA",
              apiName: "Contact",
              fields: {
                LastName: { value: "Smith", displayValue: "Smith" },
              },
            }],
          },
        };
      },
    },
  });
  server.pages["/related-list.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/related-list.html`, { waitUntil: "networkidle" });
    const text = await page.locator("c-related-list-probe").innerText({ timeout: 10000 });
    assert.match(text, /1/);
    assert.match(text, /Smith/);
    assert.deepEqual(calls[0], {
      parentRecordId: "001000000000001AAA",
      relatedListId: "Contacts",
      fields: ["Contact.LastName"],
      optionalFields: [],
      sortBy: [],
    });
  } finally {
    await browser.close();
    await server.close();
  }
});
