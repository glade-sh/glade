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
  ["/lightning/runtime/lightning/radioGroup.mjs", path.join(runtimeSourceRoot, "lightning/radioGroup.mjs")],
  ["/lightning/runtime/lightning/select.mjs", path.join(runtimeSourceRoot, "lightning/select.mjs")],
  ["/lightning/runtime/lightning/base.mjs", path.join(runtimeSourceRoot, "lightning/base.mjs")],
  ["/lightning/runtime/shell/diagnostics.mjs", path.join(runtimeSourceRoot, "shell/diagnostics.mjs")],
  ["/lightning/runtime/shell/flow-service.mjs", path.join(runtimeSourceRoot, "shell/flow-service.mjs")],
]);
const bundleRel = "force-app/main/default/lwc/validationMulticomponent";
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
  fs.writeFileSync(path.join(bundleDir, "validationMulticomponent.js-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <apiVersion>67.0</apiVersion>
  <isExposed>false</isExposed>
</LightningComponentBundle>
`);
  fs.writeFileSync(path.join(bundleDir, "validationMulticomponent.html"), `<template>
  <lightning-radio-group label="Radio choice" options={radioOptions} required value={radioValue}></lightning-radio-group>
  <lightning-select label="Select choice" options={selectOptions} required value={selectValue}></lightning-select>
  <button type="button" onclick={runFamily} disabled={running}>Run family</button>
  <output data-testid="phase" aria-live="polite">{phase}</output>
  <table data-testid="validation-state-table">
    <thead><tr><th>Phase</th><th>Radio value</th><th>Radio required</th><th>Radio check</th><th>Radio report</th><th>Radio native valid</th><th>Radio visible invalid</th><th>Radio visible message</th><th>Select value</th><th>Select required</th><th>Select check</th><th>Select report</th><th>Select native valid</th><th>Select visible invalid</th><th>Select visible message</th></tr></thead>
    <tbody><template for:each={rows} for:item="row">
      <tr key={row.phase} data-phase={row.phase}>
        <td>{row.phase}</td><td>{row.radioValueDisplay}</td><td>{row.radioRequired}</td><td>{row.radioCheck}</td><td>{row.radioReport}</td><td>{row.radioNativeValid}</td><td>{row.radioVisibleInvalid}</td><td>{row.radioVisibleMessage}</td>
        <td>{row.selectValueDisplay}</td><td>{row.selectRequired}</td><td>{row.selectCheck}</td><td>{row.selectReport}</td><td>{row.selectNativeValid}</td><td>{row.selectVisibleInvalid}</td><td>{row.selectVisibleMessage}</td>
      </tr>
    </template></tbody>
  </table>
</template>
`);
  fs.writeFileSync(path.join(bundleDir, "validationMulticomponent.js"), `import { LightningElement } from "lwc";

const radioCustomMessage = "Radio custom error.";
const selectCustomMessage = "Select custom error.";
function afterRender() { return new Promise((resolve) => requestAnimationFrame(resolve)); }

export default class ValidationMulticomponent extends LightningElement {
  phase = "ready";
  running = false;
  radioValue = "";
  selectValue = "";
  rows = [];
  radioOptions = [{ label: "North", value: "north" }, { label: "South", value: "south" }];
  selectOptions = [{ label: "", value: "" }, { label: "North", value: "north" }, { label: "South", value: "south" }];

  radioField() { return this.template.querySelector("lightning-radio-group"); }
  selectField() { return this.template.querySelector("lightning-select"); }

  async recordPhase(phase, radio, select, radioCheck, radioReport, selectCheck, selectReport) {
    this.phase = phase;
    await new Promise((resolve, reject) => {
      window.__gladeLwcValidationAcknowledge = (visible) => {
        if (!visible?.radio || !visible?.select) return reject(new Error("visible component states missing"));
        this.rows = [...this.rows, {
          phase,
          radioValueDisplay: radio.value === "" ? "(empty)" : String(radio.value ?? ""),
          radioRequired: String(Boolean(radio.required)),
          radioCheck: String(Boolean(radioCheck)),
          radioReport: String(Boolean(radioReport)),
          radioNativeValid: String(Boolean(visible.radio.valid)),
          radioVisibleInvalid: String(Boolean(visible.radio.invalid)),
          radioVisibleMessage: String(visible.radio.message || "(none)"),
          selectValueDisplay: select.value === "" ? "(empty)" : String(select.value ?? ""),
          selectRequired: String(Boolean(select.required)),
          selectCheck: String(Boolean(selectCheck)),
          selectReport: String(Boolean(selectReport)),
          selectNativeValid: String(Boolean(visible.select.valid)),
          selectVisibleInvalid: String(Boolean(visible.select.invalid)),
          selectVisibleMessage: String(visible.select.message || "(none)"),
        }];
        window.__gladeLwcValidationAcknowledge = undefined;
        resolve();
      };
    });
  }

  async runFamily() {
    if (this.running) return;
    this.running = true;
    const radio = this.radioField(), select = this.selectField();
    await this.recordPhase("required-empty", radio, select, radio.checkValidity(), radio.reportValidity(), select.checkValidity(), select.reportValidity());
    this.radioValue = this.selectValue = "north";
    radio.value = select.value = "north";
    await afterRender();
    await this.recordPhase("valid-selection", radio, select, radio.checkValidity(), radio.reportValidity(), select.checkValidity(), select.reportValidity());
    radio.setCustomValidity(radioCustomMessage);
    select.setCustomValidity(selectCustomMessage);
    await this.recordPhase("custom-error", radio, select, radio.checkValidity(), radio.reportValidity(), select.checkValidity(), select.reportValidity());
    radio.setCustomValidity("");
    select.setCustomValidity("");
    await this.recordPhase("cleared-error", radio, select, radio.checkValidity(), radio.reportValidity(), select.checkValidity(), select.reportValidity());
    this.phase = "complete";
    this.running = false;
  }
}
`);
}

function compileConsumer(projectRoot, outDir) {
  const metadata = bundleRel + "/validationMulticomponent.js-meta.xml";
  const config = {
    projectRoot,
    outDir,
    namespace: "c",
    lwcFiles: [bundleRel + "/validationMulticomponent.js"],
    lwcHtmlFiles: [bundleRel + "/validationMulticomponent.html"],
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
  if (!compiled.modules?.["c:validationMulticomponent"]) {
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
    "lightning/radioGroup": origin + "/lightning/runtime/lightning/radioGroup.mjs",
    "lightning/select": origin + "/lightning/runtime/lightning/select.mjs",
    "c/validationMulticomponent": origin + "/lightning/modules/c/validationMulticomponent/validationMulticomponent.js",
  };
  return [
    "<!doctype html>",
    '<html><head><meta charset="utf-8"><title>Radio and select validation family</title>',
    '<script>window.process = { env: { NODE_ENV: "production" } };</script>',
    '<script type="importmap">' + JSON.stringify({ imports }) + "</script>",
    "</head><body>",
    '<pre id="framework-profile">' + JSON.stringify(framework) + "</pre>",
    '<main id="host"></main>',
    '<script type="module">',
    'import "@lwc/synthetic-shadow";',
    'import { createElement } from "lwc";',
    'import ValidationMulticomponent from "c/validationMulticomponent";',
    'const consumer = createElement("c-validation-multicomponent", { is: ValidationMulticomponent });',
    'document.getElementById("host").appendChild(consumer);',
    "window.__validationMulticomponentMounted = true;",
    "const consumerRoot = consumer.shadowRoot;",
    'if (!consumerRoot) throw new Error("compiled consumer shadow root is unavailable for visible-state observation");',
    'let lastObservedPhase = "";',
    "const observer = new MutationObserver(() => {",
    '  const phase = consumerRoot.querySelector("[data-testid=\\\"phase\\\"]")?.textContent.trim();',
    '  if (!["required-empty", "valid-selection", "custom-error", "cleared-error"].includes(phase) || phase === lastObservedPhase) return;',
    "  lastObservedPhase = phase;",
    "  requestAnimationFrame(() => {",
    '    const radioHost = consumerRoot.querySelector("lightning-radio-group");',
    '    const selectHost = consumerRoot.querySelector("lightning-select");',
    "    const nativeState = (host, selector) => {",
    "      const controls = [...(host?.shadowRoot?.querySelectorAll(selector) || [])];",
    '      if (!controls.length) throw new Error("component native validation control is unavailable: " + selector);',
    "      return {",
    "        valid: controls.every((control) => control.validity.valid),",
    '        invalid: controls.some((control) => control.matches(":invalid")),',
    '        message: controls.map((control) => control.validationMessage).find(Boolean) || "",',
    "      };",
    "    };",
    "    const acknowledge = window.__gladeLwcValidationAcknowledge;",
    '    if (typeof acknowledge !== "function") throw new Error("consumer is not waiting for visible-state observation");',
    '    acknowledge({ radio: nativeState(radioHost, \'input[type="radio"]\'), select: nativeState(selectHost, "select") });',
    '    if (phase === "cleared-error") observer.disconnect();',
    "  });",
    "});",
    'observer.observe(consumerRoot, { childList: true, characterData: true, subtree: true });',
    "</script></body></html>",
  ].join("\n");
}

export async function startValidationMulticomponentServer({ port = 0 } = {}) {
  const workspace = fs.mkdtempSync(path.join(os.tmpdir(), "glade-lwc-validation-multicomponent-"));
  const projectRoot = path.join(workspace, "project");
  const compiledRoot = path.join(workspace, "compiled");
  let server;
  try {
    const framework = {
      declaredComponentApiVersion: apiVersion,
      compilerVersion: packageVersion("@lwc/compiler"),
      engineDomVersion: packageVersion("@lwc/engine-dom"),
      syntheticShadowVersion: packageVersion("@lwc/synthetic-shadow"),
      componentReferenceBuildIdentity: "v262.0.0",
      salesforceApiVersion: null,
      versionAxis: "candidate framework packages, component-reference build, declared bundle API, and Salesforce runtime evidence are distinct",
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
      if (url.pathname === "/validation-multicomponent.html") {
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
      url: "http://127.0.0.1:" + server.address().port + "/validation-multicomponent.html",
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
  if (args.some((arg) => arg !== "--serve-only" && arg !== "--port=8942")) {
    throw new Error("supported options are --serve-only and --port=8942");
  }
  const server = await startValidationMulticomponentServer({ port: 8942 });
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
