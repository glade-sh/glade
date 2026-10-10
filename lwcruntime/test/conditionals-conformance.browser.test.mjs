import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createRequire } from "node:module";
import fs from "node:fs";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import { repoRoot } from "./helpers.mjs";

const toolchain = process.env.GLADE_LWC_TOOLCHAIN_DIR || path.join(repoRoot, "third_party/lwc");
const requireToolchain = createRequire(path.join(toolchain, "compile.mjs"));
const { transformSync } = requireToolchain("@lwc/compiler");
const playwrightPath = process.env.GLADE_LWC_PLAYWRIGHT_MODULE;
const { chromium } = await import(playwrightPath ? pathToFileURL(path.join(playwrightPath, "index.mjs")).href : "playwright");
const cases = JSON.parse(fs.readFileSync(path.join(repoRoot, "internal/lwc/testdata/conformance/lwc_conditionals.json")));

test("API 67 compiled conditionals match the Salesforce family capture", async () => {
  const fixture = path.join(repoRoot, "testdata/lwc-conditionals");
  const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-conditionals-"));
  let browser;
  let server;
  try {
    const bundle = "force-app/main/default/lwc/familyConditional";
    const config = { projectRoot: fixture, outDir, namespace: "c", lwcFiles: [bundle + "/familyConditional.js"], lwcHtmlFiles: [bundle + "/familyConditional.html"], lwcMetaFiles: [bundle + "/familyConditional.js-meta.xml"], lwcApiVersions: { [bundle]: 67 } };
    const compiled = spawnSync(process.execPath, [path.join(repoRoot, "third_party/lwc/compile.mjs")], { input: JSON.stringify(config), encoding: "utf8", env: process.env, timeout: 120000 });
    assert.equal(compiled.status, 0, compiled.stderr);
    assert.ok(JSON.parse(compiled.stdout).modules["c:familyConditional"]);
    server = http.createServer((req, res) => {
      const url = new URL(req.url, "http://localhost");
      if (url.pathname === "/") {
        res.setHeader("content-type", "text/html");
        res.end(`<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">{"imports":{"lwc":"/engine.js","@lwc/synthetic-shadow":"/shadow.js","c/familyConditional":"/modules/c/familyConditional/familyConditional.js"}}</script><main></main><script type="module">import "@lwc/synthetic-shadow";import {createElement} from "lwc";import FamilyConditional from "c/familyConditional";document.querySelector("main").appendChild(createElement("c-family-conditional",{is:FamilyConditional}));</script>`);
        return;
      }
      let file;
      if (url.pathname === "/engine.js") file = path.join(toolchain, "node_modules/@lwc/engine-dom/dist/index.js");
      else if (url.pathname === "/shadow.js") file = path.join(toolchain, "node_modules/@lwc/synthetic-shadow/dist/index.js");
      else if (url.pathname.startsWith("/modules/")) {
        const target = path.resolve(outDir, url.pathname.slice(9));
        if (target.startsWith(outDir + path.sep)) file = target;
      }
      if (!file || !fs.existsSync(file)) { res.writeHead(404); res.end(); return; }
      res.setHeader("content-type", "application/javascript");
      res.end(fs.readFileSync(file));
    });
    await new Promise((resolve, reject) => { server.once("error", reject); server.listen(0, "127.0.0.1", resolve); });
    browser = await chromium.launch({ headless: true, ...(process.env.GLADE_LWC_BROWSER_EXECUTABLE ? { executablePath: process.env.GLADE_LWC_BROWSER_EXECUTABLE } : {}) });
    const page = await browser.newPage();
    const errors = [];
    page.on("pageerror", error => errors.push(error.message));
    await page.goto(`http://127.0.0.1:${server.address().port}/`);
    const sections = page.locator("section[data-case]");
    await sections.last().waitFor();
    const before = await sections.evaluateAll(nodes => Object.fromEntries(nodes.map(n => [n.dataset.case, n.textContent])));
    const runtime = cases.filter(c => !c.errorCode);
    assert.equal(Object.keys(before).length, runtime.length);
    for (const c of runtime) assert.equal(before[c.id], c.initialExpected, c.id + " initial");
    await page.getByRole("button", { name: "Advance", exact: true }).click();
    await page.locator('section[data-case="reactive_chain_00"]').getByText("A", { exact: true }).waitFor();
    const after = await sections.evaluateAll(nodes => Object.fromEntries(nodes.map(n => [n.dataset.case, { text: n.textContent, tags: Array.from(n.querySelectorAll("span,template")).map(e => e.tagName.toLowerCase()) }])));
    for (const c of runtime) {
      assert.equal(after[c.id].text, c.expected, c.id);
      assert.deepEqual(after[c.id].tags, c.expectedTags, c.id + " elements");
    }
    assert.deepEqual(errors, []);
  } finally {
    await browser?.close();
    if (server) await new Promise(resolve => server.close(resolve));
    fs.rmSync(outDir, { recursive: true, force: true });
  }
});

test("API 67 compiler rejects the same invalid conditional templates as Salesforce", () => {
  for (const c of cases.filter(c => c.errorCode)) {
    let diagnostic = "";
    try {
      const result = transformSync(c.template, "probe.html", { namespace: "c", name: "probe", apiVersion: 67 });
      diagnostic = JSON.stringify(result.warnings || []);
    } catch (error) {
      diagnostic = error.message;
    }
    assert.match(diagnostic, new RegExp(c.errorCode), c.id);
  }
});
