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

const fixture = "testdata/local-tests/lwc-shell";
const gladeOutJS = path.join(repoRoot, "internal/lwcruntime/embed/glade.out.js");

test("uiLayoutApi getLayout wire renders local record layout", async (t) => {
  if (!fs.existsSync(path.join(repoRoot, "third_party/lwc/node_modules"))) {
    t.skip("run npm install in third_party/lwc");
  }
  const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-layout-"));
  compileFixture(fixture, outDir);

  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    manifest: {
      modules: {
        "c:layoutprobe": {
          url: "/lightning/modules/c/layoutProbe/layoutProbe.js",
          tag: "c-layout-probe",
        },
      },
    },
  };
  const moduleScript = `
    import "/lightning/glade.out.js";
    await new Promise((resolve) => {
      window.$Lightning.use("c:lightningOut", () => {
        window.$Lightning.createComponent("c:layoutProbe", {
          objectApiName: "Account",
          recordTypeId: "012000000000123",
          layoutType: "Full",
          mode: "Create",
          formFactor: "Small"
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
      "/lightning/wire/getLayout": (payload) => {
        calls.push(payload);
        return {
          data: {
            id: "00h000000000001",
            layoutType: "Full",
            mode: "Create",
            objectApiName: "Account",
            recordTypeId: "012000000000123",
            sections: [{
              id: "section-1",
              heading: "Account Information",
              columns: 2,
              rows: 1,
              layoutRows: [{
                layoutItems: [{
                  fieldApiName: "Name",
                  label: "Account Name",
                  required: true,
                  editableForNew: true,
                  editableForUpdate: true,
                  uiBehavior: "Required",
                  layoutComponents: [{ apiName: "Name", componentType: "Field" }],
                }],
              }],
            }],
          },
        };
      },
    },
  });
  server.pages["/layout.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/layout.html`, { waitUntil: "networkidle" });
    const text = await page.locator("c-layout-probe").innerText({ timeout: 10000 });
    assert.match(text, /Account Information/);
    assert.match(text, /Name/);
    assert.match(text, /Create/);
    assert.deepEqual(calls[0], {
      objectApiName: "Account",
      recordTypeId: "012000000000123",
      layoutType: "Full",
      mode: "Create",
      formFactor: "Small",
    });
  } finally {
    await browser.close();
    await server.close();
  }
});
