import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import { repoRoot } from "./helpers.mjs";

function visualforceAjaxScriptFromGo() {
  const source = fs.readFileSync(path.join(repoRoot, "internal/visualforce/ajax.go"), "utf8");
  const start = source.indexOf("return `<script>");
  const end = source.indexOf("\n}\n\nfunc VisualforceAjaxSubmitHook", start);
  assert.notEqual(start, -1, "VisualforceAjaxScript return not found");
  assert.notEqual(end, -1, "VisualforceAjaxScript end not found");
  const expr = source.slice(start + "return ".length, end);
  const parts = [];
  const token = /`([\s\S]*?)`|ViewStateActionFieldName\(\)|ViewStateFormFieldName\(\)/g;
  for (const match of expr.matchAll(token)) {
    if (match[1] !== undefined) {
      parts.push(match[1]);
    } else if (match[0] === "ViewStateActionFieldName()") {
      parts.push("__vf_action");
    } else if (match[0] === "ViewStateFormFieldName()") {
      parts.push("__vf_viewstate");
    }
  }
  return parts.join("");
}

test("Visualforce AJAX script scopes actionRegion fields and toggles actionStatus", async (t) => {
  let browser;
  try {
    browser = await chromium.launch({ headless: true });
  } catch (err) {
    t.skip(`cannot launch chromium: ${err.message}`);
    return;
  }

  const page = await browser.newPage();
  try {
    await page.setContent(`<!DOCTYPE html>
<html>
<head>${visualforceAjaxScriptFromGo()}</head>
<body>
  <form id="f" action="/apex/Ajax">
    <input type="hidden" name="__vf_action" value="" />
    <input type="hidden" name="__vf_viewstate" value="view-state" />
    <input type="hidden" name="__vf_csrf" value="csrf-token" />
    <input name="outside" value="wide" />
    <span id="editor" data-vf-region="editor">
      <input name="inside" value="narrow" />
    </span>
  </form>
  <span class="actionStatus" data-status="saveStatus">
    <span class="actionStatusStart" hidden>Saving</span>
    <span class="actionStatusStop">Saved</span>
  </span>
  <button id="submit" onclick="window.GLADEVF.submit(document.getElementById('f'), '{!save}', 'count', {status:'saveStatus', region:document.getElementById('editor'), params:[{name:'delta', value:'5'}]}); return false;">Save</button>
</body>
</html>`);

    await page.evaluate(() => {
      window.__posted = "";
      window.__resolveFetch = null;
      window.fetch = (_url, init) => {
        window.__posted = String(init.body);
        return new Promise((resolve) => {
          window.__resolveFetch = () => resolve({
            json: () => Promise.resolve({
              targets: {},
              viewState: "next-view-state",
              messages: [],
            }),
          });
        });
      };
    });

    assert.equal(await page.locator(".actionStatusStart").evaluate((el) => el.hidden), true);
    assert.equal(await page.locator(".actionStatusStop").evaluate((el) => el.hidden), false);

    await page.locator("#submit").click();
    await page.waitForFunction(() => window.__posted.includes("__vf_ajax=1"));

    const posted = await page.evaluate(() => window.__posted);
    assert.match(posted, /inside=narrow/);
    assert.match(posted, /delta=5/);
    assert.match(posted, /__vf_csrf=csrf-token/);
    assert.match(posted, /__vf_viewstate=view-state/);
    assert.doesNotMatch(posted, /outside=wide/);
    assert.equal(await page.locator(".actionStatusStart").evaluate((el) => el.hidden), false);
    assert.equal(await page.locator(".actionStatusStop").evaluate((el) => el.hidden), true);

    await page.evaluate(() => window.__resolveFetch());
    await page.waitForFunction(() => document.querySelector(".actionStatusStart").hidden);
    assert.equal(await page.locator(".actionStatusStop").evaluate((el) => el.hidden), false);
    assert.equal(await page.locator('input[name="__vf_viewstate"]').inputValue(), "next-view-state");
  } finally {
    await page.close();
    await browser.close();
  }
});
