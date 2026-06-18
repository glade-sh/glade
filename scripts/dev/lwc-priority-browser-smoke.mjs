#!/usr/bin/env node
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import path from "node:path";

const require = createRequire(import.meta.url);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
let chromium;
try {
  ({ chromium } = require("playwright"));
} catch (_err) {
  ({ chromium } = require(path.join(repoRoot, "lwcruntime/node_modules/playwright")));
}

const args = process.argv.slice(2);
const discover = args.includes("--discover");
const targets = args.filter((arg) => arg !== "--discover");
if (!targets.length) {
  throw new Error("usage: node scripts/dev/lwc-priority-browser-smoke.mjs [--discover] <url>...");
}

const browser = await chromium.launch();
let failed = false;
const urls = discover ? await discoverRouteURLs(targets) : targets;
for (const url of urls) {
  const page = await browser.newPage();
  const messages = [];
  page.on("console", (msg) => {
    if (["error", "warning"].includes(msg.type())) {
      messages.push(`${msg.type()}: ${msg.text()}`);
    }
  });
  page.on("pageerror", (err) => messages.push(`pageerror: ${err.message}`));
  const response = await page.goto(url, { waitUntil: "networkidle" });
  const missing = await page.locator("[data-glade-diagnostic-severity='error']").count();
  if (!response?.ok() || messages.some((line) => /GLADELWC081|404|failed to load resource/i.test(line)) || missing > 0) {
    failed = true;
    console.error(JSON.stringify({ url, status: response?.status(), messages, diagnostics: missing }, null, 2));
  } else {
    console.log(`ok ${url}`);
  }
  await page.close();
}
await browser.close();
process.exit(failed ? 1 : 0);

async function discoverRouteURLs(rootURLs) {
  const out = [];
  for (const rootURL of rootURLs) {
    const page = await browser.newPage();
    const response = await page.goto(rootURL, { waitUntil: "networkidle" });
    if (!response?.ok()) {
      failed = true;
      console.error(JSON.stringify({ url: rootURL, status: response?.status(), messages: ["root discovery failed"] }, null, 2));
      await page.close();
      continue;
    }
    out.push(rootURL);
    const hrefs = await page.locator("[data-glade-route-link]").evaluateAll((links) =>
      links.map((link) => link.href).filter(Boolean),
    );
    out.push(...hrefs);
    await page.close();
  }
  return [...new Set(out)];
}
