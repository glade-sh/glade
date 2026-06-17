import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import { repoRoot } from "./helpers.mjs";

function serveRuntimeFile(urlPath, res) {
  const routes = {
    "/lightning/runtime/shims/community.js": path.join(repoRoot, "lwcruntime/src/shims/community.mjs"),
    "/lightning/runtime/shims/community.mjs": path.join(repoRoot, "lwcruntime/src/shims/community.mjs"),
    "/lightning/runtime/shims/site.js": path.join(repoRoot, "lwcruntime/src/shims/site.mjs"),
    "/lightning/runtime/shims/site.mjs": path.join(repoRoot, "lwcruntime/src/shims/site.mjs"),
    "/lightning/runtime/shell/community-host.js": path.join(repoRoot, "lwcruntime/src/shell/community-host.mjs"),
    "/lightning/runtime/shell/community-host.mjs": path.join(repoRoot, "lwcruntime/src/shell/community-host.mjs"),
    "/lightning/runtime/shell/diagnostics.js": path.join(repoRoot, "lwcruntime/src/shell/diagnostics.mjs"),
    "/lightning/runtime/shell/diagnostics.mjs": path.join(repoRoot, "lwcruntime/src/shell/diagnostics.mjs"),
  };
  const filePath = routes[urlPath];
  if (!filePath || !fs.existsSync(filePath)) {
    res.writeHead(404);
    res.end("missing " + urlPath);
    return;
  }
  res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
  res.end(fs.readFileSync(filePath));
}

function startCommunityServer(context) {
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, "http://localhost");
    if (url.pathname === "/community.html") {
      const contextScript =
        context === undefined
          ? ""
          : `<script type="application/json" id="glade-lwc-context">${JSON.stringify(context)}</script>`;
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(`<!DOCTYPE html>
<html>
<body data-glade-community-shell>
  ${contextScript}
  <script type="module" src="/test-entry.js"></script>
</body>
</html>`);
      return;
    }
    if (url.pathname === "/test-entry.js") {
      res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
      res.end(`
import {
  readCommunityContext,
  readCommunityValue,
} from "/lightning/runtime/shims/community.js";
import {
  readSiteId,
} from "/lightning/runtime/shims/site.js";
import {
  applyCommunityHost,
} from "/lightning/runtime/shell/community-host.js";
import {
  diagnostics,
} from "/lightning/runtime/shell/diagnostics.js";

const host = applyCommunityHost(document.body);
window.__communityResults = {
  context: readCommunityContext(),
  basePath: readCommunityValue("basePath", "/s"),
  networkId: readCommunityValue("networkId", ""),
  siteId: readSiteId(),
	  host,
	  bodyDataset: { ...document.body.dataset },
	  diagnostics,
	};
	`);
      return;
    }
    serveRuntimeFile(url.pathname, res);
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

test("community runtime shims read local shell context and host dataset", async () => {
  const server = await startCommunityServer({
    kind: "communityPage",
    community: {
      site: "Partner_Portal",
      basePath: "/partners",
      siteId: "0DM000000000001",
      networkId: "0DB000000000001",
      guest: true,
      language: "en-US",
    },
  });
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/community.html`, { waitUntil: "networkidle" });
    const results = await page.evaluate(() => window.__communityResults);
    assert.equal(results.basePath, "/partners");
    assert.equal(results.networkId, "0DB000000000001");
    assert.equal(results.siteId, "0DM000000000001");
    assert.equal(results.context.guest, true);
    assert.equal(results.host.site, "Partner_Portal");
    assert.equal(results.bodyDataset.gladeCommunityGuest, "true");
  } finally {
    await browser.close();
    await server.close();
  }
});

test("community runtime shims default basePath and empty IDs", async () => {
  const server = await startCommunityServer({
    kind: "communityPage",
    community: { site: "Partner_Portal" },
  });
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/community.html`, { waitUntil: "networkidle" });
    const results = await page.evaluate(() => window.__communityResults);
    assert.equal(results.basePath, "/s");
	    assert.equal(results.networkId, "");
	    assert.equal(results.siteId, "");
	    assert.equal(results.bodyDataset.gladeCommunitySite, "Partner_Portal");
	    assert.ok(results.diagnostics.some((entry) => entry.code === "GLADELWC102"));
	  } finally {
	    await browser.close();
	    await server.close();
	  }
});

test("community runtime shims report missing community context", async () => {
  const server = await startCommunityServer(undefined);
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/community.html`, { waitUntil: "networkidle" });
    const results = await page.evaluate(() => window.__communityResults);
    assert.equal(results.basePath, "/s");
    assert.equal(results.networkId, "");
    assert.equal(results.siteId, "");
    assert.ok(results.diagnostics.some((entry) => entry.code === "GLADELWC100"));
    assert.ok(results.diagnostics.some((entry) => entry.code === "GLADELWC102"));
  } finally {
    await browser.close();
    await server.close();
  }
});
