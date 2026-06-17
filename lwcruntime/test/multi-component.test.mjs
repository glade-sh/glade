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

test("multiple Lightning Out components share callback status, wires, labels, resources, and events", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-multi-"));
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
        "c:recordwirehost": {
          url: "/lightning/modules/c/recordWireHost/recordWireHost.js",
          tag: "c-record-wire-host",
        },
        "c:labelresourcehost": {
          url: "/lightning/modules/c/labelResourceHost/labelResourceHost.js",
          tag: "c-label-resource-host",
        },
        "c:eventchild": {
          url: "/lightning/modules/c/eventChild/eventChild.js",
          tag: "c-event-child",
        },
      },
    },
  };
  const moduleScript = `
    import "/lightning/glade.out.js";
    window.__callbacks = {};
    window.__selected = null;
    for (const id of ["apexHost", "recordHost", "labelResourceHost", "eventHost"]) {
      const host = document.createElement("div");
      host.id = id;
      document.body.appendChild(host);
    }
    function create(qualified, attrs, locator) {
      return new Promise((resolve) => {
        window.$Lightning.createComponent(qualified, attrs, locator, function(cmp, status, errorMessage) {
          window.__callbacks[qualified] = {
            tag: cmp && cmp.tagName.toLowerCase(),
            status,
            argCount: arguments.length,
            messageWasUndefined: errorMessage === undefined,
          };
          resolve(cmp);
        });
      });
    }
    await new Promise((resolve) => {
      window.$Lightning.use("c:lightningOut", async () => {
        await create("c:apexWireHost", { recordId: "001XX0000000001" }, "apexHost");
        await create("c:recordWireHost", { recordId: "001XX0000000001" }, "recordHost");
        await create("c:labelResourceHost", {}, "labelResourceHost");
        const eventChild = await create("c:eventChild", {}, "eventHost");
        eventChild.addEventListener("select", (e) => { window.__selected = e.detail.id; });
        resolve();
      });
    });
  `;

  const server = await startLightningServer({
    compiledDir: outDir,
    gladeOutJS,
    pages: {},
    shimConfig: {
      labels: { "c.Greeting": "Hello from Glade" },
      resources: { WidgetAssets: "/resource/WidgetAssets" },
    },
    wireHandlers: {
      "/lightning/wire/apex": (payload) => ({
        data: `items:${payload.params?.recordId ?? ""}`,
      }),
      "/lightning/wire/getRecord": () => ({
        data: {
          fields: {
            Name: { value: "Acme", displayValue: "Acme" },
          },
        },
      }),
    },
  });
  server.pages["/multi.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/multi.html`, { waitUntil: "networkidle" });

    const callbacks = await page.evaluate(() => window.__callbacks);
    for (const qualified of ["c:apexWireHost", "c:recordWireHost", "c:labelResourceHost", "c:eventChild"]) {
      assert.equal(callbacks[qualified].status, "SUCCESS", qualified);
      assert.equal(callbacks[qualified].argCount, 2, qualified);
      assert.equal(callbacks[qualified].messageWasUndefined, true, qualified);
    }

    assert.equal(
      await page.locator("c-apex-wire-host .items").innerText({ timeout: 10000 }),
      "items:001XX0000000001",
    );
    assert.equal(await page.locator("c-record-wire-host .name").innerText(), "Acme");
    assert.equal(await page.locator("c-label-resource-host .label").innerText(), "Hello from Glade");
    assert.equal(await page.locator("c-label-resource-host .resource").innerText(), "/resource/WidgetAssets");

    await page.locator("c-event-child button").click({ timeout: 10000 });
    assert.equal(await page.evaluate(() => window.__selected), "1");
  } finally {
    await browser.close();
    await server.close();
  }
});
