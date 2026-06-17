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

test("uiObjectInfoApi wires render object info and picklist values", async (t) => {
  if (!fs.existsSync(path.join(repoRoot, "third_party/lwc/node_modules"))) {
    t.skip("run npm install in third_party/lwc");
  }
  const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-object-info-"));
  compileFixture(fixture, outDir);

  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    manifest: {
      modules: {
        "c:objectinfoprobe": {
          url: "/lightning/modules/c/objectInfoProbe/objectInfoProbe.js",
          tag: "c-object-info-probe",
        },
      },
    },
  };
  const moduleScript = `
    import "/lightning/glade.out.js";
    await new Promise((resolve) => {
      window.$Lightning.use("c:lightningOut", () => {
        window.$Lightning.createComponent("c:objectInfoProbe", {
          objectApiName: "Account",
          recordTypeId: "012000000000123",
          fieldApiName: "Account.Rating"
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
      "/lightning/wire/getObjectInfo": (payload) => {
        calls.push(["object", payload]);
        return { data: { apiName: "Account", label: "Account", defaultRecordTypeId: "012000000000123" } };
      },
      "/lightning/wire/getPicklistValues": (payload) => {
        calls.push(["picklist", payload]);
        return { data: { values: [{ value: "Warm", label: "Warm" }] } };
      },
    },
  });
  server.pages["/object-info.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/object-info.html`, { waitUntil: "networkidle" });
    const text = await page.locator("c-object-info-probe").innerText({ timeout: 10000 });
    assert.match(text, /Account/);
    assert.match(text, /Warm/);
    assert.equal(calls.length, 2);
    const callsByName = new Map(calls);
    assert.deepEqual(callsByName.get("object"), { objectApiName: "Account" });
    assert.deepEqual(callsByName.get("picklist"), { fieldApiName: "Account.Rating", recordTypeId: "012000000000123" });
  } finally {
    await browser.close();
    await server.close();
  }
});
