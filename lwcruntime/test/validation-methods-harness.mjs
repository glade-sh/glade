import fs from "node:fs";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { repoRoot } from "./helpers.mjs";

const toolchainRoot = process.env.GLADE_LWC_TOOLCHAIN_DIR || path.join(repoRoot, "third_party/lwc");
const nodeModulesRoot = path.join(toolchainRoot, "node_modules");
const runtimeSourceRoot = path.join(repoRoot, "lwcruntime/src");
const runtimeFiles = new Map([
  ["/lightning/runtime/lightning/textarea.mjs", path.join(runtimeSourceRoot, "lightning/textarea.mjs")],
  ["/lightning/runtime/lightning/base.mjs", path.join(runtimeSourceRoot, "lightning/base.mjs")],
  ["/lightning/runtime/shell/diagnostics.mjs", path.join(runtimeSourceRoot, "shell/diagnostics.mjs")],
  ["/lightning/runtime/shell/flow-service.mjs", path.join(runtimeSourceRoot, "shell/flow-service.mjs")],
]);
const bundleRel = "force-app/main/default/lwc/validationMethods";
const apiVersion = 67;

function packageVersion(packageName) {
  const packagePath = path.join(nodeModulesRoot, packageName, "package.json");
  if (!fs.existsSync(packagePath)) {
    throw new Error("required retained LWC package is missing: " + packagePath);
  }
  return JSON.parse(fs.readFileSync(packagePath, "utf8")).version;
}

function writeConsumer(projectRoot) {
  const bundleDir = path.join(projectRoot, bundleRel);
  fs.mkdirSync(bundleDir, { recursive: true });
  fs.writeFileSync(path.join(bundleDir, "validationMethods.js-meta.xml"), [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">',
    "  <apiVersion>67.0</apiVersion>",
    "  <isExposed>false</isExposed>",
    "</LightningComponentBundle>",
    "",
  ].join("\n"));
  fs.writeFileSync(path.join(bundleDir, "validationMethods.html"), [
    "<template>",
    '  <lightning-textarea label="Validation field" required value=""></lightning-textarea>',
    '  <button type="button" onclick={checkRequired}>Check required</button>',
    '  <button type="button" onclick={setNonemptyValue}>Set nonempty value</button>',
    '  <button type="button" onclick={checkValid}>Check valid value</button>',
    '  <button type="button" onclick={setCustomError}>Set custom error</button>',
    '  <button type="button" onclick={clearCustomError}>Clear custom error</button>',
    '  <output data-testid="validation-state" aria-live="polite">{state}</output>',
    "</template>",
    "",
  ].join("\n"));
  fs.writeFileSync(path.join(bundleDir, "validationMethods.js"), [
    'import { LightningElement } from "lwc";',
    "",
    "export default class ValidationMethods extends LightningElement {",
    '  state = JSON.stringify({ step: "ready" });',
    "",
    "  field() {",
    '    return this.template.querySelector("lightning-textarea");',
    "  }",
    "",
    "  record(step, values) {",
    "    this.state = JSON.stringify(Object.assign({ step }, values));",
    "  }",
    "",
    "  checkRequired() {",
    "    const field = this.field();",
    "    const check = field.checkValidity();",
    "    const report = field.reportValidity();",
    '    this.record("required-empty", { required: field.required, value: field.value, check, report });',
    "  }",
    "",
    "  setNonemptyValue() {",
    '    this.field().value = "ready";',
    '    this.record("property-set", { required: this.field().required, value: this.field().value });',
    "  }",
    "",
    "  checkValid() {",
    "    const field = this.field();",
    "    const check = field.checkValidity();",
    "    const report = field.reportValidity();",
    '    this.record("value-valid", { required: field.required, value: field.value, check, report });',
    "  }",
    "",
    "  setCustomError() {",
    '    this.field().setCustomValidity("Review this value.");',
    "    const field = this.field();",
    "    const check = field.checkValidity();",
    "    const report = field.reportValidity();",
    '    this.record("custom-error", { value: field.value, check, report });',
    "  }",
    "",
    "  clearCustomError() {",
    '    this.field().setCustomValidity("");',
    "    const field = this.field();",
    "    const check = field.checkValidity();",
    "    const report = field.reportValidity();",
    '    this.record("custom-error-cleared", { value: field.value, check, report });',
    "  }",
    "}",
    "",
  ].join("\n"));
}

function compileConsumer(projectRoot, outDir) {
  const metadata = bundleRel + "/validationMethods.js-meta.xml";
  const config = {
    projectRoot,
    outDir,
    namespace: "c",
    lwcFiles: [bundleRel + "/validationMethods.js"],
    lwcHtmlFiles: [bundleRel + "/validationMethods.html"],
    lwcMetaFiles: [metadata],
    lwcApiVersions: { [bundleRel]: apiVersion },
  };
  const result = spawnSync(
    process.execPath,
    [path.join(repoRoot, "third_party/lwc/compile.mjs")],
    {
      cwd: path.join(repoRoot, "third_party/lwc"),
      env: process.env,
      input: JSON.stringify(config),
      encoding: "utf8",
    },
  );
  if (result.status !== 0) {
    throw new Error(result.stderr || result.stdout || "LWC validation consumer compilation failed");
  }
  const compiled = JSON.parse(result.stdout);
  if (!compiled.modules?.["c:validationMethods"]) {
    throw new Error("compiled validation consumer is missing from compiler manifest");
  }
}

function resolveUnder(root, relativePath) {
  const target = path.resolve(root, relativePath);
  return target === root || target.startsWith(root + path.sep) ? target : null;
}

function contentType(filePath) {
  return filePath.endsWith(".html") ? "text/html; charset=utf-8" : "application/javascript; charset=utf-8";
}

function pageHTML(origin, framework) {
  const imports = {
    lwc: origin + "/lightning/vendor/lwc.js",
    "@lwc/synthetic-shadow": origin + "/lightning/vendor/synthetic-shadow.js",
    "@glade/shell/diagnostics": origin + "/lightning/runtime/shell/diagnostics.mjs",
    "lightning/textarea": origin + "/lightning/runtime/lightning/textarea.mjs",
    "c/validationMethods": origin + "/lightning/modules/c/validationMethods/validationMethods.js",
  };
  return [
    "<!doctype html>",
    '<html><head><meta charset="utf-8">',
    "<script>window.process = { env: { NODE_ENV: \"production\" } };</script>",
    '<script type="importmap">' + JSON.stringify({ imports }) + "</script>",
    "</head><body>",
    '<pre id="framework-profile">' + JSON.stringify(framework) + "</pre>",
    '<main id="host"></main>',
    "<script type=\"module\">",
    'import "@lwc/synthetic-shadow";',
    'import { createElement } from "lwc";',
    'import ValidationMethods from "c/validationMethods";',
    'document.getElementById("host").appendChild(createElement("c-validation-methods", { is: ValidationMethods }));',
    "window.__validationMethodsMounted = true;",
    "</script></body></html>",
  ].join("\n");
}

export async function startValidationMethodsServer({ port = 0 } = {}) {
  const workspace = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-validation-methods-"));
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
      versionAxis: "candidate framework packages and declared bundle API version are distinct from Salesforce runtime evidence",
    };
    writeConsumer(projectRoot);
    compileConsumer(projectRoot, compiledRoot);
    server = http.createServer((req, res) => {
      if (req.method !== "GET" && req.method !== "HEAD") {
        res.writeHead(405);
        res.end();
        return;
      }
      const url = new URL(req.url, "http://127.0.0.1");
      if (url.pathname === "/favicon.ico") {
        res.writeHead(204);
        res.end();
        return;
      }
      if (url.pathname === "/validation-methods.html") {
        const body = pageHTML("http://127.0.0.1:" + server.address().port, framework);
        res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
        res.end(req.method === "HEAD" ? undefined : body);
        return;
      }
      let filePath = null;
      let relativePath = "";
      try {
        relativePath = decodeURIComponent(url.pathname);
      } catch (_error) {
        res.writeHead(400);
        res.end("invalid path");
        return;
      }
      if (relativePath === "/lightning/vendor/lwc.js") {
        filePath = path.join(nodeModulesRoot, "@lwc/engine-dom/dist/index.js");
      } else if (relativePath === "/lightning/vendor/synthetic-shadow.js") {
        filePath = path.join(nodeModulesRoot, "@lwc/synthetic-shadow/dist/index.js");
      } else if (runtimeFiles.has(relativePath)) {
        filePath = runtimeFiles.get(relativePath);
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
    let closePromise;
    return {
      url: "http://127.0.0.1:" + server.address().port + "/validation-methods.html",
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
    if (server?.listening) {
      await new Promise((resolve) => server.close(resolve));
    }
    fs.rmSync(workspace, { recursive: true, force: true });
    throw error;
  }
}

async function serveOnly() {
  const args = process.argv.slice(2);
  if (args.filter((arg) => arg === "--serve-only").length !== 1) {
    throw new Error("pass --serve-only exactly once");
  }
  const portArgs = args.filter((arg) => arg.startsWith("--port="));
  if (portArgs.length > 1) {
    throw new Error("pass at most one --port=8942 option");
  }
  if (args.some((arg) => arg !== "--serve-only" && !arg.startsWith("--port="))) {
    throw new Error("supported options are --serve-only and --port=8942");
  }
  if (portArgs.length && Number(portArgs[0].slice("--port=".length)) !== 8942) {
    throw new Error("manual serve-only mode uses the fixed loopback port 8942");
  }
  const server = await startValidationMethodsServer({ port: 8942 });
  process.stdout.write(JSON.stringify({
    url: server.url,
    bindAddress: "127.0.0.1:8942",
    framework: server.framework,
    mode: "serve-only",
  }) + "\n");
  let stopping = false;
  const stop = () => {
    if (stopping) return;
    stopping = true;
    server.close().then(
      () => process.exit(0),
      (error) => {
        process.stderr.write(error.message + "\n");
        process.exit(1);
      },
    );
  };
  process.on("SIGTERM", stop);
  process.on("SIGINT", stop);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  serveOnly().catch((error) => {
    process.stderr.write((error.stack || error.message) + "\n");
    process.exitCode = 1;
  });
}
