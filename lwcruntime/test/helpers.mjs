import fs from "node:fs";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import { spawn, spawnSync } from "node:child_process";
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
  if (pathname.startsWith("/lightning/shims/resourceUrl/")) {
    const token = pathname.slice("/lightning/shims/resourceUrl/".length).replace(/\.js$/, "");
    const value = shimConfig.resources?.[token] ?? `/resource/${token}`;
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

function lightningStubJS() {
  return `(function(){` +
    `function n(v){return String(v||"").trim().toLowerCase();}` +
    `function c(){var el=document.getElementById("glade-lightning-config");if(!el){return {outApps:[],manifest:{modules:{}}};}try{return JSON.parse(el.textContent||"{}");}catch(_e){return {outApps:[],manifest:{modules:{}}};}}` +
    `function m(cfg){return cfg&&cfg.manifest&&cfg.manifest.modules||{};}` +
    `function e(cb,msg){if(typeof cb==="function"){cb(null,"ERROR",msg);}}` +
    `window.__gladeLightningPending=window.__gladeLightningPending||[];` +
    `window.$Lightning=window.$Lightning||{` +
    `use:function(a,cb){var cfg=c();var apps=(cfg.outApps||[]).map(n);if(apps.indexOf(n(a))===-1){console.error("[glade] Lightning Out app not found",a);e(cb,"Lightning Out app not found: "+a);return;}window.__gladeLightningPending.push(["use",a,cb]);},` +
    `createComponent:function(q,p,l,cb){var cfg=c();if(!m(cfg)[n(q)]){console.error("[glade] Lightning component not found",q);e(cb,"Lightning component not found: "+q);return;}window.__gladeLightningPending.push(["create",q,p,l,cb]);}` +
    `};` +
    `})();`;
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
  <script>${lightningStubJS()}</script>
</head>
<body>
  <div id="host"></div>
  <script type="module">${moduleScript}</script>
</body>
</html>`;
}

export async function startVisualforceDevServer(t, { projectRel, pagePath = "/apex/MultiWidgetHost" } = {}) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-vf-server-"));
  const binary = path.join(tmpDir, process.platform === "win32" ? "glade.exe" : "glade");
  const build = spawnSync("go", ["build", "-o", binary, "./cmd/glade"], {
    cwd: repoRoot,
    encoding: "utf8",
  });
  if (build.status !== 0) {
    t.skip(`cannot build local glade binary: ${buildFailureSummary(build.stderr || build.stdout)}`);
    return null;
  }

  const readyFile = path.join(tmpDir, "ready.json");
  const projectRoot = path.join(repoRoot, projectRel || ".");
  const child = spawn(
    binary,
    ["dev", "vf", "--project", projectRoot, "--addr", "127.0.0.1:0", "--ready-file", readyFile],
    {
      cwd: repoRoot,
      env: { ...process.env, GLADE_HOME: repoRoot },
      stdio: ["ignore", "pipe", "pipe"],
    },
  );
  let stdout = "";
  let stderr = "";
  child.stdout.on("data", (chunk) => {
    stdout += String(chunk);
  });
  child.stderr.on("data", (chunk) => {
    stderr += String(chunk);
  });

  try {
    const ready = await waitForReadyFile(readyFile, child, () => stdout + stderr);
    await waitForHTTP(`${ready.url}${pagePath}`, child, () => stdout + stderr);
    return {
      baseURL: ready.url,
      pages: ready.pages || [],
      close: () => stopProcess(child),
    };
  } catch (err) {
    await stopProcess(child);
    throw err;
  }
}

export async function startLWCDevServer(t, {
  projectRel,
  pagePath = "/lwc/preview/component/c/contextProbe",
} = {}) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-server-"));
  const binary = path.join(tmpDir, process.platform === "win32" ? "glade.exe" : "glade");
  const build = spawnSync("go", ["build", "-o", binary, "./cmd/glade"], {
    cwd: repoRoot,
    encoding: "utf8",
  });
  if (build.status !== 0) {
    t.skip(`cannot build local glade binary: ${buildFailureSummary(build.stderr || build.stdout)}`);
    return null;
  }

  const readyFile = path.join(tmpDir, "ready.json");
  const projectRoot = path.join(repoRoot, projectRel || ".");
  const child = spawn(
    binary,
    ["dev", "lwc", "--project", projectRoot, "--addr", "127.0.0.1:0", "--ready-file", readyFile],
    {
      cwd: repoRoot,
      env: { ...process.env, GLADE_HOME: repoRoot },
      stdio: ["ignore", "pipe", "pipe"],
    },
  );
  let stdout = "";
  let stderr = "";
  child.stdout.on("data", (chunk) => {
    stdout += String(chunk);
  });
  child.stderr.on("data", (chunk) => {
    stderr += String(chunk);
  });

  try {
    const ready = await waitForReadyFile(readyFile, child, () => stdout + stderr);
    await waitForHTTP(`${ready.url}${pagePath}`, child, () => stdout + stderr);
    return {
      baseURL: ready.url,
      routes: ready.routes || [],
      close: () => stopProcess(child),
    };
  } catch (err) {
    await stopProcess(child);
    throw err;
  }
}

function buildFailureSummary(text) {
  const lines = String(text || "no output").trim().split(/\r?\n/).filter(Boolean);
  const useful = lines.filter((line) => !line.startsWith("# "));
  return (useful.length ? useful : lines).slice(0, 2).join("; ") || "no output";
}

function waitForReadyFile(filePath, child, readOutput) {
  const deadline = Date.now() + 30000;
  return new Promise((resolve, reject) => {
    const timer = setInterval(() => {
      if (child.exitCode !== null) {
        clearInterval(timer);
        reject(new Error(`Visualforce dev server exited before ready file\n${readOutput()}`));
        return;
      }
      if (fs.existsSync(filePath)) {
        try {
          const ready = JSON.parse(fs.readFileSync(filePath, "utf8"));
          clearInterval(timer);
          resolve(ready);
          return;
        } catch (_err) {
          // The writer may still have the file open. Try again on the next tick.
        }
      }
      if (Date.now() > deadline) {
        clearInterval(timer);
        reject(new Error(`timed out waiting for Visualforce dev server ready file\n${readOutput()}`));
      }
    }, 50);
  });
}

async function waitForHTTP(url, child, readOutput) {
  const deadline = Date.now() + 30000;
  let lastError = "";
  while (Date.now() <= deadline) {
    if (child.exitCode !== null) {
      throw new Error(`Visualforce dev server exited before HTTP readiness\n${readOutput()}`);
    }
    try {
      const response = await fetch(url);
      if (response.ok) {
        await response.arrayBuffer();
        return;
      }
      lastError = `HTTP ${response.status}`;
    } catch (err) {
      lastError = err.message;
    }
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  throw new Error(`timed out waiting for ${url}: ${lastError}\n${readOutput()}`);
}

function stopProcess(child) {
  if (!child || child.exitCode !== null) {
    return Promise.resolve();
  }
  return new Promise((resolve) => {
    const killTimer = setTimeout(() => {
      if (child.exitCode === null) {
        child.kill("SIGKILL");
      }
    }, 2000);
    child.once("exit", () => {
      clearTimeout(killTimer);
      resolve();
    });
    child.kill("SIGTERM");
  });
}
