import { createRequire } from "node:module";

const input = JSON.parse(await new Promise((resolve) => {
  let data = "";
  process.stdin.on("data", (chunk) => { data += chunk; });
  process.stdin.on("end", () => resolve(data));
}));
const { chromium } = createRequire(import.meta.url)(input.module);
const WATCH = ["slds-form-element", "slds-has-error", "slds-combobox", "slds-is-open", "slds-input", "slds-textarea", "slds-is-selected", "slds-not-selected"];
const walk = (node, fn) => { fn(node); for (const key of ["children", "shadowRoots"]) for (const child of node[key] || []) walk(child, fn); };
const attrs = (node) => { const out = {}; const list = node.attributes || []; for (let i = 0; i < list.length; i += 2) out[list[i]] = list[i + 1]; return out; };

const browser = await chromium.launch({ headless: true, executablePath: input.executable || undefined });
try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (error) => errors.push(String(error)));
  await page.goto(input.url);
  await page.waitForFunction(() => window.__probesReady === true, null, { timeout: 15000 }).catch(() => {});
  if (errors.length) throw new Error("page errors: " + errors.join("; "));
  await page.locator('[data-probe="combobox_open"] [role="combobox"]').click();
  await page.waitForTimeout(100);
  const cdp = await page.context().newCDPSession(page);
  const { root } = await cdp.send("DOM.getDocument", { depth: -1, pierce: true });
  const local = {};
  walk(root, (node) => {
    const probe = attrs(node)["data-probe"];
    if (!probe) return;
    const tokens = new Set();
    walk(node, (child) => { for (const token of (attrs(child).class || "").split(/\s+/)) if (token) tokens.add(token); });
    local[probe] = WATCH.filter((token) => tokens.has(token)).join(" ");
  });
  const mismatches = [];
  for (const [id, api, route, result, expected] of input.rows) {
    if (route !== "browser" || result !== "rendered") throw new Error(`unexpected native row ${id} ${api}`);
    if (local[id] !== expected) mismatches.push(`${id} ${api}: local "${local[id] ?? "<missing>"}" native "${expected}"`);
  }
  if (mismatches.length) {
    console.error(mismatches.join("\n"));
    process.exit(1);
  }
} finally {
  await browser.close();
}
