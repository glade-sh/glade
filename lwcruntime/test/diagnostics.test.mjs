import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import {
  harnessHTML,
  requireLWCToolchain,
  repoRoot,
  startLightningServer,
} from "./helpers.mjs";

const gladeOutJS = path.join(repoRoot, "internal/lwcruntime/embed/glade.out.js");

test("bootstrap stub reports ERROR when Lightning Out app is missing", async () => {
  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    manifest: { modules: {} },
  };
  const moduleScript = `
    window.__useDiagnostic = null;
    window.$Lightning.use("c:missingOut", function(cmp, status, errorMessage) {
      window.__useDiagnostic = {
        cmpIsNull: cmp === null,
        status,
        errorMessage,
        argCount: arguments.length,
      };
    });
  `;

  const server = await startLightningServer({
    compiledDir: fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-diagnostics-")),
    gladeOutJS,
    pages: {},
  });
  server.pages["/diagnostics.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/diagnostics.html`, { waitUntil: "networkidle" });
    await page.waitForTimeout(50);
    assert.deepEqual(await page.evaluate(() => window.__useDiagnostic), {
      cmpIsNull: true,
      status: "ERROR",
      errorMessage: "GLADELWC080 Lightning Out app missing: c:missingOut",
      argCount: 3,
    });
  } finally {
    await browser.close();
    await server.close();
  }
});

test("bootstrap stub reports ERROR when Lightning component alias is missing", async () => {
  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    manifest: { modules: {} },
  };
  const moduleScript = `
    window.__componentDiagnostic = null;
    window.$Lightning.createComponent("c:missingWidget", {}, "host", function(cmp, status, errorMessage) {
      window.__componentDiagnostic = {
        cmpIsNull: cmp === null,
        status,
        errorMessage,
        argCount: arguments.length,
      };
    });
  `;

  const server = await startLightningServer({
    compiledDir: fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-diagnostics-")),
    gladeOutJS,
    pages: {},
  });
  server.pages["/missing-component.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/missing-component.html`, { waitUntil: "networkidle" });
    await page.waitForTimeout(50);
    assert.deepEqual(await page.evaluate(() => window.__componentDiagnostic), {
      cmpIsNull: true,
      status: "ERROR",
      errorMessage: "GLADELWC081 Lightning Out dependency missing: c:missingWidget",
      argCount: 3,
    });
  } finally {
    await browser.close();
    await server.close();
  }
});

test("runtime reports ERROR when Lightning component alias is missing after use", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    manifest: { modules: {} },
  };
  const moduleScript = `
    import "/lightning/glade.out.js";
    window.__runtimeComponentDiagnostic = null;
    await new Promise((resolve) => {
      window.$Lightning.use("c:lightningOut", function() {
        window.$Lightning.createComponent("c:missingWidget", {}, "host", function(cmp, status, errorMessage) {
          window.__runtimeComponentDiagnostic = {
            cmpIsNull: cmp === null,
            status,
            errorMessage,
            argCount: arguments.length,
          };
          resolve();
        });
      });
    });
  `;

  const server = await startLightningServer({
    compiledDir: fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-diagnostics-")),
    gladeOutJS,
    pages: {},
  });
  server.pages["/runtime-missing-component.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/runtime-missing-component.html`, { waitUntil: "networkidle" });
    assert.deepEqual(await page.evaluate(() => window.__runtimeComponentDiagnostic), {
      cmpIsNull: true,
      status: "ERROR",
      errorMessage: "GLADELWC081 Lightning Out dependency missing: c:missingWidget",
      argCount: 3,
    });
  } finally {
    await browser.close();
    await server.close();
  }
});

test("runtime reports ERROR when Lightning Out app is missing after load", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    manifest: { modules: {} },
  };
  const moduleScript = `
    import "/lightning/glade.out.js";
    window.__runtimeUseDiagnostic = null;
    await new Promise((resolve) => {
      window.$Lightning.use("c:missingOut", function(cmp, status, errorMessage) {
        window.__runtimeUseDiagnostic = {
          cmpIsNull: cmp === null,
          status,
          errorMessage,
          argCount: arguments.length,
        };
        resolve();
      });
    });
  `;

  const server = await startLightningServer({
    compiledDir: fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-diagnostics-")),
    gladeOutJS,
    pages: {},
  });
  server.pages["/runtime-missing-app.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/runtime-missing-app.html`, { waitUntil: "networkidle" });
    assert.deepEqual(await page.evaluate(() => window.__runtimeUseDiagnostic), {
      cmpIsNull: true,
      status: "ERROR",
      errorMessage: "GLADELWC080 Lightning Out app missing: c:missingOut",
      argCount: 3,
    });
  } finally {
    await browser.close();
    await server.close();
  }
});
