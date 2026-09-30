import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { repoRoot } from "./helpers.mjs";

const toolchainRoot = process.env.GLADE_LWC_TOOLCHAIN_DIR || path.join(repoRoot, "third_party/lwc");
const toolchainNodeModules = path.join(toolchainRoot, "node_modules");
const lightningRoot = path.join(repoRoot, "lwcruntime/src/lightning");
const shellRoot = path.join(repoRoot, "lwcruntime/src/shell");
const componentFiles = ["base", "input", "combobox"];

function packageVersion(packageName) {
  const packagePath = path.join(toolchainNodeModules, packageName, "package.json");
  if (!fs.existsSync(packagePath)) {
    throw new Error(`required retained framework package is missing: ${packagePath}`);
  }
  return JSON.parse(fs.readFileSync(packagePath, "utf8")).version;
}

function pageHtml(origin, framework) {
  const imports = {
    lwc: `${origin}/lightning/vendor/lwc.js`,
    "@lwc/synthetic-shadow": `${origin}/lightning/vendor/synthetic-shadow.js`,
    "@glade/shell/diagnostics": `${origin}/lightning/runtime/shell/diagnostics.mjs`,
    "lightning/input": `${origin}/lightning/runtime/lightning/input.mjs`,
    "lightning/combobox": `${origin}/lightning/runtime/lightning/combobox.mjs`,
  };

  return `<!doctype html>
<html><head>
  <meta charset="utf-8">
  <title>Local input and combobox event observation</title>
  <script>window.process = { env: { NODE_ENV: "production" } };</script>
  <script type="importmap">${JSON.stringify({ imports })}</script>
</head><body>
  <main>
    <h1>Local input and combobox event observation</h1>
    <p>Change the text to a non-empty value and clear it; press Enter and leave the field. Open the combobox and select an option.</p>
    <div id="outer"><div id="host"></div></div>
    <h2>Observed public events and rendered values</h2>
    <pre id="event-diagnostics" aria-live="polite">Starting local component runtime…</pre>
  </main>
  <script type="module">
    import "@lwc/synthetic-shadow";
    import { createElement } from "lwc";
    import Input from "lightning/input";
    import Combobox from "lightning/combobox";

    const host = document.getElementById("host");
    const outer = document.getElementById("outer");
    const output = document.getElementById("event-diagnostics");
    const events = [];
    const outerEvents = [];
    let diagnosticsScheduled = false;

    function eventRecord(component, event) {
      return {
        component,
        target: event.target?.localName ?? null,
        type: event.type,
        detail: event.detail === undefined ? null : event.detail,
        flags: {
          bubbles: event.bubbles,
          composed: event.composed,
          cancelable: event.cancelable,
        },
        defaultPrevented: event.defaultPrevented,
      };
    }

    function renderedState() {
      return {
        input: {
          value: input.value ?? null,
          renderedValue: input.shadowRoot?.querySelector("input")?.value ?? null,
        },
        combobox: {
          value: combobox.value ?? null,
          renderedValue: combobox.shadowRoot?.querySelector("select")?.value ?? null,
        },
      };
    }

    function renderDiagnostics() {
      output.textContent = JSON.stringify({ framework: ${JSON.stringify(framework)}, events, outerEvents, renderedState: renderedState() }, null, 2);
    }

    function scheduleDiagnostics() {
      if (diagnosticsScheduled) {
        return;
      }
      diagnosticsScheduled = true;
      requestAnimationFrame(() => {
        diagnosticsScheduled = false;
        renderDiagnostics();
      });
    }

    function observe(component, componentName, eventNames) {
      for (const type of eventNames) {
        component.addEventListener(type, (event) => {
          event.preventDefault();
          events.push(eventRecord(componentName, event));
          scheduleDiagnostics();
        });
      }
    }

    const input = createElement("lightning-input", { is: Input });
    input.label = "Text value";
    input.type = "text";
    input.value = "";
    observe(input, "lightning-input", ["change", "commit"]);
    host.appendChild(input);

    const combobox = createElement("lightning-combobox", { is: Combobox });
    combobox.label = "Choice";
    combobox.value = "";
    combobox.options = [
      { label: "Choose an option", value: "" },
      { label: "North", value: "north" },
      { label: "South", value: "south" },
    ];
    observe(combobox, "lightning-combobox", ["change", "open"]);
    host.appendChild(combobox);

    for (const type of ["change", "commit", "open"]) {
      outer.addEventListener(type, (event) => {
        event.preventDefault();
        outerEvents.push(eventRecord("outer", event));
        scheduleDiagnostics();
      });
    }
    renderDiagnostics();
  </script>
</body></html>`;
}

export async function startInputComboboxServer({ port = 0 } = {}) {
  const framework = {
    engineDomVersion: packageVersion("@lwc/engine-dom"),
    syntheticShadowVersion: packageVersion("@lwc/synthetic-shadow"),
    engineAxis: "local framework/toolchain version; not Salesforce API version",
  };
  const files = new Map([
    ["/lightning/vendor/lwc.js", path.join(toolchainNodeModules, "@lwc/engine-dom/dist/index.js")],
    ["/lightning/vendor/synthetic-shadow.js", path.join(toolchainNodeModules, "@lwc/synthetic-shadow/dist/index.js")],
    ["/lightning/runtime/shell/diagnostics.mjs", path.join(shellRoot, "diagnostics.mjs")],
    ["/lightning/runtime/shell/flow-service.mjs", path.join(shellRoot, "flow-service.mjs")],
    ...componentFiles.map((name) => [
      `/lightning/runtime/lightning/${name}.mjs`,
      path.join(lightningRoot, `${name}.mjs`),
    ]),
  ]);

  for (const filePath of files.values()) {
    if (!fs.existsSync(filePath)) {
      throw new Error(`required local input/combobox observation file is missing: ${filePath}`);
    }
  }

  const server = http.createServer((req, res) => {
    const url = new URL(req.url, "http://127.0.0.1");
    if (url.pathname === "/input-combobox.html") {
      const origin = `http://127.0.0.1:${server.address().port}`;
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(pageHtml(origin, framework));
      return;
    }
    const filePath = files.get(url.pathname);
    if (!filePath) {
      res.writeHead(404);
      res.end("not found");
      return;
    }
    res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
    res.end(fs.readFileSync(filePath));
  });

  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(port, "127.0.0.1", resolve);
  });
  let closePromise;
  return {
    url: `http://127.0.0.1:${server.address().port}/input-combobox.html`,
    framework,
    close() {
      if (!closePromise) {
        closePromise = new Promise((resolve, reject) => {
          server.close((error) => error ? reject(error) : resolve());
        });
      }
      return closePromise;
    },
  };
}

async function serveOnly() {
  const portArgs = process.argv.slice(2).filter((arg) => arg.startsWith("--port="));
  if (portArgs.length > 1) {
    throw new Error("pass at most one --port=PORT option");
  }
  const port = portArgs.length ? Number(portArgs[0].slice("--port=".length)) : 8942;
  if (!Number.isInteger(port) || port < 0 || port > 65535) {
    throw new Error("--port must be an integer from 0 through 65535");
  }
  const server = await startInputComboboxServer({ port });
  process.stdout.write(`${JSON.stringify({ url: server.url, framework: server.framework, mode: "serve-only" })}\n`);
  let stopping = false;
  const stop = () => {
    if (stopping) {
      return;
    }
    stopping = true;
    server.close().then(() => process.exit(0), (error) => {
      process.stderr.write(`${error.message}\n`);
      process.exit(1);
    });
  };
  process.on("SIGTERM", stop);
  process.on("SIGINT", stop);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url) && process.argv.includes("--serve-only")) {
  serveOnly().catch((error) => {
    process.stderr.write(`${error.stack || error.message}\n`);
    process.exitCode = 1;
  });
}
