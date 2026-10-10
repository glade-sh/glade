import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { repoRoot } from "./helpers.mjs";

const toolchainRoot = process.env.GLADE_LWC_TOOLCHAIN_DIR || path.join(repoRoot, "third_party/lwc");
const nodeModulesRoot = path.join(toolchainRoot, "node_modules");
const apiVersion = 67;
const port = 8942;
const bundles = [
  { name: "scopedSlotsParent", rel: "force-app/main/default/lwc/scopedSlotsParent" },
  { name: "scopedSlotsChild", rel: "force-app/main/default/lwc/scopedSlotsChild" },
];

function scratchTempRoot() {
  const configured = process.env.TMPDIR;
  if (!configured || !path.isAbsolute(configured)) {
    throw new Error("guard-owned TMPDIR is required for scoped-slot fixture output");
  }
  const root = path.resolve(configured);
  if (!fs.statSync(root).isDirectory()) {
    throw new Error("guard-owned TMPDIR must be an existing directory");
  }
  return root;
}

function createWorkspace() {
  return fs.mkdtempSync(path.join(scratchTempRoot(), "glade-lwc-scoped-slots-"));
}

function packageVersion(packageName) {
  const packagePath = path.join(nodeModulesRoot, packageName, "package.json");
  if (!fs.existsSync(packagePath)) {
    throw new Error("required retained LWC package is missing: " + packagePath);
  }
  return JSON.parse(fs.readFileSync(packagePath, "utf8")).version;
}

function writeBundle(projectRoot, bundle, { javascript, html }) {
  const bundleDir = path.join(projectRoot, bundle.rel);
  fs.mkdirSync(bundleDir, { recursive: true });
  fs.writeFileSync(path.join(bundleDir, `${bundle.name}.js-meta.xml`), [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">',
    "  <apiVersion>67.0</apiVersion>",
    "  <isExposed>false</isExposed>",
    "</LightningComponentBundle>",
    "",
  ].join("\n"));
  fs.writeFileSync(path.join(bundleDir, `${bundle.name}.js`), javascript);
  fs.writeFileSync(path.join(bundleDir, `${bundle.name}.html`), html);
}

function compilerConfig(projectRoot, outDir, selectedBundles) {
  return {
    projectRoot,
    outDir,
    namespace: "c",
    lwcFiles: selectedBundles.map(({ name, rel }) => `${rel}/${name}.js`),
    lwcHtmlFiles: selectedBundles.map(({ name, rel }) => `${rel}/${name}.html`),
    lwcMetaFiles: selectedBundles.map(({ name, rel }) => `${rel}/${name}.js-meta.xml`),
    lwcApiVersions: Object.fromEntries(selectedBundles.map(({ rel }) => [rel, apiVersion])),
  };
}

function invokeCompiler(projectRoot, outDir, selectedBundles) {
  const result = spawnSync(
    process.execPath,
    [path.join(repoRoot, "third_party/lwc/compile.mjs")],
    {
      cwd: toolchainRoot,
      env: process.env,
      input: JSON.stringify(compilerConfig(projectRoot, outDir, selectedBundles)),
      encoding: "utf8",
      maxBuffer: 128 * 1024,
      timeout: 120_000,
    },
  );
  if (result.error) throw result.error;
  return result;
}

function requireCompiledBundles(result, selectedBundles) {
  if (result.status !== 0) {
    throw new Error((result.stderr || result.stdout || "LWC compilation failed").slice(0, 4096));
  }
  let compiled;
  try {
    compiled = JSON.parse(result.stdout);
  } catch (error) {
    throw new Error("LWC compiler returned invalid JSON: " + error.message);
  }
  for (const { name } of selectedBundles) {
    if (!compiled.modules?.[`c:${name}`]) {
      throw new Error("compiled scoped-slot module is missing: c:" + name);
    }
  }
}

function writeValidFixture(projectRoot) {
  writeBundle(projectRoot, bundles[0], {
    javascript: `import { LightningElement } from "lwc";
export default class ScopedSlotsParent extends LightningElement {}
`,
    html: `<template>
  <c-scoped-slots-child>
    <template lwc:slot-data="slotData">
      <span data-testid="scoped-slot-value">{slotData.label}</span>
    </template>
  </c-scoped-slots-child>
</template>
`,
  });
  writeBundle(projectRoot, bundles[1], {
    javascript: `import { LightningElement } from "lwc";
export default class ScopedSlotsChild extends LightningElement {
  static renderMode = "light";
  slotData = { label: "child-provided-slot-data" };
}
`,
    html: `<template lwc:render-mode="light">
  <slot lwc:slot-bind={slotData}></slot>
</template>
`,
  });
}

function resolveUnder(root, relativePath) {
  const target = path.resolve(root, relativePath);
  return target === root || target.startsWith(root + path.sep) ? target : null;
}

function contentType(filePath) {
  return filePath.endsWith(".html")
    ? "text/html; charset=utf-8"
    : "application/javascript; charset=utf-8";
}

function pageHTML(origin, framework) {
  const imports = {
    lwc: `${origin}/lightning/vendor/lwc.js`,
    "@lwc/synthetic-shadow": `${origin}/lightning/vendor/synthetic-shadow.js`,
    "c/scopedSlotsParent": `${origin}/lightning/modules/c/scopedSlotsParent/scopedSlotsParent.js`,
    "c/scopedSlotsChild": `${origin}/lightning/modules/c/scopedSlotsChild/scopedSlotsChild.js`,
  };
  return [
    "<!doctype html>",
    '<html><head><meta charset="utf-8"><title>Scoped slot contract</title>',
    '<script>window.process = { env: { NODE_ENV: "production" } };</script>',
    '<script type="importmap">' + JSON.stringify({ imports }) + "</script>",
    "</head><body>",
    '<pre id="framework-profile">' + JSON.stringify(framework) + "</pre>",
    '<main id="host"></main>',
    '<script type="module">',
    'import "@lwc/synthetic-shadow";',
    'import { createElement } from "lwc";',
    'import ScopedSlotsParent from "c/scopedSlotsParent";',
    'document.getElementById("host").appendChild(createElement("c-scoped-slots-parent", { is: ScopedSlotsParent }));',
    "window.__scopedSlotsMounted = true;",
    "</script></body></html>",
  ].join("\n");
}

export async function startScopedSlotsServer() {
  const workspace = createWorkspace();
  const projectRoot = path.join(workspace, "project");
  const compiledRoot = path.join(workspace, "compiled");
  let server;
  try {
    const framework = {
      declaredComponentApiVersion: apiVersion,
      compilerVersion: packageVersion("@lwc/compiler"),
      engineDomVersion: packageVersion("@lwc/engine-dom"),
      syntheticShadowVersion: packageVersion("@lwc/synthetic-shadow"),
      salesforceApiVersion: null,
      versionAxis: "local compiler and engine identity is separate from Salesforce runtime evidence",
    };
    writeValidFixture(projectRoot);
    requireCompiledBundles(invokeCompiler(projectRoot, compiledRoot, bundles), bundles);

    server = http.createServer((req, res) => {
      if (req.method !== "GET" && req.method !== "HEAD") {
        res.writeHead(405);
        res.end();
        return;
      }
      const url = new URL(req.url, `http://127.0.0.1:${port}`);
      if (url.pathname === "/favicon.ico") {
        res.writeHead(204);
        res.end();
        return;
      }
      if (url.pathname === "/scoped-slots.html") {
        const body = pageHTML(`http://127.0.0.1:${port}`, framework);
        res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
        res.end(req.method === "HEAD" ? undefined : body);
        return;
      }

      let relativePath;
      try {
        relativePath = decodeURIComponent(url.pathname);
      } catch {
        res.writeHead(400);
        res.end("invalid path");
        return;
      }
      let filePath = null;
      if (relativePath === "/lightning/vendor/lwc.js") {
        filePath = path.join(nodeModulesRoot, "@lwc/engine-dom/dist/index.js");
      } else if (relativePath === "/lightning/vendor/synthetic-shadow.js") {
        filePath = path.join(nodeModulesRoot, "@lwc/synthetic-shadow/dist/index.js");
      } else if (relativePath.startsWith("/lightning/modules/")) {
        filePath = resolveUnder(compiledRoot, relativePath.slice("/lightning/modules/".length));
      }
      if (!filePath || !fs.existsSync(filePath) || !fs.statSync(filePath).isFile()) {
        res.writeHead(404);
        res.end("not found");
        return;
      }
      res.writeHead(200, { "Content-Type": contentType(filePath) });
      res.end(req.method === "HEAD" ? undefined : fs.readFileSync(filePath));
    });
    await new Promise((resolve, reject) => {
      server.once("error", reject);
      server.listen(port, "127.0.0.1", resolve);
    });
    if (server.address()?.address !== "127.0.0.1" || server.address()?.port !== port) {
      throw new Error("scoped-slot server did not bind the fixed loopback endpoint");
    }

    let closePromise;
    return {
      url: `http://127.0.0.1:${port}/scoped-slots.html`,
      framework,
      close() {
        if (!closePromise) {
          closePromise = new Promise((resolve, reject) => {
            server.close((error) => {
              fs.rmSync(workspace, { recursive: true, force: true });
              if (error) reject(error);
              else resolve();
            });
          });
        }
        return closePromise;
      },
    };
  } catch (error) {
    if (server?.listening) await new Promise((resolve) => server.close(resolve));
    fs.rmSync(workspace, { recursive: true, force: true });
    throw error;
  }
}

export function compileShadowModeDiagnostic() {
  const workspace = createWorkspace();
  try {
    const projectRoot = path.join(workspace, "project");
    const invalidBundle = {
      name: "scopedSlotsShadowMode",
      rel: "force-app/main/default/lwc/scopedSlotsShadowMode",
    };
    writeBundle(projectRoot, invalidBundle, {
      javascript: `import { LightningElement } from "lwc";
export default class ScopedSlotsShadowMode extends LightningElement {
  slotData = { label: "invalid-shadow-slot-data" };
}
`,
      html: `<template>
  <slot lwc:slot-bind={slotData}></slot>
</template>
`,
    });
    const result = invokeCompiler(projectRoot, path.join(workspace, "invalid-compiled"), [invalidBundle]);
    if (result.error) throw result.error;
    return {
      exitCode: result.status,
      output: `${result.stderr || ""}\n${result.stdout || ""}`.slice(0, 4096),
    };
  } finally {
    fs.rmSync(workspace, { recursive: true, force: true });
  }
}
