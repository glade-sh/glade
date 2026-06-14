import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import { harnessHTML, repoRoot, startLightningServer } from "./helpers.mjs";

const gladeOutJS = path.join(repoRoot, "internal/lwcruntime/embed/glade.out.js");

test("Lightning Out runtime reports dependency, module, name, and service diagnostics", async () => {
  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    outAppDependencies: {
      "c:lightningout": ["c:missingModule"],
    },
    manifest: {
      modules: {
        "c:eventchild": {
          url: "/lightning/modules/c/eventChild/eventChild.js",
          tag: "c-event-child",
        },
        "c:missingmodule": {
          url: "/lightning/modules/c/missingModule/missingModule.js",
          tag: "c-missing-module",
        },
      },
    },
  };
  const moduleScript = `
    import "/lightning/glade.out.js";
    window.__diagnostics = {};
    function attempt(name) {
      return new Promise((resolve) => {
        window.$Lightning.createComponent(name, {}, "host", function(cmp, status, errorMessage) {
          window.__diagnostics[name] = {
            cmpIsNull: cmp === null,
            status,
            errorMessage,
            argCount: arguments.length,
          };
          resolve();
        });
      });
    }
    await new Promise((resolve) => {
      window.$Lightning.use("c:lightningOut", async () => {
        await attempt("badName");
        await attempt("lightning:navigation");
        await attempt("c:eventChild");
        await attempt("c:missingModule");
        resolve();
      });
    });
  `;

  const server = await startLightningServer({
    compiledDir: fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-lightningout-")),
    gladeOutJS,
    pages: {},
  });
  server.pages["/lightningout-diagnostics.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/lightningout-diagnostics.html`, { waitUntil: "networkidle" });
    assert.deepEqual(await page.evaluate(() => window.__diagnostics), {
      badName: {
        cmpIsNull: true,
        status: "ERROR",
        errorMessage: "Bad Lightning component name: badName",
        argCount: 3,
      },
      "lightning:navigation": {
        cmpIsNull: true,
        status: "ERROR",
        errorMessage: "Unsupported Lightning service: lightning:navigation",
        argCount: 3,
      },
      "c:eventChild": {
        cmpIsNull: true,
        status: "ERROR",
        errorMessage: "Lightning dependency not found: c:eventChild",
        argCount: 3,
      },
      "c:missingModule": {
        cmpIsNull: true,
        status: "ERROR",
        errorMessage: "Lightning LWC module not found: c:missingModule",
        argCount: 3,
      },
    });
  } finally {
    await browser.close();
    await server.close();
  }
});
