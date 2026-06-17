import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import { repoRoot } from "./helpers.mjs";

function serveShellServiceFile(urlPath, res) {
  const normalizedPath = urlPath.replace(/\.mjs$/, ".js");
  const routes = {
    "/lightning/runtime/shell/navigation-service.js": path.join(repoRoot, "lwcruntime/src/shell/navigation-service.mjs"),
    "/lightning/runtime/shell/toast-service.js": path.join(repoRoot, "lwcruntime/src/shell/toast-service.mjs"),
    "/lightning/runtime/shell/message-service.js": path.join(repoRoot, "lwcruntime/src/shell/message-service.mjs"),
    "/lightning/runtime/shell/diagnostics.js": path.join(repoRoot, "lwcruntime/src/shell/diagnostics.mjs"),
  };
  const filePath = routes[normalizedPath];
  if (!filePath || !fs.existsSync(filePath)) {
    res.writeHead(404);
    res.end("missing " + urlPath);
    return;
  }
  res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
  res.end(fs.readFileSync(filePath));
}

function startShellServiceServer() {
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, "http://localhost");
    if (url.pathname === "/services.html") {
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(`<!DOCTYPE html>
<html>
<head>
  <script type="application/json" id="glade-lightning-config">${JSON.stringify({
    pageReference: {
      type: "standard__recordPage",
      attributes: { objectApiName: "Account", recordId: "001000000000001AAA" },
      state: {},
    },
  })}</script>
  <script type="application/json" id="glade-lwc-context">${JSON.stringify({
    kind: "communityPage",
    pageName: "Account",
    community: {
      site: "Partner_Portal",
      basePath: "/partners",
      siteId: "0DM000000000001",
      networkId: "0DB000000000001",
      guest: true,
      language: "en-US",
    },
  })}</script>
</head>
<body><div id="host"></div><script type="module" src="/test-entry.js"></script></body>
</html>`);
      return;
    }
    if (url.pathname === "/test-entry.js") {
      res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
      res.end(`
import {
  CurrentPageReferenceAdapter,
  generateUrl,
  navigate,
} from "/lightning/runtime/shell/navigation-service.js";
import {
  clearToasts,
  getToasts,
  installToastService,
  recordToast,
} from "/lightning/runtime/shell/toast-service.js";
import {
  MessageContext,
  publish,
  subscribe,
  unsubscribe,
} from "/lightning/runtime/shell/message-service.js";

window.__serviceResults = {};
window.__serviceResults.urls = {
  record: await generateUrl({ type: "standard__recordPage", attributes: { objectApiName: "Account", recordId: "001000000000001AAA" } }),
  relationship: await generateUrl({ type: "standard__recordRelationshipPage", attributes: { objectApiName: "Account", recordId: "001000000000001AAA", relationshipApiName: "Contacts" } }),
  navItem: await generateUrl({ type: "standard__navItemPage", attributes: { apiName: "Accounts" } }),
  app: await generateUrl({ type: "standard__app", attributes: { appTarget: "standard__Sales" } }),
  nestedApp: await generateUrl({ type: "standard__app", attributes: { appTarget: "standard__Sales", pageRef: { type: "standard__recordPage", attributes: { objectApiName: "Account", recordId: "001000000000001AAA" }, state: { c__mode: "demo" } } } }),
  named: await generateUrl({ type: "standard__namedPage", attributes: { pageName: "home" } }),
  component: await generateUrl({ type: "standard__component", attributes: { componentName: "c__helloWorld" } }),
  quickAction: await generateUrl({ type: "standard__quickAction", attributes: { apiName: "Account.NewTask", objectApiName: "Account", recordId: "001000000000001AAA" } }),
  webPage: await generateUrl({ type: "standard__webPage", attributes: { url: "/apex/AccountHost" } }),
  communityNamed: await generateUrl({ type: "comm__namedPage", attributes: { name: "Account" }, state: { c__view: "summary" } }),
  communityLogin: await generateUrl({ type: "comm__loginPage", attributes: { actionName: "login" } }),
  communityManagedContent: await generateUrl({ type: "comm__managedContentPage", attributes: { contentKey: "welcome" } }),
  communityRecord: await generateUrl({ type: "comm__recordPage", attributes: { objectApiName: "Account", recordId: "001000000000001AAA", actionName: "view" } }),
  communityRelationship: await generateUrl({ type: "comm__recordRelationshipPage", attributes: { objectApiName: "Account", recordId: "001000000000001AAA", relationshipApiName: "Contacts" } }),
};
try {
  await generateUrl({ type: "standard__objectPage", attributes: { objectApiName: "Account", actionName: "new" } });
} catch (err) {
  window.__serviceResults.objectError = { code: err.code, message: err.message };
}
try {
  await generateUrl({ type: "standard__quickAction", attributes: { apiName: "Account.NewTask" } });
} catch (err) {
  window.__serviceResults.quickActionError = { code: err.code, message: err.message };
}
try {
  await generateUrl({ type: "comm__unsupportedPage", attributes: { name: "Other" } });
} catch (err) {
  window.__serviceResults.communityError = { code: err.code, message: err.message };
}
const pageRefs = [];
const adapter = new CurrentPageReferenceAdapter((pageRef) => pageRefs.push(pageRef));
adapter.connect();
window.__serviceResults.pageRefs = pageRefs;
window.__assignedUrl = null;
await navigate({ type: "standard__navItemPage", attributes: { apiName: "Reports" } }, { assign: (nextUrl) => { window.__assignedUrl = nextUrl; } });
window.__serviceResults.assignedUrl = window.__assignedUrl;

clearToasts();
const disposeToasts = installToastService(document.body);
document.dispatchEvent(new CustomEvent("lightning__showtoast", {
  bubbles: true,
  composed: true,
  detail: { title: "Saved", message: "Account updated", variant: "success" },
}));
recordToast({ title: "Direct", message: "Captured", variant: "info" });
window.__serviceResults.toasts = getToasts();
window.__serviceResults.toastText = document.querySelector("[data-glade-toast-region]")?.textContent || "";
disposeToasts();

const messages = [];
const ctx = new MessageContext();
const sub = subscribe(ctx, { name: "AccountChannel" }, (message) => messages.push(message));
publish(ctx, { name: "AccountChannel" }, { recordId: "001000000000001AAA" });
unsubscribe(sub);
publish(ctx, { name: "AccountChannel" }, { recordId: "ignored" });
window.__serviceResults.messages = messages;
`);
      return;
    }
    serveShellServiceFile(url.pathname, res);
  });
  return new Promise((resolve) => {
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      resolve({
        baseURL: `http://127.0.0.1:${port}`,
        close: () => new Promise((r) => server.close(r)),
      });
    });
  });
}

test("navigation services generate URLs and stable unsupported errors", async () => {
  const server = await startShellServiceServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/services.html`, { waitUntil: "networkidle" });
    const results = await page.evaluate(() => window.__serviceResults);
    assert.deepEqual(results.urls, {
      record: "/lwc/preview/record/Account/001000000000001AAA",
      relationship: "/lwc/preview/record/Account/001000000000001AAA?relationship=Contacts",
      navItem: "/lwc/preview/tab/Accounts",
      app: "/lwc/preview/app/standard__Sales",
      nestedApp: "/lwc/preview/record/Account/001000000000001AAA?c__mode=demo&app=standard__Sales",
      named: "/lwc/preview/home",
      component: "/lwc/preview/cmp/c/helloWorld",
      quickAction: "/lwc/preview/action/Account/001000000000001AAA/NewTask",
      webPage: "/apex/AccountHost",
      communityNamed: "/lwc/preview/community/Partner_Portal/Account?c__view=summary",
      communityLogin: "/lwc/preview/community/Partner_Portal/login",
      communityManagedContent: "/lwc/preview/community/Partner_Portal/Account?contentKey=welcome&contentType=managedContent",
      communityRecord: "/lwc/preview/community/Partner_Portal/Account?recordId=001000000000001AAA&objectApiName=Account&actionName=view",
      communityRelationship: "/lwc/preview/community/Partner_Portal/Account?recordId=001000000000001AAA&objectApiName=Account&relationship=Contacts",
    });
    assert.equal(results.objectError.code, "GLADELWC042");
    assert.equal(results.quickActionError.code, "GLADELWC041");
    assert.equal(results.communityError.code, "GLADELWC103");
    assert.equal(results.pageRefs[0].type, "standard__recordPage");
    assert.equal(results.assignedUrl, "/lwc/preview/tab/Reports");
  } finally {
    await browser.close();
    await server.close();
  }
});

test("services capture toasts and deliver in-page messages", async () => {
  const server = await startShellServiceServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/services.html`, { waitUntil: "networkidle" });
    const results = await page.evaluate(() => window.__serviceResults);
    assert.equal(results.toasts.length, 2);
    assert.equal(results.toasts[0].title, "Saved");
    assert.match(results.toastText, /Account updated/);
    assert.deepEqual(results.messages, [{ recordId: "001000000000001AAA" }]);
  } finally {
    await browser.close();
    await server.close();
  }
});
