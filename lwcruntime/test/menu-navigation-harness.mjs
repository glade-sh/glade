import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { repoRoot } from "./helpers.mjs";

const toolchainRoot = process.env.GLADE_LWC_TOOLCHAIN_DIR || path.join(repoRoot, "third_party/lwc");
const toolchainNodeModules = path.join(toolchainRoot, "node_modules");
const lightningRoot = path.join(repoRoot, "lwcruntime/src/lightning");
const shellRoot = path.join(repoRoot, "lwcruntime/src/shell");
const componentNames = ["base", "buttonMenu", "menuItem", "verticalNavigation", "verticalNavigationItem"];

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
    "lightning/buttonMenu": `${origin}/lightning/runtime/lightning/buttonMenu.mjs`,
    "lightning/menuItem": `${origin}/lightning/runtime/lightning/menuItem.mjs`,
    "lightning/verticalNavigation": `${origin}/lightning/runtime/lightning/verticalNavigation.mjs`,
    "lightning/verticalNavigationItem": `${origin}/lightning/runtime/lightning/verticalNavigationItem.mjs`,
  };

  return `<!doctype html>
<html><head>
  <meta charset="utf-8">
  <title>Local menu/navigation event observation</title>
  <script>window.process = { env: { NODE_ENV: "production" } };</script>
  <script type="importmap">${JSON.stringify({ imports })}</script>
</head><body>
  <main>
    <h1>Local LWC menu/navigation event observation</h1>
    <p>Click each menu trigger, then its item. Click both navigation items. The panel reports emitted events and rendered DOM state only.</p>
    <div id="outer"><div id="host"></div></div>
    <h2>Observed framework events and rendered item state</h2>
    <pre id="event-diagnostics" aria-live="polite">Starting local component runtime…</pre>
  </main>
  <script type="module">
    import "@lwc/synthetic-shadow";
    import { createElement } from "lwc";
    import ButtonMenu from "lightning/buttonMenu";
    import MenuItem from "lightning/menuItem";
    import VerticalNavigation from "lightning/verticalNavigation";
    import VerticalNavigationItem from "lightning/verticalNavigationItem";

    const host = document.getElementById("host");
    const outer = document.getElementById("outer");
    const output = document.getElementById("event-diagnostics");
    const events = [];
    const outerEvents = [];
    const postInteractionObservations = [];
    const pendingDomObservations = [];
    const menuInstances = [];
    const navigationItems = [];
    let lastInteractionEventCount = 0;
    let diagnosticsScheduled = false;

    function eventRecord(component, event, menuLabel = null) {
      return {
        component,
        ...(menuLabel === null ? {} : { menuLabel }),
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

    function renderedElement(element, selector) {
      const node = element.shadowRoot?.querySelector(selector);
      if (!node) {
        return null;
      }
      return {
        text: (node.textContent || "").trim(),
        className: node.getAttribute("class"),
        ariaCurrent: node.getAttribute("aria-current"),
        ariaSelected: node.getAttribute("aria-selected"),
        ariaExpanded: node.getAttribute("aria-expanded"),
      };
    }

    function renderedDomState() {
      const menus = menuInstances.map(({ menu, items }) => ({
        label: menu.label ?? null,
        openProperty: menu.open ?? null,
        trigger: renderedElement(menu, "button"),
        items: items.map((item) => ({
          label: item.label ?? null,
          name: item.name ?? null,
          value: item.value ?? null,
          selectedProperty: item.selected ?? null,
          rendered: renderedElement(item, '[role="menuitem"]'),
        })),
      }));
      const renderedNavigationItems = navigationItems.map((item) => ({
        label: item.label ?? null,
        name: item.name ?? null,
        selectedProperty: item.selected ?? null,
        rendered: renderedElement(item, "a"),
      }));
      return { menus, navigationItems: renderedNavigationItems };
    }

    function renderDiagnostics() {
      output.textContent = JSON.stringify({
        framework: ${JSON.stringify(framework)},
        events,
        outerEvents,
        postInteractionObservations,
        renderedDomState: renderedDomState(),
      }, null, 2);
    }

    function scheduleDiagnostics(observation = null) {
      if (observation) {
        pendingDomObservations.push(observation);
      }
      if (diagnosticsScheduled) {
        return;
      }
      diagnosticsScheduled = true;
      requestAnimationFrame(() => {
        diagnosticsScheduled = false;
        const state = renderedDomState();
        for (const pending of pendingDomObservations.splice(0)) {
          pending.renderedDomState = state;
        }
        renderDiagnostics();
      });
    }

    function makeMenu(label, cancelSelect = false, itemSpec = null) {
      const menu = createElement("lightning-button-menu", { is: ButtonMenu });
      menu.label = label;
      for (const type of ["open", "close", "select"]) {
        menu.addEventListener(type, (event) => {
          if (cancelSelect && type === "select") {
            event.preventDefault();
          }
          events.push(eventRecord("lightning-button-menu", event, label));
          scheduleDiagnostics();
        });
      }
      host.appendChild(menu);
      const items = [];
      if (itemSpec) {
        const item = createElement("lightning-menu-item", { is: MenuItem });
        item.label = itemSpec.label;
        item.value = itemSpec.value;
        menu.appendChild(item);
        items.push(item);
      }
      menuInstances.push({ menu, items });
      return menu;
    }

    makeMenu("Toggle actions");
    makeMenu("Select action", false, { label: "Approve", value: "approve" });
    makeMenu("Cancel action", true, { label: "Do not apply", value: "" });

    const navigation = createElement("lightning-vertical-navigation", { is: VerticalNavigation });
    navigation.addEventListener("beforeselect", (event) => {
      if (event.detail?.name === "blocked") {
        event.preventDefault();
      }
      events.push(eventRecord("lightning-vertical-navigation", event));
      scheduleDiagnostics();
    });
    navigation.addEventListener("select", (event) => {
      event.preventDefault();
      events.push(eventRecord("lightning-vertical-navigation", event));
      scheduleDiagnostics();
    });
    host.appendChild(navigation);
    for (const [name, label] of [["", "Allowed"], ["blocked", "Blocked"]]) {
      const item = createElement("lightning-vertical-navigation-item", { is: VerticalNavigationItem });
      item.name = name;
      item.label = label;
      navigation.appendChild(item);
      navigationItems.push(item);
    }

    for (const type of ["active", "open", "close", "select", "beforeselect"]) {
      outer.addEventListener(type, (event) => {
        outerEvents.push(eventRecord("outer", event));
        scheduleDiagnostics();
      });
    }
    outer.addEventListener("click", (event) => {
      const observation = {
        interactionNumber: postInteractionObservations.length + 1,
        target: event.target?.localName ?? null,
        text: (event.target?.textContent || "").trim().replace(/\\s+/g, " ").slice(0, 80),
        emittedEventsSincePriorClick: events.slice(lastInteractionEventCount).map((record) => ({ ...record })),
        renderedDomState: null,
        interpretation: "Observed sequence only; retained event records do not specify default-action or selected-state outcomes.",
      };
      lastInteractionEventCount = events.length;
      postInteractionObservations.push(observation);
      scheduleDiagnostics(observation);
    });
    scheduleDiagnostics();
  </script>
</body></html>`;
}

export async function startMenuNavigationServer({ port = 0 } = {}) {
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
    ...componentNames.map((name) => [
      `/lightning/runtime/lightning/${name}.mjs`,
      path.join(lightningRoot, `${name}.mjs`),
    ]),
  ]);

  for (const filePath of files.values()) {
    if (!fs.existsSync(filePath)) {
      throw new Error(`required local LWC observation file is missing: ${filePath}`);
    }
  }

  const server = http.createServer((req, res) => {
    const url = new URL(req.url, "http://127.0.0.1");
    if (url.pathname === "/menu-navigation.html") {
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
    url: `http://127.0.0.1:${server.address().port}/menu-navigation.html`,
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
  const server = await startMenuNavigationServer({ port });
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
