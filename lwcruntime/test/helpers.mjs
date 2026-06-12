import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
export const repoRoot = path.resolve(__dirname, "../..");
const wireAdapterPath = path.join(repoRoot, "lwcruntime/src/shims/wire-adapter.mjs");

export const salesforceImportMap = {
  "@salesforce/apex/": "/lightning/shims/apex/",
  "@salesforce/label/": "/lightning/shims/label/",
  "@salesforce/schema/": "/lightning/shims/schema/",
  "@salesforce/resourceUrl/": "/lightning/shims/resourceUrl/",
  "lightning/uiRecordApi": "/lightning/shims/lightning/uiRecordApi.js",
};

export function compileFixture(fixtureRel, outDir) {
  const fixture = path.join(repoRoot, fixtureRel);
  const config = JSON.stringify({
    projectRoot: fixture,
    outDir,
    namespace: "c",
    lwcFiles: walkFiles(fixture, ".js"),
    lwcHtmlFiles: walkFiles(fixture, ".html"),
  });
  const result = spawnSync("node", ["compile.mjs"], {
    cwd: path.join(repoRoot, "third_party/lwc"),
    input: config,
    encoding: "utf8",
  });
  if (result.status !== 0) {
    throw new Error(result.stderr || result.stdout || "compile failed");
  }
  return JSON.parse(result.stdout);
}

function walkFiles(root, suffix) {
  const out = [];
  const stack = [root];
  while (stack.length) {
    const dir = stack.pop();
    for (const name of fs.readdirSync(dir)) {
      const full = path.join(dir, name);
      const stat = fs.statSync(full);
      if (stat.isDirectory()) {
        stack.push(full);
        continue;
      }
      if (name.endsWith(suffix) && full.includes(`${path.sep}lwc${path.sep}`)) {
        out.push(path.relative(root, full));
      }
    }
  }
  return out;
}

function apexShimJS(className, methodName) {
  return `import { createApexWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export default createApexWireAdapter(${JSON.stringify(className)}, ${JSON.stringify(methodName)});`;
}

function schemaShimJS(objectName, fieldName) {
  const token = `${objectName}.${fieldName}`;
  return `const token = {
  fieldApiName: ${JSON.stringify(fieldName)},
  objectApiName: ${JSON.stringify(objectName)},
  toString() { return ${JSON.stringify(token)}; },
};
export default token;`;
}

function uiRecordApiJS() {
  return `import { createGetRecordWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export const getRecord = createGetRecordWireAdapter();`;
}

function handleWireRequest(url, req, res, wireHandlers) {
  let body = "";
  req.on("data", (chunk) => {
    body += chunk;
  });
  req.on("end", () => {
    const payload = body ? JSON.parse(body) : {};
    const handler = wireHandlers[url.pathname];
    const result = handler ? handler(payload) : { error: { message: "unknown wire endpoint" } };
    res.writeHead(200, { "Content-Type": "application/json; charset=utf-8" });
    res.end(JSON.stringify(result));
  });
}

function handleShimRequest(url, res, shimConfig) {
  const pathname = url.pathname;
  if (pathname === "/lightning/shims/core/wire-adapter.js") {
    res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
    res.end(fs.readFileSync(wireAdapterPath));
    return true;
  }
  if (pathname === "/lightning/shims/lightning/uiRecordApi.js") {
    res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
    res.end(uiRecordApiJS());
    return true;
  }
  if (pathname.startsWith("/lightning/shims/apex/")) {
    const token = pathname.slice("/lightning/shims/apex/".length).replace(/\.js$/, "");
    const dot = token.lastIndexOf(".");
    const className = token.slice(0, dot);
    const methodName = token.slice(dot + 1);
    res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
    res.end(apexShimJS(className, methodName));
    return true;
  }
  if (pathname.startsWith("/lightning/shims/schema/")) {
    const token = pathname.slice("/lightning/shims/schema/".length).replace(/\.js$/, "");
    const dot = token.lastIndexOf(".");
    const objectName = token.slice(0, dot);
    const fieldName = token.slice(dot + 1);
    res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
    res.end(schemaShimJS(objectName, fieldName));
    return true;
  }
  if (pathname.startsWith("/lightning/shims/label/")) {
    const token = pathname.slice("/lightning/shims/label/".length).replace(/\.js$/, "");
    const value = shimConfig.labels?.[token] ?? token;
    res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
    res.end(`export default ${JSON.stringify(value)};\n`);
    return true;
  }
  return false;
}

export function startLightningServer({
  compiledDir,
  gladeOutJS,
  pages = {},
  wireHandlers = {},
  shimConfig = {},
}) {
  const vendorRoot = path.join(repoRoot, "third_party/lwc/node_modules");
  const htmlPages = { ...pages };
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, "http://localhost");
    if (url.pathname.startsWith("/lightning/wire/") && req.method === "POST") {
      handleWireRequest(url, req, res, wireHandlers);
      return;
    }
    if (handleShimRequest(url, res, shimConfig)) {
      return;
    }
    let filePath = "";
    if (url.pathname === "/lightning/glade.out.js") {
      filePath = gladeOutJS;
    } else if (url.pathname === "/lightning/vendor/lwc.js") {
      filePath = path.join(vendorRoot, "@lwc/engine-dom/dist/index.js");
    } else if (url.pathname === "/lightning/vendor/synthetic-shadow.js") {
      filePath = path.join(vendorRoot, "@lwc/synthetic-shadow/dist/index.js");
    } else if (url.pathname.startsWith("/lightning/modules/")) {
      const rel = url.pathname.slice("/lightning/modules/".length);
      filePath = path.join(compiledDir, rel);
    } else if (url.pathname === "/test.html" || htmlPages[url.pathname]) {
      const html = htmlPages[url.pathname] || "<!DOCTYPE html><html><head></head><body></body></html>";
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(html);
      return;
    } else {
      res.writeHead(404);
      res.end("not found");
      return;
    }
    if (!fs.existsSync(filePath)) {
      res.writeHead(404);
      res.end("missing " + filePath);
      return;
    }
    res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
    res.end(fs.readFileSync(filePath));
  });
  return new Promise((resolve) => {
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      const handle = {
        port,
        baseURL: `http://127.0.0.1:${port}`,
        pages: htmlPages,
        close: () => new Promise((r) => server.close(() => r())),
      };
      resolve(handle);
    });
  });
}

function localLWCImportMap(config, origin) {
  const imports = {};
  const modules = config?.manifest?.modules || {};
  for (const entry of Object.values(modules)) {
    const url = entry?.url || "";
    const prefix = "/lightning/modules/";
    if (!url.startsWith(prefix)) {
      continue;
    }
    const parts = url.slice(prefix.length).split("/");
    if (parts.length < 3) {
      continue;
    }
    const moduleNS = parts[0];
    const component = parts[parts.length - 2];
    const absolute = `${origin}${url}`;
    imports[`${moduleNS}/${component}`] = absolute;
    if (moduleNS !== "c") {
      imports[`c/${component}`] = absolute;
    }
  }
  return imports;
}

export function harnessHTML(baseURL, config, moduleScript) {
  const origin = baseURL || "http://localhost";
  const imports = {
    lwc: `${origin}/lightning/vendor/lwc.js`,
    "@lwc/synthetic-shadow": `${origin}/lightning/vendor/synthetic-shadow.js`,
  };
  for (const [key, value] of Object.entries(salesforceImportMap)) {
    imports[key] = `${origin}${value}`;
  }
  for (const [key, value] of Object.entries(localLWCImportMap(config, origin))) {
    imports[key] = value;
  }
  return `<!DOCTYPE html>
<html>
<head>
  <script>window.process = { env: { NODE_ENV: "production" } };</script>
  <script type="importmap">${JSON.stringify({ imports })}</script>
  <script type="application/json" id="glade-lightning-config">${JSON.stringify(config)}</script>
  <script>window.__gladeLightningPending=window.__gladeLightningPending||[];window.$Lightning=window.$Lightning||{use:function(a,c){window.__gladeLightningPending.push(["use",a,c]);},createComponent:function(q,p,l,c){window.__gladeLightningPending.push(["create",q,p,l,c]);}};</script>
</head>
<body>
  <div id="host"></div>
  <script type="module">${moduleScript}</script>
</body>
</html>`;
}
