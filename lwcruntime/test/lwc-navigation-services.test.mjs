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
    "/lightning/runtime/shell/emp-service.js": path.join(repoRoot, "lwcruntime/src/shell/emp-service.mjs"),
    "/lightning/runtime/shell/workspace-service.js": path.join(repoRoot, "lwcruntime/src/shell/workspace-service.mjs"),
    "/lightning/runtime/shell/flow-service.js": path.join(repoRoot, "lwcruntime/src/shell/flow-service.mjs"),
    "/lightning/runtime/shell/community-service.js": path.join(repoRoot, "lwcruntime/src/shell/community-service.mjs"),
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
      routeParams: { recordId: "001000000000001AAA" },
      managedContent: {
        welcome: { contentKey: "welcome", title: "Welcome", body: "Local content" },
      },
    },
    flow: {
      apiName: "Membership_Flow",
      inputVariables: { recordId: "001000000000001AAA" },
      availableActions: ["NEXT", "FINISH"],
    },
    workspace: {
      console: true,
      focusedTabId: "workspace-tab-1",
      tabs: [{ tabId: "workspace-tab-1", label: "Account", url: "/lwc/preview/record/Account/001000000000001AAA", workspaceTab: true }],
      utilities: [{ id: "utilityProbe", label: "Utility", componentName: "c:utilityProbe", url: "/lwc/preview/utility/Support_Utility" }],
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
  APPLICATION_SCOPE,
  clearMessages,
  createMessageContext,
  publish,
  subscribe,
  unsubscribe,
} from "/lightning/runtime/shell/message-service.js";
import {
  __gladePublish as publishEmp,
  clearEmpSubscriptions,
  subscribe as subscribeEmp,
  unsubscribe as unsubscribeEmp,
} from "/lightning/runtime/shell/emp-service.js";
import {
  configureWorkspace,
  getAllTabInfo,
  getFocusedTabInfo,
  openTab,
} from "/lightning/runtime/shell/workspace-service.js";
import {
  dispatchStatusChange,
  readFlowContext,
} from "/lightning/runtime/shell/flow-service.js";
import {
  readManagedContent,
  readRouteParam,
} from "/lightning/runtime/shell/community-service.js";

window.__serviceResults = {};
window.__serviceResults.urls = {
  record: await generateUrl({ type: "standard__recordPage", attributes: { objectApiName: "Account", recordId: "001000000000001AAA" } }),
  relationship: await generateUrl({ type: "standard__recordRelationshipPage", attributes: { objectApiName: "Account", recordId: "001000000000001AAA", relationshipApiName: "Contacts" } }),
  flow: await generateUrl({ type: "standard__flow", attributes: { flowApiName: "Membership_Flow" }, state: { c__recordId: "001000000000001AAA" } }),
  externalRecord: await generateUrl({ type: "standard__externalRecordPage", attributes: { objectType: "cms", recordId: "cms-001", actionName: "view" }, state: { c__source: "nav" } }),
  externalRelationship: await generateUrl({ type: "standard__externalRecordRelationshipPage", attributes: { objectType: "cms", recordId: "cms-001", relationshipApiName: "Assets", actionName: "view" }, state: { c__source: "nav" } }),
  knowledgeArticle: await generateUrl({ type: "standard__knowledgeArticlePage", attributes: { articleType: "FAQ__kav", urlName: "reset-password", language: "en_US" }, state: { c__source: "nav" } }),
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
window.__externalAssignedUrl = null;
await navigate({ type: "standard__externalRecordPage", attributes: { objectType: "cms", recordId: "cms-404" }, state: { c__source: "button" } }, { assign: (nextUrl) => { window.__externalAssignedUrl = nextUrl; } });
window.__serviceResults.externalAssignedUrl = window.__externalAssignedUrl;

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

const scopedMessages = [];
const appContext = createMessageContext();
const scopedSub = subscribe(appContext, { messageChannelName: "ScopedChannel" }, (message) => scopedMessages.push(message), { scope: APPLICATION_SCOPE });
publish(appContext, { messageChannelName: "ScopedChannel" }, { id: "one" });
unsubscribe(scopedSub);
publish(appContext, { messageChannelName: "ScopedChannel" }, { id: "ignored" });
window.__serviceResults.scopedMessages = scopedMessages;

clearMessages();
const componentScope = {
  first: [],
  second: [],
  application: [],
};
const firstContext = createMessageContext();
const secondContext = createMessageContext();
subscribe(firstContext, { messageChannelName: "ComponentScopedChannel" }, (message) => componentScope.first.push(message));
subscribe(secondContext, { messageChannelName: "ComponentScopedChannel" }, (message) => componentScope.second.push(message));
subscribe(secondContext, { messageChannelName: "ComponentScopedChannel" }, (message) => componentScope.application.push(message), { scope: APPLICATION_SCOPE });
publish(firstContext, { messageChannelName: "ComponentScopedChannel" }, { id: "first" });
publish(secondContext, { messageChannelName: "ComponentScopedChannel" }, { id: "second" });
window.__serviceResults.componentScope = componentScope;

clearEmpSubscriptions();
const empMessages = [];
const empSub = await subscribeEmp("/event/Local__e", -1, (event) => empMessages.push(event));
publishEmp("/event/Local__e", { payload: { Name__c: "Probe" }, replayId: 1 });
await unsubscribeEmp(empSub);
publishEmp("/event/Local__e", { payload: { Name__c: "Ignored" }, replayId: 2 });
window.__serviceResults.empMessages = empMessages;

configureWorkspace({
  console: true,
  tabs: [{ tabId: "workspace-tab-1", label: "Account", url: "/lwc/preview/record/Account/001000000000001AAA", workspaceTab: true }],
  focusedTabId: "workspace-tab-1",
  utilities: [{ id: "utilityProbe", label: "Utility", componentName: "c:utilityProbe", url: "/lwc/preview/utility/Support_Utility" }],
});
const newTabId = await openTab({ label: "Reports", url: "/lwc/preview/tab/Reports" });
window.__serviceResults.workspace = {
  focused: await getFocusedTabInfo(),
  all: await getAllTabInfo(),
  newTabId,
};

const flowEvents = [];
const flowHost = document.getElementById("host");
flowHost.addEventListener("statuschange", (event) => flowEvents.push(event.detail));
dispatchStatusChange(flowHost, { status: "FINISHED_SCREEN", outputVariables: [{ name: "recordId", value: "001000000000001AAA" }] });
window.__serviceResults.flow = { context: readFlowContext(), events: flowEvents };

window.__serviceResults.community = {
  routeRecordId: readRouteParam("recordId"),
  managedContent: readManagedContent("welcome"),
};
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
      flow: "/lwc/preview/flow/Membership_Flow?c__recordId=001000000000001AAA",
      externalRecord: "/lwc/preview/home?glade__unavailablePageReference=standard__externalRecordPage&glade__recordId=cms-001&glade__objectType=cms&glade__actionName=view&c__source=nav",
      externalRelationship: "/lwc/preview/home?glade__unavailablePageReference=standard__externalRecordRelationshipPage&glade__recordId=cms-001&glade__objectType=cms&glade__relationshipApiName=Assets&glade__actionName=view&c__source=nav",
      knowledgeArticle: "/lwc/preview/home?glade__unavailablePageReference=standard__knowledgeArticlePage&glade__articleType=FAQ__kav&glade__urlName=reset-password&glade__language=en_US&c__source=nav",
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
    assert.equal(results.externalAssignedUrl, "/lwc/preview/home?glade__unavailablePageReference=standard__externalRecordPage&glade__recordId=cms-404&glade__objectType=cms&c__source=button");
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
    assert.deepEqual(results.scopedMessages, [{ id: "one" }]);
    assert.deepEqual(results.componentScope.first, [{ id: "first" }]);
    assert.deepEqual(results.componentScope.second, [{ id: "second" }]);
    assert.deepEqual(results.componentScope.application, [{ id: "first" }, { id: "second" }]);
    assert.deepEqual(results.empMessages, [{ payload: { Name__c: "Probe" }, replayId: 1 }]);
    assert.equal(results.workspace.focused.label, "Reports");
    assert.equal(results.workspace.all.length, 2);
    assert.equal(results.workspace.newTabId, "workspace-tab-2");
    assert.equal(results.flow.context.apiName, "Membership_Flow");
    assert.equal(results.flow.events[0].status, "FINISHED_SCREEN");
    assert.equal(results.community.routeRecordId, "001000000000001AAA");
    assert.equal(results.community.managedContent.title, "Welcome");
  } finally {
    await browser.close();
    await server.close();
  }
});
