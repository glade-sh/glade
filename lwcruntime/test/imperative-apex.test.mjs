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

test("apex import supports wire and imperative invocation", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-imperative-"));
  compileFixture(fixture, outDir);

  const config = {
    namespace: "c",
    outApps: ["c:lightningout"],
    manifest: {
      modules: {
        "c:wireprobe": {
          url: "/lightning/modules/c/wireProbe/wireProbe.js",
          tag: "c-wire-probe",
        },
      },
    },
  };
  const moduleScript = `
    import "/lightning/glade.out.js";
    await new Promise((resolve) => {
      window.$Lightning.use("c:lightningOut", () => {
        window.$Lightning.createComponent("c:wireProbe", { industry: "Technology" }, "host", resolve);
      });
    });
  `;

  const server = await startLightningServer({
    compiledDir: outDir,
    gladeOutJS,
    pages: {},
    wireHandlers: {
      "/lightning/wire/apex": (payload) => {
        if (payload.method === "wireAccounts") {
          return { data: [{ Id: "001000000000001AAA", Name: `wire:${payload.params?.industry ?? ""}` }] };
        }
        if (payload.method === "imperativeAccount") {
          return { data: { Id: "001000000000002AAA", Name: payload.params?.name ?? "" } };
        }
        return { error: { message: `unknown method ${payload.method}` } };
      },
    },
  });
  server.pages["/imperative.html"] = harnessHTML(server.baseURL, config, moduleScript);

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/imperative.html`, { waitUntil: "networkidle" });
    await page.getByRole("button", { name: "Load Imperative" }).click();
    const host = page.locator("c-wire-probe");
    await host.getByText("Imperative Shell Account", { exact: true }).waitFor({ timeout: 10000 });
    const text = await host.innerText();
    assert.match(text, /wire:Technology/);
    assert.match(text, /Imperative Shell Account/);
  } finally {
    await browser.close();
    await server.close();
  }
});
